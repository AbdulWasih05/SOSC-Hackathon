package check

import (
	"math"
	"testing"

	"github.com/paulmach/orb"

	"palkwatch/internal/alert"
	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
	"palkwatch/internal/state"
)

// mpaZone is a square MPA covering the Gulf of Mannar demo box.
func mpaZone() *geo.Zone {
	poly := orb.Polygon{{{79.0, 8.9}, {79.3, 8.9}, {79.3, 9.3}, {79.0, 9.3}, {79.0, 8.9}}}
	return geo.NewZone("gom-mnp", "Gulf of Mannar MNP", geo.KindMPA, geo.CountryUnknown, poly)
}

// newSweeperWith builds a sweeper over one MPA and one patrol, with a collecting
// sink, and injects one vessel state.
func newSweeperWith(t *testing.T, v state.VesselState) (*Sweeper, *[]alert.Alert, uint32) {
	t.Helper()
	st := state.New()
	const mmsi = uint32(419000123)
	st.Update(mmsi, func(state.VesselState, bool) state.VesselState { return v })

	cold := state.NewCold()
	cold.SetName(mmsi, "TEST TRAWLER")
	patrols := []geo.Patrol{{ID: "P1", Lat: 9.28, Lon: 79.30, MaxSpeedKn: 25}}

	got := &[]alert.Alert{}
	sink := func(a alert.Alert) { *got = append(*got, a) }
	sw := NewSweeper(st, cold, []*geo.Zone{mpaZone()}, patrols, alert.NewCounters(), alert.NewIDGen(), sink)
	return sw, got, mmsi
}

func TestDarkEventFires(t *testing.T) {
	// Moving (8 kn) vessel inside the MPA, last seen at t=0.
	v := state.VesselState{LastLat: 9.05, LastLon: 79.15, LastTsMs: 1_720_000_000_000, LastSeenNs: 0, SpeedKn: 8, HeadingDeg: 210, ZoneMask: 1}
	sw, got, mmsi := newSweeperWith(t, v)

	sw.Tick(100 * int64(1e9)) // 100 s of silence; threshold is 6*10 = 60 s

	if len(*got) != 1 {
		t.Fatalf("want 1 dark alert, got %d", len(*got))
	}
	a := (*got)[0]
	if a.Kind != alert.KindDark || a.Severity != alert.SeverityCritical {
		t.Fatalf("kind/severity = %s/%s, want DARK_EVENT/CRITICAL", a.Kind, a.Severity)
	}
	if a.MMSI != mmsi || a.Name != "TEST TRAWLER" {
		t.Fatalf("mmsi/name = %d/%q", a.MMSI, a.Name)
	}
	if a.Cone == nil {
		t.Fatalf("dark alert must carry a cone")
	}
	if a.Cone.HeadingDeg != 210 || a.Cone.SpreadDeg != 30 {
		t.Fatalf("cone heading/spread = %.0f/%.0f, want 210/30", a.Cone.HeadingDeg, a.Cone.SpreadDeg)
	}
	wantRadius := coneR0M + 0.1*geo.KnotsToMPS(8)*100
	if math.Abs(a.Cone.RadiusM-wantRadius) > 0.5 {
		t.Fatalf("cone radius = %.1f, want ~%.1f", a.Cone.RadiusM, wantRadius)
	}
	if len(a.Intercepts) != 1 || a.Intercepts[0].PatrolID != "P1" {
		t.Fatalf("want one intercept for P1, got %+v", a.Intercepts)
	}
	if a.Intercepts[0].EtaS <= 0 {
		t.Fatalf("intercept eta = %.1f, want > 0", a.Intercepts[0].EtaS)
	}
}

func TestDarkEventDedupPerEpisode(t *testing.T) {
	v := state.VesselState{LastLat: 9.05, LastLon: 79.15, LastSeenNs: 0, SpeedKn: 8, HeadingDeg: 210, ZoneMask: 1}
	sw, got, _ := newSweeperWith(t, v)

	sw.Tick(100 * int64(1e9))
	sw.Tick(101 * int64(1e9)) // same silence episode (LastSeenNs unchanged)

	if len(*got) != 1 {
		t.Fatalf("want 1 alert across two ticks of one episode, got %d", len(*got))
	}
}

func TestDarkEventNegatives(t *testing.T) {
	tests := []struct {
		name string
		v    state.VesselState
	}{
		{"anchored (speed <= 1kn)", state.VesselState{LastLat: 9.05, LastLon: 79.15, SpeedKn: 0.5}},
		{"open ocean (far from any zone)", state.VesselState{LastLat: 5.0, LastLon: 85.0, SpeedKn: 8}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sw, got, _ := newSweeperWith(t, tt.v)
			sw.Tick(100 * int64(1e9))
			if len(*got) != 0 {
				t.Fatalf("want no alert, got %d", len(*got))
			}
		})
	}
}

// TestEngineDarkFlow drives the real path: a message flows through the worker
// (setting LastSeenNs), the vessel then goes silent, and the sweeper raises the
// dark event. No hand-injected state.
func TestEngineDarkFlow(t *testing.T) {
	st := state.New()
	cold := state.NewCold()
	grid := geo.NewGrid([]*geo.Zone{mpaZone()}, geo.CellDeg)
	counters := alert.NewCounters()
	ids := alert.NewIDGen()
	patrols := []geo.Patrol{{ID: "P1", Lat: 9.28, Lon: 79.30, MaxSpeedKn: 25}}

	var got []alert.Alert
	sink := func(a alert.Alert) { got = append(got, a) }

	proc := NewProcessor(st, cold, grid, counters, ids, sink)
	sweeper := NewSweeper(st, cold, []*geo.Zone{mpaZone()}, patrols, counters, ids, sink)

	seen := ingest.NowNs()
	msg := ingest.Message{MMSI: 419000777, Lat: 9.05, Lon: 79.15, SpeedKn: 8, HeadingDeg: 210, TsMs: 1_720_000_000_000, IngestNs: seen}
	proc.NewWorker().Handle([]ingest.Message{msg})

	if len(got) != 0 {
		t.Fatalf("first sighting must not alert, got %d", len(got))
	}

	sweeper.Tick(seen + 100*int64(1e9)) // 100 s later, still silent

	if len(got) != 1 || got[0].Kind != alert.KindDark {
		t.Fatalf("want one DARK_EVENT after silence, got %+v", got)
	}
	if got[0].Cone == nil || len(got[0].Intercepts) != 1 {
		t.Fatalf("dark alert missing cone or intercept: %+v", got[0])
	}
}

func TestDarkEventNotYetSilent(t *testing.T) {
	// 8 kn -> expected 10 s -> threshold 60 s. At 30 s it is not yet dark.
	v := state.VesselState{LastLat: 9.05, LastLon: 79.15, LastSeenNs: 0, SpeedKn: 8}
	sw, got, _ := newSweeperWith(t, v)
	sw.Tick(30 * int64(1e9))
	if len(*got) != 0 {
		t.Fatalf("want no alert before threshold, got %d", len(*got))
	}
}
