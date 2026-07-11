package risk

import "testing"

const nowMs int64 = 1_700_000_000_000

func TestTier(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{0, TierLow}, {39, TierLow},
		{40, TierElevated}, {64, TierElevated},
		{65, TierHigh}, {84, TierHigh},
		{85, TierCritical}, {100, TierCritical},
	}
	for _, c := range cases {
		if got := Tier(c.score); got != c.want {
			t.Errorf("Tier(%d) = %s, want %s", c.score, got, c.want)
		}
	}
}

func TestCrossedUpTo(t *testing.T) {
	cases := []struct {
		prev, cur, want string
		expect          bool
	}{
		{TierElevated, TierHigh, TierHigh, true},    // entered HIGH
		{TierHigh, TierHigh, TierHigh, false},       // already HIGH, no re-fire
		{TierElevated, TierCritical, TierHigh, true}, // jumped past HIGH
		{TierHigh, TierCritical, TierCritical, true}, // entered CRITICAL
		{TierCritical, TierHigh, TierHigh, false},    // dropping down, not upward
		{"", TierElevated, TierHigh, false},          // never reached HIGH
	}
	for _, c := range cases {
		if got := crossedUpTo(c.prev, c.cur, c.want); got != c.expect {
			t.Errorf("crossedUpTo(%q,%q,%q) = %v, want %v", c.prev, c.cur, c.want, got, c.expect)
		}
	}
}

// TestScoreFresh checks factor composition and tiers with age-0 events (decay
// = 1), the scenario the demo relies on: watch the evidence accumulate.
func TestScoreFresh(t *testing.T) {
	cases := []struct {
		name      string
		r         Record
		wantScore int
		wantTier  string
		wantCodes []string
	}{
		{"empty", Record{}, 0, TierLow, nil},
		{"zone only", Record{zoneTs: nowMs}, 30, TierLow, []string{CodeZone}},
		{"zone+dark", Record{zoneTs: nowMs, dark: []int64{nowMs}}, 45, TierElevated, []string{CodeZone, CodeDark}},
		{"zone+dark+spoof", Record{zoneTs: nowMs, dark: []int64{nowMs}, spoofTs: nowMs}, 60, TierElevated, []string{CodeZone, CodeDark, CodeSpoof}},
		{"zone+2dark+spoof", Record{zoneTs: nowMs, dark: []int64{nowMs, nowMs}, spoofTs: nowMs}, 70, TierHigh, []string{CodeZone, CodeDark, CodeSpoof}},
		{"zone+4dark+spoof->critical", Record{zoneTs: nowMs, dark: []int64{nowMs, nowMs, nowMs, nowMs}, spoofTs: nowMs}, 90, TierCritical, []string{CodeZone, CodeDark, CodeSpoof}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := c.r
			got, factors := score(&r, nowMs)
			if got != c.wantScore {
				t.Errorf("score = %d, want %d", got, c.wantScore)
			}
			if Tier(got) != c.wantTier {
				t.Errorf("tier = %s, want %s", Tier(got), c.wantTier)
			}
			if len(factors) != len(c.wantCodes) {
				t.Fatalf("got %d factors, want %d", len(factors), len(c.wantCodes))
			}
			for i, code := range c.wantCodes {
				if factors[i].Code != code {
					t.Errorf("factor[%d].Code = %s, want %s", i, factors[i].Code, code)
				}
			}
		})
	}
}

// TestScoreDecayAndWindow checks that a factor halves at 24h and drops out of
// the 48h window.
func TestScoreDecayAndWindow(t *testing.T) {
	// Zone violation 24h old: 30 * 0.5 = 15.
	half := Record{zoneTs: nowMs - halfLifeMs}
	if got, _ := score(&half, nowMs); got != 15 {
		t.Errorf("24h-old zone score = %d, want 15", got)
	}
	// Zone violation older than the 48h window: dropped entirely.
	old := Record{zoneTs: nowMs - windowMs - 1}
	if got, factors := score(&old, nowMs); got != 0 || len(factors) != 0 {
		t.Errorf("aged-out zone score = %d with %d factors, want 0/0", got, len(factors))
	}
}
