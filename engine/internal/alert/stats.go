package alert

import (
	"math"
	"sync/atomic"
)

// Counters holds the live engine telemetry. All counters are cumulative and
// updated with atomics (hot-path rule 4: no per-message logging, aggregate with
// atomic counters instead).
type Counters struct {
	Ingested   atomic.Uint64
	Processed  atomic.Uint64
	Dropped    atomic.Uint64
	Alerts     atomic.Uint64
	InlineHist Hist
	SweepHist  Hist
	// Risk-engine gauges (P0), written by the 5s risk sweep. RiskSweepUs is the
	// last sweep's duration in microseconds; Scored is the size of the scored
	// active set. Both stay 0 when the risk engine is off.
	RiskSweepUs atomic.Uint64
	Scored      atomic.Int64
}

// NewCounters returns zeroed counters with initialized histograms.
func NewCounters() *Counters {
	return &Counters{}
}

// Snapshot builds a Metrics contract frame from the current counters. ratePerS
// and activeVessels are supplied by the caller (rate is a per-second delta).
func (c *Counters) Snapshot(ratePerS float64, activeVessels int) Metrics {
	return Metrics{
		Type:           "metrics",
		IngestedTotal:  c.Ingested.Load(),
		ProcessedTotal: c.Processed.Load(),
		DroppedTotal:   c.Dropped.Load(),
		RatePerS:       ratePerS,
		LatencyUs: LatencyStats{
			InlineP50: c.InlineHist.Percentile(50),
			InlineP99: c.InlineHist.Percentile(99),
			SweepP50:  c.SweepHist.Percentile(50),
			SweepP99:  c.SweepHist.Percentile(99),
		},
		ActiveVessels: activeVessels,
		AlertsTotal:   c.Alerts.Load(),
		RiskSweepUs:   float64(c.RiskSweepUs.Load()),
		ScoredVessels: int(c.Scored.Load()),
	}
}

// histBuckets covers 1 microsecond up to ~2 seconds in power-of-two steps.
const histBuckets = 32

// Hist is a lock-free log2 latency histogram in microseconds. Bucket i counts
// samples in [2^i, 2^(i+1)) us. Coarse by a factor of two, which is plenty for
// a p50/p99 HUD and honest enough for the bench.
type Hist struct {
	buckets [histBuckets]atomic.Uint64
}

// Record adds one sample given in microseconds.
func (h *Hist) Record(us float64) {
	i := 0
	if us >= 1 {
		i = int(math.Floor(math.Log2(us)))
	}
	if i < 0 {
		i = 0
	}
	if i >= histBuckets {
		i = histBuckets - 1
	}
	h.buckets[i].Add(1)
}

// Percentile returns the approximate p-th percentile in microseconds. It
// returns the geometric midpoint of the containing bucket to reduce bias.
func (h *Hist) Percentile(p float64) float64 {
	var counts [histBuckets]uint64
	var total uint64
	for i := 0; i < histBuckets; i++ {
		counts[i] = h.buckets[i].Load()
		total += counts[i]
	}
	if total == 0 {
		return 0
	}
	target := uint64(p / 100.0 * float64(total))
	var cum uint64
	for i := 0; i < histBuckets; i++ {
		cum += counts[i]
		if cum >= target {
			return math.Ldexp(1.5, i) // 2^i * 1.5, midpoint of [2^i, 2^(i+1))
		}
	}
	return math.Ldexp(1.5, histBuckets-1)
}
