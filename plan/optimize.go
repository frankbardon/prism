package plan

import "fmt"

// Pass is one optimizer transformation. Apply returns the (possibly
// new) DAG, a flag indicating whether anything changed, and an error.
// Passes MUST NOT mutate the input DAG — they return a fresh *DAG with
// structural sharing (WithNode / WithoutNode) for the changes they
// introduce.
type Pass interface {
	Name() string
	Apply(*DAG) (*DAG, bool, error)
}

// DefaultPasses is the canonical pass list the executor would run if
// given no override. P07 populates it with the 5 passes from
// design/05-dag-executor.md, ordered per D047 (semantics-preserving
// passes first, sampling last). The slice is intentionally NOT
// pre-populated here — passes live in plan/passes/, which imports
// plan/, so registration happens via plan.SetDefaultPasses called
// from a small init shim in plan/passes/register.go. Optimize works
// correctly against either an empty or populated slice; tests cover
// both shapes.
var DefaultPasses []Pass

// SetDefaultPasses installs the canonical pass list. Called by the
// plan/passes package's init function. Exposed so external consumers
// can substitute a custom list before invoking Optimize.
func SetDefaultPasses(ps []Pass) {
	DefaultPasses = ps
}

// optimizeMaxIterations caps the fixed-point loop so a pair of passes
// that toggle each other's state cannot spin forever. Documented as a
// safety net: hitting the cap surfaces as an error, which means the
// pass interaction needs to be fixed.
const optimizeMaxIterations = 100

// Optimize runs the pass list to fixed point: the loop repeats until
// no pass mutates the DAG. Each iteration runs every pass once;
// passes can re-enable each other across iterations.
//
// A bounded iteration cap guards against pathological pass
// interactions. Hitting the cap returns a generic error, not an
// AppError code — it indicates a developer bug, not a user-visible
// fault. Future profiling could surface this as a debug-only metric.
func Optimize(d *DAG, passes []Pass) (*DAG, error) {
	if d == nil {
		return nil, fmt.Errorf("plan.Optimize: nil DAG")
	}
	for i := 0; i < optimizeMaxIterations; i++ {
		changed := false
		for _, p := range passes {
			next, did, err := p.Apply(d)
			if err != nil {
				return nil, fmt.Errorf("plan.Optimize: pass %s: %w", p.Name(), err)
			}
			if did {
				d = next
				changed = true
			}
		}
		if !changed {
			return d, nil
		}
	}
	return nil, fmt.Errorf("plan.Optimize: did not reach fixed point in %d iterations", optimizeMaxIterations)
}
