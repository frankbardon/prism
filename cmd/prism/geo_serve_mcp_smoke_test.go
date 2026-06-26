package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// E2-S2 wires --geodata-dir / PRISM_GEODATA into the `serve` and `mcp`
// leaves so geo render requests resolve tier geometry after the host
// embed removal. These smokes drive both transports out-of-process
// (mirroring the existing serve_smoke harness) against a freshly built
// binary so the new flag is guaranteed present, and assert:
//   - with the dir configured, a geoshape render succeeds;
//   - with no dir configured, the encode boundary surfaces
//     PRISM_GEODATA_DIR_UNSET through the transport's error envelope.
//
// A fresh binary in a temp dir (not the shared bin/prism) avoids a stale
// build silently lacking the new flag.

// buildPrismBinary compiles cmd/prism into a temp file and returns its
// path. Guarantees the binary under test carries the flags added in this
// story rather than a stale shared bin/prism.
func buildPrismBinary(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	bin := filepath.Join(t.TempDir(), "prism")
	build := exec.Command("go", "build", "-o", bin, "./cmd/prism")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build prism: %v\n%s", err, out)
	}
	return bin
}

// startServerBin launches `<bin> serve --port 0 <extraArgs...>` and
// returns the bound port + a kill function, reusing portRegex from
// serve_smoke_test.go.
func startServerBin(t *testing.T, bin string, extraArgs ...string) (port string, kill func()) {
	t.Helper()
	args := append([]string{"serve", "--port", "0"}, extraArgs...)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		t.Fatalf("stderr pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start serve: %v", err)
	}
	scanner := bufio.NewScanner(stderr)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !scanner.Scan() {
			break
		}
		if m := portRegex.FindStringSubmatch(scanner.Text()); len(m) == 2 {
			port = m[1]
			break
		}
	}
	if port == "" {
		cancel()
		_ = cmd.Wait()
		t.Fatal("serve: bound port not seen in stderr within 5s")
	}
	go func() { _, _ = io.Copy(io.Discard, stderr) }()
	kill = func() {
		cancel()
		_ = cmd.Wait()
	}
	return port, kill
}

// geoSceneBody wraps the committed geo_world.json geoshape spec in the
// /prism/scene request envelope ({"spec": <spec>}).
func geoSceneBody(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(repoFile(t, "examples", "specs", "geo_world.json"))
	if err != nil {
		t.Fatalf("read geo_world.json: %v", err)
	}
	body, err := json.Marshal(map[string]json.RawMessage{"spec": raw})
	if err != nil {
		t.Fatalf("marshal scene request: %v", err)
	}
	return body
}

// TestPrismServeGeodataDir asserts a geoshape render through the serve
// /prism/scene endpoint succeeds when --geodata-dir points at the
// committed tier bundles. The out-of-process server has no ambient host
// bundle dir (the package TestMain shim does not cross the process
// boundary), so the flag itself is what makes the geoshape resolve.
func TestPrismServeGeodataDir(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode skips network smoke")
	}
	bin := buildPrismBinary(t)
	geoDir := repoFile(t, "geodata")
	port, kill := startServerBin(t, bin, "--geodata-dir", geoDir)
	defer kill()

	resp, out := postSceneRequest(t, port, geoSceneBody(t))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, truncBody(out, 400))
	}
	if !strings.Contains(string(out), `"geoshape"`) {
		t.Fatalf("expected geoshape mark in scene JSON:\n%s", truncBody(out, 400))
	}
}

// TestPrismServeGeodataDirUnset asserts that a geoshape render through
// serve with no directory configured surfaces PRISM_GEODATA_DIR_UNSET in
// the /prism/scene error envelope. PRISM_GEODATA is neutralised so an
// ambient env var cannot mask the unset path.
func TestPrismServeGeodataDirUnset(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode skips network smoke")
	}
	t.Setenv("PRISM_GEODATA", "")
	bin := buildPrismBinary(t)
	port, kill := startServerBin(t, bin)
	defer kill()

	resp, out := postSceneRequest(t, port, geoSceneBody(t))
	if resp.StatusCode == 200 {
		t.Fatalf("expected non-200 with no geodata dir, got 200: %s", truncBody(out, 400))
	}
	if !strings.Contains(string(out), "PRISM_GEODATA_DIR_UNSET") {
		t.Fatalf("expected PRISM_GEODATA_DIR_UNSET in error envelope:\n%s", truncBody(out, 400))
	}
}

// startMCPClient launches `<bin> mcp <extraArgs...>` over stdio,
// initialises the session, and returns a connected client. The client is
// closed on cleanup.
func startMCPClient(t *testing.T, bin string, extraArgs ...string) *mcpclient.Client {
	t.Helper()
	args := append([]string{"mcp"}, extraArgs...)
	cli, err := mcpclient.NewStdioMCPClient(bin, nil, args...)
	if err != nil {
		t.Fatalf("start mcp client: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var initReq mcpgo.InitializeRequest
	initReq.Params.ProtocolVersion = mcpgo.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcpgo.Implementation{Name: "prism-geo-smoke", Version: "0"}
	if _, err := cli.Initialize(ctx, initReq); err != nil {
		t.Fatalf("mcp initialize: %v", err)
	}
	return cli
}

// callPlotGeoshape invokes the prism_plot tool on the committed
// geo_world.json geoshape spec and returns the tool result.
func callPlotGeoshape(t *testing.T, cli *mcpclient.Client) *mcpgo.CallToolResult {
	t.Helper()
	raw, err := os.ReadFile(repoFile(t, "examples", "specs", "geo_world.json"))
	if err != nil {
		t.Fatalf("read geo_world.json: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var req mcpgo.CallToolRequest
	req.Params.Name = "prism_plot"
	req.Params.Arguments = map[string]any{"spec": string(raw), "format": "svg"}
	res, err := cli.CallTool(ctx, req)
	if err != nil {
		t.Fatalf("CallTool prism_plot: %v", err)
	}
	return res
}

// mcpResultText concatenates the TextContent entries of a tool result.
func mcpResultText(res *mcpgo.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcpgo.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// TestPrismMCPGeodataDir asserts a geoshape render through the mcp
// prism_plot tool succeeds when --geodata-dir is configured.
func TestPrismMCPGeodataDir(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode skips stdio smoke")
	}
	bin := buildPrismBinary(t)
	geoDir := repoFile(t, "geodata")
	cli := startMCPClient(t, bin, "--geodata-dir", geoDir)

	res := callPlotGeoshape(t, cli)
	text := mcpResultText(res)
	if res.IsError {
		t.Fatalf("prism_plot returned error: %s", truncBody([]byte(text), 400))
	}
	var payload struct {
		Bytes string `json:"bytes"`
		Mime  string `json:"mime"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("plot result parse: %v\n%s", err, truncBody([]byte(text), 400))
	}
	if payload.Mime != "image/svg+xml" {
		t.Errorf("mime = %q; want image/svg+xml", payload.Mime)
	}
	decoded, _ := base64.StdEncoding.DecodeString(payload.Bytes)
	if !strings.Contains(string(decoded), "prism-mark-geoshape") {
		t.Fatalf("expected geoshape mark class in rendered SVG:\n%s", truncBody(decoded, 400))
	}
}

// TestPrismMCPGeodataDirUnset asserts a geoshape render through the mcp
// prism_plot tool with no directory configured surfaces
// PRISM_GEODATA_DIR_UNSET as a tool error.
func TestPrismMCPGeodataDirUnset(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode skips stdio smoke")
	}
	t.Setenv("PRISM_GEODATA", "")
	bin := buildPrismBinary(t)
	cli := startMCPClient(t, bin)

	res := callPlotGeoshape(t, cli)
	text := mcpResultText(res)
	if !res.IsError {
		t.Fatalf("expected prism_plot tool error with no geodata dir; got: %s", truncBody([]byte(text), 400))
	}
	if !strings.Contains(text, "PRISM_GEODATA_DIR_UNSET") {
		t.Fatalf("expected PRISM_GEODATA_DIR_UNSET in tool error:\n%s", truncBody([]byte(text), 400))
	}
}
