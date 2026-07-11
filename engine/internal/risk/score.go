package risk

import (
	"math"
	"sync"

	"palkwatch/internal/alert"
)

// Factor codes and base points (P0 uses only signals the engine already fires).
const (
	CodeZone  = "ZONE"
	CodeDark  = "DARK"
	CodeSpoof = "SPOOF"

	ptsZone      = 30 // recent protected-zone violation
	ptsSpoof     = 15 // recent position spoof / teleport
	ptsDarkFirst = 15 // first dark event in the window
	ptsDarkMore  = 10 // each additional dark event in the window

	maxDarkKept = 16 // bound per-vessel dark history (a chronic offender)
	maxScore    = 100
)

// Record is one vessel's factor history plus the last sweep's cached score. It
// lives only in the Store (the scored active set), never in the hot VesselState
// shard, so it is free to hold a slice (CLAUDE.md rule 1 governs the hot struct).
type Record struct {
	zoneTs  int64   // wall ms of the last zone violation (0 = none)
	spoofTs int64   // wall ms of the last spoof
	dark    []int64 // wall ms of recent dark events, pruned to the window

	// Cached by the 5s sweep for the position broadcaster and the drawer.
	lastTier   string
	curScore   int
	curTier    string
	curFactors []alert.Factor
}

// Store is the scored active set: only vessels with at least one factor event
// appear here, so the 5s sweep is over a small map, not the whole vessel table.
type Store struct {
	mu sync.Mutex
	m  map[uint32]*Record
}

// NewStore returns an empty store.
func NewStore() *Store { return &Store{m: make(map[uint32]*Record)} }

func (s *Store) rec(mmsi uint32) *Record {
	r := s.m[mmsi]
	if r == nil {
		r = &Record{}
		s.m[mmsi] = r
	}
	return r
}

// RecordZone notes a zone violation for mmsi at wall time nowMs. Called from the
// inline geofence check only when it emits (per alert, not per message).
func (s *Store) RecordZone(mmsi uint32, nowMs int64) {
	s.mu.Lock()
	s.rec(mmsi).zoneTs = nowMs
	s.mu.Unlock()
}

// RecordSpoof notes a spoof for mmsi at wall time nowMs.
func (s *Store) RecordSpoof(mmsi uint32, nowMs int64) {
	s.mu.Lock()
	s.rec(mmsi).spoofTs = nowMs
	s.mu.Unlock()
}

// RecordDark notes a dark event for mmsi at wall time nowMs.
func (s *Store) RecordDark(mmsi uint32, nowMs int64) {
	s.mu.Lock()
	r := s.rec(mmsi)
	r.dark = append(r.dark, nowMs)
	if len(r.dark) > maxDarkKept {
		r.dark = r.dark[len(r.dark)-maxDarkKept:]
	}
	s.mu.Unlock()
}

// Snapshot returns the last sweep's cached score, tier and factor breakdown for
// mmsi (for the position feed and the Score Breakdown Drawer). ok is false when
// the vessel is not in the scored set.
func (s *Store) Snapshot(mmsi uint32) (int, string, []alert.Factor, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.m[mmsi]
	if r == nil {
		return 0, "", nil, false
	}
	return r.curScore, r.curTier, r.curFactors, true
}

// score computes the current 0-100 score and its factor breakdown for r at
// nowMs, pruning dark events that have aged out of the window. Every factor's
// points are decayed with a 24h half-life; the score is the sum, clamped to 100.
// Callers hold s.mu.
func score(r *Record, nowMs int64) (int, []alert.Factor) {
	factors := make([]alert.Factor, 0, 3)
	total := 0

	// ZONE: a recent protected-zone violation.
	if inWindow(r.zoneTs, nowMs) {
		p := roundPts(ptsZone, decay(nowMs-r.zoneTs))
		if p > 0 {
			factors = append(factors, alert.Factor{Code: CodeZone, Label: "Protected-zone violation", Points: p, TsMs: r.zoneTs})
			total += p
		}
	}

	// DARK: count recent dark events; first is worth more, each additional adds.
	// Decay by the most recent event so a fresh disappearance dominates.
	if kept := pruneDark(r, nowMs); len(kept) > 0 {
		base := ptsDarkFirst + ptsDarkMore*(len(kept)-1)
		last := kept[len(kept)-1]
		p := roundPts(base, decay(nowMs-last))
		if p > 0 {
			label := "Went dark near protected zone"
			if len(kept) > 1 {
				label = "Repeated dark events near protected zone"
			}
			factors = append(factors, alert.Factor{Code: CodeDark, Label: label, Points: p, TsMs: last})
			total += p
		}
	}

	// SPOOF: a recent position spoof / teleport.
	if inWindow(r.spoofTs, nowMs) {
		p := roundPts(ptsSpoof, decay(nowMs-r.spoofTs))
		if p > 0 {
			factors = append(factors, alert.Factor{Code: CodeSpoof, Label: "Position spoof / teleport", Points: p, TsMs: r.spoofTs})
			total += p
		}
	}

	if total > maxScore {
		total = maxScore
	}
	return total, factors
}

// pruneDark drops dark timestamps outside the window and returns the survivors.
func pruneDark(r *Record, nowMs int64) []int64 {
	if len(r.dark) == 0 {
		return nil
	}
	kept := r.dark[:0]
	for _, ts := range r.dark {
		if nowMs-ts <= windowMs {
			kept = append(kept, ts)
		}
	}
	r.dark = kept
	return kept
}

// roundPts rounds a base weight scaled by its decay multiplier to whole points.
func roundPts(base int, mult float64) int {
	return int(math.Round(float64(base) * mult))
}
