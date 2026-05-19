package marks

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// ViolinResolution is the default sample count along the value axis
// per violin group. See D061.
const ViolinResolution = 64

// encodeViolin emits one AreaGeom per category group whose Upper and
// Lower arrays are symmetric around the band center, shaped by an
// Epanechnikov KDE with Silverman bandwidth (D061).
//
// Orientation: vertical (x=category, y=quantitative). Same as boxplot.
func encodeViolin(in Inputs) ([]scene.Mark, error) {
	if in.X.Field == "" || in.Y.Field == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"violin mark requires both x (category) and y (quantitative) channel bindings.",
			map[string]any{"Field": "<xy>", "Source": "<encoding>", "Available": joinFieldNames(in.Table)},
		)
	}
	band, ok := in.X.Scale.(BandScaler)
	if !ok {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"violin mark requires a band scale on the category axis.",
			map[string]any{"Field": in.X.Field, "Source": "<scale>", "Available": "band"},
		)
	}
	bandWidth := band.BandWidth()

	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("violin: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}

	// Group rows by category, preserving first-seen order.
	groupValues := map[string][]float64{}
	groupOrder := []string{}
	for i, xv := range xs {
		cat, ok := xv.(string)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("violin category at row %d is not string (got %T).", i, xv),
				map[string]any{"Field": in.X.Field, "Source": "<x>", "Available": "string"},
			)
		}
		yv, ok := toFloat64(ys[i])
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("violin value at row %d is not numeric (got %T).", i, ys[i]),
				map[string]any{"Field": in.Y.Field, "Source": "<y>", "Available": "numeric"},
			)
		}
		if _, seen := groupValues[cat]; !seen {
			groupOrder = append(groupOrder, cat)
		}
		groupValues[cat] = append(groupValues[cat], yv)
	}

	// Resolution override.
	res := ViolinResolution
	if in.Mark != nil && in.Mark.ViolinResolution != nil && *in.Mark.ViolinResolution > 0 {
		res = *in.Mark.ViolinResolution
	}

	out := make([]scene.Mark, 0, len(groupOrder))
	for _, g := range groupOrder {
		vals := groupValues[g]
		if len(vals) == 0 {
			continue
		}
		left, err := in.X.Scale.Apply(g)
		if err != nil {
			return nil, err
		}
		center := left + bandWidth/2
		half := bandWidth / 2

		mean := vmean(vals)
		stdev := vstdev(vals, mean)
		h := silverman(vals, stdev)

		// Sample axis: extend ±2h beyond data range to smooth tails.
		mn, mx := vals[0], vals[0]
		for _, v := range vals[1:] {
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
		}
		mn -= 2 * h
		mx += 2 * h
		if mx == mn {
			mx = mn + 1
		}
		step := (mx - mn) / float64(res-1)

		samples := make([]float64, res)
		densities := make([]float64, res)
		maxD := 0.0
		for i := 0; i < res; i++ {
			x := mn + float64(i)*step
			samples[i] = x
			d := epanechnikovKDE(x, vals, h)
			densities[i] = d
			if d > maxD {
				maxD = d
			}
		}
		// Normalise densities to [0, half].
		if maxD == 0 {
			maxD = 1
		}
		upper := make([][2]float64, res)
		lower := make([][2]float64, res)
		for i := 0; i < res; i++ {
			y, _ := in.Y.Scale.Apply(samples[i])
			off := (densities[i] / maxD) * half
			upper[i] = [2]float64{center + off, y}
			lower[i] = [2]float64{center - off, y}
		}
		out = append(out, scene.Mark{
			Type:  scene.MarkArea,
			ID:    fmt.Sprintf("violin-%s", g),
			Style: in.Style,
			Area: &scene.AreaGeom{
				Upper: upper,
				Lower: lower,
				Curve: scene.CurveLinear,
			},
		})
		_ = mean // mean is informational; kept for future symmetric centering options.
	}
	return out, nil
}

// epanechnikovKernel returns K(u) = 0.75 * (1 - u²) for |u| ≤ 1,
// else 0. The asymptotically optimal kernel by MISE. See D061.
func epanechnikovKernel(u float64) float64 {
	if u < -1 || u > 1 {
		return 0
	}
	return 0.75 * (1 - u*u)
}

// epanechnikovKDE evaluates the kernel density estimate at x given
// sample values + bandwidth h.
//
//	f(x) = (1 / (n*h)) * Σ K((x - xi) / h)
func epanechnikovKDE(x float64, values []float64, h float64) float64 {
	if h == 0 || len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, xi := range values {
		sum += epanechnikovKernel((x - xi) / h)
	}
	return sum / (float64(len(values)) * h)
}

// silverman returns h = 1.06 * stdev * n^(-1/5). Falls back to 1.0
// when stdev == 0 or n < 2 (degenerate cases). See D061.
func silverman(values []float64, stdev float64) float64 {
	n := len(values)
	if n < 2 || stdev == 0 {
		return 1.0
	}
	return 1.06 * stdev * math.Pow(float64(n), -1.0/5.0)
}

func vmean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range values {
		s += v
	}
	return s / float64(len(values))
}

func vstdev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	s := 0.0
	for _, v := range values {
		d := v - mean
		s += d * d
	}
	return math.Sqrt(s / float64(len(values)-1))
}
