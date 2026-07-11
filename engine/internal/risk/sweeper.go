package risk

import (
	"math"

	"palkwatch/internal/alert"
	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
	"palkwatch/internal/state"
)

// Sweeper scores the active set on a 5-second tick and raises a tier-transition
// alert when a vessel first crosses into HIGH (ILLEGAL_FISHING_SUSPECTED) or
// CRITICAL (BOARDING_RECOMMENDED). Scoring is by design a judgment cycle, not a
// per-message reflex: the sweep reads factor timestamps the checks recorded and
// runs entirely off the hot path. The alert is never a millisecond claim.
type Sweeper struct {
	store    *Store
	state    *state.Shards
	cold     *state.Cold
	patrols  []geo.Patrol
	counters *alert.Counters
	ids      *alert.IDGen
	sink     alert.Sink
}

// NewSweeper builds a risk sweeper. ids is shared with the other alert sources so
// alert IDs never collide; sink is the same dashboard websocket seam.
func NewSweeper(store *Store, st *state.Shards, cold *state.Cold, patrols []geo.Patrol, counters *alert.Counters, ids *alert.IDGen, sink alert.Sink) *Sweeper {
	return &Sweeper{store: store, state: st, cold: cold, patrols: patrols, counters: counters, ids: ids, sink: sink}
}

// transition is a queued tier-crossing to emit after the store lock is released.
type transition struct {
	mmsi     uint32
	kind     string
	severity string
	score    int
	factors  []alert.Factor
}

// Tick scores every vessel in the active set at wall time nowMs, caches the
// result for the position feed and drawer, and queues any upward tier crossing.
// Alerts are emitted after the lock is released. nowMs is wall-clock ms.
func (rs *Sweeper) Tick(nowMs int64) {
	start := ingest.NowNs()

	var out []transition
	rs.store.mu.Lock()
	for mmsi, r := range rs.store.m {
		sc, factors := score(r, nowMs)
		r.curScore = sc
		r.curTier = Tier(sc)
		r.curFactors = factors
		switch {
		case crossedUpTo(r.lastTier, r.curTier, TierCritical):
			out = append(out, transition{mmsi, alert.KindBoarding, alert.SeverityCritical, sc, factors})
		case crossedUpTo(r.lastTier, r.curTier, TierHigh):
			out = append(out, transition{mmsi, alert.KindIllegalSuspected, alert.SeverityHigh, sc, factors})
		}
		r.lastTier = r.curTier
	}
	n := len(rs.store.m)
	rs.store.mu.Unlock()

	for _, t := range out {
		rs.emit(nowMs, t)
	}

	rs.counters.Scored.Store(int64(n))
	rs.counters.RiskSweepUs.Store(uint64((ingest.NowNs() - start) / 1000))
}

// emit sends one tier-transition alert. BOARDING_RECOMMENDED carries the live
// intercept solutions for each patrol asset; both kinds carry the score and the
// full factor breakdown so the dashboard shows the evidence, not a verdict.
func (rs *Sweeper) emit(nowMs int64, t transition) {
	v, ok := rs.state.Load(t.mmsi)
	a := alert.Alert{
		ID:       rs.ids.Next(),
		Kind:     t.kind,
		Severity: t.severity,
		MMSI:     t.mmsi,
		Name:     rs.cold.Name(t.mmsi),
		TsMs:     nowMs,
		Detail:   map[string]any{"tier": Tier(t.score)},
		Score:    t.score,
		Factors:  t.factors,
	}
	if ok {
		a.Lat = v.LastLat
		a.Lon = v.LastLon
	}
	if t.kind == alert.KindBoarding && ok {
		a.Intercepts = make([]alert.Intercept, 0, len(rs.patrols))
		for _, p := range rs.patrols {
			ic := geo.SolveIntercept(p, v.LastLat, v.LastLon, float64(v.SpeedKn), float64(v.HeadingDeg))
			a.Intercepts = append(a.Intercepts, alert.Intercept{
				PatrolID:   ic.PatrolID,
				HeadingDeg: round1(ic.HeadingDeg),
				EtaS:       round1(ic.EtaS),
				Feasible:   ic.Feasible,
			})
		}
	}
	rs.counters.Alerts.Add(1)
	rs.sink(a)
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }
