package mcpserve_test

import (
	"bufio"
	"context"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/frankbardon/prism/mcpserve"
	"github.com/frankbardon/prism/rpc"
)

// initializeRoundTrip runs Serve over a pair of in-memory pipes, writes a
// single MCP initialize request, and returns the one response line the server
// writes back. It fails the test (rather than returning an error) on a read
// failure or a 5s stall, so callers can assert directly on the payload.
func initializeRoundTrip(t *testing.T, facade *rpc.PrismServer, opts mcpserve.Options) string {
	t.Helper()

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveErr := make(chan error, 1)
	go func() { serveErr <- mcpserve.Serve(ctx, facade, opts, inR, outW) }()

	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":` +
		`{"protocolVersion":"2024-11-05","capabilities":{},` +
		`"clientInfo":{"name":"mcpserve-test","version":"0"}}}` + "\n"
	go func() { _, _ = inW.Write([]byte(initReq)) }()

	type readResult struct {
		line string
		err  error
	}
	lines := make(chan readResult, 1)
	go func() {
		line, err := bufio.NewReader(outR).ReadString('\n')
		lines <- readResult{line, err}
	}()

	defer func() {
		cancel()
		_ = inW.Close()
		_ = outW.Close()
	}()

	select {
	case got := <-lines:
		if got.err != nil {
			t.Fatalf("read initialize response: %v", got.err)
		}
		return got.line
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for initialize response")
	}
	return ""
}

// TestServe_InitializeRoundTrip drives the public entry point end-to-end: it
// sends an MCP initialize request over an in-memory transport and asserts the
// server identifies itself. This proves Serve wires the mcp/gosdk adapter
// through to a working JSON-RPC loop.
func TestServe_InitializeRoundTrip(t *testing.T) {
	line := initializeRoundTrip(t, &rpc.PrismServer{}, mcpserve.Options{Version: "9.9.9"})

	if !strings.Contains(line, `"result"`) {
		t.Fatalf("response is not a result: %s", line)
	}
	if !strings.Contains(line, "prism") {
		t.Fatalf("response does not identify the prism server: %s", line)
	}
}

// TestServe_NilFacade covers a nil *rpc.PrismServer: the zero value is a
// working server (rpc.PrismServer field docs), so a nil facade must still
// initialize rather than panic.
func TestServe_NilFacade(t *testing.T) {
	line := initializeRoundTrip(t, nil, mcpserve.Options{})

	if !strings.Contains(line, `"result"`) {
		t.Fatalf("response is not a result: %s", line)
	}
	if !strings.Contains(line, "prism") {
		t.Fatalf("response does not identify the prism server: %s", line)
	}
}

// TestServeTransport_InMemorySession mounts Prism on a caller-supplied
// transport — the motivating use case for ServeTransport — and drives it with a
// genuine go-sdk client rather than hand-rolled JSON-RPC. It proves the session
// initializes against the real server identity and that the tool catalog is
// live behind it.
func TestServeTransport_InMemorySession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientT, serverT := mcpsdk.NewInMemoryTransports()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- mcpserve.ServeTransport(ctx, &rpc.PrismServer{}, mcpserve.Options{Version: "9.9.9"}, serverT)
	}()

	// Connecting blocks on the net.Pipe until the server half reads, so it runs
	// off the test goroutine behind a wall-clock guard: a server that never
	// starts must fail the test, not stall the suite.
	type connectResult struct {
		session *mcpsdk.ClientSession
		err     error
	}
	connected := make(chan connectResult, 1)
	go func() {
		client := mcpsdk.NewClient(&mcpsdk.Implementation{
			Name:    "mcpserve-transport-test",
			Version: "0",
		}, nil)
		session, err := client.Connect(ctx, clientT, nil)
		connected <- connectResult{session, err}
	}()

	var session *mcpsdk.ClientSession
	select {
	case got := <-connected:
		if got.err != nil {
			t.Fatalf("connect client session: %v", got.err)
		}
		session = got.session
	case <-time.After(5 * time.Second):
		t.Fatal("timed out establishing the in-memory session")
	}

	// mcpsdk.InMemoryTransport exposes no Close of its own; closing the session
	// tears down the client half of the pipe and cancelling ctx unwinds the
	// server half, which together retire both transports.
	defer func() {
		_ = session.Close()
		cancel()
		select {
		case <-serveErr:
		case <-time.After(5 * time.Second):
			t.Error("ServeTransport did not return after session close and ctx cancellation")
		}
	}()

	init := session.InitializeResult()
	if init == nil || init.ServerInfo == nil {
		t.Fatal("session reports no initialize result")
	}
	if init.ServerInfo.Name != "prism" {
		t.Fatalf("serverInfo.name = %q, want %q", init.ServerInfo.Name, "prism")
	}

	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	names := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}
	for _, want := range []string{"prism_plot", "prism_validate"} {
		if !slices.Contains(names, want) {
			t.Fatalf("tools/list omits %q; got %v", want, names)
		}
	}
}

// TestServe_DefaultVersion asserts an empty Options.Version falls back to the
// package default rather than advertising an empty version string.
func TestServe_DefaultVersion(t *testing.T) {
	line := initializeRoundTrip(t, &rpc.PrismServer{}, mcpserve.Options{})

	if !strings.Contains(line, `"version":"1.0.0"`) {
		t.Fatalf("initialize result does not advertise the default version: %s", line)
	}
}
