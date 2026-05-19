package resolve_test

import (
	"errors"
	"testing"

	"github.com/frankbardon/prism/encode/resolve"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
)

func TestPrismResolveUnifyNumeric(t *testing.T) {
	layers := []resolve.LayerDomain{
		{LayerID: "a", Channel: scene.ChannelY, Type: scene.ScaleLinear,
			Values: []any{0.0, 50.0}},
		{LayerID: "b", Channel: scene.ChannelY, Type: scene.ScaleLinear,
			Values: []any{40.0, 100.0}},
	}
	ty, dom, err := resolve.Unify(layers)
	if err != nil {
		t.Fatalf("Unify: %v", err)
	}
	if ty != scene.ScaleLinear {
		t.Errorf("type=%v, want linear", ty)
	}
	if len(dom) != 2 {
		t.Fatalf("domain=%v, want [min,max]", dom)
	}
	if dom[0].(float64) != 0 || dom[1].(float64) != 100 {
		t.Errorf("domain=%v, want [0,100]", dom)
	}
}

func TestPrismResolveUnifyCategorical(t *testing.T) {
	layers := []resolve.LayerDomain{
		{LayerID: "a", Channel: scene.ChannelX, Type: scene.ScaleBand,
			Values: []any{"alpha", "beta"}},
		{LayerID: "b", Channel: scene.ChannelX, Type: scene.ScaleBand,
			Values: []any{"beta", "gamma"}},
	}
	ty, dom, err := resolve.Unify(layers)
	if err != nil {
		t.Fatalf("Unify: %v", err)
	}
	if ty != scene.ScaleBand {
		t.Errorf("type=%v, want band", ty)
	}
	want := []string{"alpha", "beta", "gamma"}
	if len(dom) != len(want) {
		t.Fatalf("domain=%v, want %v", dom, want)
	}
	for i := range want {
		if dom[i].(string) != want[i] {
			t.Errorf("domain[%d]=%v, want %s", i, dom[i], want[i])
		}
	}
}

func TestPrismResolveUnifyTemporal(t *testing.T) {
	layers := []resolve.LayerDomain{
		{LayerID: "a", Channel: scene.ChannelX, Type: scene.ScaleTime,
			Values: []any{"2026-01-01", "2026-01-02"}},
		{LayerID: "b", Channel: scene.ChannelX, Type: scene.ScaleTime,
			Values: []any{"2026-01-03"}},
	}
	ty, dom, err := resolve.Unify(layers)
	if err != nil {
		t.Fatalf("Unify: %v", err)
	}
	if ty != scene.ScaleTime {
		t.Errorf("type=%v, want time", ty)
	}
	if len(dom) != 2 {
		t.Fatalf("domain=%v, want [min,max]", dom)
	}
	mn, mx := dom[0].(float64), dom[1].(float64)
	if !(mn < mx) {
		t.Errorf("expected mn < mx; got mn=%v mx=%v", mn, mx)
	}
}

func TestPrismResolveUnifyMixedRaises005(t *testing.T) {
	cases := []struct {
		name string
		a, b scene.ScaleType
		va   []any
		vb   []any
	}{
		{"quantitative+band", scene.ScaleLinear, scene.ScaleBand,
			[]any{1.0, 2.0}, []any{"x", "y"}},
		{"quantitative+temporal", scene.ScaleLinear, scene.ScaleTime,
			[]any{1.0, 2.0}, []any{"2026-01-01"}},
		{"band+temporal", scene.ScaleBand, scene.ScaleTime,
			[]any{"x", "y"}, []any{"2026-01-01"}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			layers := []resolve.LayerDomain{
				{LayerID: "a", Channel: scene.ChannelY, Type: c.a, Values: c.va},
				{LayerID: "b", Channel: scene.ChannelY, Type: c.b, Values: c.vb},
			}
			_, _, err := resolve.Unify(layers)
			if err == nil {
				t.Fatal("expected PRISM_PLAN_005, got nil")
			}
			var ae *prismerrors.AppError
			if !errors.As(err, &ae) || ae.Code != "PRISM_PLAN_005" {
				t.Errorf("expected PRISM_PLAN_005, got %v", err)
			}
		})
	}
}

func TestPrismResolveFromSpecDefaults(t *testing.T) {
	m := resolve.FromSpec(nil)
	cases := []struct {
		ch   scene.Channel
		mode resolve.Mode
	}{
		{scene.ChannelX, resolve.ModeShared},
		{scene.ChannelY, resolve.ModeShared},
		{scene.ChannelColor, resolve.ModeIndependent},
		{scene.ChannelSize, resolve.ModeIndependent},
	}
	for _, c := range cases {
		got := m[c.ch].Scale
		if got != c.mode {
			t.Errorf("channel %s: Scale=%v, want %v", c.ch, got, c.mode)
		}
	}
}

func TestPrismResolveFromSpecOverlay(t *testing.T) {
	r := &spec.Resolve{
		Scale: &spec.ResolveChannelMap{Y: "independent"},
	}
	m := resolve.FromSpec(r)
	if got := m[scene.ChannelY].Scale; got != resolve.ModeIndependent {
		t.Errorf("y Scale=%v, want independent", got)
	}
	if got := m[scene.ChannelY].Axis; got != resolve.ModeIndependent {
		t.Errorf("y Axis=%v, want independent (axis follows scale)", got)
	}
	if got := m[scene.ChannelX].Scale; got != resolve.ModeShared {
		t.Errorf("x Scale=%v, want shared (default)", got)
	}
}

func TestPrismResolveFromSpecAxisOnlyOverride(t *testing.T) {
	// resolve.axis.y: independent + resolve.scale.y unset → scale
	// stays at default (shared) but axis flips.
	r := &spec.Resolve{
		Axis: &spec.ResolveChannelMap{Y: "independent"},
	}
	m := resolve.FromSpec(r)
	if got := m[scene.ChannelY].Scale; got != resolve.ModeShared {
		t.Errorf("y Scale=%v, want shared (default)", got)
	}
	if got := m[scene.ChannelY].Axis; got != resolve.ModeIndependent {
		t.Errorf("y Axis=%v, want independent", got)
	}
}

func TestPrismResolveUnifySingleLayerPassthrough(t *testing.T) {
	layers := []resolve.LayerDomain{
		{LayerID: "a", Channel: scene.ChannelY, Type: scene.ScaleLog,
			Values: []any{1.0, 100.0}},
	}
	ty, dom, err := resolve.Unify(layers)
	if err != nil {
		t.Fatalf("Unify: %v", err)
	}
	// Single-layer pass-through keeps the original type.
	if ty != scene.ScaleLog {
		t.Errorf("type=%v, want log (passthrough)", ty)
	}
	if len(dom) != 2 {
		t.Errorf("domain=%v, want passthrough", dom)
	}
}
