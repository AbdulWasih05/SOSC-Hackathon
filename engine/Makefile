# Palk Watch engine. Run targets from the engine/ directory.
.PHONY: run demo live firehose scenario emit-fake bench test

# DEMO: replay the recorded Danish AIS (aisdk) file. Real data, reproducible,
# fully offline (no network, no venue wifi). This is the demo data source.
run: demo
demo:
	go run ./cmd/palkwatch -csv data/aisdk-2025-02-27.csv

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

# Benchmark. Prints methodology header now; measured 60s loop lands H4-H6.
bench:
	go run ./cmd/bench

# Tests. Table-driven coverage lives in check/ and geo/.
test:
	go test ./...
