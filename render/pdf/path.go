package pdf

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/signintech/gopdf"

	prismerrors "github.com/frankbardon/prism/errors"
)

// pathCmd is one parsed SVG path command.
type pathCmd struct {
	Op   rune
	Args []float64
}

// parsePath tokenises an SVG `d` string into a sequence of commands.
// Supported commands: M / m / L / l / H / h / V / v / Q / q / C / c
// / A / a / Z / z. Every other command returns
// PRISM_RENDER_PDF_UNSUPPORTED_PATH per D092.
//
// Chained coordinates (e.g. "M 0 0 10 10 20 20") expand into one
// initial command + subsequent implicit-L (or implicit-l for the
// relative form). M / m's repeated form becomes L / l, matching SVG
// spec.
func parsePath(d string) ([]pathCmd, error) {
	d = strings.TrimSpace(d)
	if d == "" {
		return nil, prismerrors.New(
			"PRISM_RENDER_PDF_UNSUPPORTED_PATH",
			"Empty SVG path string.",
			map[string]any{"Got": ""},
		)
	}

	out := make([]pathCmd, 0, 16)
	var cur rune
	var nums []float64
	i := 0
	for i < len(d) {
		ch := rune(d[i])
		switch {
		case unicode.IsSpace(ch), ch == ',':
			i++
			continue
		case unicode.IsLetter(ch):
			if cur != 0 {
				if err := emitChunk(&out, cur, nums); err != nil {
					return nil, err
				}
				nums = nums[:0]
			}
			cur = ch
			i++
		case ch == '-' || ch == '+' || ch == '.' || (ch >= '0' && ch <= '9'):
			n, consumed, err := readNumber(d[i:])
			if err != nil {
				return nil, prismerrors.New(
					"PRISM_RENDER_PDF_UNSUPPORTED_PATH",
					fmt.Sprintf("Malformed number near %q at offset %d: %v", d[i:min(i+8, len(d))], i, err),
					map[string]any{"Got": ch},
				)
			}
			nums = append(nums, n)
			i += consumed
		default:
			return nil, prismerrors.New(
				"PRISM_RENDER_PDF_UNSUPPORTED_PATH",
				fmt.Sprintf("Unexpected character %q in path data at offset %d.", ch, i),
				map[string]any{"Got": string(ch)},
			)
		}
	}
	if cur != 0 {
		if err := emitChunk(&out, cur, nums); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// emitChunk turns one command-letter + accumulated args into one or
// more pathCmd entries, expanding chained coordinates per SVG spec.
func emitChunk(out *[]pathCmd, op rune, args []float64) error {
	switch op {
	case 'M', 'm':
		if len(args) < 2 || len(args)%2 != 0 {
			return badArity(op, len(args), 2)
		}
		*out = append(*out, pathCmd{Op: op, Args: append([]float64(nil), args[:2]...)})
		// Subsequent pairs become implicit L/l per spec.
		impl := 'L'
		if op == 'm' {
			impl = 'l'
		}
		for k := 2; k < len(args); k += 2 {
			*out = append(*out, pathCmd{Op: impl, Args: append([]float64(nil), args[k:k+2]...)})
		}
	case 'L', 'l', 'T', 't':
		if op == 'T' || op == 't' {
			return unsupportedCmd(op)
		}
		if len(args) < 2 || len(args)%2 != 0 {
			return badArity(op, len(args), 2)
		}
		for k := 0; k < len(args); k += 2 {
			*out = append(*out, pathCmd{Op: op, Args: append([]float64(nil), args[k:k+2]...)})
		}
	case 'H', 'h', 'V', 'v':
		if len(args) < 1 {
			return badArity(op, len(args), 1)
		}
		for k := 0; k < len(args); k++ {
			*out = append(*out, pathCmd{Op: op, Args: []float64{args[k]}})
		}
	case 'Q', 'q':
		if len(args) < 4 || len(args)%4 != 0 {
			return badArity(op, len(args), 4)
		}
		for k := 0; k < len(args); k += 4 {
			*out = append(*out, pathCmd{Op: op, Args: append([]float64(nil), args[k:k+4]...)})
		}
	case 'C', 'c':
		if len(args) < 6 || len(args)%6 != 0 {
			return badArity(op, len(args), 6)
		}
		for k := 0; k < len(args); k += 6 {
			*out = append(*out, pathCmd{Op: op, Args: append([]float64(nil), args[k:k+6]...)})
		}
	case 'A', 'a':
		// 7 args per arc: rx, ry, x-axis-rotation, large-arc, sweep, x, y
		if len(args) < 7 || len(args)%7 != 0 {
			return badArity(op, len(args), 7)
		}
		for k := 0; k < len(args); k += 7 {
			*out = append(*out, pathCmd{Op: op, Args: append([]float64(nil), args[k:k+7]...)})
		}
	case 'Z', 'z':
		*out = append(*out, pathCmd{Op: 'Z', Args: nil})
	case 'S', 's':
		return unsupportedCmd(op)
	default:
		return unsupportedCmd(op)
	}
	return nil
}

func badArity(op rune, got, want int) error {
	return prismerrors.New(
		"PRISM_RENDER_PDF_UNSUPPORTED_PATH",
		fmt.Sprintf("Path command %q expects multiples of %d coordinates, got %d.", op, want, got),
		map[string]any{"Got": string(op), "Args": got},
	)
}

func unsupportedCmd(op rune) error {
	return prismerrors.New(
		"PRISM_RENDER_PDF_UNSUPPORTED_PATH",
		fmt.Sprintf("Path command %q is not supported by the PDF renderer (D092: supported subset is M/L/H/V/Q/C/A/Z plus relative forms).", op),
		map[string]any{"Got": string(op)},
	)
}

// readNumber pulls one float-literal off the front of s, returning
// the parsed value plus byte count consumed.
func readNumber(s string) (float64, int, error) {
	end := 0
	seenDot := false
	seenExp := false
	for end < len(s) {
		ch := s[end]
		switch {
		case ch >= '0' && ch <= '9':
			end++
		case ch == '-' || ch == '+':
			if end == 0 {
				end++
				continue
			}
			// Exponent sign is allowed; otherwise this signals a
			// new number.
			if end > 0 && (s[end-1] == 'e' || s[end-1] == 'E') {
				end++
			} else {
				goto done
			}
		case ch == '.':
			if seenDot {
				goto done
			}
			seenDot = true
			end++
		case ch == 'e' || ch == 'E':
			if seenExp {
				goto done
			}
			seenExp = true
			end++
		default:
			goto done
		}
	}
done:
	if end == 0 {
		return 0, 0, fmt.Errorf("empty number")
	}
	v, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		return 0, end, err
	}
	return v, end, nil
}

// emitPath walks the parsed command stream and emits the equivalent
// gopdf Curve / Line / Polygon calls. Because gopdf doesn't expose
// MoveTo / LineTo as raw operators, the renderer accumulates a
// polyline of points between move-tos and flushes them via Polygon
// when a Z is hit, or via consecutive Line calls when no Z appears.
//
// pen carries the running point so chained relative commands resolve
// correctly. start carries the most recent moveto destination so Z
// snaps the polyline closed at the right point.
func emitPath(pdf *gopdf.GoPdf, cmds []pathCmd, style string) error {
	var pen [2]float64
	var startSub [2]float64
	var poly []gopdf.Point

	flush := func(closed bool) {
		if len(poly) == 0 {
			return
		}
		if closed && len(poly) >= 3 {
			pdf.Polygon(poly, style)
		} else {
			// Open polyline — emit pairwise lines.
			for i := 1; i < len(poly); i++ {
				pdf.Line(poly[i-1].X, poly[i-1].Y, poly[i].X, poly[i].Y)
			}
		}
		poly = poly[:0]
	}

	push := func(x, y float64) {
		poly = append(poly, gopdf.Point{X: x, Y: y})
	}

	for _, c := range cmds {
		switch c.Op {
		case 'M':
			flush(false)
			pen[0], pen[1] = c.Args[0], c.Args[1]
			startSub = pen
			push(pen[0], pen[1])
		case 'm':
			flush(false)
			pen[0] += c.Args[0]
			pen[1] += c.Args[1]
			startSub = pen
			push(pen[0], pen[1])
		case 'L':
			pen[0], pen[1] = c.Args[0], c.Args[1]
			push(pen[0], pen[1])
		case 'l':
			pen[0] += c.Args[0]
			pen[1] += c.Args[1]
			push(pen[0], pen[1])
		case 'H':
			pen[0] = c.Args[0]
			push(pen[0], pen[1])
		case 'h':
			pen[0] += c.Args[0]
			push(pen[0], pen[1])
		case 'V':
			pen[1] = c.Args[0]
			push(pen[0], pen[1])
		case 'v':
			pen[1] += c.Args[0]
			push(pen[0], pen[1])
		case 'Q', 'q':
			// Q x1 y1 x y → degree-elevate to cubic for direct
			// emission. C1 = P0 + 2/3 * (P1 - P0); C2 = P2 + 2/3
			// * (P1 - P2).
			x1, y1, x, y := c.Args[0], c.Args[1], c.Args[2], c.Args[3]
			if c.Op == 'q' {
				x1 += pen[0]
				y1 += pen[1]
				x += pen[0]
				y += pen[1]
			}
			c1x := pen[0] + 2.0/3.0*(x1-pen[0])
			c1y := pen[1] + 2.0/3.0*(y1-pen[1])
			c2x := x + 2.0/3.0*(x1-x)
			c2y := y + 2.0/3.0*(y1-y)
			// Flush accumulated polyline before emitting curve.
			flush(false)
			pdf.Curve(pen[0], pen[1], c1x, c1y, c2x, c2y, x, y, "")
			pen[0], pen[1] = x, y
			// Restart polyline at the curve endpoint so subsequent
			// L commands continue from here.
			push(pen[0], pen[1])
		case 'C', 'c':
			c1x, c1y := c.Args[0], c.Args[1]
			c2x, c2y := c.Args[2], c.Args[3]
			x, y := c.Args[4], c.Args[5]
			if c.Op == 'c' {
				c1x += pen[0]
				c1y += pen[1]
				c2x += pen[0]
				c2y += pen[1]
				x += pen[0]
				y += pen[1]
			}
			flush(false)
			pdf.Curve(pen[0], pen[1], c1x, c1y, c2x, c2y, x, y, "")
			pen[0], pen[1] = x, y
			push(pen[0], pen[1])
		case 'A', 'a':
			rx := c.Args[0]
			ry := c.Args[1]
			phiDeg := c.Args[2]
			largeFlag := c.Args[3] != 0
			sweepFlag := c.Args[4] != 0
			x, y := c.Args[5], c.Args[6]
			if c.Op == 'a' {
				x += pen[0]
				y += pen[1]
			}
			cx, cy, t1, dt := endpointToCenter(pen[0], pen[1], rx, ry, phiDeg*3.141592653589793/180.0, largeFlag, sweepFlag, x, y)
			segs := arcToBeziers(cx, cy, rx, ry, t1, dt, phiDeg*3.141592653589793/180.0)
			flush(false)
			for _, s := range segs {
				pdf.Curve(s.X0, s.Y0, s.C1X, s.C1Y, s.C2X, s.C2Y, s.X1, s.Y1, "")
			}
			pen[0], pen[1] = x, y
			push(pen[0], pen[1])
		case 'Z', 'z':
			push(startSub[0], startSub[1])
			flush(true)
			pen = startSub
		}
	}
	flush(false)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
