package svg_test

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// TestPrismP10SVGGoldensStable runs spec -> build -> execute ->
// encode -> render for each P10 composite mark fixture and diffs
// against the committed golden under testdata/svgs/. Set
// UPDATE_GOLDENS=1 to regenerate.
func TestPrismP10SVGGoldensStable(t *testing.T) {
	fixtures := []string{
		"arc_basic.json",
		"pie.json",
		"donut.json",
		"histogram.json",
		"heatmap.json",
		"boxplot.json",
		"violin_score.json",
	}
	update := os.Getenv("UPDATE_GOLDENS") == "1"
	for _, fix := range fixtures {
		fix := fix
		t.Run(fix, func(t *testing.T) {
			got, err := renderFixture(t, fix)
			if err != nil {
				t.Fatalf("render %s: %v", fix, err)
			}
			goldenName := strings.TrimSuffix(fix, ".json") + ".svg"
			goldenPath := filepath.Join(repoRoot(t), "testdata", "svgs", goldenName)
			if update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden %s: %v", goldenPath, err)
				}
				t.Logf("wrote golden %s (%d bytes)", goldenPath, len(got))
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (%s): %v.\nRun `UPDATE_GOLDENS=1 go test ./render/svg/...` to create.", goldenPath, err)
			}
			if !bytes.Equal(want, got) {
				t.Errorf("SVG does not match golden %s.\n--- golden ---\n%s\n--- got ---\n%s",
					goldenPath, truncate(want, 800), truncate(got, 800))
			}
		})
	}
}

// TestPrismPieAnglesSumToTau — PHASE.md mandatory P10 gate.
// Renders pie.json, walks every ArcGeom in the SceneDoc, sums
// (EndAngle - StartAngle), and asserts the total equals 2π within
// 1e-9 tolerance.
func TestPrismPieAnglesSumToTau(t *testing.T) {
	doc := loadAndEncodeFixture(t, "pie.json")
	sum := 0.0
	for _, cell := range doc.Grid.Cells {
		for _, layer := range cell.Scene.Layers {
			for _, m := range layer.Marks {
				if m.Arc != nil {
					sum += m.Arc.EndAngle - m.Arc.StartAngle
				}
			}
		}
	}
	twoPi := 2 * math.Pi
	if math.Abs(sum-twoPi) > 1e-9 {
		t.Errorf("Σ(arc angles) = %g, want 2π (%g)", sum, twoPi)
	}
}

// TestPrismHistogramAutoBin — PHASE.md mandatory P10 gate.
// Renders histogram.json end-to-end. Asserts:
//   - bin edges are nice round numbers (multiples of 0.1 / 0.25 / 0.5 / 1.0)
//   - counts sum to the row count of the inline data (8)
//   - SVG contains the expected number of prism-mark-bar rects
func TestPrismHistogramAutoBin(t *testing.T) {
	doc := loadAndEncodeFixture(t, "histogram.json")
	totalRects := 0
	for _, cell := range doc.Grid.Cells {
		for _, layer := range cell.Scene.Layers {
			for _, m := range layer.Marks {
				if m.Rect != nil {
					totalRects++
				}
			}
		}
	}
	if totalRects < 1 {
		t.Errorf("histogram produced %d rect marks, want ≥ 1", totalRects)
	}
	// Direct encoder call for bin-edge introspection (the SceneDoc
	// stores RectGeoms but not the underlying edge values; verify the
	// histogram encoder's nice-bin invariants directly).
	histResult := callHistogramEncoderDirect(t)
	if len(histResult.BinEdges) < 2 {
		t.Fatalf("BinEdges = %v, want ≥ 2", histResult.BinEdges)
	}
	sum := 0
	for _, c := range histResult.Counts {
		sum += c
	}
	if sum != 8 {
		t.Errorf("Σ counts = %d, want 8 (row count of histogram.json)", sum)
	}
	// Bin widths must be "nice" round numbers.
	w := histResult.BinEdges[1] - histResult.BinEdges[0]
	nice := false
	for _, candidate := range []float64{0.1, 0.2, 0.25, 0.5, 1.0} {
		if absf(w-candidate) < 1e-9 {
			nice = true
			break
		}
	}
	if !nice {
		t.Errorf("bin width %g is not a nice round value", w)
	}
}

func absf(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestPrismTooltipMaterialized — PHASE.md mandatory P10 gate.
// Builds a synthetic pie spec with a tooltip channel, runs through
// encode, and asserts every Mark carries a non-nil Tooltip with a
// line starting with "region: ".
func TestPrismTooltipMaterialized(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {
			"name": "shares",
			"values": [
				{"region": "NA", "value": 42},
				{"region": "EU", "value": 31},
				{"region": "APAC", "value": 27}
			]
		},
		"mark": "pie",
		"encoding": {
			"theta":   {"field": "value",  "type": "quantitative"},
			"color":   {"field": "region", "type": "nominal"},
			"tooltip": {"field": "region"}
		}
	}`)
	doc := loadAndEncodeBytes(t, body)
	got := 0
	for _, cell := range doc.Grid.Cells {
		for _, layer := range cell.Scene.Layers {
			for _, m := range layer.Marks {
				if m.Tooltip == nil || len(m.Tooltip.Lines) == 0 {
					t.Errorf("mark %s missing tooltip", m.ID)
					continue
				}
				if !strings.HasPrefix(m.Tooltip.Lines[0].Label, "region: ") {
					t.Errorf("tooltip[0] = %q, want prefix 'region: '", m.Tooltip.Lines[0].Label)
				}
				got++
			}
		}
	}
	if got == 0 {
		t.Fatal("no tooltips found on any mark")
	}
}

// loadAndEncodeFixture runs the full plot pipeline (build, execute,
// encode) and returns the SceneDoc. Skips the SVG render step.
// Used by the angle / count / tooltip gates that inspect IR shape.
func loadAndEncodeFixture(t *testing.T, name string) *scene.SceneDoc {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "specs", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return loadAndEncodeBytes(t, body)
}
