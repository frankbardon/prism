package marks

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/spec"
)

func TestPrismTooltipSingleField(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.1, 0.2, 0.3},
	})
	ch := &spec.TooltipChannel{
		Single: &spec.TextChannel{Field: "score"},
	}
	tts := BuildTooltips(tbl, ch, 3)
	if len(tts) != 3 {
		t.Fatalf("len(tooltips) = %d, want 3", len(tts))
	}
	for i, tt := range tts {
		if tt == nil || len(tt.Lines) != 1 {
			t.Fatalf("tooltip[%d] = %+v, want 1 line", i, tt)
		}
		if !strings.HasPrefix(tt.Lines[0].Label, "score: ") {
			t.Errorf("tooltip[%d].label = %q, want prefix 'score: '", i, tt.Lines[0].Label)
		}
	}
}

func TestPrismTooltipMultiField(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"a": []float64{1.0, 2.0, 3.0},
		"b": []string{"x", "y", "z"},
	})
	ch := &spec.TooltipChannel{
		Multi: []spec.TextChannel{
			{Field: "a"},
			{Field: "b"},
		},
	}
	tts := BuildTooltips(tbl, ch, 3)
	if len(tts) != 3 {
		t.Fatalf("len = %d, want 3", len(tts))
	}
	for i, tt := range tts {
		if len(tt.Lines) != 2 {
			t.Fatalf("tooltip[%d] = %d lines, want 2", i, len(tt.Lines))
		}
	}
}

func TestPrismTooltipFormatSpecifier(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.12345, 0.6789},
	})
	ch := &spec.TooltipChannel{
		Single: &spec.TextChannel{Field: "score", Format: ".2f"},
	}
	tts := BuildTooltips(tbl, ch, 2)
	if !strings.HasSuffix(tts[0].Lines[0].Label, "0.12") {
		t.Errorf("formatted label = %q, want suffix '0.12'", tts[0].Lines[0].Label)
	}
	if !strings.HasSuffix(tts[1].Lines[0].Label, "0.68") {
		t.Errorf("formatted label = %q, want suffix '0.68'", tts[1].Lines[0].Label)
	}
}

func TestPrismTooltipMissingField(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.1},
	})
	ch := &spec.TooltipChannel{
		Single: &spec.TextChannel{Field: "absent"},
	}
	tts := BuildTooltips(tbl, ch, 1)
	if len(tts) != 1 || !strings.Contains(tts[0].Lines[0].Label, "<missing>") {
		t.Errorf("expected <missing> marker, got %q", tts[0].Lines[0].Label)
	}
}

func TestPrismTooltipNilChannel(t *testing.T) {
	tbl := buildTable(t, map[string]any{"x": []float64{1}})
	tts := BuildTooltips(tbl, nil, 1)
	if tts != nil {
		t.Errorf("nil channel should yield nil tooltips, got %v", tts)
	}
}
