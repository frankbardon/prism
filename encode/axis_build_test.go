package encode

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// TestBuildAxisGridToggle asserts that AxisOpts.Grid=false suppresses
// grid line emission.
func TestBuildAxisGridToggle(t *testing.T) {
	scale := &LinearScale{DomainMin: 0, DomainMax: 100, RangeMin: 0, RangeMax: 500}
	plot := scene.Rect{X: 40, Y: 20, W: 500, H: 400}
	opts := DefaultAxisOpts("score")
	opts.Grid = false
	a := BuildAxisWithOpts(scale, scene.ChannelY, scene.AxisPositionLeft, plot, opts)
	if len(a.Grid) != 0 {
		t.Errorf("expected no grid lines when Grid=false, got %d", len(a.Grid))
	}
}

// TestBuildAxisMinorTicks asserts that linear axes emit minor ticks
// between every pair of majors when AxisOpts.MinorTicks=true.
func TestBuildAxisMinorTicks(t *testing.T) {
	scale := &LinearScale{DomainMin: 0, DomainMax: 10, RangeMin: 0, RangeMax: 500}
	plot := scene.Rect{X: 40, Y: 20, W: 500, H: 400}
	opts := DefaultAxisOpts("score")
	a := BuildAxisWithOpts(scale, scene.ChannelY, scene.AxisPositionLeft, plot, opts)
	majors, minors := 0, 0
	for _, t := range a.Ticks {
		if t.Minor {
			minors++
		} else {
			majors++
		}
	}
	if majors < 2 {
		t.Fatalf("expected >=2 majors, got %d", majors)
	}
	if minors != majors-1 {
		t.Errorf("expected %d minor ticks (one per gap), got %d", majors-1, minors)
	}
}

// TestBuildAxisLabelAngle asserts LabelAngle threads through to the
// scene.Axis.LabelAngle field.
func TestBuildAxisLabelAngle(t *testing.T) {
	scale := &LinearScale{DomainMin: 0, DomainMax: 100, RangeMin: 0, RangeMax: 500}
	plot := scene.Rect{X: 40, Y: 20, W: 500, H: 400}
	opts := DefaultAxisOpts("score")
	opts.LabelAngle = 45
	a := BuildAxisWithOpts(scale, scene.ChannelX, scene.AxisPositionBottom, plot, opts)
	if a.LabelAngle != 45 {
		t.Errorf("LabelAngle = %g, want 45", a.LabelAngle)
	}
}

// TestApplyLabelOverlapParity asserts the parity-skip pass marks every
// other colliding label as hidden.
func TestApplyLabelOverlapParity(t *testing.T) {
	// 5 ticks, each 5px apart with 30-char labels → all overlap.
	ticks := []scene.Tick{
		{Value: 1.0, Pixel: 0, Label: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{Value: 2.0, Pixel: 5, Label: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{Value: 3.0, Pixel: 10, Label: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{Value: 4.0, Pixel: 15, Label: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{Value: 5.0, Pixel: 20, Label: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
	}
	out := applyLabelOverlap(ticks, "parity", scene.AxisPositionBottom)
	hidden := 0
	for _, t := range out {
		if t.LabelHidden {
			hidden++
		}
	}
	if hidden == 0 {
		t.Errorf("expected some labels hidden in parity mode, got 0")
	}
}
