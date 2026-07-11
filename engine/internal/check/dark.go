package check

import (
	"palkwatch/internal/alert"
	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
	"palkwatch/internal/state"
)

// Dark-event parameters (CLAUDE.md alert taxonomy).
const (
	// SilenceMultiplier is the default (synthetic) threshold: a vessel is dark
	// when its silence exceeds this multiple of the expected reporting interval
	// for its last speed class. The real feed uses RealFeedSilenceMultiplier.
	SilenceMultiplier = 6.0
	// RealFeedSilenceMultiplier is the higher threshold used on the live feed,
	// where community AIS coverage gaps are routine: 10x keeps ordinary gaps
	// from masquerading as dark events (CLAUDE.md: "6x synthetic, 10x real feed").
	RealFeedSilenceMultiplier = 10.0
	// DarkMinSpeedKn: a vessel must have been moving to be a dark event. An
	// anchored vessel going quiet is not a crime.
	DarkMinSpeedKn = 1.0
	// DarkNearNm: last position must be within this distance of a zone of
	// interest, so open-ocean coverage gaps never alert.
	DarkNearNm = 5.0
	// coneR0M is the GPS tolerance floor of the dead-reckoning radius.
	coneR0M = 100.0
	// coneSpreadDeg is the fixed angular spread of the cone.
	coneSpreadDeg = 30.0
)

// Sweeper detects dark events on a 1s tick. Detecting absence is inherently
// sweep-based, not inline: the alert is bounded by the 1s tick plus the silence
// threshold, never claimed as millisecond latency.
type Sweeper struct {
	state    *state.Shards
	cold     *state.Cold
	zones    []*geo.Zone
	patrols  []geo.Patrol
	counters *alert.Counters
	ids      *alert.IDGen
	sink     alert.Sink

	// silenceMult is the silence threshold multiplier (6x synthetic default,
	// 10x on the live feed via SetSilenceMultiplier).
	silenceMult float64

	// fired records the LastSeenNs at which we last alerted for an MMSI, so a
	// dark vessel fires once per silence episode, not every tick. Owned solely
	// by the sweeper goroutine; no lock needed.
	fired map[uint32]int64
}

// NewSweeper builds a sweeper. ids is shared with the inline Processor. Only
// non-EEZ zones (the small protected areas) count as zones of interest for the
// proximity filter: the EEZ spans the whole region, so treating it as "of
// interest" would defeat the filter that keeps open-ocean gaps from alerting.
func NewSweeper(st *state.Shards, cold *state.Cold, zones []*geo.Zone, patrols []geo.Patrol, counters *alert.Counters, ids *alert.IDGen, sink alert.Sink) *Sweeper {
	interest := make([]*geo.Zone, 0, len(zones))
	for _, z := range zones {
		if z.Kind != geo.KindEEZ {
			interest = append(interest, z)
		}
	}
	if len(interest) == 0 {
		interest = zones // fallback: if only EEZ zones are configured, use them
	}
	return &Sweeper{
		state:       st,
		cold:        cold,
		zones:       interest,
		patrols:     patrols,
		counters:    counters,
		ids:         ids,
		sink:        sink,
		silenceMult: SilenceMultiplier,
		fired:       make(map[uint32]int64),
	}
}

// SetSilenceMultiplier overrides the dark-event silence threshold. Call before
// the sweeper's first Tick (the live feed sets RealFeedSilenceMultiplier).
func (s *Sweeper) SetSilenceMultiplier(m float64) { s.silenceMult = m }

type darkHit struct {
	mmsi     uint32
	v        state.VesselState
	silenceS float64
}

// Tick runs one sweep at the given monotonic time. Candidates are collected
// under the shard read locks, then emitted after the locks are released.
func (s *Sweeper) Tick(nowNs int64) {
	start := ingest.NowNs()

	var hits []darkHit
	s.state.ForEach(func(mmsi uint32, v state.VesselState) {
		if float64(v.SpeedKn) <= DarkMinSpeedKn {
			return
		}
		silenceS := float64(nowNs-v.LastSeenNs) / 1e9
		if silenceS <= s.silenceMult*expectedIntervalS(v.SpeedKn) {
			return
		}
		if !s.nearZone(v.LastLat, v.LastLon) {
			return
		}
		hits = append(hits, darkHit{mmsi: mmsi, v: v, silenceS: silenceS})
	})

	for _, h := range hits {
		// Distinguish "never fired" (absent) from "fired at this LastSeenNs" so a
		// LastSeenNs of 0 is not mistaken for the map's zero value.
		if last, ok := s.fired[h.mmsi]; ok && last == h.v.LastSeenNs {
			continue // already alerted for this silence episode
		}
		s.fired[h.mmsi] = h.v.LastSeenNs
		s.emit(h)
	}

	s.counters.SweepHist.Record(float64(ingest.NowNs()-start) / 1000.0)
}

// expectedIntervalS is the AIS reporting interval by speed class, in seconds.
// Anchored (180s) is unreachable here because a dark event requires speed > 1kn.
func expectedIntervalS(speedKn float32) float64 {
	switch {
	case speedKn <= 14:
		return 10
	case speedKn <= 23:
		return 6
	default:
		return 2
	}
}

func (s *Sweeper) nearZone(lat, lon float64) bool {
	for _, z := range s.zones {
		if z.DistanceNm(lat, lon) <= DarkNearNm {
			return true
		}
	}
	return false
}

func (s *Sweeper) nearestZoneID(lat, lon float64) string {
	best := ""
	min := -1.0
	for _, z := range s.zones {
		d := z.DistanceNm(lat, lon)
		if min < 0 || d < min {
			min = d
			best = z.ID
		}
	}
	return best
}

func (s *Sweeper) emit(h darkHit) {
	v := h.v
	speedMPS := geo.KnotsToMPS(float64(v.SpeedKn))

	// Dead-reckon the likely current position from the last fix, and size the
	// uncertainty radius per rule 5: r(t) = r0 + 0.1 * lastSpeed * dt.
	reachM := speedMPS * h.silenceS
	predLat, predLon := geo.DeadReckon(v.LastLat, v.LastLon, float64(v.HeadingDeg), reachM)
	radiusM := coneR0M + 0.1*speedMPS*h.silenceS

	cone := &alert.Cone{
		Lat:        v.LastLat,
		Lon:        v.LastLon,
		HeadingDeg: float64(v.HeadingDeg),
		SpreadDeg:  coneSpreadDeg,
		RadiusM:    round1(radiusM),
	}

	intercepts := make([]alert.Intercept, 0, len(s.patrols))
	for _, p := range s.patrols {
		ic := geo.SolveIntercept(p, predLat, predLon, float64(v.SpeedKn), float64(v.HeadingDeg))
		intercepts = append(intercepts, alert.Intercept{
			PatrolID:   ic.PatrolID,
			HeadingDeg: round1(ic.HeadingDeg),
			EtaS:       round1(ic.EtaS),
			Feasible:   ic.Feasible,
		})
	}

	s.counters.Alerts.Add(1)
	s.sink(alert.Alert{
		ID:         s.ids.Next(),
		Kind:       alert.KindDark,
		Severity:   alert.SeverityCritical,
		MMSI:       h.mmsi,
		Name:       s.cold.Name(h.mmsi),
		TsMs:       v.LastTsMs,
		Lat:        v.LastLat,
		Lon:        v.LastLon,
		ZoneID:     s.nearestZoneID(v.LastLat, v.LastLon),
		Detail:     map[string]any{"silence_s": round1(h.silenceS)},
		Cone:       cone,
		Intercepts: intercepts,
	})
}

func round1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}
