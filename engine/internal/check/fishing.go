package check

import (
	"math"

	"palkwatch/internal/alert"
	"palkwatch/internal/state"
)

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
	var sumAbsCogDelta float64
	var maxCogDelta float64

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

			// Shortest angular distance
			delta := currCog - prevCog
			for delta > 180 {
				delta -= 360
			}
			for delta < -180 {
				delta += 360
			}
			absDelta := math.Abs(delta)
			sumAbsCogDelta += absDelta
			if absDelta > maxCogDelta {
				maxCogDelta = absDelta
			}
		}
	}

	avgSpeed := sumSpeed / float64(n)
	recentSpeed := float64(fixes[n-1].SpeedKn)
	firstSpeed := float64(fixes[0].SpeedKn)

	// TRAWLING: consistently slow (2-5 kn), low speed variance, gradual heading.
	if minSpeed >= 1.5 && maxSpeed <= 5.5 && avgSpeed >= 2.0 && avgSpeed <= 4.5 {
		if maxCogDelta < 45.0 {
			return alert.KindTrawling, true, "SOG consistently 2.0-4.5 kn"
		}
	}

	// LONGLINING: erratic sawtooth speed (oscillates > 7.5 to < 2.5), course stable.
	if maxSpeed >= 7.5 && minSpeed <= 2.5 && sumAbsCogDelta < 120.0 {
		// To ensure it's oscillating and not just a single slow-down, check variance or 
		// just rely on the min/max spread within a short window.
		return alert.KindLonglining, true, "Sawtooth SOG pattern detected"
	}

	// PURSE SEINING: 360 Loop (cumulative COG > 270), drops to < 2.5 kn.
	if sumAbsCogDelta > 270.0 && maxSpeed > 7.0 && recentSpeed <= 2.5 && firstSpeed > 5.0 {
		return alert.KindPurseSeining, true, "360 Loop followed by dead stop"
	}

	return "", false, ""
}
