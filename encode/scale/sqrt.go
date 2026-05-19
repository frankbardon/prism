package scale

import "github.com/frankbardon/prism/encode/scene"

// SqrtScale is PowScale with exp=0.5 surfaced as its own type so
// scene.ScaleType reads correctly.
type SqrtScale struct {
	Inner PowScale
}

// Apply implements Scale.
func (s *SqrtScale) Apply(value any) (float64, error) {
	inner := s.Inner
	inner.Exp = 0.5
	return inner.Apply(value)
}

// Domain implements Scale.
func (s *SqrtScale) Domain() []any { return s.Inner.Domain() }

// Range implements Scale.
func (s *SqrtScale) Range() [2]float64 { return s.Inner.Range() }

// Type implements Scale.
func (s *SqrtScale) Type() scene.ScaleType { return scene.ScaleSqrt }
