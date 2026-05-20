package marks

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// TestPrismFunnelStages — PHASE.md mandatory P11 gate. Verifies the
// trapezoid widths match input proportions to 1e-9 and the encoder
// emits one path + one text per stage.
func TestPrismFunnelStages(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"stage": []string{"Visit", "Signup", "Checkout", "Purchase"},
		"count": []float64{1000, 400, 200, 100},
	})
	in := Inputs{
		Table:  tbl,
		Layout: plotRect(), // X=40, Y=20, W=740, H=540
		X:      Channel{Field: "stage"},
		Y:      Channel{Field: "count"},
	}
	marks, err := encodeFunnel(in)
	if err != nil {
		t.Fatalf("encodeFunnel: %v", err)
	}
	// 4 stages × (1 path + 1 text) = 8 marks.
	if len(marks) != 8 {
		t.Fatalf("want 8 marks, got %d", len(marks))
	}
	paths := 0
	texts := 0
	for _, m := range marks {
		if m.Path != nil {
			paths++
		}
		if m.Text != nil {
			texts++
		}
	}
	if paths != 4 || texts != 4 {
		t.Errorf("paths=%d texts=%d, want 4/4", paths, texts)
	}
	// Check widths by parsing the path d-strings. Per D066:
	//   topW[i] = values[i] / maxV * plot.W
	//   botW[i] = values[i+1] / maxV * plot.W (or topW[N-1] for last)
	maxV := 1000.0
	plotW := 740.0
	wantTop := []float64{
		1000 / maxV * plotW,
		400 / maxV * plotW,
		200 / maxV * plotW,
		100 / maxV * plotW,
	}
	wantBot := []float64{
		400 / maxV * plotW,
		200 / maxV * plotW,
		100 / maxV * plotW,
		100 / maxV * plotW, // last stage: bot = top
	}
	idx := 0
	for _, m := range marks {
		if m.Path == nil {
			continue
		}
		topW, botW := extractTrapezoidWidths(t, m.Path.D)
		if math.Abs(topW-wantTop[idx]) > 1e-9 {
			t.Errorf("stage %d topW = %g, want %g", idx, topW, wantTop[idx])
		}
		if math.Abs(botW-wantBot[idx]) > 1e-9 {
			t.Errorf("stage %d botW = %g, want %g", idx, botW, wantBot[idx])
		}
		idx++
	}
}

// extractTrapezoidWidths parses a funnel-generated SVG path d-string.
// Format: "M tlx,tly L trx,try L brx,bry L blx,bly Z".
// Returns (topW, botW) = (trx - tlx, brx - blx).
func extractTrapezoidWidths(t *testing.T, d string) (float64, float64) {
	t.Helper()
	tokens := []string{}
	cur := ""
	for _, ch := range d {
		switch ch {
		case 'M', 'L', 'Z', ' ':
			if cur != "" {
				tokens = append(tokens, cur)
				cur = ""
			}
		default:
			cur += string(ch)
		}
	}
	if cur != "" {
		tokens = append(tokens, cur)
	}
	if len(tokens) < 4 {
		t.Fatalf("malformed d-string: %q (tokens=%v)", d, tokens)
	}
	parsePair := func(s string) (float64, float64) {
		parts := strings.Split(s, ",")
		if len(parts) != 2 {
			t.Fatalf("malformed pair: %q", s)
		}
		x, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			t.Fatalf("parse %q: %v", parts[0], err)
		}
		y, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			t.Fatalf("parse %q: %v", parts[1], err)
		}
		return x, y
	}
	tlx, _ := parsePair(tokens[0])
	trx, _ := parsePair(tokens[1])
	brx, _ := parsePair(tokens[2])
	blx, _ := parsePair(tokens[3])
	return trx - tlx, brx - blx
}
