package geo

import (
	"testing"

	"github.com/paulmach/orb"
)

// squareZone builds a unit-square MPA zone from (0,0) to (1,1) in lon/lat.
func squareZone() *Zone {
	poly := orb.Polygon{{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}}
	return &Zone{ID: "sq", Kind: KindMPA, Poly: poly, bound: poly.Bound()}
}

func TestGridInside(t *testing.T) {
	g := NewGrid([]*Zone{squareZone()}, 0.1)

	tests := []struct {
		name     string
		lat, lon float64
		want     bool
	}{
		{"center", 0.5, 0.5, true},
		{"interior cell", 0.35, 0.65, true},
		{"outside east", 0.5, 1.5, false},
		{"outside south", -0.5, 0.5, false},
		{"just inside near edge", 0.02, 0.5, true},
		{"just outside near edge", -0.02, 0.5, false},
	}
	var buf []int
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf = g.Inside(tt.lat, tt.lon, buf)
			got := len(buf) == 1 && buf[0] == 0
			if got != tt.want {
				t.Fatalf("Inside(%.2f,%.2f) membership = %v (zones=%v), want %v", tt.lat, tt.lon, got, buf, tt.want)
			}
		})
	}
}

// TestGridMatchesExact checks the grid answer equals a direct polygon test over
// a dense sample, so the inside/boundary fast paths never disagree with orb.
func TestGridMatchesExact(t *testing.T) {
	z := squareZone()
	g := NewGrid([]*Zone{z}, 0.1)
	// Offset samples off the exact polygon edges (0 and 1), where orb's
	// point-on-boundary result is ambiguous, so the comparison is decisive.
	var buf []int
	for iy := -5; iy <= 15; iy++ {
		for ix := -5; ix <= 15; ix++ {
			lon := float64(ix)*0.1 + 0.037
			lat := float64(iy)*0.1 + 0.061
			buf = g.Inside(lat, lon, buf)
			gridInside := len(buf) == 1
			exact := z.Contains(lon, lat)
			if gridInside != exact {
				t.Fatalf("disagreement at (%.2f,%.2f): grid=%v exact=%v", lat, lon, gridInside, exact)
			}
		}
	}
}
