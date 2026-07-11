package check

import (
	"fmt"
	"sync/atomic"

	"palkwatch/internal/alert"
	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
	"palkwatch/internal/state"
)

// Processor ties the vessel table, spatial grid, and inline checks together. It
// is shared by all workers: the grid is read-only after build and the state is
// sharded, so concurrent Process calls are safe. Per-worker scratch lives on
// Worker, not here.
type Processor struct {
	state    *state.Shards
	cold     *state.Cold
	grid     *geo.Grid
	counters *alert.Counters
	sink     alert.Sink
	seq      atomic.Uint64
}

// NewProcessor builds a processor. sink receives finished alerts (use
// alert.Discard for benchmarks).
func NewProcessor(st *state.Shards, cold *state.Cold, grid *geo.Grid, counters *alert.Counters, sink alert.Sink) *Processor {
	return &Processor{state: st, cold: cold, grid: grid, counters: counters, sink: sink}
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
			LastLat:    m.Lat,
			LastLon:    m.Lon,
			LastTsMs:   m.TsMs,
			SpeedKn:    m.SpeedKn,
			HeadingDeg: m.HeadingDeg,
			FlagCode:   m.FlagCode,
			ZoneMask:   newMask,
		}
	})
	p.counters.Processed.Add(1)

	// Geofence: zones entered on this fix (outside-to-inside). Skip the first
	// sighting so appearing already-inside is not a false transition.
	if existed {
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
	}

	// Inline latency: ingest-to-emit for the per-message path, recorded for
	// every message so the histogram is populated regardless of alert rate.
	p.counters.InlineHist.Record(float64(ingest.NowNs()-m.IngestNs) / 1000.0)
}

func (p *Processor) emitZone(m *ingest.Message, z *geo.Zone) {
	id := p.seq.Add(1)
	p.counters.Alerts.Add(1)
	p.sink(alert.Alert{
		ID:       fmt.Sprintf("a-%06d", id),
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
