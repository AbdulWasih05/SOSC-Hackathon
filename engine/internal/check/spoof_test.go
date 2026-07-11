package check

import (
	"math"
	"testing"
)

func TestSpoofTeleport(t *testing.T) {
	// Reference point in the Gulf of Mannar.
	const lat0, lon0 = 9.0, 79.0
	tests := []struct {
		name             string
		lat, lon         float64
		prevTsMs, tsMs   int64
		wantSpoof        bool
		wantSpeedAtLeast float64 // 0 to skip
	}{
		{"stationary", lat0, lon0, 0, 10_000, false, 0},
		{"slow 21kn", 9.001, lon0, 0, 10_000, false, 0}, // ~111 m in 10 s
		{"58kn under threshold", 9.0027, lon0, 0, 10_000, false, 0},
		{"62kn over threshold", 9.0029, lon0, 0, 10_000, true, 60},
		{"teleport 60nm in 60s", 10.0, lon0, 0, 60_000, true, 60},
		{"dup 60nm same timestamp", 10.0, lon0, 5_000, 5_000, true, 0},
		{"far apart, slow, stale (30kn over 2h)", 10.0, lon0, 0, 7_200_000, false, 0},
		{"backwards time ignored for speed", 9.5, lon0, 60_000, 0, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpoof, gotSpeed := SpoofTeleport(lat0, lon0, tt.prevTsMs, tt.lat, tt.lon, tt.tsMs)
			if gotSpoof != tt.wantSpoof {
				t.Fatalf("spoof = %v (speed %.1f), want %v", gotSpoof, gotSpeed, tt.wantSpoof)
			}
			if tt.wantSpeedAtLeast > 0 && gotSpeed < tt.wantSpeedAtLeast {
				t.Fatalf("implied speed = %.1f kn, want >= %.1f", gotSpeed, tt.wantSpeedAtLeast)
			}
			if gotSpoof && math.IsInf(gotSpeed, 0) {
				t.Fatalf("implied speed must be finite for JSON, got Inf")
			}
		})
	}
}
