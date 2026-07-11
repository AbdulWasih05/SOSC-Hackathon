package geo

import (
	"math"
	"testing"
)

func TestDistanceM(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lon1, lat2, lon2 float64
		wantM                  float64
		tolM                   float64
	}{
		{"same point", 9.1, 79.4, 9.1, 79.4, 0, 0.001},
		{"0.1 deg north", 9.0, 79.0, 9.1, 79.0, 11132, 5},
		{"0.1 deg east at equatorish", 0.0, 79.0, 0.0, 79.1, 11132, 5},
		{"0.1 deg east at lat 60", 60.0, 79.0, 60.0, 79.1, 5566, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DistanceM(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if math.Abs(got-tt.wantM) > tt.tolM {
				t.Fatalf("DistanceM = %.3f m, want %.3f +/- %.3f", got, tt.wantM, tt.tolM)
			}
		})
	}
}

func TestSpeedConversions(t *testing.T) {
	// 60 knots is the spoof threshold; confirm the round trip and m/s value.
	if got := KnotsToMPS(60); math.Abs(got-30.8667) > 0.01 {
		t.Fatalf("KnotsToMPS(60) = %.4f, want ~30.8667", got)
	}
	if got := MPSToKnots(KnotsToMPS(60)); math.Abs(got-60) > 1e-9 {
		t.Fatalf("round trip knots = %.9f, want 60", got)
	}
	if got := MetersToNm(NmToMeters(50)); math.Abs(got-50) > 1e-9 {
		t.Fatalf("round trip nm = %.9f, want 50", got)
	}
}
