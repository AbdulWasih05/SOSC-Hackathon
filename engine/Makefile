# Palk Watch engine. Run targets from the engine/ directory.
.PHONY: run scenario emit-fake bench test

# Run the real engine on the firehose (Act 3): alerts, metrics, positions over ws.
run:
	go run ./cmd/palkwatch

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
