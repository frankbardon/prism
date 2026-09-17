package theme

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/spec"
)

// A progress mark draws two elements per row, so the theme carries two
// keys for it. Every bundled theme has to state both: a track that
// falls through to nothing is the bulletBandShade failure mode this
// key exists to avoid.
func TestPrismBuiltinThemesSetBothProgressKeys(t *testing.T) {
	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			th := MustGet(name)
			for _, key := range []string{MarksKeyProgress, MarksKeyProgressTrack} {
				ms, ok := th.Marks[key]
				if !ok || ms == nil {
					t.Fatalf("theme %q sets no marks.%s", name, key)
				}
				if ms.Fill == "" {
					t.Errorf("theme %q: marks.%s sets no fill", name, key)
				}
			}
		})
	}
}

// The track has to be distinguishable from the value bar sitting on
// it. A theme where both resolve to the same paint renders a progress
// chart as a solid block with no reading in it — which is exactly what
// high_contrast would have produced had it inherited the grid colour
// (pure black) that suits every other bundled theme.
func TestPrismBuiltinProgressTrackIsDistinctFromValueBar(t *testing.T) {
	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			th := MustGet(name)
			bar := th.Marks[MarksKeyProgress]
			track := th.Marks[MarksKeyProgressTrack]
			if bar == nil || track == nil {
				t.Fatalf("theme %q is missing a progress key", name)
			}
			if bar.Fill == track.Fill {
				t.Errorf("theme %q: value bar and track share fill %q", name, bar.Fill)
			}
		})
	}
}

// The track key rides the existing Marks pipeline rather than new
// plumbing, so the sparse-merge / clone path has to carry it with no
// per-key handling anywhere.
func TestPrismProgressTrackKeyMergesSparsely(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	base := &Theme{Marks: map[string]*MarkStyle{
		MarksKeyProgressTrack: {Fill: "#eeeeee", StrokeWidth: f(2)},
	}}
	over := &Theme{Marks: map[string]*MarkStyle{
		MarksKeyProgressTrack: {Fill: "#123456"},
	}}
	got := Merge(base, over).Marks[MarksKeyProgressTrack]
	if got == nil {
		t.Fatal("merged theme lost the progress_track key")
	}
	if got.Fill != "#123456" {
		t.Errorf("fill = %q, want the override's #123456", got.Fill)
	}
	if got.StrokeWidth == nil || *got.StrokeWidth != 2 {
		t.Errorf("stroke_width = %v, want the base's 2 to survive a sparse override", got.StrokeWidth)
	}
}

// css.go emits one variable per token of every Marks entry, keyed by
// the map key verbatim — so the underscore in progress_track reaches
// the stylesheet intact and downstream CSS can override the track.
func TestPrismProgressTrackEmitsCSSVariable(t *testing.T) {
	css := MustGet("light").CSSVariables()
	for _, want := range []string{
		"--prism-mark-progress-fill",
		"--prism-mark-progress_track-fill",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("CSS variables missing %s:\n%s", want, css)
		}
	}
}

// A chart restyles its own track through the spec's sparse theme
// override, the same route every other marks key takes — the key needs
// no case of its own in ApplyOverride.
func TestPrismProgressTrackReachableFromSpecOverride(t *testing.T) {
	got := ApplyOverride(MustGet("light"), &spec.ThemeOverride{
		Marks: map[string]*spec.MarkStyle{
			MarksKeyProgressTrack: {Fill: "#fef3c7"},
		},
	})
	track := got.Marks[MarksKeyProgressTrack]
	if track == nil {
		t.Fatal("override dropped the progress_track key")
	}
	if track.Fill != "#fef3c7" {
		t.Errorf("track fill = %q, want the spec override's #fef3c7", track.Fill)
	}
	// The rest of light's marks survive the sparse override.
	if bar := got.Marks[MarksKeyProgress]; bar == nil || bar.Fill != "#4c78a8" {
		t.Errorf("progress value bar = %+v, want light's #4c78a8 untouched", bar)
	}
}
