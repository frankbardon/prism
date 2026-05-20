package pdf

import (
	"strings"
	"testing"
)

// TestPrismPDFParsePathBasic — "M 0 0 L 10 10 Z" parses into 3
// commands.
func TestPrismPDFParsePathBasic(t *testing.T) {
	cmds, err := parsePath("M 0 0 L 10 10 Z")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cmds) != 3 {
		t.Fatalf("cmd count = %d, want 3 (M+L+Z)", len(cmds))
	}
	if cmds[0].Op != 'M' || cmds[1].Op != 'L' || cmds[2].Op != 'Z' {
		t.Errorf("ops = %c %c %c, want M L Z", cmds[0].Op, cmds[1].Op, cmds[2].Op)
	}
}

// TestPrismPDFParsePathChained — "M 0 0 10 10 20 20" expands into
// one M + two implicit L commands.
func TestPrismPDFParsePathChained(t *testing.T) {
	cmds, err := parsePath("M 0 0 10 10 20 20")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cmds) != 3 {
		t.Fatalf("cmd count = %d, want 3", len(cmds))
	}
	if cmds[0].Op != 'M' || cmds[1].Op != 'L' || cmds[2].Op != 'L' {
		t.Errorf("ops = %c %c %c, want M L L", cmds[0].Op, cmds[1].Op, cmds[2].Op)
	}
}

// TestPrismPDFParsePathQuadratic — "M 0 0 Q 5 10 10 0" yields one
// Q command with 4 args.
func TestPrismPDFParsePathQuadratic(t *testing.T) {
	cmds, err := parsePath("M 0 0 Q 5 10 10 0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cmds) != 2 || cmds[1].Op != 'Q' || len(cmds[1].Args) != 4 {
		t.Fatalf("cmds=%+v, want [M, Q(4 args)]", cmds)
	}
}

// TestPrismPDFParsePathCubic — "C 1 2 3 4 5 6" yields one C command
// with 6 args.
func TestPrismPDFParsePathCubic(t *testing.T) {
	cmds, err := parsePath("M 0 0 C 1 2 3 4 5 6")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cmds) != 2 || cmds[1].Op != 'C' || len(cmds[1].Args) != 6 {
		t.Fatalf("cmds=%+v, want [M, C(6 args)]", cmds)
	}
}

// TestPrismPDFParsePathArc — A command yields one cmd with 7 args.
func TestPrismPDFParsePathArc(t *testing.T) {
	cmds, err := parsePath("M 0 0 A 10 10 0 0 1 20 0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cmds) != 2 || cmds[1].Op != 'A' || len(cmds[1].Args) != 7 {
		t.Fatalf("cmds=%+v, want [M, A(7 args)]", cmds)
	}
}

// TestPrismPDFParsePathUnsupportedReturnsErr — S and T are
// explicitly rejected per D092.
func TestPrismPDFParsePathUnsupportedReturnsErr(t *testing.T) {
	for _, d := range []string{"M 0 0 S 5 5 10 10", "M 0 0 T 5 5"} {
		_, err := parsePath(d)
		if err == nil {
			t.Errorf("parsePath(%q) returned nil error, want PRISM_RENDER_PDF_UNSUPPORTED_PATH", d)
			continue
		}
		if !strings.Contains(err.Error(), "PRISM_RENDER_PDF_UNSUPPORTED_PATH") {
			t.Errorf("parsePath(%q) error %v does not mention PRISM_RENDER_PDF_UNSUPPORTED_PATH", d, err)
		}
	}
}

// TestPrismPDFParsePathRelativeForms — relative-form lowercase
// commands parse without rejection.
func TestPrismPDFParsePathRelativeForms(t *testing.T) {
	cmds, err := parsePath("m 0 0 l 10 10 z")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cmds) != 3 {
		t.Fatalf("cmd count = %d, want 3", len(cmds))
	}
	if cmds[0].Op != 'm' || cmds[1].Op != 'l' || cmds[2].Op != 'Z' {
		t.Errorf("ops = %c %c %c, want m l Z", cmds[0].Op, cmds[1].Op, cmds[2].Op)
	}
}

// TestPrismPDFParsePathEmpty — empty input errors with
// PRISM_RENDER_PDF_UNSUPPORTED_PATH.
func TestPrismPDFParsePathEmpty(t *testing.T) {
	_, err := parsePath("")
	if err == nil {
		t.Fatalf("parsePath(\"\") returned no error")
	}
}
