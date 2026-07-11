package risk

import "math"

// Time constants for factor scoring. A factor older than windowMs is dropped
// entirely; within the window, its contribution decays with a 24h half-life,
// computed from the stored timestamp at read time (no background mutation,
// CLAUDE.md P0). Both are wall-clock milliseconds.
const (
	halfLifeMs int64 = 24 * 60 * 60 * 1000
	windowMs   int64 = 48 * 60 * 60 * 1000
)

// decay returns the 24h-half-life multiplier for an event ageMs old: 1.0 at
// age 0, 0.5 at 24h, 0.25 at 48h. Negative ages (clock skew) clamp to 1.0.
func decay(ageMs int64) float64 {
	if ageMs <= 0 {
		return 1.0
	}
	return math.Exp2(-float64(ageMs) / float64(halfLifeMs))
}

// inWindow reports whether an event at tsMs is recent enough to count at nowMs.
func inWindow(tsMs, nowMs int64) bool {
	return tsMs != 0 && nowMs-tsMs <= windowMs
}
