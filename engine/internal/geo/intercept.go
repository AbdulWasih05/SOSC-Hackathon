package geo

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// Patrol is a stationed patrol asset (static config). Positions are configured,
// not integrated with any fleet system (that is the roadmap).
type Patrol struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	MaxSpeedKn float64 `json:"max_speed_kn"`
}

// Intercept is the closing solution for one patrol against a target.
type Intercept struct {
	PatrolID   string
	HeadingDeg float64
	EtaS       float64
	Feasible   bool
}

// LoadPatrols reads the patrol config JSON ({ "patrols": [...] }).
func LoadPatrols(path string) ([]Patrol, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Patrols []Patrol `json:"patrols"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse patrols: %w", err)
	}
	if len(doc.Patrols) == 0 {
		return nil, fmt.Errorf("no patrols in %s", path)
	}
	return doc.Patrols, nil
}

// SolveIntercept computes when and on what heading a patrol can meet a target
// that starts at (tLat, tLon) and keeps moving at tSpeedKn along tHeadingDeg.
// All math is flat-plane 2D kinematics in local meters (hot-path rule 8): it
// solves |T0 + Vt*tau - P0| = Sp*tau for the earliest tau >= 0. When no such
// tau exists (target outrunning the patrol) it returns the straight-line
// heading to the target's current point with feasible = false.
func SolveIntercept(p Patrol, tLat, tLon, tSpeedKn, tHeadingDeg float64) Intercept {
	// Local east/north meters with the target's current point as origin.
	cosLat := math.Cos(tLat * degToRad)
	px := (p.Lon - tLon) * metersPerDegLat * cosLat
	py := (p.Lat - tLat) * metersPerDegLat

	ts := KnotsToMPS(tSpeedKn)
	h := tHeadingDeg * degToRad
	vtx := ts * math.Sin(h) // east component
	vty := ts * math.Cos(h) // north component
	sp := KnotsToMPS(p.MaxSpeedKn)

	// d = T0 - P0, with T0 at the origin.
	dx := -px
	dy := -py
	a := (vtx*vtx + vty*vty) - sp*sp
	b := 2 * (dx*vtx + dy*vty)
	c := dx*dx + dy*dy

	tau, ok := smallestPositiveRoot(a, b, c)
	if !ok {
		return Intercept{
			PatrolID:   p.ID,
			HeadingDeg: bearingDeg(px, py, 0, 0),
			EtaS:       math.Hypot(dx, dy) / sp,
			Feasible:   false,
		}
	}
	ix := vtx * tau // intercept point relative to origin
	iy := vty * tau
	return Intercept{
		PatrolID:   p.ID,
		HeadingDeg: bearingDeg(px, py, ix, iy),
		EtaS:       tau,
		Feasible:   true,
	}
}

// bearingDeg is the compass bearing (0 = north, clockwise) from one local
// east/north point to another.
func bearingDeg(fromE, fromN, toE, toN float64) float64 {
	d := math.Atan2(toE-fromE, toN-fromN) / degToRad
	if d < 0 {
		d += 360
	}
	return d
}

// smallestPositiveRoot returns the smallest strictly positive root of
// a*t^2 + b*t + c = 0, handling the near-linear case (a ~ 0).
func smallestPositiveRoot(a, b, c float64) (float64, bool) {
	const eps = 1e-9
	if math.Abs(a) < eps {
		if math.Abs(b) < eps {
			return 0, false
		}
		t := -c / b
		if t > eps {
			return t, true
		}
		return 0, false
	}
	disc := b*b - 4*a*c
	if disc < 0 {
		return 0, false
	}
	sq := math.Sqrt(disc)
	best := math.Inf(1)
	found := false
	for _, t := range [2]float64{(-b - sq) / (2 * a), (-b + sq) / (2 * a)} {
		if t > eps && t < best {
			best = t
			found = true
		}
	}
	return best, found
}
