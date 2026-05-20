package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"github.com/twitchtv/twirp"
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/rpc"
	"github.com/frankbardon/prism/spec"
)

// serveCommand returns the `prism serve` subcommand. P14 promoted the
// P13 stop-gap into a full Twirp service mounted at
// /twirp/prism.v1.Prism/<RPC>; the P13 /prism/scene endpoint is
// retained as a thin compatibility wrapper around the Twirp Scene
// handler (D084) because the JS web component hardcodes that path.
func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Run the Prism Twirp service + the /prism/scene compatibility endpoint",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Value: "",
				Usage: "Bind address (e.g. :8080 or 127.0.0.1:8080). Overrides --host/--port when set.",
			},
			&cli.IntFlag{
				Name:  "port",
				Value: 0,
				Usage: "TCP port (0 = random, printed to stderr). Composed with --host when --addr is unset.",
			},
			&cli.StringFlag{
				Name:  "host",
				Value: "127.0.0.1",
				Usage: "Bind host (127.0.0.1 = loopback only). Composed with --port when --addr is unset.",
			},
			&cli.StringFlag{
				Name:  "cors",
				Value: "*",
				Usage: "CORS Access-Control-Allow-Origin value. Empty string disables CORS headers.",
			},
			datasetsConfigFlag(),
		},
		Action: runServe,
	}
}

// runServe binds the configured address, mounts both surfaces, and
// blocks until the context is cancelled or the listener errors.
func runServe(ctx context.Context, cmd *cli.Command) error {
	addr := resolveBindAddr(cmd)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return cli.Exit(fmt.Sprintf("listen %s: %v", addr, err), 1)
	}
	actual := lis.Addr().(*net.TCPAddr)
	// "listening on <host>:<port>" — exact prefix preserved so the
	// P13 serve_smoke_test regex (`listening on 127.0.0.1:(\d+)`)
	// keeps matching.
	fmt.Fprintf(cmd.ErrWriter,
		"prism serve: listening on %s (twirp at /twirp/prism.v1.Prism, compat at /prism/scene)\n",
		actual.String())

	registry, err := loadDatasetRegistry(cmd)
	if err != nil {
		return cli.Exit(fmt.Sprintf("load --datasets-config: %v", err), 2)
	}

	impl := &rpc.PrismServer{
		DatasetRegistry: registry,
		Fs:              afero.NewOsFs(),
	}
	twirpHandler := rpc.NewPrismServer(impl, twirp.WithServerInterceptors(rpc.ErrorInterceptor))

	mux := http.NewServeMux()
	mux.Handle(rpc.PrismPathPrefix, twirpHandler)
	mux.HandleFunc("/prism/scene", sceneCompatHandler(ctx, impl))

	corsOrigin := cmd.String("cors")
	srv := &http.Server{Handler: corsMiddleware(corsOrigin, mux)}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(lis) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return cli.Exit(fmt.Sprintf("serve: %v", err), 1)
	}
}

// resolveBindAddr applies the flag-precedence rule. --addr wins when
// set; otherwise host:port composes the bind string. Warns on stderr
// when both are present (covers the deprecated --port/--host pair
// being passed alongside the new --addr).
func resolveBindAddr(cmd *cli.Command) string {
	addr := cmd.String("addr")
	if addr == "" {
		return cmd.String("host") + ":" + strconv.Itoa(int(cmd.Int("port")))
	}
	if cmd.Int("port") != 0 || (cmd.String("host") != "" && cmd.String("host") != "127.0.0.1") {
		fmt.Fprintf(cmd.ErrWriter,
			"prism serve: --addr=%s overrides --host=%s / --port=%d\n",
			addr, cmd.String("host"), cmd.Int("port"))
	}
	return addr
}

// corsMiddleware applies the configured Access-Control-Allow-Origin
// header to every response and short-circuits OPTIONS preflights with
// 204 No Content. Empty origin = no CORS headers (production-tight
// default for embedders who run behind their own gateway).
func corsMiddleware(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── /prism/scene compatibility wrapper (D084) ──────────────────────────
//
// The P12/P13 JS web component hardcodes /prism/scene. We keep the
// route and the P13 sceneRequest envelope; selection-state synthesis
// stays here because the Twirp Scene RPC does not carry per-call
// selection state. The actual Scene compilation goes through the
// shared PrismServer in-process (so one pipeline owns the work).

// sceneRequest mirrors D081 — the JSON body the JS port POSTs.
type sceneRequest struct {
	Spec      json.RawMessage                `json:"spec"`
	Selection map[string]selectionStateInput `json:"selection,omitempty"`
	Datasets  map[string]string              `json:"datasets,omitempty"`
}

type selectionStateInput struct {
	Points []datumRefInput   `json:"points,omitempty"`
	Range  *intervalRangeInp `json:"range,omitempty"`
}

type datumRefInput struct {
	LayerID string `json:"layer_id"`
	RowID   int64  `json:"row_id"`
}

type intervalRangeInp struct {
	Channel string  `json:"channel"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
}

type errorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// sceneCompatHandler returns the /prism/scene handler. It decodes the
// P13 envelope, synthesises selection-state filters, calls the Twirp
// Scene RPC in-process, and writes the resulting scene_json body.
func sceneCompatHandler(ctx context.Context, impl *rpc.PrismServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONErr(w, http.StatusMethodNotAllowed,
				"PRISM_SERVE_METHOD",
				"only POST is supported on /prism/scene")
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_READ", err.Error())
			return
		}
		defer r.Body.Close()

		var req sceneRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_DECODE", err.Error())
			return
		}
		if len(req.Spec) == 0 {
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_DECODE", "missing required field: spec")
			return
		}
		s, err := spec.DecodeBytes(req.Spec)
		if err != nil {
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_DECODE", "spec decode: "+err.Error())
			return
		}

		// Synthesise filters from the selection state per D081.
		injected, err := synthesiseSelectionFilters(s, req.Selection)
		if err != nil {
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_SELECTION", err.Error())
			return
		}
		if len(injected) > 0 {
			s.Transform = append(injected, s.Transform...)
		}

		// Re-marshal the (possibly mutated) spec and route through
		// the Twirp Scene handler. Per request context derived from
		// the long-lived serve context so handlers see cancellation.
		mutated, mErr := json.Marshal(s)
		if mErr != nil {
			writeJSONErr(w, http.StatusInternalServerError, "PRISM_SERVE_ENCODE", "re-marshal: "+mErr.Error())
			return
		}
		// Composite specs are rejected here (P13 parity); the full
		// Twirp Scene RPC handles them. The JS port only emits flat
		// specs through this path.
		if len(s.Layer)+len(s.Concat)+len(s.HConcat)+len(s.VConcat) > 0 ||
			s.Facet != nil || s.Repeat != nil {
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_COMPOSITE",
				"composite specs not supported on /prism/scene; use /twirp/prism.v1.Prism/Scene")
			return
		}

		resp, sceneErr := impl.Scene(r.Context(), &rpc.SceneRequest{
			Spec: string(mutated),
		})
		if sceneErr != nil {
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_EXECUTE", sceneErr.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		// scene_json is a pre-marshalled SceneDoc body; ship as-is.
		_, _ = w.Write([]byte(resp.SceneJson))
	}
}

// synthesiseSelectionFilters walks the selection state map + the
// spec's selection declarations and emits one FilterTransform per
// active selection (D081 semantics).
func synthesiseSelectionFilters(s *spec.Spec, state map[string]selectionStateInput) ([]spec.Transform, error) {
	if len(state) == 0 || s == nil {
		return nil, nil
	}
	var out []spec.Transform
	for id, st := range state {
		decl, ok := s.Selection[id]
		if !ok {
			continue
		}
		if decl.Point != nil && len(st.Points) > 0 {
			rowIDs := make([]string, 0, len(st.Points))
			for _, p := range st.Points {
				rowIDs = append(rowIDs, strconv.FormatInt(p.RowID, 10))
			}
			if len(decl.Point.Fields) > 0 {
				field := decl.Point.Fields[0]
				values := lookupInlineFieldValues(s, field, st.Points)
				if len(values) > 0 {
					expr := field + " in [" + strings.Join(quotedList(values), ", ") + "]"
					out = append(out, spec.Transform{Filter: &spec.FilterTransform{Filter: expr}})
					continue
				}
			}
			expr := "__row__ in [" + strings.Join(rowIDs, ", ") + "]"
			out = append(out, spec.Transform{Filter: &spec.FilterTransform{Filter: expr}})
		}
		if decl.Interval != nil && st.Range != nil {
			field := fieldForChannel(s, st.Range.Channel)
			if field == "" {
				continue
			}
			min := strconv.FormatFloat(st.Range.Min, 'f', -1, 64)
			max := strconv.FormatFloat(st.Range.Max, 'f', -1, 64)
			expr := field + " >= " + min + " and " + field + " <= " + max
			out = append(out, spec.Transform{Filter: &spec.FilterTransform{Filter: expr}})
		}
	}
	return out, nil
}

// fieldForChannel resolves a Scene channel back to its bound field
// name from the spec encoding.
func fieldForChannel(s *spec.Spec, channel string) string {
	if s == nil || s.Encoding == nil {
		return ""
	}
	switch channel {
	case "x":
		if s.Encoding.X != nil {
			return s.Encoding.X.Field
		}
	case "y":
		if s.Encoding.Y != nil {
			return s.Encoding.Y.Field
		}
	case "x2":
		if s.Encoding.X2 != nil {
			return s.Encoding.X2.Field
		}
	case "y2":
		if s.Encoding.Y2 != nil {
			return s.Encoding.Y2.Field
		}
	}
	return ""
}

// lookupInlineFieldValues returns the values at the named field for
// each DatumRef row id, sourced from spec.Data.Values. Returns nil
// when the spec uses a non-inline data source.
func lookupInlineFieldValues(s *spec.Spec, field string, refs []datumRefInput) []string {
	if s == nil || s.Data == nil || len(s.Data.Values) == 0 {
		return nil
	}
	values := make([]string, 0, len(refs))
	for _, r := range refs {
		if int(r.RowID) < 0 || int(r.RowID) >= len(s.Data.Values) {
			continue
		}
		row := s.Data.Values[int(r.RowID)]
		v, present := row[field]
		if !present {
			continue
		}
		values = append(values, fmt.Sprintf("%v", v))
	}
	return values
}

// quotedList wraps non-numeric entries in single quotes for embedding
// in an expr-lang expression.
func quotedList(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			out[i] = v
		} else {
			out[i] = "'" + strings.ReplaceAll(v, "'", "\\'") + "'"
		}
	}
	return out
}

func writeJSONErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{ErrorCode: code, Message: msg})
}
