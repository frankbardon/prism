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
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// serveCommand returns the `prism serve --port N` subcommand. P13
// stop-gap HTTP endpoint that lets the JS port exercise server-
// reactive selections without waiting for P14's full Twirp service.
// Single route: POST /prism/scene. Contract documented in D081.
func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Run an HTTP endpoint that compiles spec + selection state to Scene IR JSON (P13 stop-gap; full service in P14)",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "port",
				Value: 0,
				Usage: "TCP port (0 = random, printed to stderr)",
			},
			&cli.StringFlag{
				Name:  "host",
				Value: "127.0.0.1",
				Usage: "Bind host (127.0.0.1 = loopback only)",
			},
			datasetsConfigFlag(),
		},
		Action: runServe,
	}
}

// sceneRequest mirrors D081 — the JSON body the JS port POSTs.
// Selection state is keyed by selection ID; values match
// scene.SelectionState (snake_case fields).
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

func runServe(ctx context.Context, cmd *cli.Command) error {
	host := cmd.String("host")
	port := cmd.Int("port")

	addr := host + ":" + strconv.Itoa(int(port))
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return cli.Exit(fmt.Sprintf("listen %s: %v", addr, err), 1)
	}
	actual := lis.Addr().(*net.TCPAddr)
	fmt.Fprintf(cmd.ErrWriter, "prism serve: listening on %s\n", actual.String())

	registry, err := loadDatasetRegistry(cmd)
	if err != nil {
		return cli.Exit(fmt.Sprintf("load --datasets-config: %v", err), 2)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/prism/scene", serveSceneHandler(ctx, registry))

	srv := &http.Server{Handler: corsMiddleware(mux)}
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

// corsMiddleware applies the P13 stop-gap CORS rule (wide-open). P14
// hardens with origin allow-list + credentials. OPTIONS preflight
// returns 204 No Content + the CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// serveSceneHandler returns the /prism/scene route handler closed over
// the dataset registry. POST only; decodes the body per D081,
// synthesises filters from the selection state, runs plotPipeline,
// returns the SceneDoc JSON.
func serveSceneHandler(ctx context.Context, registry resolve.DatasetRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONErr(w, http.StatusMethodNotAllowed, "PRISM_SERVE_METHOD", "only POST is supported on /prism/scene")
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
			// Prepend so the filter runs before any spec transforms.
			s.Transform = append(injected, s.Transform...)
		}

		// Reuse plotPipeline to build/execute/encode the (possibly
		// mutated) spec.
		buildOpts := build.Options{
			FS:              afero.NewOsFs(),
			Resolver:        resolve.New(nil),
			Backend:         inmem.New(),
			DatasetRegistry: registry,
		}
		execOpts := plan.ExecOpts{}
		encOpts := encode.EncodeOpts{Width: 800, Height: 600}

		// We can't drive plotPipeline directly (it expects a cli.Command
		// for error reporting). Inline the build/execute/encode chain
		// using the same primitives. Composite specs use the same
		// branching pattern.
		var (
			doc    interface{}
			encErr error
		)
		if build.IsComposite(s) {
			composite, berr := build.BuildComposite(s, buildOpts)
			if berr != nil {
				writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_BUILD", berr.Error())
				return
			}
			perCell := make([]map[plan.NodeID]*planTableMap, 0, len(composite.Children))
			_ = perCell
			// Composite execution is more involved; for P13 we keep the
			// stop-gap focused on flat specs (the common case for
			// selection-driven flows). Composite support tracks with
			// P14's full service.
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_COMPOSITE",
				"composite specs not supported in P13 stop-gap; full Twirp service (P14) handles them")
			return
		}

		dag, tipID, berr := build.Build(s, buildOpts)
		if berr != nil {
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_BUILD", berr.Error())
			return
		}
		res, eerr := plan.Execute(ctx, dag, execOpts)
		if eerr != nil {
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_EXECUTE", eerr.Error())
			return
		}
		if len(res.Errors) > 0 {
			var b strings.Builder
			for _, ne := range res.Errors {
				fmt.Fprintf(&b, "node %s: %v; ", ne.Node, ne.Err)
			}
			writeJSONErr(w, http.StatusBadRequest, "PRISM_SERVE_EXECUTE", b.String())
			return
		}
		doc, encErr = encode.Encode(s, res.Tables, tipID, encOpts)
		if encErr != nil {
			writeJSONErr(w, http.StatusInternalServerError, "PRISM_SERVE_ENCODE", encErr.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(doc)
	}
}

// planTableMap is a small alias used only as a placeholder above so
// the composite branch keeps its types descriptive without pulling in
// an unused import. The real Tables type lives on plan.Execute's
// result.
type planTableMap struct{}

// synthesiseSelectionFilters walks the selection state map + the spec's
// selection declarations and emits one FilterTransform per active
// selection. Returns nil when no selection is active.
//
// Filter expressions per D081:
//   - point on field F  → `F in [v1, v2, ...]`. Values come from the
//     selection state's DatumRefs by looking up the row id on the
//     spec's data values when present (inline data only — pulse-backed
//     specs work too because the executor evaluates the filter against
//     the materialised table).
//     v1: we filter on `F == 'lookup_row_value'`; for inline data we
//     read the values straight from spec.Data.Values; otherwise we
//     emit a row-index predicate using the special `__row__` env var.
//   - interval on x     → `<x-field> >= min and <x-field> <= max`.
//   - interval on y     → mirror for y-field.
//
// Unknown selection IDs in the request are ignored (defensive — the
// JS port may send stale state across a spec change).
func synthesiseSelectionFilters(s *spec.Spec, state map[string]selectionStateInput) ([]spec.Transform, error) {
	if len(state) == 0 || s == nil {
		return nil, nil
	}
	var out []spec.Transform
	for id, st := range state {
		decl, ok := s.Selection[id]
		if !ok {
			continue // stale state — silent skip
		}
		if decl.Point != nil && len(st.Points) > 0 {
			// V1 P13 path: filter by row-id list using the special
			// __row__ env var (executor already binds it per row).
			rowIDs := make([]string, 0, len(st.Points))
			for _, p := range st.Points {
				rowIDs = append(rowIDs, strconv.FormatInt(p.RowID, 10))
			}
			// Prefer the configured field when present (more meaningful
			// for users who inspect the synthesised filter); fall back
			// to row id when no field is named.
			if len(decl.Point.Fields) > 0 {
				field := decl.Point.Fields[0]
				// Build `<field> in [<values>]` using the actual row
				// values from inline data when available.
				values := lookupInlineFieldValues(s, field, st.Points)
				if len(values) > 0 {
					expr := field + " in [" + strings.Join(quotedList(values), ", ") + "]"
					out = append(out, spec.Transform{Filter: &spec.FilterTransform{Filter: expr}})
					continue
				}
			}
			// Fallback: row-index list.
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

// fieldForChannel resolves a Scene channel (x / y / etc) back to the
// spec encoding's bound field name.
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
// when the spec uses a non-inline data source (pulse-backed).
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

// quotedList wraps each entry in single quotes for embedding in an
// expr-lang expression. Numeric-looking entries pass through unquoted.
func quotedList(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			out[i] = v
		} else {
			// expr-lang accepts single-quoted strings.
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
