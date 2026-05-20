package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// portRegex extracts the bound port from `prism serve` stderr line:
// "prism serve: listening on 127.0.0.1:<port>".
var portRegex = regexp.MustCompile(`listening on 127\.0\.0\.1:(\d+)`)

// startTestServer launches `bin/prism serve --port 0` in a goroutine
// and returns the bound port + a kill function. Uses repoRoot to
// locate the binary; builds it on first use if missing.
func startTestServer(t *testing.T) (port string, kill func()) {
	t.Helper()
	root := repoRoot(t)
	prismBin := filepath.Join(root, "bin", "prism")

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, prismBin, "serve", "--port", "0")
	cmd.Dir = root
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		t.Fatalf("stderr pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start serve: %v", err)
	}

	// Read stderr until we see the "listening on" line; capture the port.
	scanner := bufio.NewScanner(stderr)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if m := portRegex.FindStringSubmatch(line); len(m) == 2 {
			port = m[1]
			break
		}
	}
	if port == "" {
		cancel()
		_ = cmd.Wait()
		t.Fatal("serve: bound port not seen in stderr within 3s")
	}
	// Drain remaining stderr in background so the pipe doesn't fill.
	go func() {
		_, _ = io.Copy(io.Discard, stderr)
	}()

	kill = func() {
		cancel()
		_ = cmd.Wait()
	}
	return port, kill
}

// repoRoot mirrors the helper used elsewhere in cmd/prism tests but
// scoped to this file for serve_smoke isolation.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(dir))
}

// postSceneRequest sends a JSON body to /prism/scene and returns the
// HTTP response + decoded body bytes. Caller closes the body.
func postSceneRequest(t *testing.T, port string, body []byte) (*http.Response, []byte) {
	t.Helper()
	url := "http://127.0.0.1:" + port + "/prism/scene"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post /prism/scene: %v", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, out
}

// TestPrismCLIServeStartsAndAccepts asserts `prism serve --port 0`
// starts, prints its bound port, and accepts a baseline POST that
// returns a valid SceneDoc.
func TestPrismCLIServeStartsAndAccepts(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode skips network smoke")
	}
	ensurePrismBinary(t)
	port, kill := startTestServer(t)
	defer kill()

	body := []byte(`{
      "spec": {
        "$schema": "urn:prism:schema:v1:spec",
        "data": {"name":"scores","values":[{"brand_id":"alpha","score":0.42},{"brand_id":"beta","score":0.71}]},
        "mark": "bar",
        "encoding": {"x":{"field":"brand_id","type":"nominal"},"y":{"field":"score","type":"quantitative"}}
      }
    }`)
	resp, out := postSceneRequest(t, port, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, string(out))
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, truncBody(out, 400))
	}
	if doc["version"] != "1.0" {
		t.Errorf("version = %v, want 1.0", doc["version"])
	}
}

// TestPrismSelectionServerReactive — mandatory PHASE.md gate. POSTs
// the same spec twice (unfiltered + filtered with a point selection)
// and asserts the filtered response has strictly fewer marks. This
// proves the server re-plans the DAG with the synthesised Filter
// transform per D081.
func TestPrismSelectionServerReactive(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode skips network smoke")
	}
	ensurePrismBinary(t)
	port, kill := startTestServer(t)
	defer kill()

	specJSON := `{
      "$schema": "urn:prism:schema:v1:spec",
      "data": {"name":"scores","values":[
        {"brand_id":"alpha","score":0.42},
        {"brand_id":"beta","score":0.71},
        {"brand_id":"gamma","score":0.58}
      ]},
      "selection": {"highlight": {"type":"point", "fields":["brand_id"]}},
      "mark": "bar",
      "encoding": {
        "x": {"field":"brand_id","type":"nominal"},
        "y": {"field":"score","type":"quantitative"}
      }
    }`

	// Unfiltered baseline.
	unfilteredBody := []byte(`{"spec":` + specJSON + `}`)
	resp1, out1 := postSceneRequest(t, port, unfilteredBody)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("unfiltered status = %d; body=%s", resp1.StatusCode, truncBody(out1, 400))
	}
	unfilteredMarks := countMarks(t, out1)
	if unfilteredMarks == 0 {
		t.Fatalf("unfiltered scene has 0 marks")
	}

	// Filtered: pick row 0 only.
	filteredBody := []byte(`{
      "spec": ` + specJSON + `,
      "selection": {"highlight": {"points":[{"layer_id":"layer-0","row_id":0}]}}
    }`)
	resp2, out2 := postSceneRequest(t, port, filteredBody)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("filtered status = %d; body=%s", resp2.StatusCode, truncBody(out2, 400))
	}
	filteredMarks := countMarks(t, out2)
	if filteredMarks >= unfilteredMarks {
		t.Errorf("filtered marks (%d) >= unfiltered marks (%d) — selection-driven filter did not engage",
			filteredMarks, unfilteredMarks)
	}
	if filteredMarks != 1 {
		t.Errorf("filtered marks = %d, want 1 (only row 0 should survive)", filteredMarks)
	}
}

// TestPrismCLIServeRejectsGET asserts non-POST methods → 405.
func TestPrismCLIServeRejectsGET(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode skips network smoke")
	}
	ensurePrismBinary(t)
	port, kill := startTestServer(t)
	defer kill()
	resp, err := http.Get("http://127.0.0.1:" + port + "/prism/scene")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /prism/scene status = %d, want 405", resp.StatusCode)
	}
}

// TestPrismCLIServeCORSPreflight asserts OPTIONS → 204 + CORS hdrs.
func TestPrismCLIServeCORSPreflight(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode skips network smoke")
	}
	ensurePrismBinary(t)
	port, kill := startTestServer(t)
	defer kill()
	req, _ := http.NewRequest(http.MethodOptions, "http://127.0.0.1:"+port+"/prism/scene", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want 204", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("missing CORS Allow-Origin: *")
	}
}

func countMarks(t *testing.T, body []byte) int {
	t.Helper()
	var doc struct {
		Grid struct {
			Cells []struct {
				Scene struct {
					Layers []struct {
						Marks []json.RawMessage `json:"marks"`
					} `json:"layers"`
				} `json:"scene"`
			} `json:"cells"`
		} `json:"grid"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("countMarks decode: %v", err)
	}
	if len(doc.Grid.Cells) == 0 || len(doc.Grid.Cells[0].Scene.Layers) == 0 {
		return 0
	}
	return len(doc.Grid.Cells[0].Scene.Layers[0].Marks)
}

func truncBody(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...[truncated]"
}

// ensurePrismBinary builds bin/prism if the binary is missing. Keeps
// the test self-bootstrapping in fresh checkouts.
func ensurePrismBinary(t *testing.T) {
	t.Helper()
	root := repoRoot(t)
	prismBin := filepath.Join(root, "bin", "prism")
	if _, err := exec.LookPath(prismBin); err == nil {
		return
	}
	// Fallback: check if file exists.
	cmd := exec.Command("ls", prismBin)
	if cmd.Run() == nil {
		return
	}
	build := exec.Command("go", "build", "-o", "bin/prism", "./cmd/prism")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build bin/prism: %v\n%s", err, out)
	}
	// Sanity: confirm the binary exists now.
	if cmd2 := exec.Command("ls", prismBin); cmd2.Run() != nil {
		t.Fatalf("bin/prism missing after build")
	}
	// Suppress unused-import lint when this branch wasn't taken.
	_ = errors.New("")
}
