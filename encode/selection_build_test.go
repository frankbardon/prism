package encode

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

func TestPrismEncodeBuildSelectionsPoint(t *testing.T) {
	in := map[string]spec.Selection{
		"highlight": {Point: &spec.PointSelection{Type: "point", Fields: []string{"brand_id"}, Encodings: []string{"color"}}},
	}
	sels := BuildSelections(in)
	if len(sels) != 1 {
		t.Fatalf("got %d selections; want 1", len(sels))
	}
	s := sels[0]
	if s.ID != "highlight" {
		t.Errorf("ID = %q; want highlight", s.ID)
	}
	if s.Kind != scene.SelectionPoint {
		t.Errorf("Kind = %q; want point", s.Kind)
	}
	if s.On != scene.EventClick {
		t.Errorf("On = %q; want click", s.On)
	}
	if s.Reactive != scene.ReactiveClient {
		t.Errorf("Reactive = %q; want client", s.Reactive)
	}
	if len(s.Encodings) != 1 || s.Encodings[0] != "color" {
		t.Errorf("Encodings = %v; want [color]", s.Encodings)
	}
}

func TestPrismEncodeBuildSelectionsInterval(t *testing.T) {
	in := map[string]spec.Selection{
		"brush": {Interval: &spec.IntervalSelection{Type: "interval", Encodings: []string{"x"}}},
	}
	sels := BuildSelections(in)
	if len(sels) != 1 {
		t.Fatalf("got %d selections; want 1", len(sels))
	}
	s := sels[0]
	if s.Kind != scene.SelectionInterval {
		t.Errorf("Kind = %q; want interval", s.Kind)
	}
	if s.On != scene.EventBrush {
		t.Errorf("On = %q; want brush", s.On)
	}
	if len(s.Channels) != 1 || s.Channels[0] != scene.ChannelX {
		t.Errorf("Channels = %v; want [x]", s.Channels)
	}
}

func TestPrismEncodeBuildSelectionsSortedKeyOrder(t *testing.T) {
	in := map[string]spec.Selection{
		"zulu":  {Point: &spec.PointSelection{Type: "point"}},
		"alpha": {Point: &spec.PointSelection{Type: "point"}},
		"mike":  {Point: &spec.PointSelection{Type: "point"}},
	}
	sels := BuildSelections(in)
	if len(sels) != 3 {
		t.Fatalf("got %d selections; want 3", len(sels))
	}
	want := []string{"alpha", "mike", "zulu"}
	for i, w := range want {
		if sels[i].ID != w {
			t.Errorf("sels[%d].ID = %q; want %q (sorted-key stability)", i, sels[i].ID, w)
		}
	}
}

func TestPrismEncodeBuildSelectionsNilOrEmpty(t *testing.T) {
	if got := BuildSelections(nil); got != nil {
		t.Errorf("nil map → %v; want nil", got)
	}
	if got := BuildSelections(map[string]spec.Selection{}); got != nil {
		t.Errorf("empty map → %v; want nil", got)
	}
}

func TestPrismEncodeBuildSelectionsPointEventOverride(t *testing.T) {
	cases := []struct {
		in   string
		want scene.SelectionEvent
	}{
		{"", scene.EventClick},
		{"click", scene.EventClick},
		{"hover", scene.EventHover},
		{"dblclick", scene.EventDblclick},
		{"bogus", scene.EventClick}, // unknown falls back to click
	}
	for _, tc := range cases {
		sels := BuildSelections(map[string]spec.Selection{
			"a": {Point: &spec.PointSelection{Type: "point", On: tc.in}},
		})
		if len(sels) != 1 || sels[0].On != tc.want {
			t.Errorf("On=%q → %v; want %v", tc.in, sels[0].On, tc.want)
		}
	}
}

func TestPrismEncodeBuildSelectionsIntervalMultiChannel(t *testing.T) {
	sels := BuildSelections(map[string]spec.Selection{
		"box": {Interval: &spec.IntervalSelection{Type: "interval", Encodings: []string{"x", "y"}}},
	})
	if len(sels) != 1 {
		t.Fatalf("got %d; want 1", len(sels))
	}
	chans := sels[0].Channels
	if len(chans) != 2 || chans[0] != scene.ChannelX || chans[1] != scene.ChannelY {
		t.Errorf("Channels = %v; want [x y]", chans)
	}
}
