// Command palkwatch is the single Palk Watch binary. The producer is swappable;
// the engine is the same everywhere. Modes: -csv replays a recorded Danish AIS
// (aisdk) file (the offline demo: real data, reproducible, no network); the
// default runs the live aisstream.io feed (North Sea); -firehose runs the
// in-memory stress feed (the 50k benchmark theatre); -scenario plays a scripted
// timeline; -fake runs the contract emitter with no engine. Every mode feeds the
// same pipeline: ingest -> state -> inline checks (geofence, spoof) -> alert bus,
// plus the 1s dark sweep, broadcasting metrics once per second and alerts as they
// fire over the dashboard websocket.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"palkwatch/internal/alert"
	"palkwatch/internal/api"
	"palkwatch/internal/check"
	"palkwatch/internal/gen"
	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
	"palkwatch/internal/state"
)

func main() {
	var (
		fake     bool
		firehose bool
		addr     string
		zones    string
		patrol   string
		scenario string
		csv      string
		speed    float64
		total    int
	)
	flag.BoolVar(&fake, "fake", false, "run the contract emitter (schema-valid alerts + metrics, no real engine)")
	flag.BoolVar(&firehose, "firehose", false, "run the in-memory firehose (Act 3 stress feed) instead of the live feed")
	flag.StringVar(&addr, "addr", ":8080", "http listen address for the dashboard websocket")
	flag.StringVar(&zones, "zones", "", "zone geojson path (default follows the mode's region)")
	flag.StringVar(&patrol, "patrol", "", "patrol config path (default follows the mode's region)")
	flag.StringVar(&scenario, "scenario", "", "scenario json path (selects scenario mode)")
	flag.StringVar(&csv, "csv", "", "aisdk CSV path to replay (selects the recorded-data demo mode)")
	flag.Float64Var(&speed, "speed", 30, "aisdk replay speed (event seconds per wall second)")
	flag.IntVar(&total, "n", 1_000_000, "firehose messages pre-generated and looped")
	flag.Parse()

	// Mode selection (precedence): -fake, -scenario, -csv, -firehose, else the
	// default live aisstream.io feed. Zone/patrol defaults follow the mode region:
	// Denmark for the CSV replay, North Sea for the live feed, Gulf of Mannar for
	// the scripted scenario / firehose.
	csvMode := csv != ""
	live := !firehose && !fake && !csvMode && scenario == ""
	if zones == "" {
		switch {
		case csvMode:
			zones = "data/zones-denmark.geojson"
		case live:
			zones = "data/zones-northsea.geojson"
		default:
			zones = "data/zones.geojson"
		}
	}
	if patrol == "" {
		switch {
		case csvMode:
			patrol = "data/patrol-denmark.json"
		case live:
			patrol = "data/patrol-northsea.json"
		default:
			patrol = "data/patrol.json"
		}
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	hub := api.NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.HandleWS)
	// Static config for the dashboard: the map loads zones and patrol markers
	// from the same binary, so there is one source of truth.
	mux.HandleFunc("/zones", serveJSONFile(zones))
	mux.HandleFunc("/patrols", serveJSONFile(patrol))
	srv := &http.Server{Addr: addr, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("addr", addr).Msg("dashboard websocket listening on /ws")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server failed")
		}
	}()

	switch {
	case fake:
		log.Info().Msg("fake emitter on: schema-valid alerts + metrics at demo rates")
		go gen.RunFake(ctx, hub)
	case scenario != "":
		go runScenario(ctx, hub, zones, patrol, scenario)
	case csvMode:
		go runAISDK(ctx, hub, zones, patrol, csv, speed)
	case firehose:
		go runFirehose(ctx, hub, zones, patrol, total)
	default:
		go runLive(ctx, hub, zones, patrol)
	}

	<-ctx.Done()
	log.Info().Msg("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}

// serveJSONFile serves a JSON/GeoJSON file with permissive CORS so the dashboard
// (served from a dev server on another port) can fetch it.
func serveJSONFile(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, path)
	}
}

// engine bundles the built pieces shared by firehose and scenario modes.
type engine struct {
	st       *state.Shards
	cold     *state.Cold
	counters *alert.Counters
	pipe     *ingest.Pipeline
	proc     *check.Processor
	sweeper  *check.Sweeper
	alertCh  chan alert.Alert
}

// buildEngine loads config and wires state, checks, and the worker pool. It does
// not start a producer. Returns nil on config error (already logged).
func buildEngine(zonesPath, patrolPath string) *engine {
	zs, err := geo.LoadZones(zonesPath)
	if err != nil {
		log.Error().Err(err).Str("path", zonesPath).Msg("failed to load zones; engine not started")
		return nil
	}
	patrols, err := geo.LoadPatrols(patrolPath)
	if err != nil {
		log.Error().Err(err).Str("path", patrolPath).Msg("failed to load patrols; engine not started")
		return nil
	}
	grid := geo.NewGrid(zs, geo.CellDeg)
	st := state.New()
	cold := state.NewCold()
	counters := alert.NewCounters()
	ids := alert.NewIDGen()

	// Alerts can fire faster than a human dashboard can read; forward them to a
	// small buffered channel and drop when full. counters.Alerts stays exact;
	// only the on-screen feed is sampled.
	alertCh := make(chan alert.Alert, 256)
	sink := func(a alert.Alert) {
		select {
		case alertCh <- a:
		default:
		}
	}
	proc := check.NewProcessor(st, cold, grid, counters, ids, sink)
	sweeper := check.NewSweeper(st, cold, zs, patrols, counters, ids, sink)

	workers := runtime.GOMAXPROCS(0)
	pipe := ingest.New(counters, workers, workers*2)
	pipe.Start(func() ingest.BatchHandler { return proc.NewWorker() })

	return &engine{st: st, cold: cold, counters: counters, pipe: pipe, proc: proc, sweeper: sweeper, alertCh: alertCh}
}

// startTelemetry launches the metrics/alert broadcaster, the vessel position
// feed, and the dark sweep.
func (e *engine) startTelemetry(ctx context.Context, hub *api.Hub) {
	go broadcast(ctx, hub, e.counters, e.st, e.alertCh)
	go broadcastPositions(ctx, hub, e.st)
	go runSweeper(ctx, e.sweeper)
}

// runFirehose runs the engine against the in-memory firehose (Act 3).
func runFirehose(ctx context.Context, hub *api.Hub, zonesPath, patrolPath string, total int) {
	e := buildEngine(zonesPath, patrolPath)
	if e == nil {
		return
	}
	msgs := gen.Firehose(total)
	seedNames(e.cold, msgs)
	log.Info().Int("messages", len(msgs)).Msg("engine running: firehose mode")
	e.startTelemetry(ctx, hub)
	e.pipe.RunFirehoseCtx(ctx, msgs)
	e.pipe.Wait()
}

// runScenario runs the engine against a scripted scenario timeline (Act 2),
// feeding messages through the 5ms-flush source path at demo rates.
func runScenario(ctx context.Context, hub *api.Hub, zonesPath, patrolPath, scenarioPath string) {
	sc, err := gen.LoadScenario(scenarioPath)
	if err != nil {
		log.Error().Err(err).Str("path", scenarioPath).Msg("failed to load scenario; engine not started")
		return
	}
	e := buildEngine(zonesPath, patrolPath)
	if e == nil {
		return
	}
	for _, v := range sc.Vessels {
		e.cold.SetName(v.MMSI, v.Name)
	}
	log.Info().Str("scenario", sc.Name).Int("frames", len(sc.Frames)).Msg("engine running: scenario mode")

	e.startTelemetry(ctx, hub)
	src := make(chan ingest.Message)
	go func() {
		sc.Play(ctx, src)
		close(src)
	}()
	e.pipe.RunSource(src)
	e.pipe.Wait()
}

// northSeaBox is the aisstream.io subscription bounding box for the live feed:
// the Dutch / southern North Sea, where community AIS coverage is densest.
// aisstream orders coordinates [lat, lon] (not GeoJSON order).
var northSeaBox = [][]float64{{51.0, 2.0}, {54.0, 7.0}}

// runLive runs the engine against the live aisstream.io feed. Real vessels flow
// through the same ingest -> checks -> alert path as the synthetic modes; the
// only difference is the producer. Volume is a real regional rate (tens/sec),
// far below the firehose; the 50k throughput floor is proven by the benchmark,
// not this mode.
func runLive(ctx context.Context, hub *api.Hub, zonesPath, patrolPath string) {
	key := gen.LoadAPIKey()
	if key == "" {
		log.Error().Msg("no aisstream.io API key found; set APIKey=... in engine/.env or export AISSTREAM_API_KEY. Falling back requires -firehose, -scenario, or -fake.")
		return
	}
	e := buildEngine(zonesPath, patrolPath)
	if e == nil {
		return
	}
	// Real AIS coverage is gappy; raise the dark-event silence threshold to 10x
	// so ordinary gaps do not masquerade as dark events (CLAUDE.md).
	e.sweeper.SetSilenceMultiplier(check.RealFeedSilenceMultiplier)
	log.Info().Str("region", "Dutch / southern North Sea").Msg("engine running: live aisstream.io feed")

	e.startTelemetry(ctx, hub)
	src := make(chan ingest.Message, 256)
	go func() {
		gen.RunLive(ctx, gen.LiveConfig{APIKey: key, BoundingBox: northSeaBox}, src, e.cold.SetName)
		close(src)
	}()
	e.pipe.RunSource(src)
	e.pipe.Wait()
}

// runAISDK runs the engine against a recorded Danish AIS CSV (the offline demo:
// real data, reproducible, no network). Same pipeline as every other mode; the
// producer streams and replays the file on a compressed event-time schedule.
func runAISDK(ctx context.Context, hub *api.Hub, zonesPath, patrolPath, csvPath string, speed float64) {
	e := buildEngine(zonesPath, patrolPath)
	if e == nil {
		return
	}
	// Replay compresses event time by `speed`, but the sweeper measures silence in
	// wall time. Scale the silence multiplier by 1/speed so a dark event still
	// means "silent for 10x the expected reporting interval in EVENT time" (what
	// "stopped transmitting" means in the recording), independent of playback
	// speed. Floor it so the wall-clock threshold stays above roughly one sweep
	// tick for the fastest speed class.
	effMult := check.RealFeedSilenceMultiplier / speed
	if effMult < 0.2 {
		effMult = 0.2
	}
	e.sweeper.SetSilenceMultiplier(effMult)
	log.Info().Str("file", csvPath).Float64("speed", speed).Float64("dark_mult", effMult).Msg("engine running: aisdk CSV replay")

	e.startTelemetry(ctx, hub)
	src := make(chan ingest.Message, 1024)
	go func() {
		if err := gen.RunAISDK(ctx, gen.AISDKConfig{Path: csvPath, Speed: speed}, src, e.cold.SetName); err != nil && ctx.Err() == nil {
			log.Error().Err(err).Str("file", csvPath).Msg("aisdk replay failed")
		}
		close(src)
	}()
	e.pipe.RunSource(src)
	e.pipe.Wait()
}

// runSweeper runs the dark-event sweep on a 1s tick (detecting absence is
// sweep-based by definition; never an inline millisecond claim).
func runSweeper(ctx context.Context, sweeper *check.Sweeper) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sweeper.Tick(ingest.NowNs())
		}
	}
}

// broadcastPositions emits a GeoJSON FeatureCollection of vessel positions
// twice a second (the frozen contract cap, regardless of ingest rate). Under
// the firehose the table has ~100k vessels, far more than a map can draw, so it
// sends a stable MMSI-strided sample plus every alert-runner vessel; in scenario
// mode the handful of vessels are all sent. Names are omitted (rule 1).
func broadcastPositions(ctx context.Context, hub *api.Hub, st *state.Shards) {
	const maxVessels = 3000
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			total := st.Len()
			stride := uint32(1)
			if total > maxVessels {
				stride = uint32((total + maxVessels - 1) / maxVessels)
			}
			feats := make([]alert.Feature, 0, maxVessels+64)
			st.ForEach(func(mmsi uint32, v state.VesselState) {
				if stride > 1 && mmsi%stride != 0 && mmsi < 420000000 {
					return
				}
				feats = append(feats, alert.NewFeature(mmsi, v.LastLat, v.LastLon, v.SpeedKn, v.HeadingDeg))
			})
			hub.Broadcast(alert.PositionMsg{
				Type: "positions",
				FC:   alert.FeatureCollection{Type: "FeatureCollection", Features: feats},
			})
		}
	}
}

// broadcast emits a metrics frame every second (rate = messages in the last
// second) and forwards alerts as they arrive, capped per second so the feed
// stays readable. It also dedups the on-screen feed per (vessel, kind): real AIS
// carries duplicate MMSIs and GPS-glitch fixes that legitimately trip the spoof
// check many times for one vessel, and a live ops feed would collapse those into
// one row. The engine's alert counter still counts every detection; only the
// visible feed is deduped.
func broadcast(ctx context.Context, hub *api.Hub, counters *alert.Counters, st *state.Shards, alertCh <-chan alert.Alert) {
	const alertsPerSecCap = 25
	const feedCooldownMs = 20_000
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	lastShown := make(map[uint64]int64) // (mmsi<<2 | kindCode) -> wall ms last shown
	var lastIngested uint64
	sent := 0
	for {
		select {
		case <-ctx.Done():
			return
		case a := <-alertCh:
			key := uint64(a.MMSI)<<2 | uint64(feedKindCode(a.Kind))
			nowMs := time.Now().UnixMilli()
			if last, ok := lastShown[key]; ok && nowMs-last < feedCooldownMs {
				continue // shown recently; engine already counted it
			}
			if sent < alertsPerSecCap {
				lastShown[key] = nowMs
				hub.Broadcast(alert.Envelope{Type: "alert", Alert: a})
				sent++
			}
		case <-ticker.C:
			now := counters.Ingested.Load()
			rate := float64(now - lastIngested)
			lastIngested = now
			hub.Broadcast(counters.Snapshot(rate, st.Len()))
			sent = 0
		}
	}
}

// feedKindCode maps an alert kind to a 2-bit code for the feed-dedup key.
func feedKindCode(kind string) uint64 {
	switch kind {
	case alert.KindZone:
		return 0
	case alert.KindSpoof:
		return 1
	case alert.KindDark:
		return 2
	default:
		return 3
	}
}

// seedNames gives the boundary-crossing runner vessels readable names so the
// alert feed shows something during the demo. Cold store only; never on the hot
// struct.
func seedNames(cold *state.Cold, msgs []ingest.Message) {
	seen := map[uint32]bool{}
	for i := range msgs {
		mmsi := msgs[i].MMSI
		if mmsi >= 420000000 && !seen[mmsi] {
			seen[mmsi] = true
			cold.SetName(mmsi, fmt.Sprintf("RUNNER-%d", mmsi-420000000))
		}
	}
}
