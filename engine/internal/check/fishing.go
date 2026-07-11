package check

import (
	"fmt"
	"math"

	"palkwatch/internal/alert"
	"palkwatch/internal/state"
)

// courseStdDeg returns the circular standard deviation of a set of headings in
// degrees. It is the honest measure of "how much is this vessel turning": near 0
// for a straight track, large for a weaving or looping one. Uses the mean
// resultant vector length R, with stddev = sqrt(-2 ln R).
func courseStdDeg(fixes []state.Fix) float64 {
	var sumSin, sumCos float64
	for _, f := range fixes {
		r := float64(f.HeadingDeg) * math.Pi / 180
		sumSin += math.Sin(r)
		sumCos += math.Cos(r)
	}
	n := float64(len(fixes))
	if n == 0 {
		return 0
	}
	R := math.Hypot(sumSin/n, sumCos/n)
	if R <= 0 {
		return 180
	}
	if R >= 1 {
		return 0
	}
	return math.Sqrt(-2*math.Log(R)) * 180 / math.Pi
}

// FishingPattern analyzes the vessel's history buffer to detect Trawling,
// Longlining, or Purse Seining. It returns the alert kind, whether one was
// detected, and a detail string.
func FishingPattern(hist *[state.HistoryCapacity]state.Fix, idx uint8) (string, bool, string) {
	var fixes []state.Fix
	for i := 0; i < state.HistoryCapacity; i++ {
		// Traverse oldest to newest
		pos := (int(idx) + i) % state.HistoryCapacity
		f := hist[pos]
		if f.TsMs != 0 {
			fixes = append(fixes, f)
		}
	}

	n := len(fixes)
	// Need at least 8 fixes to establish a pattern
	if n < 8 {
		return "", false, ""
	}

	var maxSpeed, minSpeed float64
	var sumSpeed float64
	var sumSignedCogDelta float64 // net rotation (signed), for the purse-seine loop

	minSpeed = float64(fixes[0].SpeedKn)
	maxSpeed = minSpeed

	for i := 0; i < n; i++ {
		s := float64(fixes[i].SpeedKn)
		sumSpeed += s
		if s < minSpeed {
			minSpeed = s
		}
		if s > maxSpeed {
			maxSpeed = s
		}
		if i > 0 {
			prevCog := float64(fixes[i-1].HeadingDeg)
			currCog := float64(fixes[i].HeadingDeg)

			// Shortest signed angular distance (positive = turning one way).
			delta := currCog - prevCog
			for delta > 180 {
				delta -= 360
			}
			for delta < -180 {
				delta += 360
			}
			sumSignedCogDelta += delta
		}
	}

	avgSpeed := sumSpeed / float64(n)
	recentSpeed := float64(fixes[n-1].SpeedKn)
	firstSpeed := float64(fixes[0].SpeedKn)
	courseStd := courseStdDeg(fixes)

	// Speed oscillations: count fast<->slow transitions with hysteresis, the
	// signature of longlining (steam, set/haul, steam, ...). A single
	// deceleration yields one crossing; a genuine sawtooth yields several.
	speedCrossings := 0
	fastState := firstSpeed >= 4.5
	for i := 0; i < n; i++ {
		s := float64(fixes[i].SpeedKn)
		if fastState && s <= 3.0 {
			fastState = false
			speedCrossings++
		} else if !fastState && s >= 6.0 {
			fastState = true
			speedCrossings++
		}
	}

	// PURSE SEINING: a near-complete loop in ONE direction (the net set) followed
	// by a stop to haul. Uses NET signed rotation, so a back-and-forth zigzag
	// (whose turns cancel) does not qualify; only a real circle does. Checked
	// first: a true set is the most specific, highest-confidence signature.
	if n >= 10 && math.Abs(sumSignedCogDelta) >= 270.0 && maxSpeed > 7.0 && firstSpeed > 5.0 && recentSpeed <= 2.5 {
		return alert.KindPurseSeining, true, fmt.Sprintf("%.0f deg net loop then stop", math.Abs(sumSignedCogDelta))
	}

	// LONGLINING: repeated set/haul cycles (a sawtooth of fast steaming and slow
	// working) along a roughly straight line. Requires GENUINE oscillation (>= 3
	// fast/slow crossings), not a single slow-down, and a stable course so a
	// zigzagging or looping vessel is excluded.
	if n >= 12 && speedCrossings >= 3 && maxSpeed >= 7.0 && minSpeed <= 2.5 && courseStd < 20.0 {
		return alert.KindLonglining, true, fmt.Sprintf("sawtooth speed, %d fast/slow cycles on a straight track", speedCrossings)
	}

	// TRAWLING: sustained slow speed (2-4.5 kn) with high course variability, the
	// real signature of towing gear (weaving, turning), not a slow straight
	// transit. Requires a nearly full history window so a brief slow-down (a ferry
	// maneuvering, a ship on approach) does not trip it, and course stddev > 25
	// degrees so straight-line traffic is excluded (CLAUDE.md P1 signature:
	// [2,5] kn AND course stddev > 25 deg).
	if n >= 12 && minSpeed >= 1.0 && maxSpeed <= 5.5 && avgSpeed >= 2.0 && avgSpeed <= 4.5 && courseStd > 25.0 {
		return alert.KindTrawling, true, fmt.Sprintf("slow 2.0-4.5 kn with course spread %.0f deg (trawl-like)", courseStd)
	}

	return "", false, ""
}
