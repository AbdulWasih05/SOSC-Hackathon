// Command bench is the `make bench` entry point. It runs the firehose through
// the real engine in-process (no network in the measured loop) and reports
// sustained msgs/sec with inline latency percentiles and honest
// ingested/processed/dropped counts, per the PRD section 7 methodology.
package main

import (
	"flag"
	"fmt"
	"runtime"
	"time"

	"palkwatch/internal/alert"
	"palkwatch/internal/check"
	"palkwatch/internal/gen"
	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
	"palkwatch/internal/state"
)

func main() {
	var (
		dur    time.Duration
		total  int
		zones  string
		patrol string
		buf    int
	)
	flag.DurationVar(&dur, "d", 60*time.Second, "measured run duration")
	flag.IntVar(&total, "n", 1_000_000, "pre-generated messages looped through the engine")
	flag.StringVar(&zones, "zones", "data/zones.geojson", "zone geojson path")
	flag.StringVar(&patrol, "patrol", "data/patrol.json", "patrol config path")
	flag.IntVar(&buf, "buf", 0, "batch-channel buffer depth (0 = 2*workers)")
	flag.Parse()

	workers := runtime.GOMAXPROCS(0)
	if buf <= 0 {
		buf = workers * 2
	}

	fmt.Println("=== Palk Watch benchmark ===")
	fmt.Printf("Go:       %s\n", runtime.Version())
	fmt.Printf("OS:       %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("CPUs:     %d\n", runtime.NumCPU())
	fmt.Printf("Workers:  %d\n", workers)
	fmt.Printf("Buffer:   %d batches\n", buf)
	fmt.Printf("Duration: %s\n", dur)
	fmt.Printf("Messages: %d pre-generated, looped\n", total)
	fmt.Println()
	fmt.Println("Methodology (PRD section 7): in-process generator, no network in the measured")
	fmt.Println("loop, sustained rate, all of ingested/processed/dropped reported.")
	fmt.Println()

	zs, err := geo.LoadZones(zones)
	if err != nil {
		fmt.Println("failed to load zones:", err)
		return
	}
	patrols, err := geo.LoadPatrols(patrol)
	if err != nil {
		fmt.Println("failed to load patrols:", err)
		return
	}
	grid := geo.NewGrid(zs, geo.CellDeg)
	st := state.New()
	cold := state.NewCold()
	counters := alert.NewCounters()
	ids := alert.NewIDGen()
	proc := check.NewProcessor(st, cold, grid, counters, ids, alert.Discard)
	sweeper := check.NewSweeper(st, cold, zs, patrols, counters, ids, alert.Discard)

	fmt.Print("generating firehose... ")
	msgs := gen.Firehose(total)
	fmt.Printf("done (%d messages)\n\n", len(msgs))

	pipe := ingest.New(counters, workers, buf)
	pipe.Start(func() ingest.BatchHandler { return proc.NewWorker() })

	// Run the dark sweep once per second alongside the firehose so sweep latency
	// (the cost of scanning the whole table) is measured. No vessel goes silent
	// under the firehose, so it finds nothing; it still times the scan.
	stopSweep := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-stopSweep:
				return
			case <-t.C:
				sweeper.Tick(ingest.NowNs())
			}
		}
	}()

	t0 := time.Now()
	pipe.RunFirehose(msgs, dur)
	pipe.Wait()
	close(stopSweep)
	elapsed := time.Since(t0)

	processed := counters.Processed.Load()
	rate := float64(processed) / elapsed.Seconds()

	fmt.Println("--- results ---")
	fmt.Printf("elapsed:        %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("msgs/sec:       %s\n", commas(uint64(rate)))
	fmt.Printf("ingested:       %s\n", commas(counters.Ingested.Load()))
	fmt.Printf("processed:      %s\n", commas(processed))
	fmt.Printf("dropped:        %s\n", commas(counters.Dropped.Load()))
	fmt.Printf("alerts:         %s\n", commas(counters.Alerts.Load()))
	fmt.Printf("active vessels: %s\n", commas(uint64(st.Len())))
	fmt.Printf("inline latency: p50 %.0f us  p99 %.0f us\n", counters.InlineHist.Percentile(50), counters.InlineHist.Percentile(99))
	fmt.Printf("sweep latency:  p50 %.0f us  p99 %.0f us (full-table scan, %d vessels)\n", counters.SweepHist.Percentile(50), counters.SweepHist.Percentile(99), st.Len())
	fmt.Println()
	if rate >= 50000 {
		fmt.Printf("TARGET MET: %s msgs/sec sustained (>= 50,000).\n", commas(uint64(rate)))
	} else {
		fmt.Printf("BELOW TARGET: %s msgs/sec sustained (< 50,000). Honest number stands; profile next.\n", commas(uint64(rate)))
	}
}

// commas formats an unsigned integer with thousands separators.
func commas(n uint64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}
