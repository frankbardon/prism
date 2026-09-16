package encode_test

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// axisDomain returns the resolved scale domain on the named channel.
func axisDomain(t *testing.T, sc *scene.Scene, channel scene.Channel) []any {
	t.Helper()
	for _, ax := range sc.Axes {
		if ax.Channel == channel {
			return ax.Scale.Domain
		}
	}
	t.Fatalf("no axis on channel %q", channel)
	return nil
}

func wantNumericDomain(t *testing.T, got []any, lo, hi float64) {
	t.Helper()
	if len(got) != 2 {
		t.Fatalf("domain = %v, want 2 bounds", got)
	}
	gotLo, ok1 := got[0].(float64)
	gotHi, ok2 := got[1].(float64)
	if !ok1 || !ok2 {
		t.Fatalf("domain = %v, want float64 bounds", got)
	}
	if gotLo != lo || gotHi != hi {
		t.Errorf("domain = [%g, %g], want [%g, %g]", gotLo, gotHi, lo, hi)
	}
}

const scaleDomainRows = `"data": {"values": [
	{"k": "a", "v": 100}, {"k": "b", "v": 110}, {"k": "c", "v": 120}
]}`

func quantSpec(t *testing.T, scaleBlock string) *scene.Scene {
	t.Helper()
	body := `{"$schema": "urn:prism:schema:v1:spec", ` + scaleDomainRows + `,
		"mark": {"type": "bar"},
		"encoding": {
			"x": {"field": "k", "type": "nominal"},
			"y": {"field": "v", "type": "quantitative"` + scaleBlock + `}
		}}`
	return encodeInline(t, body)
}

// TestPrismScaleExplicitDomainWins is the headline E2-S1 case: an
// explicit domain pins the bounds outright, overriding both the data
// extent and the zero-forcing that used to be unconditional.
func TestPrismScaleExplicitDomainWins(t *testing.T) {
	sc := quantSpec(t, `, "scale": {"zero": false, "domain": [90, 130]}`)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 90, 130)
}

// TestPrismScaleExplicitDomainOverridesNice proves nice rounding does
// not run on top of an explicit domain.
func TestPrismScaleExplicitDomainOverridesNice(t *testing.T) {
	sc := quantSpec(t, `, "scale": {"domain": [93, 127]}`)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 93, 127)
}

// TestPrismScaleZeroFalse drops the zero clamp for a positive-only
// domain; nice still rounds the remaining extent.
func TestPrismScaleZeroFalse(t *testing.T) {
	sc := quantSpec(t, `, "scale": {"zero": false}`)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 100, 120)
}

// TestPrismScaleZeroFalseNiceFalse yields the raw data extent.
func TestPrismScaleZeroFalseNiceFalse(t *testing.T) {
	sc := quantSpec(t, `, "scale": {"zero": false, "nice": false}`)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 100, 120)
}

// TestPrismScaleDefaultsZeroAndNice pins the Vega-Lite defaults: zero
// in, bounds rounded.
func TestPrismScaleDefaultsZeroAndNice(t *testing.T) {
	sc := quantSpec(t, ``)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 0, 120)
}

// TestPrismScaleNiceRoundsUpwards checks a ragged extent is widened to
// the nearest nice bound rather than left as-is.
func TestPrismScaleNiceRoundsUpwards(t *testing.T) {
	body := `{"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [{"k": "a", "v": 3}, {"k": "b", "v": 97}]},
		"mark": {"type": "bar"},
		"encoding": {
			"x": {"field": "k", "type": "nominal"},
			"y": {"field": "v", "type": "quantitative"}
		}}`
	sc := encodeInline(t, body)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 0, 100)
}

// TestPrismScaleNiceFalseKeepsRawExtent is the opt-out.
func TestPrismScaleNiceFalseKeepsRawExtent(t *testing.T) {
	body := `{"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [{"k": "a", "v": 3}, {"k": "b", "v": 97}]},
		"mark": {"type": "bar"},
		"encoding": {
			"x": {"field": "k", "type": "nominal"},
			"y": {"field": "v", "type": "quantitative", "scale": {"nice": false}}
		}}`
	sc := encodeInline(t, body)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 0, 97)
}

// TestPrismScaleSqrtHonoursZeroAndDomain checks the knobs reach the
// pow / sqrt families too.
func TestPrismScaleSqrtHonoursZeroAndDomain(t *testing.T) {
	sc := quantSpec(t, `, "scale": {"type": "sqrt", "zero": false, "nice": false}`)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 100, 120)

	sc = quantSpec(t, `, "scale": {"type": "pow", "exponent": 2, "domain": [90, 130]}`)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 90, 130)
}

// TestPrismScaleLogKeepsNoZeroForcing locks in the asymmetry the story
// calls out: log must not acquire zero-forcing, and must not nice its
// domain by default.
func TestPrismScaleLogKeepsNoZeroForcing(t *testing.T) {
	sc := quantSpec(t, `, "scale": {"type": "log"}`)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 100, 120)

	// An explicit zero:true cannot drag a log domain to zero either.
	sc = quantSpec(t, `, "scale": {"type": "log", "zero": true}`)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 100, 120)
}

// TestPrismScaleLogNiceRoundsToPowers is the opt-in behaviour.
func TestPrismScaleLogNiceRoundsToPowers(t *testing.T) {
	sc := quantSpec(t, `, "scale": {"type": "log", "nice": true}`)
	wantNumericDomain(t, axisDomain(t, sc, scene.ChannelY), 100, 1000)
}

// TestPrismScaleTimeDomainAndNice checks the temporal family: nice
// rounds to the calendar boundary the tick generator picks, an
// explicit domain pins ISO bounds, and nice:false keeps the extent.
func TestPrismScaleTimeDomainAndNice(t *testing.T) {
	const rows = `"data": {"values": [
		{"t": "2024-02-11T00:00:00Z", "v": 1},
		{"t": "2024-05-20T00:00:00Z", "v": 2}
	]}`
	timeSpec := func(scaleBlock string) *scene.Scene {
		return encodeInline(t, `{"$schema": "urn:prism:schema:v1:spec", `+rows+`,
			"mark": {"type": "line"},
			"encoding": {
				"x": {"field": "t", "type": "temporal"`+scaleBlock+`},
				"y": {"field": "v", "type": "quantitative"}
			}}`)
	}
	ms := func(iso string) float64 {
		ts, err := time.Parse(time.RFC3339, iso)
		if err != nil {
			t.Fatalf("parse %s: %v", iso, err)
		}
		return float64(ts.UnixMilli())
	}

	// ~3 months of span picks month ticks, so the bounds snap to the
	// first of the month either side.
	wantNumericDomain(t, axisDomain(t, timeSpec(``), scene.ChannelX),
		ms("2024-02-01T00:00:00Z"), ms("2024-06-01T00:00:00Z"))

	wantNumericDomain(t, axisDomain(t, timeSpec(`, "scale": {"nice": false}`), scene.ChannelX),
		ms("2024-02-11T00:00:00Z"), ms("2024-05-20T00:00:00Z"))

	wantNumericDomain(t,
		axisDomain(t, timeSpec(`, "scale": {"domain": ["2024-01-01T00:00:00Z", "2024-12-31T00:00:00Z"]}`), scene.ChannelX),
		ms("2024-01-01T00:00:00Z"), ms("2024-12-31T00:00:00Z"))
}

// TestPrismScaleBandDomainPinsOrder retires the "arrange your layers so
// the desired category lands first" workaround.
func TestPrismScaleBandDomainPinsOrder(t *testing.T) {
	sc := quantSpec(t, ``)
	got := axisDomain(t, sc, scene.ChannelX)
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("baseline band domain = %v, want data order a,b,c", got)
	}

	body := `{"$schema": "urn:prism:schema:v1:spec", ` + scaleDomainRows + `,
		"mark": {"type": "bar"},
		"encoding": {
			"x": {"field": "k", "type": "nominal", "scale": {"domain": ["c", "b", "a"]}},
			"y": {"field": "v", "type": "quantitative"}
		}}`
	got = axisDomain(t, encodeInline(t, body), scene.ChannelX)
	want := []string{"c", "b", "a"}
	if len(got) != len(want) {
		t.Fatalf("band domain = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("band domain[%d] = %v, want %q", i, got[i], w)
		}
	}
}

// TestPrismScaleBandDomainAppendsUnlisted keeps a partial domain from
// silently dropping rows: listed categories lead, the rest follow in
// first-seen order.
func TestPrismScaleBandDomainAppendsUnlisted(t *testing.T) {
	body := `{"$schema": "urn:prism:schema:v1:spec", ` + scaleDomainRows + `,
		"mark": {"type": "bar"},
		"encoding": {
			"x": {"field": "k", "type": "nominal", "scale": {"domain": ["c"]}},
			"y": {"field": "v", "type": "quantitative"}
		}}`
	got := axisDomain(t, encodeInline(t, body), scene.ChannelX)
	want := []string{"c", "a", "b"}
	if len(got) != len(want) {
		t.Fatalf("band domain = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("band domain[%d] = %v, want %q", i, got[i], w)
		}
	}
}

// TestPrismScaleSharedLayerDomainRespectsExplicit covers the
// cross-layer resolution path: the union of the layers' extents must
// not overwrite an author-pinned domain.
func TestPrismScaleSharedLayerDomainRespectsExplicit(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"resolve": {"scale": {"y": "shared"}},
		"layer": [
			{
				"data": {"values": [{"k": "a", "v": 10}, {"k": "b", "v": 20}]},
				"mark": "bar",
				"encoding": {
					"x": {"field": "k", "type": "nominal"},
					"y": {"field": "v", "type": "quantitative", "scale": {"domain": [5, 95]}}
				}
			},
			{
				"data": {"values": [{"k": "a", "w": 40}, {"k": "b", "w": 50}]},
				"mark": "rule",
				"encoding": {
					"x": {"field": "k", "type": "nominal"},
					"y": {"field": "w", "type": "quantitative"}
				}
			}
		]
	}`)
	s, c, all := decodeAndRunComposite(t, body)
	doc, err := encode.EncodeComposite(s, c, all, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	if doc.Grid.Shared.Y == nil {
		t.Fatal("Grid.Shared.Y is nil; want a shared y-axis")
	}
	wantNumericDomain(t, doc.Grid.Shared.Y.Scale.Domain, 5, 95)
}

// TestPrismScaleMalformedDomainRejected surfaces PRISM_SPEC_041 at
// encode time for callers that skip validate.
func TestPrismScaleMalformedDomainRejected(t *testing.T) {
	body := `{"$schema": "urn:prism:schema:v1:spec", ` + scaleDomainRows + `,
		"mark": {"type": "bar"},
		"encoding": {
			"x": {"field": "k", "type": "nominal"},
			"y": {"field": "v", "type": "quantitative", "scale": {"domain": [130, 90]}}
		}}`
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewMemMapFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{}); err == nil {
		t.Fatal("reversed domain encoded without error")
	}
}
