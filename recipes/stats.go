package recipes

import (
	"math"
	"sort"
)

// The few statistics a quantile-quantile plot needs. They are chart
// geometry (where a point and its reference line sit), not analysis:
// anything model-specific — residuals, leverage, Cook's distance and
// its contours, which points to thin — is the caller's.

// NormalQuantile is the standard normal quantile function Φ⁻¹(p) (R's
// qnorm), by Wichura's AS241 algorithm (about 1e-16 relative error).
// It returns -Inf at 0, +Inf at 1 and NaN outside [0, 1].
func NormalQuantile(p float64) float64 {
	switch {
	case math.IsNaN(p) || p < 0 || p > 1:
		return math.NaN()
	case p == 0:
		return math.Inf(-1)
	case p == 1:
		return math.Inf(1)
	}
	q := p - 0.5
	if math.Abs(q) <= 0.425 {
		r := 0.180625 - q*q
		return q * (((((((r*2509.0809287301226727+33430.575583588128105)*r+67265.770927008700853)*r+
			45921.953931549871457)*r+13731.693765509461125)*r+1971.5909503065514427)*r+133.14166789178437745)*r +
			3.387132872796366608) /
			(((((((r*5226.495278852545925+28729.085735721942674)*r+39307.89580009271061)*r+
				21213.794301586595867)*r+5394.1960214247511077)*r+687.1870074920579083)*r+42.313330701600911252)*r + 1)
	}
	r := p
	if q > 0 {
		r = 1 - p
	}
	r = math.Sqrt(-math.Log(r))
	var x float64
	if r <= 5 {
		r -= 1.6
		x = (((((((r*7.7454501427834140764e-4+0.0227238449892691845833)*r+0.24178072517745061177)*r+
			1.27045825245236838258)*r+3.64784832476320460504)*r+5.7694972214606914055)*r+4.6303378461565452959)*r +
			1.42343711074968357734) /
			(((((((r*1.05075007164441684324e-9+5.475938084995344946e-4)*r+0.0151986665636164571966)*r+
				0.14810397642748007459)*r+0.68976733498510000455)*r+1.6763848301838038494)*r+2.05319162663775882187)*r + 1)
	} else {
		r -= 5
		x = (((((((r*2.01033439929228813265e-7+2.71155556874348757815e-5)*r+0.0012426609473880784386)*r+
			0.026532189526576123093)*r+0.29656057182850489123)*r+1.7848265399172913358)*r+5.4637849111641143699)*r +
			6.6579046435011037772) /
			(((((((r*2.04426310338993978564e-15+1.4215117583164458887e-7)*r+1.8463183175100546818e-5)*r+
				7.868691311456132591e-4)*r+0.0148753612908506148525)*r+0.13692988092273580531)*r+0.59983220655588793769)*r + 1)
	}
	if q < 0 {
		return -x
	}
	return x
}

// PPoints are the n plotting positions R's ppoints(n) gives a QQ plot:
// (i − a) / (n + 1 − 2a) for i = 1…n, a = 3/8 when n ≤ 10, else 1/2.
func PPoints(n int) []float64 {
	if n <= 0 {
		return nil
	}
	a := 0.5
	if n <= 10 {
		a = 3.0 / 8
	}
	out := make([]float64, n)
	for i := range out {
		out[i] = (float64(i+1) - a) / (float64(n) + 1 - 2*a)
	}
	return out
}

// Quantile is the type-7 (R's default, linear interpolation between
// closest ranks) p-quantile of sorted values; NaN when there are none.
func Quantile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return math.NaN()
	}
	at := float64(n-1) * p
	i := int(math.Floor(at))
	if i+1 >= n {
		return sorted[n-1]
	}
	return sorted[i] + (sorted[i+1]-sorted[i])*(at-float64(i))
}

// NormalQQ pairs a sample with the normal quantiles it is plotted
// against: theoretical[k] is the ppoints quantile of the k-th smallest
// value, sorted[k] that value; order[k] is its index in sample, so a
// caller can carry each point's other columns (a tooltip) across. The
// line through the first and third quartiles of both (R's qqline) is
// intercept + slope·x — draw it with Polyline.
func NormalQQ(sample []float64) (theoretical, sorted []float64, order []int, intercept, slope float64) {
	n := len(sample)
	order = make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return sample[order[a]] < sample[order[b]] })
	sorted = make([]float64, n)
	for k, i := range order {
		sorted[k] = sample[i]
	}
	pp := PPoints(n)
	theoretical = make([]float64, n)
	for k, p := range pp {
		theoretical[k] = NormalQuantile(p)
	}
	if n < 2 {
		return theoretical, sorted, order, math.NaN(), math.NaN()
	}
	y1, y3 := Quantile(sorted, 0.25), Quantile(sorted, 0.75)
	x1, x3 := NormalQuantile(0.25), NormalQuantile(0.75)
	slope = (y3 - y1) / (x3 - x1)
	intercept = y1 - slope*x1
	return theoretical, sorted, order, intercept, slope
}
