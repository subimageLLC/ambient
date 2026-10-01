package chartc

import (
	"math"
	"strings"
)

var eighths = []rune(" ▁▂▃▄▅▆▇█")

var brailleBits = [2][4]int{{1, 2, 4, 8}, {16, 32, 64, 128}}

func BucketMax(vals []float64, n int) []float64 {
	if len(vals) == 0 {
		return nil
	}
	if len(vals) <= n {
		return vals
	}
	size := float64(len(vals)) / float64(n)
	out := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		lo := int(float64(i) * size)
		hi := int(float64(i+1) * size)
		if hi < lo+1 {
			hi = lo + 1
		}
		if hi > len(vals) {
			hi = len(vals)
		}
		m := vals[lo]
		for _, v := range vals[lo:hi] {
			if v > m {
				m = v
			}
		}
		out = append(out, m)
	}
	return out
}

type Line struct {
	Rows    []string
	Lo      *float64
	Hi      *float64
	LastRow int
	Empty   bool
}

func LineGrid(vals []float64, height, width int) Line {
	if len(vals) < 2 {
		return Line{Rows: blankRows(height, width), LastRow: -1, Empty: true}
	}
	W, H := width*2, height*4
	canvas := make([]int, height*width)
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	rng := hi - lo
	if rng == 0 {
		rng = 1
	}
	n := len(vals)
	type pt struct{ x, y int }
	pts := make([]pt, n)
	for i, v := range vals {
		x := int(math.Round(float64(i) * float64(W-1) / float64(n-1)))
		y := int(math.Round((1 - (v-lo)/rng) * float64(H-1)))
		pts[i] = pt{x, y}
	}
	setdot := func(x, y int) {
		if x >= 0 && x < W && y >= 0 && y < H {
			canvas[(y/4)*width+x/2] |= brailleBits[x%2][y%4]
		}
	}
	for i := 0; i+1 < n; i++ {
		x0, y0, x1, y1 := pts[i].x, pts[i].y, pts[i+1].x, pts[i+1].y
		if x1 == x0 {
			for y := min(y0, y1); y <= max(y0, y1); y++ {
				setdot(x0, y)
			}
			continue
		}
		for x := x0; x != x1; {
			y := int(math.Round(float64(y0) + float64(x-x0)*float64(y1-y0)/float64(x1-x0)))
			setdot(x, y)
			if x1 > x0 {
				x++
			} else {
				x--
			}
		}
		for y := y0; y != y1; {
			x := int(math.Round(float64(x0) + float64(y-y0)*float64(x1-x0)/float64(y1-y0)))
			setdot(x, y)
			if y1 > y0 {
				y++
			} else {
				y--
			}
		}
		setdot(x1, y1)
	}
	rows := make([]string, height)
	for r := 0; r < height; r++ {
		var b strings.Builder
		for c := 0; c < width; c++ {
			b.WriteRune(rune(0x2800 + canvas[r*width+c]))
		}
		rows[r] = b.String()
	}
	return Line{Rows: rows, Lo: &lo, Hi: &hi, LastRow: pts[n-1].y / 4}
}

func VolGrid(vals []float64, height, width int) []string {
	vals = BucketMax(vals, width)
	if len(vals) == 0 {
		return blankRows(height, width)
	}
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	rng := hi - lo
	if rng == 0 {
		rng = 1
	}
	rows := make([]strings.Builder, height)
	for _, v := range vals {
		e := int((v - lo) / rng * float64(height*8-1))
		full, part := e/8, e%8
		top := height - 1 - full
		for r := 0; r < height; r++ {
			switch {
			case r < top:
				rows[r].WriteRune(' ')
			case r == top:
				rows[r].WriteRune(eighths[part])
			default:
				rows[r].WriteRune('▒')
			}
		}
	}
	out := make([]string, height)
	for i := range out {
		out[i] = rows[i].String()
	}
	return out
}

func blankRows(height, width int) []string {
	rows := make([]string, height)
	for i := range rows {
		rows[i] = strings.Repeat(" ", width)
	}
	return rows
}

func Ascii(rows []string) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		var b strings.Builder
		for _, r := range row {
			switch {
			case r == ' ':
				b.WriteRune(' ')
			case r >= 0x2800 && r <= 0x28FF:
				if r == 0x2800 {
					b.WriteRune(' ')
				} else {
					b.WriteRune('#')
				}
			default:
				b.WriteRune('#')
			}
		}
		out[i] = b.String()
	}
	return out
}
