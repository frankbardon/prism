package layout

import (
	"fmt"
	"math"
)

// ForcePosition is one node's resolved coordinates after a
// force-directed layout pass. X and Y land in a unit-ish coordinate
// space; the encoder scales / translates to the plot rect.
type ForcePosition struct {
	ID string
	X  float64
	Y  float64
}

// ForceOpts controls the Fruchterman-Reingold convergence loop. All
// fields take sane defaults when zero.
type ForceOpts struct {
	// Iterations is the number of relaxation steps. Default 200.
	// Capped at 2000 to bound encode-time CPU.
	Iterations int
	// LinkDistance is the preferred edge length. Default 30.
	LinkDistance float64
	// Charge is the repulsion strength (negative magnitudes pull
	// nodes apart more strongly). Default -30.
	Charge float64
	// Width / Height bound the layout box. Default 400 × 300.
	Width, Height float64
	// Seed drives the deterministic initial position assignment.
	// Default 42.
	Seed int64
}

// ForceLayout runs a Fruchterman-Reingold force-directed layout on
// g. Returns one ForcePosition per node in declaration order. The
// algorithm is deterministic for a fixed Seed so SVG goldens stay
// stable across runs.
//
// Convergence: O(iterations × n²) pair-ops; the encoder bounds n via
// the dispatch's PRISM_ENCODE_NETWORK_BUDGET cap (out of scope here).
//
// Returns PRISM_ENCODE_NETWORK_NONFINITE-equivalent error via the
// caller's prismerrors wrapper when any node position becomes
// non-finite (NaN / Inf) — guards against pathological inputs that
// blow up the gradient.
func ForceLayout(g *Graph, opts ForceOpts) ([]ForcePosition, error) {
	if g == nil || g.NodeCount() == 0 {
		return nil, fmt.Errorf("force-layout: empty graph")
	}
	iterations := opts.Iterations
	if iterations <= 0 {
		iterations = 200
	}
	if iterations > 2000 {
		iterations = 2000
	}
	link := opts.LinkDistance
	if link <= 0 {
		link = 30
	}
	charge := opts.Charge
	if charge == 0 {
		charge = -30
	}
	width := opts.Width
	if width <= 0 {
		width = 400
	}
	height := opts.Height
	if height <= 0 {
		height = 300
	}
	seed := opts.Seed
	if seed == 0 {
		seed = 42
	}

	n := g.NodeCount()
	// Deterministic seed: a simple linear congruential generator so
	// results stay byte-stable across Go versions (math/rand defaults
	// don't promise that).
	rng := &lcg{state: uint64(seed)}
	pos := make([]ForcePosition, n)
	for i, node := range g.Nodes {
		pos[i] = ForcePosition{
			ID: node.ID,
			X:  rng.uniform(0, width),
			Y:  rng.uniform(0, height),
		}
	}

	idIndex := make(map[string]int, n)
	for i, p := range pos {
		idIndex[p.ID] = i
	}

	k := math.Sqrt((width * height) / float64(n))
	temperature := width / 10
	cool := temperature / float64(iterations)

	displacement := make([]ForcePosition, n)
	for iter := 0; iter < iterations; iter++ {
		for i := range displacement {
			displacement[i].X = 0
			displacement[i].Y = 0
		}
		// Repulsive forces — every pair of nodes pushes apart.
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				dx := pos[i].X - pos[j].X
				dy := pos[i].Y - pos[j].Y
				dist := math.Hypot(dx, dy)
				if dist == 0 {
					dist = 0.001
					dx = 0.001
				}
				force := (k * k / dist) * math.Abs(charge) / 30.0
				ux := dx / dist
				uy := dy / dist
				displacement[i].X += ux * force
				displacement[i].Y += uy * force
				displacement[j].X -= ux * force
				displacement[j].Y -= uy * force
			}
		}
		// Attractive forces — every edge pulls its endpoints together
		// towards the preferred link distance.
		for _, e := range g.Edges {
			ai, aok := idIndex[e.From]
			bi, bok := idIndex[e.To]
			if !aok || !bok || ai == bi {
				continue
			}
			dx := pos[ai].X - pos[bi].X
			dy := pos[ai].Y - pos[bi].Y
			dist := math.Hypot(dx, dy)
			if dist == 0 {
				continue
			}
			force := (dist * dist / k) * (dist / link)
			ux := dx / dist
			uy := dy / dist
			displacement[ai].X -= ux * force
			displacement[ai].Y -= uy * force
			displacement[bi].X += ux * force
			displacement[bi].Y += uy * force
		}
		// Apply displacement, capped by temperature, clamped to box.
		for i := range pos {
			d := math.Hypot(displacement[i].X, displacement[i].Y)
			if d == 0 {
				continue
			}
			limit := math.Min(d, temperature)
			pos[i].X += displacement[i].X * (limit / d)
			pos[i].Y += displacement[i].Y * (limit / d)
			pos[i].X = math.Max(0, math.Min(width, pos[i].X))
			pos[i].Y = math.Max(0, math.Min(height, pos[i].Y))
			if !isFinite(pos[i].X) || !isFinite(pos[i].Y) {
				return nil, fmt.Errorf("force-layout: node %s position non-finite at iteration %d", pos[i].ID, iter)
			}
		}
		temperature -= cool
		if temperature < 0 {
			temperature = 0
		}
	}
	return pos, nil
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// lcg is a tiny linear congruential PRNG so seeded output is
// byte-stable across Go runtime updates (math/rand doesn't promise
// that). Constants from Numerical Recipes.
type lcg struct{ state uint64 }

func (l *lcg) next() uint64 {
	l.state = l.state*1664525 + 1013904223
	return l.state
}

func (l *lcg) uniform(lo, hi float64) float64 {
	return lo + (float64(l.next()&0xFFFFFFFF)/float64(1<<32))*(hi-lo)
}
