package compile

import (
	"errors"
	"strings"
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
)

// TestPrismExpressionPassthrough is the PHASE.md test gate. Pulse
// expression syntax must round-trip through CompileExpression +
// Eval without translation.
func TestPrismExpressionPassthrough(t *testing.T) {
	prog, err := CompileExpression("score > 50")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	cases := []struct {
		env  map[string]any
		want bool
	}{
		{map[string]any{"score": 75.0}, true},
		{map[string]any{"score": 25.0}, false},
		{map[string]any{"score": 50.0}, false},
	}
	for _, c := range cases {
		got, err := prog.EvalBool(c.env)
		if err != nil {
			t.Errorf("eval %v: %v", c.env, err)
			continue
		}
		if got != c.want {
			t.Errorf("eval %v: got %v want %v", c.env, got, c.want)
		}
	}
}

// TestPrismExpressionTypeCoercion confirms numeric / boolean
// coercion across the EvalFloat / EvalBool surfaces.
func TestPrismExpressionTypeCoercion(t *testing.T) {
	t.Run("int-plus-float", func(t *testing.T) {
		prog, err := CompileExpression("age + 0.5")
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		got, err := prog.EvalFloat(map[string]any{"age": int64(35)})
		if err != nil {
			t.Fatalf("eval: %v", err)
		}
		if got != 35.5 {
			t.Errorf("got %v, want 35.5", got)
		}
	})

	t.Run("categorical-equality", func(t *testing.T) {
		prog, err := CompileExpression("brand == 'alpha'")
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		got, err := prog.EvalBool(map[string]any{"brand": "alpha"})
		if err != nil {
			t.Fatalf("eval: %v", err)
		}
		if !got {
			t.Errorf("expected true for alpha")
		}
	})
}

// TestPrismExpressionParseError surfaces PRISM_COMPILE_002 for
// syntactically invalid expressions.
func TestPrismExpressionParseError(t *testing.T) {
	_, err := CompileExpression("score >")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if ae.Code != "PRISM_COMPILE_002" {
		t.Errorf("code = %q, want PRISM_COMPILE_002", ae.Code)
	}
	if !strings.Contains(ae.Message, "compile") && !strings.Contains(ae.Message, "parse") {
		// Loose substring check — expr-lang's error string varies.
		t.Logf("message: %s", ae.Message)
	}
}

// TestPrismExpressionRuntimeError surfaces PRISM_COMPILE_002 for
// runtime evaluation failures (non-bool result for EvalBool, etc.).
func TestPrismExpressionRuntimeError(t *testing.T) {
	prog, err := CompileExpression("score")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	_, err = prog.EvalBool(map[string]any{"score": 1.5})
	if err == nil {
		t.Fatal("expected eval error, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if ae.Code != "PRISM_COMPILE_002" {
		t.Errorf("code = %q, want PRISM_COMPILE_002", ae.Code)
	}
}
