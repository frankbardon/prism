package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
)

// orderChannelSpec builds a minimal point spec carrying the given
// order channel.
func orderChannelSpec(order *spec.OrderChannel) *spec.Spec {
	return &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "point"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "k", Type: "nominal"},
			},
			Y: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "v", Type: "quantitative"},
			},
			Order: order,
		},
	}
}

func TestPrismOrderChannelShape(t *testing.T) {
	cases := []struct {
		name  string
		order *spec.OrderChannel
		want  int
	}{
		{"unbound", nil, 0},
		{"single ok", &spec.OrderChannel{
			Single: &spec.OrderChannelEntry{Field: "rank"},
		}, 0},
		{"ascending ok", &spec.OrderChannel{
			Single: &spec.OrderChannelEntry{Field: "rank", Sort: "ascending"},
		}, 0},
		{"descending ok", &spec.OrderChannel{
			Single: &spec.OrderChannelEntry{Field: "rank", Sort: "descending"},
		}, 0},
		{"short alias ok", &spec.OrderChannel{
			Single: &spec.OrderChannelEntry{Field: "rank", Sort: "desc"},
		}, 0},
		{"no field", &spec.OrderChannel{
			Single: &spec.OrderChannelEntry{Type: "quantitative"},
		}, 1},
		{"bad direction", &spec.OrderChannel{
			Single: &spec.OrderChannelEntry{Field: "rank", Sort: "reverse"},
		}, 1},
		{"array mixed", &spec.OrderChannel{Multi: []spec.OrderChannelEntry{
			{Field: "tier"},
			{Field: "rank", Sort: "DESC"},
			{Sort: "ascending"},
		}}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := OrderChannelShape{}.Check(orderChannelSpec(tc.order), nil)
			if len(errs) != tc.want {
				t.Fatalf("got %d errors, want %d: %v", len(errs), tc.want, errs)
			}
			for _, e := range errs {
				if e.Code != "PRISM_SPEC_055" {
					t.Fatalf("got code %q, want PRISM_SPEC_055", e.Code)
				}
			}
		})
	}
}

func TestPrismOrderChannelAggregate(t *testing.T) {
	cases := []struct {
		name  string
		order *spec.OrderChannel
		want  int
	}{
		{"unbound", nil, 0},
		{"no aggregate", &spec.OrderChannel{
			Single: &spec.OrderChannelEntry{Field: "rank"},
		}, 0},
		{"aggregate rejected", &spec.OrderChannel{
			Single: &spec.OrderChannelEntry{Field: "rev", Aggregate: "sum"},
		}, 1},
		{"array, one aggregated", &spec.OrderChannel{Multi: []spec.OrderChannelEntry{
			{Field: "tier"},
			{Field: "rev", Aggregate: "mean"},
		}}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := OrderChannelAggregate{}.Check(orderChannelSpec(tc.order), nil)
			if len(errs) != tc.want {
				t.Fatalf("got %d errors, want %d: %v", len(errs), tc.want, errs)
			}
			for _, e := range errs {
				if e.Code != "PRISM_SPEC_056" {
					t.Fatalf("got code %q, want PRISM_SPEC_056", e.Code)
				}
			}
		})
	}
}

// TestPrismOrderChannelRulesWalkChildren pins that both rules reach a
// layer child's own encoding, not just the root.
func TestPrismOrderChannelRulesWalkChildren(t *testing.T) {
	child := orderChannelSpec(&spec.OrderChannel{
		Single: &spec.OrderChannelEntry{Field: "rev", Aggregate: "sum", Sort: "sideways"},
	})
	root := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Layer:  []*spec.Spec{child},
	}
	if got := len(OrderChannelShape{}.Check(root, nil)); got != 1 {
		t.Fatalf("shape rule: got %d errors, want 1", got)
	}
	if got := len(OrderChannelAggregate{}.Check(root, nil)); got != 1 {
		t.Fatalf("aggregate rule: got %d errors, want 1", got)
	}
}
