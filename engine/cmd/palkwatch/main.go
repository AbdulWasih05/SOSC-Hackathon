// Command palkwatch is the single Palk Watch binary. With -fake it runs the H1
// contract emitter. Otherwise it runs the real engine: firehose -> ingest ->
// state -> inline checks (geofence, spoof) -> alert bus, broadcasting metrics
// once per second and alerts as they fire over the dashboard websocket.
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
		fake  bool
		addr  string
		zones string
		total int
	)
	flag.BoolVar(&fake, "fake", false, "run the contract emitter (schema-valid alerts + metrics, no real engine)")
	flag.StringVar(&addr, "addr", ":8080", "http listen address for the dashboard websocket")
	flag.StringVar(&zones, "zones", "data/zones.geojson", "zone geojson path")
	flag.IntVar(&total, "n", 1_000_000, "firehose messages pre-generated and looped")
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	hub := api.NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.HandleWS)
	srv := &http.Server{Addr: addr, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("addr", addr).Msg("dashboard websocket listening on /ws")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server failed")
		}
	}()

	if fake {
		log.Info().Msg("fake emitter on: schema-valid alerts + metrics at demo rates")
		go gen.RunFake(ctx, hub)
	} else {
		go runEngine(ctx, hub, zones, total)
	}

	<-ctx.Done()
	log.Info().Msg("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}

// runEngine builds and runs the real engine against the firehose, broadcasting
// telemetry over the websocket until ctx is cancelled.
func runEngine(ctx context.Context, hub *api.Hub, zonesPath string, total int) {
	zs, err := geo.LoadZones(zonesPath)
	if err != nil {
		log.Error().Err(err).Str("path", zonesPath).Msg("failed to load zones; engine not started")
		return
	}
	grid := geo.NewGrid(zs, geo.CellDeg)
	st := state.New()
	cold := state.NewCold()
	counters := alert.NewCounters()

	// Alerts fire far faster than a human dashboard can read; forward them to a
	// small buffered channel and drop when full. counters.Alerts stays exact;
	// only the on-screen feed is sampled.
	alertCh := make(chan alert.Alert, 256)
	sink := func(a alert.Alert) {
		select {
		case alertCh <- a:
		default:
		}
	}
	proc := check.NewProcessor(st, cold, grid, counters, sink)

	workers := runtime.GOMAXPROCS(0)
	pipe := ingest.New(counters, workers, workers*2)
	pipe.Start(func() ingest.BatchHandler { return proc.NewWorker() })

	msgs := gen.Firehose(total)
	seedNames(cold, msgs)
	log.Info().Int("messages", len(msgs)).Int("workers", workers).Msg("engine running on firehose")

	go broadcast(ctx, hub, counters, st, alertCh)
	pipe.RunFirehoseCtx(ctx, msgs)
	pipe.Wait()
}

// broadcast emits a metrics frame every second (rate = messages in the last
// second) and forwards alerts as they arrive, capped per second so the feed
// stays readable.
func broadcast(ctx context.Context, hub *api.Hub, counters *alert.Counters, st *state.Shards, alertCh <-chan alert.Alert) {
	const alertsPerSecCap = 25
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastIngested uint64
	sent := 0
	for {
		select {
		case <-ctx.Done():
			return
		case a := <-alertCh:
			if sent < alertsPerSecCap {
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
