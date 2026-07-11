package geo

import (
	"math"
	"testing"
)

func TestDeadReckon(t *testing.T) {
	const lat, lon = 9.0, 79.0
	tests := []struct {
		name             string
		heading, distM   float64
		wantLat, wantLon float64
		tol              float64
	}{
		{"north 11132m", 0, 11132, 9.1, 79.0, 1e-3},
		{"south 11132m", 180, 11132, 8.9, 79.0, 1e-3},
		{"east 11132m", 90, 11132, 9.0, 79.1012, 1e-3},
		{"west 11132m", 270, 11132, 9.0, 78.8988, 1e-3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLat, gotLon := DeadReckon(lat, lon, tt.heading, tt.distM)
			if math.Abs(gotLat-tt.wantLat) > tt.tol || math.Abs(gotLon-tt.wantLon) > tt.tol {
				t.Fatalf("DeadReckon = (%.5f,%.5f), want (%.5f,%.5f)", gotLat, gotLon, tt.wantLat, tt.wantLon)
			}
		})
	}
}

func TestSolveInterceptStationaryTarget(t *testing.T) {
	// Patrol ~11.132 km north of a nearly stationary target; it should steer
	// due south and arrive in distance / speed seconds.
	p := Patrol{ID: "P1", Lat: 9.1, Lon: 79.0, MaxSpeedKn: 20}
	ic := SolveIntercept(p, 9.0, 79.0, 0.001, 0) // target crawling north

	if !ic.Feasible {
		t.Fatalf("expected feasible intercept")
	}
	if math.Abs(ic.HeadingDeg-180) > 1.0 {
		t.Fatalf("heading = %.1f, want ~180 (due south)", ic.HeadingDeg)
	}
	wantEta := 11132.0 / KnotsToMPS(20) // ~1082 s
	if math.Abs(ic.EtaS-wantEta) > 30 {
		t.Fatalf("eta = %.1f s, want ~%.1f", ic.EtaS, wantEta)
	}
}

func TestSolveInterceptInfeasible(t *testing.T) {
	// Slow patrol, fast target sprinting away to the north: no intercept.
	p := Patrol{ID: "P2", Lat: 9.0, Lon: 79.0, MaxSpeedKn: 5}
	ic := SolveIntercept(p, 9.05, 79.0, 40, 0) // target north of patrol, heading north fast
	if ic.Feasible {
		t.Fatalf("expected infeasible intercept, got eta %.1f", ic.EtaS)
	}
}

func TestSolveInterceptMovingTargetAheadInTime(t *testing.T) {
	// Target moving east; patrol to the south must lead the target, so the
	// intercept heading is east of due north (between 0 and 90).
	p := Patrol{ID: "P1", Lat: 8.9, Lon: 79.0, MaxSpeedKn: 30}
	ic := SolveIntercept(p, 9.0, 79.0, 10, 90)
	if !ic.Feasible {
		t.Fatalf("expected feasible")
	}
	if ic.HeadingDeg <= 0 || ic.HeadingDeg >= 90 {
		t.Fatalf("lead heading = %.1f, want strictly between 0 and 90", ic.HeadingDeg)
	}
}
