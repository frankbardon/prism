package recipes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism"
	"github.com/frankbardon/prism/internal/validatorutil"
	"github.com/frankbardon/prism/recipes"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
	_ "github.com/frankbardon/prism/validate/rules"
)

// golden checks one recipe's spec end to end: it validates (shape and
// semantic), compiles with no inert-field or other warning, renders,
// and matches the committed spec JSON and SVG under testdata/
// (UPDATE_GOLDENS=1 rewrites them). The returned SVG is for extra
// structural assertions.
func golden(t *testing.T, name string, s *spec.Spec) string {
	t.Helper()
	body, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body = append(body, '\n')

	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	shape, err := validate.NewShapeValidator()
	if err != nil {
		t.Fatalf("shape validator: %v", err)
	}
	for _, e := range shape.Validate(raw) {
		t.Errorf("shape: %s: %s", e.InstanceLocation, e.Message)
	}
	typed, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, e := range validate.NewDefaultSemanticValidator().Validate(typed, validatorutil.BuildLookup(typed, afero.NewMemMapFs())) {
		t.Errorf("semantic: %s %s", e.Code, e.Message)
	}

	plan, err := prism.CompileJSON(context.Background(), body, prism.CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	for _, d := range plan.Diagnostics {
		t.Errorf("diagnostic %s: %s", d.Code, d.Message)
	}
	svg, _, err := prism.RenderPlan(plan, render.RenderOpts{Format: "svg"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	compare(t, filepath.Join("testdata", name+".spec.json"), body)
	compare(t, filepath.Join("testdata", name+".svg"), svg)
	return string(svg)
}

func compare(t *testing.T, path string, got []byte) {
	t.Helper()
	if os.Getenv("UPDATE_GOLDENS") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (UPDATE_GOLDENS=1 to create): %v", path, err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("%s does not match (UPDATE_GOLDENS=1 to regenerate)", path)
	}
}

func must(t *testing.T) func(*spec.Spec, error) *spec.Spec {
	return func(s *spec.Spec, err error) *spec.Spec {
		t.Helper()
		if err != nil {
			t.Fatalf("recipe: %v", err)
		}
		return s
	}
}

var sig = &recipes.Color{
	Field:  recipes.Field{Name: "state", Title: "Significance"},
	Domain: []string{"significant", "not significant"},
}

func TestForest(t *testing.T) {
	rows := []recipes.Row{
		{"test": "Conversion", "diff": 1.8, "low": 0.4, "high": 3.2, "p": 0.012, "state": "significant"},
		{"test": "Retention", "diff": -0.6, "low": -2.1, "high": 0.9, "p": 0.43, "state": "not significant"},
		{"test": "Revenue", "diff": 0.9, "low": nil, "high": nil, "p": 0.08, "state": "not significant"},
	}
	s := must(t)(recipes.Forest(rows, recipes.ForestOptions{
		Label:      recipes.Field{Name: "test", Title: "Test"},
		Estimate:   recipes.Field{Name: "diff", Title: "Difference (pp)", Format: ".3~g"},
		Low:        "low",
		High:       "high",
		Color:      sig,
		References: []float64{0},
		Tooltip: []recipes.Field{
			{Name: "test", Title: "Test"},
			{Name: "diff", Title: "Difference", Format: ".3~g"},
			{Name: "p", Title: "p-value", Format: ".2~g"},
		},
		Frame: recipes.Frame{Title: "Proportion tests"},
	}))
	svg := golden(t, "forest", s)
	if got := strings.Count(svg, `class="prism-mark-rule"`); got != 3 {
		t.Errorf("rules = %d, want the zero rule and two intervals", got)
	}
	if !strings.Contains(svg, "<title>Test: Conversion&#xA;Difference: 1.8&#xA;p-value: 0.012</title>") {
		t.Errorf("tooltip lines not titled / formatted:\n%s", svg)
	}
	// Three bands, first at the top: Conversion's dot sits highest.
	if h := s.Height; h == nil || *h.Number != recipes.BandHeight(3) {
		t.Errorf("height %v, want BandHeight(3)", h)
	}
}

func TestForestPanelsStacked(t *testing.T) {
	a := must(t)(recipes.Forest([]recipes.Row{{"t": "A", "d": 1.0, "l": 0.5, "h": 1.5}}, recipes.ForestOptions{
		Label: recipes.F("t"), Estimate: recipes.F("d"), Low: "l", High: "h", References: []float64{0}, HideLabels: true,
		Frame: recipes.Frame{Title: "A"},
	}))
	b := must(t)(recipes.Forest([]recipes.Row{{"t": "B", "d": 120.0, "l": 80.0, "h": 160.0}, {"t": "C", "d": 40.0, "l": -10.0, "h": 90.0}}, recipes.ForestOptions{
		Label: recipes.F("t"), Estimate: recipes.F("d"), Low: "l", High: "h", References: []float64{0},
		Frame: recipes.Frame{Title: "B and C"},
	}))
	svg := golden(t, "forest_stack", recipes.Stack(a, b))
	if !strings.Contains(svg, `viewBox="0 0 800 `+ftoa(recipes.BandHeight(1)+recipes.BandHeight(2)+20)+`"`) {
		t.Errorf("stack is not sized by its panels: %s", svg[:200])
	}
}

func TestBar(t *testing.T) {
	rows := []recipes.Row{
		{"region": "North", "sales": 120.0, "lo": 100.0, "hi": 140.0},
		{"region": "South", "sales": 80.0, "lo": 60.0, "hi": 95.0},
		{"region": "East", "sales": 150.0},
	}
	golden(t, "bar_ci", must(t)(recipes.Bar(rows, recipes.BarOptions{
		Category: recipes.Field{Name: "region", Title: "Region"},
		Value:    recipes.Field{Name: "sales", Title: "Sales", Format: ",.0f"},
		Order:    []string{"East", "North", "South"},
		Low:      "lo", High: "hi",
		Tooltip: []recipes.Field{{Name: "region", Title: "Region"}, {Name: "sales", Title: "Sales", Format: ",.0f"}},
	})))
	golden(t, "bar_horizontal", must(t)(recipes.Bar(rows, recipes.BarOptions{
		Category: recipes.F("region"), Value: recipes.F("sales"), Horizontal: true,
	})))
}

func TestGroupedBar(t *testing.T) {
	rows := []recipes.Row{
		{"q": "Q1", "region": "East", "v": 62.0}, {"q": "Q1", "region": "West", "v": 58.0},
		{"q": "Q2", "region": "East", "v": 71.0}, {"q": "Q2", "region": "West", "v": 74.0},
		{"q": 3, "region": "East", "v": 66.0},
	}
	svg := golden(t, "grouped_bar", must(t)(recipes.GroupedBar(rows, recipes.GroupedBarOptions{
		Category: recipes.Field{Name: "q", Title: "Quarter"},
		Group:    recipes.Field{Name: "region", Title: "Region"},
		Value:    recipes.Field{Name: "v", Title: "Revenue"},
		Tooltip:  []recipes.Field{recipes.F("q"), recipes.F("region"), recipes.F("v")},
	})))
	if got := strings.Count(svg, `class="prism-mark-bar"`); got != 5 {
		t.Errorf("bars = %d, want 5 (a numeric category still bands)", got)
	}
}

func TestLine(t *testing.T) {
	rows := []recipes.Row{
		{"month": "2024-02", "series": "a", "v": 3.0}, {"month": "2024-01", "series": "a", "v": 1.0},
		{"month": "2024-01", "series": "b", "v": 2.0}, {"month": "2024-02", "series": "b", "v": 2.5},
	}
	golden(t, "multi_line", must(t)(recipes.Line(rows, recipes.LineOptions{
		X: recipes.Field{Name: "month", Title: "Month"}, Y: recipes.Field{Name: "v", Title: "Value"},
		Series:  &recipes.Color{Field: recipes.Field{Name: "series", Title: "Series"}},
		Points:  true,
		Tooltip: []recipes.Field{recipes.F("month"), recipes.F("series"), {Name: "v", Format: ".2f"}},
	})))
}

func TestScatterDiagnostics(t *testing.T) {
	sample := []float64{-1.2, 0.3, 2.4, -0.4, 0.1, 0.9, -2.2, 1.1}
	theory, sorted, order, b0, b1 := recipes.NormalQQ(sample)
	rows := make([]recipes.Row, len(sample))
	for k := range sorted {
		rows[k] = recipes.Row{"q": theory[k], "r": sorted[k], "id": order[k]}
	}
	lo, hi := theory[0], theory[len(theory)-1]
	golden(t, "normal_qq", must(t)(recipes.Scatter(rows, recipes.ScatterOptions{
		X: recipes.Field{Name: "q", Title: "Theoretical quantiles"},
		Y: recipes.Field{Name: "r", Title: "Standardized residuals"},
		References: []recipes.Reference{
			recipes.Polyline([][2]float64{{lo, b0 + b1*lo}, {hi, b0 + b1*hi}}),
			recipes.RuleY(0),
		},
		Tooltip: []recipes.Field{{Name: "id", Title: "Row"}, {Name: "r", Title: "Residual", Format: ".3~g"}},
	})))
}

func TestHeatmap(t *testing.T) {
	var rows []recipes.Row
	names := []string{"a", "b", "c"}
	corr := [][]float64{{1, 0.6, -0.3}, {0.6, 1, 0.1}, {-0.3, 0.1, 1}}
	for i, r := range names {
		for j, c := range names {
			rows = append(rows, recipes.Row{"row": r, "col": c, "r": corr[i][j]})
		}
	}
	golden(t, "heatmap_corr", must(t)(recipes.Heatmap(rows, recipes.HeatmapOptions{
		X: recipes.F("col"), Y: recipes.F("row"),
		Value:  recipes.Field{Name: "r", Title: "Correlation", Format: ".2f"},
		Domain: []float64{-1, 1}, Scheme: "rdbu", Labels: true, Legend: true, XAxisTop: true,
		Tooltip: []recipes.Field{recipes.F("row"), recipes.F("col"), {Name: "r", Format: ".2f"}},
		Frame:   recipes.Frame{Title: "Correlation"},
	})))
}

func TestBoxAndTiles(t *testing.T) {
	golden(t, "box", must(t)(recipes.Box([]recipes.Row{
		{"g": "A", "wl": 1.0, "q1": 2.0, "med": 3.0, "q3": 4.0, "wh": 6.0},
		{"g": "B", "wl": 2.0, "q1": 3.5, "med": 4.0, "q3": 5.0, "wh": 7.0},
	}, recipes.BoxOptions{
		Category: recipes.Field{Name: "g", Title: "Group"}, Value: recipes.Field{Title: "Score"},
		WhiskerLow: "wl", WhiskerHigh: "wh", Q1: "q1", Q3: "q3", Center: "med",
	})))
	golden(t, "tiles", must(t)(recipes.Tiles([]recipes.Row{
		{"m": "Revenue", "v": 1234567.0}, {"m": "Orders", "v": 4521.0},
	}, recipes.TilesOptions{Label: recipes.F("m"), Value: recipes.Field{Name: "v", Format: ",.0f"}})))
}

func TestSmallMultiples(t *testing.T) {
	var rows []recipes.Row
	for _, s := range []string{"north", "south", "east"} {
		for i, m := range []string{"jan", "feb", "mar"} {
			rows = append(rows, recipes.Row{"s": s, "m": m, "v": float64(i + len(s))})
		}
	}
	lo, hi, _ := recipes.Extent(rows, "v")
	grid, err := recipes.SmallMultiples(rows, "s", []string{"south"}, 2, func(key string, cell []recipes.Row, i int) (*spec.Spec, error) {
		return recipes.Line(cell, recipes.LineOptions{X: recipes.F("m"), XOrder: []string{"jan", "feb", "mar"}, Y: recipes.F("v"), Domain: []float64{lo, hi}})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Concat) != 3 || grid.Columns == nil || *grid.Columns != 2 || *grid.Concat[0].Title.Text != "south" {
		t.Fatalf("grid %+v, want 3 cells, 2 columns, south first", grid)
	}
	golden(t, "small_multiples", grid)
}

// TestDarkVariantPairsLayeredRecipes: a layered recipe under a theme
// with a dark_variant paints its marks and legend through paired
// variables, every one declared for both schemes.
func TestDarkVariantPairsLayeredRecipes(t *testing.T) {
	s := must(t)(recipes.Forest([]recipes.Row{
		{"t": "A", "d": 1.0, "l": 0.5, "h": 1.5, "state": "significant"},
		{"t": "B", "d": -0.2, "l": -0.9, "h": 0.4, "state": "not significant"},
	}, recipes.ForestOptions{Label: recipes.F("t"), Estimate: recipes.F("d"), Low: "l", High: "h", Color: sig, References: []float64{0}}))
	s.Theme = &spec.ThemeOverride{DarkVariant: "dark"}
	plan, err := prism.Compile(context.Background(), s, prism.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := prism.RenderPlan(plan, render.RenderOpts{Format: "svg"})
	if err != nil {
		t.Fatal(err)
	}
	svg := string(out)
	for _, cls := range []string{"prism-mark-point", "prism-legend-swatch"} {
		for _, el := range strings.Split(svg, "<")[1:] {
			if strings.Contains(el, `class="`+cls+`"`) && !strings.Contains(el, "var(--prism-resolved-") {
				t.Errorf("%s not paired: <%s", cls, el)
			}
		}
	}
	if strings.Count(svg, "--prism-resolved-") < 6 {
		t.Errorf("expected paired declarations in the style block")
	}
}

func TestRequiredOptions(t *testing.T) {
	rows := []recipes.Row{{"a": "x", "b": 1.0}}
	if _, err := recipes.Forest(rows, recipes.ForestOptions{Label: recipes.F("a")}); err == nil || !strings.Contains(err.Error(), "Estimate") {
		t.Errorf("Forest without Estimate: %v", err)
	}
	if _, err := recipes.Bar(nil, recipes.BarOptions{Category: recipes.F("a"), Value: recipes.F("b")}); !errors.Is(err, recipes.ErrNoRows) {
		t.Errorf("Bar with no rows: %v, want ErrNoRows", err)
	}
	if _, err := recipes.Box(rows, recipes.BoxOptions{Category: recipes.F("a")}); err == nil {
		t.Error("Box with no statistic: want an error")
	}
}

func TestStats(t *testing.T) {
	for _, c := range []struct{ p, want float64 }{
		{0.975, 1.959963984540054}, {0.025, -1.959963984540054}, {0.5, 0},
		{0.3, -0.5244005127080407}, {0.001, -3.090232306167813}, {1e-10, -6.361340902404056},
		{1e-12, -7.034483825301131},
	} {
		if got := recipes.NormalQuantile(c.p); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("NormalQuantile(%v) = %v, want %v", c.p, got, c.want)
		}
	}
	if !math.IsInf(recipes.NormalQuantile(0), -1) || !math.IsNaN(recipes.NormalQuantile(1.5)) {
		t.Error("NormalQuantile edges")
	}
	// R: ppoints(5) = 0.1190476 0.3095238 0.5 0.6904762 0.8809524
	pp := recipes.PPoints(5)
	if math.Abs(pp[0]-0.119047619) > 1e-8 || pp[2] != 0.5 {
		t.Errorf("PPoints(5) = %v", pp)
	}
	// R: ppoints(12)[1] = (1 - .5) / 12
	if p := recipes.PPoints(12); math.Abs(p[0]-0.5/12) > 1e-12 {
		t.Errorf("PPoints(12)[0] = %v", p[0])
	}
	if q := recipes.Quantile([]float64{1, 2, 3, 4}, 0.25); math.Abs(q-1.75) > 1e-12 {
		t.Errorf("Quantile type 7 = %v, want 1.75", q)
	}
	// R: qqline on c(1,2,3,4,10): quartiles 2 and 4 against
	// qnorm(.25/.75) = ∓0.6744898 → slope 1.482602, intercept 3.
	_, sorted, order, b0, b1 := recipes.NormalQQ([]float64{10, 1, 3, 2, 4})
	if math.Abs(b1-1.482602) > 1e-6 || math.Abs(b0-3) > 1e-9 || sorted[0] != 1 || order[0] != 1 {
		t.Errorf("NormalQQ line %v + %v·x, sorted %v order %v", b0, b1, sorted, order)
	}
}

func ftoa(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
