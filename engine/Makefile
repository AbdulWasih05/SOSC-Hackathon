# Palk Watch engine. Run targets from the engine/ directory.
.PHONY: run demo demo-fast live firehose scenario emit-fake bench bench-firehose test

# DEMO: replay the recorded Danish AIS (aisdk) file. Real data, reproducible,
# fully offline (no network, no venue wifi). This is the demo data source.
# Default speed 30x is watchable; demo-fast replays at 300x to push real data
# through the engine above the 50k/s floor (~55k/s) for the throughput moment.
run: demo
demo:
	go run ./cmd/palkwatch -csv data/aisdk-2025-02-27.csv

demo-fast:
	go run ./cmd/palkwatch -csv data/aisdk-2025-02-27.csv -speed 300

# Run the real engine on the LIVE aisstream.io feed (North Sea): real vessels,
# real alerts. Needs APIKey=... in engine/.env (never committed).
live:
	go run ./cmd/palkwatch

# Run the real engine on the in-memory firehose (Act 3 stress feed / 50k theatre).
firehose:
	go run ./cmd/palkwatch -firehose

# Run the scripted scenario (Act 2): zone, spoof, dark + intercept in sequence.
scenario:
	go run ./cmd/palkwatch -scenario data/scenario.json

# Emit schema-valid alerts + metrics + positions for the frontend, no real engine.
emit-fake:
	go run ./cmd/palkwatch -fake

# Benchmark on REAL Danish AIS data: the file is parsed once into memory, then
# looped through the engine at full speed (no network, no replay pacing) to
# measure honest throughput on real AIS message shapes. This is the headline
# number. -n caps how many real messages are parsed from the 3 GB file.
bench:
	go run ./cmd/bench -csv data/aisdk-2025-02-27.csv -n 5000000

# Benchmark on the synthetic firehose: ~98k distinct vessels crafted to keep the
# geofence firing. This is the headroom number (a fuller state table than the
# real sample), reported alongside the real-data number, not instead of it.
bench-firehose:
	go run ./cmd/bench

# Tests. Table-driven coverage lives in check/ and geo/.
test:
	go test ./...
