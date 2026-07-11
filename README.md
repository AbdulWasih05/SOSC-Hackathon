# Palk Watch

Real-time maritime dark-vessel detection. A single Go binary ingests 50,000+
vessel AIS position messages per second on a laptop and raises three alerts in
real time: zone violations, position spoofing, and dark events (vessels that
stop transmitting), plus intercept geometry for patrol response. React +
MapLibre GL dashboard.

> Everyone alerts when a ship enters a zone. The crime happens when a ship
> disappears. We alert on absence in milliseconds, compute the intercept, and
> run on hardware a coast guard station already owns.

## Benchmark

```
cd engine
make bench
```

Reports sustained msgs/sec and inline/sweep p50/p99 latency over a 60-second
in-process run, with ingested/processed/dropped counts and machine specs. If the
rate is under 50k, the output says the real number. (Measured loop lands H4-H6;
the stub currently prints the methodology header.)

## Run

```
cd engine
make emit-fake     # schema-valid alerts + metrics for the dashboard, no engine
make run           # the real engine (websocket only until H4)
make test          # tests (table-driven coverage in check/ and geo/)
```

Dashboard connects to the websocket at `ws://localhost:8080/ws`.

## Layout

```
engine/    Go: cmd/{palkwatch,bench}, internal/{ingest,state,geo,check,alert,api,gen}, data/
dashboard/ React + MapLibre GL: src/{map,panels}
docs/pitch README.md
```

Scope, constraints, and hot-path rules live in CLAUDE.md.
