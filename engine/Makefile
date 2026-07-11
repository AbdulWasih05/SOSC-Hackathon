# Palk Watch engine. Run targets from the engine/ directory.
.PHONY: run emit-fake bench test

# Run the binary (real engine once wired; websocket only for now).
run:
	go run ./cmd/palkwatch

# Emit schema-valid alerts + metrics for the frontend, no real engine.
emit-fake:
	go run ./cmd/palkwatch -fake

# Benchmark. Prints methodology header now; measured 60s loop lands H4-H6.
bench:
	go run ./cmd/bench

# Tests. Table-driven coverage lives in check/ and geo/.
test:
	go test ./...
