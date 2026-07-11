// Command palkwatch is the single Palk Watch binary. At H1 it hosts the
// dashboard websocket and, with -fake, runs the contract emitter so the
// frontend is unblocked. The real ingest/state/check engine wires in here as it
// lands (H4 onward).
package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"palkwatch/internal/api"
	"palkwatch/internal/gen"
)

func main() {
	var (
		fake bool
		addr string
	)
	flag.BoolVar(&fake, "fake", false, "run the contract emitter (schema-valid alerts + metrics, no real engine)")
	flag.StringVar(&addr, "addr", ":8080", "http listen address for the dashboard websocket")
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	hub := api.NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.HandleWS)
	srv := &http.Server{Addr: addr, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if fake {
		log.Info().Msg("fake emitter on: schema-valid alerts + metrics at demo rates")
		go gen.RunFake(ctx, hub)
	} else {
		log.Warn().Msg("real engine not built yet; run with -fake to emit demo data")
	}

	go func() {
		log.Info().Str("addr", addr).Msg("dashboard websocket listening on /ws")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down")
	shutCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}
