package check

import (
	"math"
	"time"

	"palkwatch/internal/alert"
	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
	"palkwatch/internal/state"
)

// RiskRecorder receives a factor event when a check emits an alert. It is set
// only when the risk engine is on (SetRisk); nil means no scoring. Recording
// happens per alert, never per message, so the per-message hot path is unchanged
// (CLAUDE.md rule G4). nowMs is wall-clock milliseconds. The concrete
// implementation is risk.Store, kept behind this interface so check does not
// depend on the risk package.
type RiskRecorder interface {
	RecordZone(mmsi uint32, nowMs int64)
	RecordSpoof(mmsi uint32, nowMs int64)
	RecordDark(mmsi uint32, nowMs int64)
}

// Processor ties the vessel table, spatial grid, and inline checks together. It
// is shared by all workers: the grid is read-only after build and the state is
// sharded, so concurrent Process calls are safe. Per-worker scratch lives on
// Worker, not here.
type Processor struct {
	state    *state.Shards
	cold     *state.Cold
	grid     *geo.Grid
	counters *alert.Counters
	ids      *alert.IDGen
	sink     alert.Sink
	risk     RiskRecorder // nil unless the risk engine is on
}

// SetRisk wires the risk factor recorder. Call before Start; the field is then
// read-only while workers run, so no lock is needed.
func (p *Processor) SetRisk(r RiskRecorder) { p.risk = r }

// NewProcessor builds a processor. ids is shared with the dark sweeper so alert
// IDs never collide. sink receives finished alerts (use alert.Discard for
// benchmarks).
func NewProcessor(st *state.Shards, cold *state.Cold, grid *geo.Grid, counters *alert.Counters, ids *alert.IDGen, sink alert.Sink) *Processor {
	return &Processor{state: st, cold: cold, grid: grid, counters: counters, ids: ids, sink: sink}
}

// Worker is a per-goroutine handler with private scratch. Create one per worker
// via NewWorker so the grid scratch slice needs no locking.
type Worker struct {
	p       *Processor
	zoneBuf []int
}

// NewWorker returns a worker bound to the processor.
func (p *Processor) NewWorker() *Worker {
	return &Worker{p: p, zoneBuf: make([]int, 0, 8)}
}

// Handle implements ingest.BatchHandler.
func (w *Worker) Handle(batch []ingest.Message) {
	for i := range batch {
		w.handle(&batch[i])
	}
}

func (w *Worker) handle(m *ingest.Message) {
	p := w.p

	// Zones the vessel is inside right now, as a bitmask.
	w.zoneBuf = p.grid.Inside(m.Lat, m.Lon, w.zoneBuf)
	var newMask uint32
	for _, zi := range w.zoneBuf {
		newMask |= 1 << uint(zi)
	}

	prev, existed := p.state.Update(m.MMSI, func(prev state.VesselState, _ bool) state.VesselState {
		return state.VesselState{
			LastLat:            m.Lat,
			LastLon:            m.Lon,
			LastTsMs:           m.TsMs,
			LastSeenNs:         m.IngestNs,
			SpeedKn:            m.SpeedKn,
			HeadingDeg:         m.HeadingDeg,
			FlagCode:           m.FlagCode,
			ZoneMask:           newMask,
			LastFishingAlertMs: prev.LastFishingAlertMs,
		}
	})
	p.counters.Processed.Add(1)

	// Inline checks run only once a baseline fix exists, so a vessel's first
	// sighting cannot be a false transition or a false teleport.
	if existed {
		// Geofence: zones entered on this fix (outside-to-inside).
		entered := newMask &^ prev.ZoneMask
		for _, zi := range w.zoneBuf {
			if entered&(1<<uint(zi)) == 0 {
				continue
			}
			z := p.grid.Zones()[zi]
			if ZoneViolation(z.Kind, z.CountryCode, m.FlagCode) {
				p.emitZone(m, z)
			}
		}
		// Spoof: physically impossible jump from the previous fix.
		if prev.LastTsMs != 0 {
			if spoof, impliedKn := SpoofTeleport(prev.LastLat, prev.LastLon, prev.LastTsMs, m.Lat, m.Lon, m.TsMs); spoof {
				p.emitSpoof(m, impliedKn)
			}
		}
		
		// Fishing Pattern Recognition
		if kind, ok, detail := FishingPattern(&prev.History, prev.HistoryIdx); ok {
			// Debounce: 30 minutes (1,800,000 ms)
			if m.TsMs-prev.LastFishingAlertMs > 1_800_000 {
				p.emitFishing(m, kind, detail)
				// Record the alert time so we don't spam
				p.state.Update(m.MMSI, func(curr state.VesselState, _ bool) state.VesselState {
					curr.LastFishingAlertMs = m.TsMs
					return curr
				})
			}
		}
	}

	// Inline latency: ingest-to-emit for the per-message path, recorded for
	// every message so the histogram is populated regardless of alert rate.
	p.counters.InlineHist.Record(float64(ingest.NowNs()-m.IngestNs) / 1000.0)
}

func (p *Processor) emitZone(m *ingest.Message, z *geo.Zone) {
	p.counters.Alerts.Add(1)
	if p.risk != nil {
		p.risk.RecordZone(m.MMSI, time.Now().UnixMilli())
	}
	p.sink(alert.Alert{
		ID:       p.ids.Next(),
		Kind:     alert.KindZone,
		Severity: alert.SeverityHigh,
		MMSI:     m.MMSI,
		Name:     p.cold.Name(m.MMSI),
		TsMs:     m.TsMs,
		Lat:      m.Lat,
		Lon:      m.Lon,
		ZoneID:   z.ID,
		Detail:   map[string]any{"zone_kind": z.Kind},
	})
}

func (p *Processor) emitSpoof(m *ingest.Message, impliedKn float64) {
	p.counters.Alerts.Add(1)
	if p.risk != nil {
		p.risk.RecordSpoof(m.MMSI, time.Now().UnixMilli())
	}
	p.sink(alert.Alert{
		ID:       p.ids.Next(),
		Kind:     alert.KindSpoof,
		Severity: alert.SeverityHigh,
		MMSI:     m.MMSI,
		Name:     p.cold.Name(m.MMSI),
		TsMs:     m.TsMs,
		Lat:      m.Lat,
		Lon:      m.Lon,
		Detail:   map[string]any{"implied_speed_kn": math.Round(impliedKn*10) / 10},
	})
}

func (p *Processor) emitFishing(m *ingest.Message, kind string, detail string) {
	p.counters.Alerts.Add(1)
	p.sink(alert.Alert{
		ID:       p.ids.Next(),
		Kind:     kind,
		Severity: alert.SeverityHigh,
		MMSI:     m.MMSI,
		Name:     p.cold.Name(m.MMSI),
		TsMs:     m.TsMs,
		Lat:      m.Lat,
		Lon:      m.Lon,
		Detail:   map[string]any{"pattern": detail},
	})
}
