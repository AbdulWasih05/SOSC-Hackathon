# CLAUDE.md — Palk Watch (PS5 Maritime Surveillance, 24h Hackathon)

You are building a hackathon project under hard time and scope constraints. This file is the constitution. When any instruction, idea, or "improvement" conflicts with this file, this file wins. If the human asks for something that violates this file, point to the relevant section and ask for explicit confirmation before proceeding.

## What this project is

A single Go binary that ingests 50,000+ vessel AIS position messages per second on a laptop and raises three alert types in real time: zone violations, position spoofing, and dark events (vessels that stop transmitting), plus intercept geometry for patrol response. React + MapLibre GL dashboard. Palk Strait / Gulf of Mannar stage.

Pitch sentence, keep every decision aligned to it: "Everyone alerts when a ship enters a zone. The crime happens when a ship disappears. We alert on absence in milliseconds, compute the intercept, and run on hardware a coast guard station already owns."

## Problem statement constraints (hard requirements)

1. **No heavy distributed pipelines.** No Kafka, no Flink, no Spark, no Redis, no external queue, no database. One Go process. Memory is the database. This is a feature, not a limitation: pitch it as sovereignty and deployability.
2. **Throughput floor: 50,000 msgs/sec on standard hardware.** Constraint is a direction: an honest measured number with a pprof profile and scaling narrative is acceptable; a fake or unmeasured claim is not.
3. **Millisecond geofence alert latency.** Applies to inline alerts (zone, spoof). Dark events are sweep-based (1s tick) by definition of detecting absence. Never blur this distinction in code comments, README, or UI labels.

## Judging criteria (every review loop scores against these)

Innovation, Feasibility, Impact, Execution, Presentation. Judges are recent alumni engineers: they reward working demos, honest benchmarks, and clean repos; they punish enterprise cosplay, unverifiable claims, and scope theater.

## Scope: three alerts + intercept. Nothing else.

| Alert | Trigger | Where |
|---|---|---|
| ZONE_VIOLATION | outside-to-inside transition of restricted zone, or EEZ cross by foreign-flagged | inline, per message |
| SPOOF_TELEPORT | implied speed between fixes > 60 kn, or duplicate MMSI > 50 nm apart within 60s | inline, per message |
| DARK_EVENT | silence > 6x expected interval AND last speed > 1 kn AND last position within 5 nm of zone of interest | sweeper, 1s tick |

DARK_EVENT payload includes dead-reckoning cone (origin, heading, spread, radius scalar r(t) = r0 + 0.1 * lastSpeed * dt) and intercept solution per configured patrol asset (heading, ETA, feasible flag).

Expected AIS reporting intervals by speed class: anchored 180s, 0-14 kn 10s, 14-23 kn 6s, >23 kn 2s. Silence multiplier: 6x synthetic, 10x real feed.

## OUT OF SCOPE. Never build these, never suggest building these:

- Rendezvous/transshipment detection (cut with honor; spec lives in PRD Appendix A, slide only)
- SMS, email, Twilio, webhooks, authority routing, notifications of any kind beyond the dashboard websocket
- Auth, users, rate limiting, sessions
- Redis, Postgres, SQLite, any persistence beyond in-memory + optional JSON export
- Docker, docker-compose, nginx, CI/CD, deployment
- ML models of any kind
- Multiple frontend pages, routing, settings pages, dark/light toggle
- Cargo checks (AIS has no cargo compliance data)
- H3 / hexagonal indexing (only justified by cut rendezvous feature)
- Hand-rolled lock-free data structures on the main branch
- New dependencies (see dependency freeze)

If a task seems to need one of these, stop and say so instead of building it.

## Binding hot-path rules (violations are bugs, not style choices)

1. **Value structs in shards.** `map[uint32]VesselState`, VesselState is flat: fixed-size numeric fields only. No strings, slices, maps, or pointers inside. Vessel names/metadata live in a separate cold map touched only on alert emission.
2. **Batch through channels with mandatory timeout flush.** 512-message batches, one channel op per batch. Ingest loop selects on batch-full OR a 5ms ticker; on tick, flush whatever is buffered, even one message. 5ms, not more: at demo message rates the flush timer is the latency floor, and it must stay inside the single-digit-millisecond claim. Never remove or lengthen this ticker.
3. **Zero allocations in the loop where avoidable.** Reuse batch slices; alert structs come from a sync.Pool.
4. **Zero logging per message.** Atomic counters aggregated per second. Alerts, errors, lifecycle log via zerolog. A log statement inside the per-message path is a defect.
5. **Cone is a scalar.** Backend emits origin, heading, spread, r(t). Frontend draws the polygon. Backend never builds cone geometry.
6. **Boring first.** Straightforward channel-based engine complete and tagged `v0-boring` before ANY optimization. Optimizations happen on branches. Any optimization branch that burns 2 hours without landing gets abandoned; revert to tag.
7. **MMSI is uint32 everywhere.** Schema, shards, payloads, generator. 9 digits fits.
8. **Flat-plane math for local distances.** Equirectangular projection (scale lon by cos(lat), then 2D Cartesian) for intercept vectors, cone math, and the spoof teleport distance check. One shared helper in `geo/`. Haversine and GIS libraries are banned from the hot path.

## Dependency freeze

Go: gorilla/websocket, paulmach/orb, rs/zerolog, stretchr/testify, stdlib. That is the complete list.
Frontend: react, maplibre-gl, and whatever vite scaffolds. No UI kits, no state libraries, no axios (fetch + native WebSocket).
Adding any dependency requires the human to say "approved: <dep name>" explicitly.

## Frozen contracts (frontend builds against these; changing them requires human approval)

Alert message (websocket, JSON):
```json
{
  "type": "alert",
  "alert": {
    "id": "a-000123",
    "kind": "ZONE_VIOLATION | SPOOF_TELEPORT | DARK_EVENT",
    "severity": "HIGH | CRITICAL",
    "mmsi": 419001234,
    "name": "string (from cold map, may be empty)",
    "ts_ms": 1720700000000,
    "lat": 9.1234, "lon": 79.4567,
    "zone_id": "gulf-of-mannar-mnp",
    "detail": {},
    "cone": { "lat": 0, "lon": 0, "heading_deg": 0, "spread_deg": 30, "radius_m": 0 },
    "intercepts": [ { "patrol_id": "P1", "heading_deg": 0, "eta_s": 0, "feasible": true } ]
  }
}
```
`cone` and `intercepts` present only on DARK_EVENT. `detail` carries kind-specific fields (e.g. implied_speed_kn for spoof).

Metrics message (websocket, JSON, 1/sec):
```json
{
  "type": "metrics",
  "ingested_total": 0, "processed_total": 0, "dropped_total": 0,
  "rate_per_s": 0,
  "latency_us": { "inline_p50": 0, "inline_p99": 0, "sweep_p50": 0, "sweep_p99": 0 },
  "active_vessels": 0, "alerts_total": 0
}
```

Vessel positions to frontend: batched GeoJSON FeatureCollection updates for a single MapLibre GeoJSONSource, throttled to max 2/sec regardless of ingest rate. Only alert-involved vessels get smooth per-frame animation.

## Repo layout (do not restructure)

```
engine/cmd/palkwatch/  engine/cmd/bench/
engine/internal/{ingest,state,geo,check,alert,api,gen}
engine/data/           # zones geojson, scenario json, patrol config
dashboard/src/{map,panels}
docs/pitch/            README.md
```

## Benchmark methodology (make bench must implement exactly this)

- Generator in-process, pre-generated messages in memory, no network in the measured loop.
- 60-second sustained run. Report msgs/sec, inline p50/p99 and sweep p50/p99 (microseconds), ingested/processed/dropped counts. All of them, always.
- Machine specs and OS printed in the output header.
- If the number is below 50k, the output says the real number. Never inflate, never cherry-pick a burst peak as sustained.

## Milestones and the cut order

H1 schemas live in this file (done). H4 firehose + geofence firing. **H6: v0-boring tagged, complete.** H8 frontend renders from fake emitter; Act 1 recording captured. H12 dark events + cones + intercept end to end. H16 scenario mode + first full run-through + latency histogram. H18 benchmark locked. H20 PPT + video. H22 rehearsals. Nothing new after H22.

If behind schedule, cut in this order: intercept geometry, then dark events. NEVER cut: geofence, spoof, benchmark, the demo scenario. If a cut happens, update the pitch materials in the same commit.

## Self-review loop protocol

After completing each milestone (and before marking any task done), run this review as a strict judge, in writing, in the session:

1. **Constraint audit:** does the change violate any hard requirement or hot-path rule above? Check rules 1-8 explicitly against the diff.
2. **Scope audit:** did anything from the out-of-scope list creep in? Any new dependency? Any new file outside the layout?
3. **Latency honesty:** do any comments, labels, or docs claim millisecond latency for sweep-based alerts? Fix immediately.
4. **Judge lens:** score the current state 1-10 on Execution and Presentation-readiness. If Execution < 7, name the single biggest gap and fix it before new features.
5. **Demo test:** can the scenario run end to end right now? If integration has been broken for more than 1 hour of work, integration repair preempts everything.
6. **Benchmark drift:** if the hot path changed, re-run `make bench` (or the quick 10s variant) and record the number in docs/BENCH_LOG.md with the commit hash. A regression > 15% blocks merge.

Write the review as a short block: PASS/FAIL per item, one line each. A FAIL means fix before moving on, no exceptions.

## Style and conduct

- Go: gofmt, small functions, table-driven tests for check/ and geo/ only (the logic that can be wrong quietly). No test theater elsewhere.
- Commits: imperative, small, working. Never commit a broken main; branches for risk.
- No em dashes in any written material (README, slides, pitch text).
- When uncertain between two designs, pick the one that is easier to demo and easier to delete.
- Time is the scarcest resource. If a task estimate exceeds 90 minutes, break it down or flag it.