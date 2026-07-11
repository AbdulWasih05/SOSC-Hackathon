package check

import (
	"testing"

	"palkwatch/internal/alert"
	"palkwatch/internal/state"
)

// buildHist fills a history buffer oldest-to-newest from parallel speed/heading
// slices, with idx 0 so FishingPattern reads them in order.
func buildHist(speeds, headings []float32) (*[state.HistoryCapacity]state.Fix, uint8) {
	var h [state.HistoryCapacity]state.Fix
	for i := 0; i < len(speeds) && i < state.HistoryCapacity; i++ {
		h[i] = state.Fix{TsMs: int64(1000 + i*10000), SpeedKn: speeds[i], HeadingDeg: headings[i]}
	}
	return &h, 0
}

func rep(v float32, n int) []float32 {
	s := make([]float32, n)
	for i := range s {
		s[i] = v
	}
	return s
}

func TestFishingPattern(t *testing.T) {
	// 16-point tracks.
	circle := []float32{0, 30, 60, 90, 120, 150, 180, 210, 240, 270, 300, 330, 0, 30, 60, 90} // one direction
	weave := []float32{0, 45, 90, 135, 180, 225, 270, 315, 0, 45, 90, 135, 180, 225, 270, 315}
	zig := []float32{0, 60, 0, 60, 0, 60, 0, 60, 0, 60, 0, 60, 0, 60, 0, 60}

	cases := []struct {
		name     string
		speeds   []float32
		headings []float32
		want     string // "" means no detection
	}{
		{
			name:     "trawling: slow, high course variability",
			speeds:   []float32{3, 2.5, 4, 3.5, 2.2, 4.2, 3, 2.8, 3.5, 4, 2.5, 3, 3.8, 2.6, 3.3, 4.1},
			headings: weave,
			want:     alert.KindTrawling,
		},
		{
			name:     "longlining: sawtooth speed, straight course",
			speeds:   []float32{8, 2, 8, 2, 8, 2, 8, 2, 8, 2, 8, 2, 8, 2, 8, 2},
			headings: rep(90, 16),
			want:     alert.KindLonglining,
		},
		{
			name:     "purse seining: one-direction loop then stop",
			speeds:   []float32{9, 9, 8, 8, 9, 8, 9, 8, 7, 6, 5, 4, 3, 2, 1, 1},
			headings: circle,
			want:     alert.KindPurseSeining,
		},
		{
			name:     "ferry into port: single decel, straight (was a longlining false positive)",
			speeds:   []float32{10, 10, 9, 9, 8, 7, 6, 5, 4, 3, 2, 1, 1, 1, 1, 1},
			headings: rep(90, 16),
			want:     "",
		},
		{
			name:     "zigzag decel: high absolute turning but no net loop (was a purse false positive)",
			speeds:   []float32{9, 9, 8, 8, 7, 7, 6, 5, 4, 3, 2, 1, 1, 1, 1, 1},
			headings: zig,
			want:     "",
		},
		{
			name:     "straight slow transit: slow but no course variability (not trawling)",
			speeds:   rep(3, 16),
			headings: rep(90, 16),
			want:     "",
		},
		{
			name:     "too few fixes",
			speeds:   []float32{3, 3, 3, 3},
			headings: []float32{0, 90, 180, 270},
			want:     "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hist, idx := buildHist(c.speeds, c.headings)
			kind, ok, _ := FishingPattern(hist, idx)
			if c.want == "" {
				if ok {
					t.Errorf("expected no detection, got %s", kind)
				}
				return
			}
			if !ok || kind != c.want {
				t.Errorf("got (%q, %v), want %s", kind, ok, c.want)
			}
		})
	}
}
