// Package risk computes a per-vessel IUU (illegal, unreported, unregulated)
// fishing suspicion score from signals the engine already raises. The score is
// explainable: every point is a decayed contribution from a readable factor
// (a zone violation, a dark event, a spoof). Scoring is not a per-message cost.
// Inline checks and the dark sweep only record a factor timestamp when their
// alert fires (rare, off the hot path, CLAUDE.md rule G4); a 5-second sweep over
// the small scored active set turns those timestamps into a score and raises a
// tier-transition alert. The system never outputs "illegal: yes/no": it ranks
// suspicion with visible evidence so patrols prioritize, and authorities verify.
package risk

// Tier thresholds. 0-39 LOW, 40-64 ELEVATED, 65-84 HIGH, 85+ CRITICAL.
const (
	TierLow      = "LOW"
	TierElevated = "ELEVATED"
	TierHigh     = "HIGH"
	TierCritical = "CRITICAL"

	thHigh     = 65
	thElevated = 40
	thCritical = 85
)

// Tier maps a 0-100 score to its tier label.
func Tier(score int) string {
	switch {
	case score >= thCritical:
		return TierCritical
	case score >= thHigh:
		return TierHigh
	case score >= thElevated:
		return TierElevated
	default:
		return TierLow
	}
}

// tierRank orders tiers so the sweep can detect an upward crossing.
func tierRank(tier string) int {
	switch tier {
	case TierCritical:
		return 3
	case TierHigh:
		return 2
	case TierElevated:
		return 1
	default:
		return 0
	}
}

// crossedUpTo reports whether a transition from prev to cur is an upward
// crossing that first reaches or passes want (used to fire HIGH/CRITICAL alerts
// once, on entry, not every sweep while the vessel sits in the tier).
func crossedUpTo(prev, cur, want string) bool {
	return tierRank(cur) >= tierRank(want) && tierRank(prev) < tierRank(want)
}
