package chartc

import (
	"strings"
	"testing"
)

func TestBucketMaxKeepsMaxPerBucket(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	got := BucketMax(vals, 3)
	want := []float64{3, 6, 10}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("bucket %d = %v want %v", i, got[i], want[i])
		}
	}
}

func TestBucketMaxShortPassthrough(t *testing.T) {
	vals := []float64{5, 1, 9}
	got := BucketMax(vals, 10)
	if len(got) != 3 || got[2] != 9 {
		t.Fatalf("passthrough failed: %v", got)
	}
}

func TestLineGridDimensions(t *testing.T) {
	vals := make([]float64, 100)
	for i := range vals {
		vals[i] = float64(i)
	}
	line := LineGrid(vals, 11, 80)
	if len(line.Rows) != 11 {
		t.Fatalf("rows %d want 11", len(line.Rows))
	}
	if line.Empty {
		t.Fatal("flagged empty with real data")
	}
	if line.Lo == nil || *line.Lo != 0 || *line.Hi != 99 {
		t.Fatalf("lo/hi wrong: %v %v", line.Lo, line.Hi)
	}
	for _, r := range line.Rows {
		if len([]rune(r)) != 80 {
			t.Fatalf("row width %d want 80", len([]rune(r)))
		}
	}
}

func TestLineGridSingleValueBlank(t *testing.T) {
	line := LineGrid([]float64{42}, 4, 10)
	if !line.Empty || line.LastRow != -1 {
		t.Fatalf("expected empty: %+v", line)
	}
	if line.Rows[0] != strings.Repeat(" ", 10) {
		t.Fatalf("expected blank rows, got %q", line.Rows[0])
	}
}

func TestLineGridEndpoints(t *testing.T) {
	line := LineGrid([]float64{0, 10}, 1, 4)
	if line.LastRow != 0 {
		t.Fatalf("rising line should end in top row, last_row=%d", line.LastRow)
	}
	line = LineGrid([]float64{10, 0}, 1, 4)
	if line.LastRow != 0 && line.LastRow != 1 {
		t.Fatalf("falling line should end at bottom, last_row=%d", line.LastRow)
	}
}

func TestLineGridDrawsSomething(t *testing.T) {
	vals := []float64{1, 5, 2, 8, 3, 9, 4}
	line := LineGrid(vals, 5, 20)
	dots := 0
	for _, r := range line.Rows {
		for _, c := range r {
			if c != 0x2800 && c != ' ' {
				dots++
			}
		}
	}
	if dots == 0 {
		t.Fatal("no dots drawn")
	}
}

func TestVolGridShape(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	rows := VolGrid(vals, 2, 10)
	if len(rows) != 2 {
		t.Fatalf("rows %d want 2", len(rows))
	}
	r0 := []rune(rows[0])
	r1 := []rune(rows[1])
	if r0[9] != '▇' {
		t.Errorf("tallest bar should cap with a 7/8 block on top, got %q", string(r0[9]))
	}
	if r1[9] != '▒' {
		t.Errorf("tallest bar should fill below with shade, got %q", string(r1[9]))
	}
	if r0[0] != ' ' || r1[0] != ' ' {
		t.Errorf("smallest bar should be blank, got %q %q", string(r0[0]), string(r1[0]))
	}
}

func TestVolGridEmpty(t *testing.T) {
	rows := VolGrid(nil, 2, 5)
	if rows[0] != "     " || rows[1] != "     " {
		t.Fatalf("expected blanks, got %q %q", rows[0], rows[1])
	}
}

func TestAsciiDegrade(t *testing.T) {
	vals := []float64{1, 5, 2, 8}
	line := LineGrid(vals, 2, 8)
	rows := Ascii(line.Rows)
	for _, r := range rows {
		for _, c := range r {
			if c != ' ' && c != '#' {
				t.Fatalf("ascii mode should only emit space/#, got %q", string(c))
			}
		}
	}
}
