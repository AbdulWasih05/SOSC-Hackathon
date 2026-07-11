# PRD v2: Palk Watch (working name)
## Real-Time Maritime Dark Vessel Detection Engine
### 24-Hour Hackathon Build, Problem Statement 5
### v2: post judge/mentor review. Changes: rendezvous pre-cut, intercept geometry promoted into demo, live act pre-recorded, hot-path hardening, latency histogram HUD, H6 boring-version checkpoint.
### v3: batch timeout flush (5ms) fixing the low-rate latency trap, MMSI locked to uint32 everywhere, flat-plane local projection for all hot-path and intercept distance math.

---

## 1. One-liner

A single-binary stream processing engine that ingests 50,000+ vessel position messages per second on a laptop and detects what incumbent platforms miss: ships that vanish and ships that lie about their position, then shows the patrol boat how to catch them.

## 2. Positioning and the story

**The gap we exploit.** Incumbent platforms (Skylight by Allen Institute, Global Fishing Watch) have solved detection of vessels that transmit AIS honestly. Their own published materials admit the weaknesses:

- Many fishing vessels do not carry AIS at all; small and artisanal vessels are invisible.
- Roughly 6% of global commercial fishing hides inside deliberate AIS transmission gaps (GFW analysis of 28 billion signals).
- Spoofing hardware is commercially available; manipulation concentrates exactly where monitoring matters most.
- Their satellite fallbacks (SAR, night lights) have hours of latency, not milliseconds.
- They are cloud services run from the US. No on-prem, no sovereignty, no offline.
- Per Skylight's own director: detection is global, the bottleneck is acting on it.

**Our pitch sentence.** "Everyone alerts when a ship enters a zone. The crime happens when a ship disappears. We alert on absence in milliseconds, compute the intercept, and run on hardware a coast guard station already owns."

**Stage.** Palk Strait and Gulf of Mannar Marine National Park. Real Indian waters, live conflict zone for IUU fishing, boundaries from Marine Regions shapefiles.

## 3. Scope

### In scope (the product)
1. Go stream engine, one process, zero external services in the hot path.
2. Three alert types (exact triggers in section 5): zone violation, spoof/teleport, dark event.
3. Intercept geometry on dark events: stationed patrol marker, computed intercept heading and ETA rendered when the cone blooms.
4. Synthetic AIS replay generator with scripted violation scenarios.
5. aisstream.io ingest goroutine, used to record Act 1 the night before. Not stage-critical.
6. MapLibre GL dashboard: vessel field (data-driven GeoJSONSource), zones, alert feed, throughput HUD, live p50/p99 latency histogram per alert type, dead reckoning cones, intercept vectors.
7. Reproducible benchmark command documented in README.

### Cut with honor (one slide each, "designed, spec'd, cut for scope honesty")
- **Rendezvous detection.** Full spec preserved in Appendix A. Cut reasons: only pair-state alert, weakest demo beat, ~5 team hours returned, one failure mode removed.
- **Live venue streaming during pitch.** Replaced by pre-recorded real-feed act (section 8).

### Out of scope (do not touch, even at 3am when it feels like a good idea)
- SMS, email, Twilio, webhooks, authority routing
- Auth, rate limiting, user accounts
- Redis, Postgres, any database. Memory is the database.
- Docker, nginx, deployment anything
- ML fishing behavior classification
- Multiple frontend pages, settings, analytics pages
- Cargo checks (AIS carries no cargo compliance data)
- H3 hexagonal indexing (was only justified by rendezvous; square grid suffices for geofence)
- Hand-rolled lock-free structures before profiling proves the need

## 4. Architecture

```
                    ┌──────────────────────────────────────────┐
                    │            palkwatch (one Go binary)      │
                    │                                          │
 aisstream.io ────► │ ingest ──► batch channel ──► worker pool │
 (recording only)   │            (512-msg batches)   │         │
                    │                          vessel state     │
 synthetic ───────► │                          (64 shards,      │
 replay             │                           VALUE structs)  │
 (in-process        │                                │         │
  for benchmark)    │                          spatial grid     │
                    │                                │         │
                    │                          inline checks:   │
                    │                          geofence, spoof  │
                    │                                │         │
                    │        sweeper (1s tick):      │         │
                    │        dark events + intercept geometry   │
                    │                                │         │
                    │                └────► alert bus ──► ws out│
                    │                      metrics ──► ws out   │
                    └──────────────────────────────────────────┘
                                              │
                                        React + MapLibre GL
```

### Hot-path hardening rules (from mentor review, binding)
1. **Value structs, not pointers.** Shard maps are `map[uint32]VesselState` where VesselState is flat: fixed-size numeric fields only. No strings, no slices, no nested pointers in the hot struct. Vessel names/metadata live in a separate cold map touched only on alert emission. This keeps GC mark phase off the hot data.
2. **Batch through channels, not per-message sends, WITH timeout flush.** Ingest groups messages into 512-message batches; one channel operation per batch makes channel contention irrelevant at 50k/s. **Mandatory flush rule:** the ingest loop selects on batch-full OR a 5ms ticker; on tick, flush whatever is buffered (even a single message) and reset. Without this, low-rate modes (Acts 1-2 run at 5-10 msgs/sec) would wait indefinitely to fill a batch and inline latency would explode to tens of seconds on stage. The interval is 5ms, not 50ms, deliberately: at demo rates the flush timer IS the latency floor, and 5ms keeps worst-case queueing inside the single-digit-millisecond claim shown on the Act 3 histogram, so the story stays consistent across acts. The atomic ring buffer is an OPTIMIZATION BRANCH, attempted only if pprof shows the channel as the bottleneck after H6. Never hand-roll lock-free code on the main branch.
3. **Zero allocations in the loop where avoidable.** Reuse batch slices, preallocate alert structs from a pool.
4. **Zero logging per message.** Atomic counters aggregated per second. Alerts, errors, lifecycle events log; messages never do. Sample-trace 1 in 10,000 if debugging demands.
5. **Cone is a scalar, not a polygon.** Backend emits origin, heading, spread angle, and radius r(t) = r0 + (Ev x dt), where r0 is GPS tolerance, Ev is velocity error margin (10% of last known speed). Frontend draws the polygon. Backend CPU belongs to ingestion.
6. **Boring first, tagged, then optimize.** See H6 checkpoint, section 10.
7. **MMSI is uint32, everywhere.** MMSIs are 9 decimal digits (max 999,999,999), comfortably inside uint32's range. Lock this type across the message schema, state shards, and alert payloads; it halves key width versus uint64 and saves memory bandwidth on every map lookup. Synthetic vessel IDs must also fit in uint32.
8. **Flat-plane math for all local distances.** Every hot-path and intercept distance computation uses an equirectangular local projection (scale longitude by cos(latitude), treat coordinates as 2D Cartesian), not spherical trigonometry and not a GIS library. At Gulf of Mannar scale (tens of nautical miles) the error is negligible and 2D kinematics is dramatically cheaper. Applies to: the intercept closing-vector solution, dead-reckoning cone math, and, critically, the spoof teleport distance check, which runs once per message and is the hottest distance calculation in the system. One shared projection helper in `geo/`; haversine appears nowhere in the hot path.

### Key structures
- **Vessel state table:** 64 shards by MMSI hash, RWMutex per shard, value semantics per rule 1.
- **Spatial grid:** lat/lon bins at 0.05 degrees. Zones pre-rasterized at startup into three cell classes: fully-inside (skip polygon test), outside (skip), boundary (exact point-in-polygon via orb). Square grid is sufficient with rendezvous cut.
- **Batch channel** ingest to workers, buffered; drop counter exposed and reported honestly.

### Dependencies (final list, do not grow)
- github.com/gorilla/websocket
- github.com/paulmach/orb
- github.com/rs/zerolog (boundaries only)
- github.com/stretchr/testify
- stdlib net/http for the REST endpoints

## 5. Alert taxonomy (exact triggers)

| # | Alert | Trigger condition | Runs | Severity |
|---|-------|------------------|------|----------|
| 1 | ZONE_VIOLATION | Vessel position transitions outside to inside a restricted zone (MPA) or crosses EEZ line while flagged foreign | Inline, per message | HIGH |
| 2 | SPOOF_TELEPORT | Implied speed between consecutive fixes > 60 knots, or duplicate MMSI seen > 50 nm apart within 60s | Inline, per message | HIGH |
| 3 | DARK_EVENT | No message for 6x expected reporting interval AND last known state was moving (speed > 1 kn) AND last position inside or within 5 nm of a zone of interest | Sweeper, 1s tick | CRITICAL |

**Dark event payload:** last position, heading, r(t) scalar per section 4 rule 5, PLUS intercept solution: for each stationed patrol asset (static config, 2-3 markers), closing geometry against the cone centroid: intercept heading, ETA, feasible yes/no. ~40 lines of 2D kinematics in `geo/intercept.go` on the flat-plane projection (rule 8), converting the resulting vector back to a compass heading at the end. No spherical geometry, no GIS dependency.

**Expected reporting interval by speed class (AIS standard, simplified):** anchored/moored 180s, 0-14 kn 10s, 14-23 kn 6s, >23 kn 2s. Real-feed recording uses conservative 10x multiplier to suppress coverage-gap false positives; synthetic scenarios use 6x.

**Latency claim discipline:** millisecond ingest-to-alert applies to alerts 1 and 2 (inline), and the 5ms batch flush ticker (rule 2) bounds worst-case queueing delay at low message rates, so the claim holds in scenario mode, not just under the firehose. Alert 3 is sweep-based with a 1s detection tick on top of the silence threshold. The live latency histogram (section 8) displays these separately so the distinction is shown, not hidden.

## 6. Data plan

1. **Zones:** Marine Regions EEZ boundaries (India, Sri Lanka), Gulf of Mannar Marine National Park polygon. Download at hour 0, simplify to < 500 vertices per polygon (mapshaper), commit GeoJSON to repo.
2. **Real-feed recording:** aisstream.io websocket, Palk Strait bounding box, run through the actual engine the night before / hour 0-2. Screen-capture 30-60s of real vessels flowing with real names. This recording is Act 1.
3. **Synthetic generator, two modes:**
   - **Firehose mode:** N synthetic vessels, realistic speed/heading distributions, pre-generated into memory, fired as fast as the engine consumes. This is the benchmark.
   - **Scenario mode:** deterministic JSON timeline. Named vessels execute the story: trawler crosses into Gulf of Mannar (alert 1), vessel teleports (alert 2), fishing vessel goes dark 3 nm from the MPA, cone blooms, intercept vector renders (alert 3 + intercept). Replayable, demo-safe.

## 7. Benchmark methodology (README section, verbatim commitment)

- Generator runs in-process; no network in the measured loop.
- Sustained msgs/sec over 60 seconds, p50/p99 ingest-to-alert-emit latency, timestamps inside the process.
- Report ingested AND processed AND dropped counts. All three.
- Run on the team's weakest laptop; quote machine specs AND operating system in README and slide. Prefer an existing Linux machine on the team; if none, stock Ubuntu; if Windows, say so honestly. No new-distro installs during hackathon week.
- Single command: `make bench`. Judges can clone and run.
- If under 50k: honest number, pprof flame graph screenshot, and the scaling path. Constraint is a direction; a measured 30k with a profile beats a claimed 50k that wobbles under questioning.

## 8. Demo script (3 minutes, three acts)

**Act 1, Real (30s, pre-recorded).** Screen recording: real Palk Strait AIS traffic flowing through this engine, captured the previous night. Caption on screen: "recorded live, [date], our engine, real ships." Line: "these are real vessels in the most contested fishing waters in South Asia, processed by the same binary you're about to see live." Zero venue-wifi dependency.

**Act 2, The story (90s, live, scenario mode).** The alerts fire in sequence on cue. Peak moment: the trawler's dot goes grey, the dead reckoning cone blooms across the marine park boundary, CRITICAL alert slides in, and the patrol marker draws its intercept vector with an ETA. Line: "Skylight's satellite fallback sees this gap in hours. We saw it in 1.2 seconds, and we already know Patrol B catches them in 41 minutes."

**Act 3, The firehose (30s, live, benchmark mode).** Throughput HUD climbs past the target, and the per-alert-type p50/p99 latency histogram sits beside it, single-digit milliseconds on inline alerts, live. Line: "one binary, this laptop, no cloud." Two verifiable numbers on screen at once.

Close: roadmap slide (full interdiction dispatch, radar/SAR triggers into the same alert bus, sovereignty pitch) + the cut-with-honor slide (rendezvous spec, why it was cut).

## 9. Team split

| Member | Owns | Hours 0-12 | Hours 12-24 |
|--------|------|-----------|-------------|
| A (Go) | Engine | Boring version to completion by H6 (see checkpoint), then geofence + spoof solid, metrics | Dark events + intercept geometry, benchmark tuning on branch, pprof |
| B (Frontend) | Dashboard | MapLibre GeoJSONSource pipeline, fake emitter contract, vessel layer, alert feed | Cone + intercept layers, latency histogram HUD, scenario choreography with C |
| C (Data/Scenarios) | Generator + zones | Shapefiles, GeoJSON prep, firehose generator, aisstream recording session | Scenario script JSON, demo timeline tuning, backup recordings |
| D (Pitch) | Submission | PPT skeleton, README, competitive research slides | Demo video, pitch script, rehearsal direction, Q&A drills |

**Contract-first rule:** alert JSON schema and metrics schema frozen by hour 1. B builds against a fake emitter from hour 2. Frontend never blocks on the engine.

**A-protection rule (from review, binding):** A writes the straightforward channel-based engine to full completion by H6, tags it `v0-boring`, and only then attempts optimizations on a branch. If any optimization branch burns 2 hours without landing, revert to tag and move on. The demo must always have a running engine to point at. Nothing saves constraint 2 if A sinks; this rule is the flotation device.

## 10. Hour-by-hour milestones

- **H1:** Schemas frozen. Repo scaffold (trimmed tree). Zones downloaded. aisstream recording session scheduled.
- **H4:** Engine ingests firehose, geofence alerts fire, first throughput number known.
- **H6: BORING-VERSION CHECKPOINT.** Complete channel-based engine tagged `v0-boring`: ingest, state, grid, geofence, spoof, metrics, all working at whatever speed it runs. GO/NO-GO: if under 10k msgs/sec here, fix batch size and grid resolution before any cleverness.
- **H8:** Frontend renders vessel field from fake emitter. Firehose generator done. Act 1 recording captured.
- **H12:** Dark events + cones + intercept geometry end to end. First full integration.
- **H16:** Scenario mode scripted and choreographed. First full demo run-through. Latency histogram on HUD.
- **H18:** Benchmark locked on worst laptop, numbers + flame graph in README.
- **H20:** PPT done, demo video recorded from scenario mode.
- **H22:** Two full pitch rehearsals with live demo.
- **H24:** Buffer. Nothing new after H22.

## 11. Risks and fallbacks

| Risk | Mitigation |
|------|-----------|
| Throughput lands at 20-30k | Honest number + pprof profile + scaling slide. v0-boring tag guarantees a working engine exists regardless. |
| A blocked / optimization rabbit hole | A-protection rule: revert to `v0-boring` tag after any 2-hour stall. Cut order if behind: intercept geometry first, dark events second. Never cut geofence + spoof + benchmark. |
| Venue wifi | Nothing on stage needs it. Act 1 is a recording; Acts 2-3 are localhost. |
| GC pauses spike p99 | Value-struct rule already applied; if p99 still spikes, GOGC tuning and alert-pool preallocation are the H18 levers. |
| MapLibre learning curve | Fallback deck.gl ScatterplotLayer; last resort Leaflet canvas renderer with capped visible vessels. |
| Judge asks "Skylight is free" | Rehearsed 30s answer: AIS-invisible vessels, hours-latency satellite fallback, cloud sovereignty, intercept layer they don't have. D owns it. |
| Judge asks "where's rendezvous / transshipment detection" | Cut-with-honor slide + Appendix A spec. "We chose depth on three alerts over breadth on four; here is the full design." |

## 12. Judge Q&A prep (D drills these)

1. Where does real data come from in production? Terrestrial AIS receivers coast guards already operate, satellite AIS feeds; engine is source-agnostic, demonstrated by running both aisstream and synthetic through the same ingest interface.
2. What about vessels with no AIS at all? Honestly out of scope for this engine; dark-event detection covers deliberate disabling, and the roadmap feeds radar/SAR triggers into the same alert bus.
3. Why Go, why one binary? Sovereignty and deployability: a coast guard station with one server runs this. Incumbents require cloud connectivity to a US nonprofit.
4. Millisecond latency, on which alerts exactly? Inline alerts (zone, spoof): yes, shown live on the histogram. Dark events: bounded by the 1s sweep plus the silence threshold, by definition of detecting absence. The histogram displays them separately.
5. How do you know 50k/sec matters? Skylight processes 5 billion messages daily at 50k/sec across a cloud fleet. We target that rate on one laptop.
6. False positives on dark events? Speed-class-aware expected intervals, conservative multipliers on real data, zone-proximity filter so open-ocean coverage gaps never alert.
7. Is the intercept for real? Geometry is real (closing-vector math, live ETA). Patrol positions are configured, not integrated with any fleet system; that integration is the roadmap.

## 13. Trimmed repo tree

```
palkwatch/
├── engine/                     # Go
│   ├── cmd/palkwatch/main.go
│   ├── cmd/bench/main.go       # make bench entry
│   ├── internal/
│   │   ├── ingest/             # aisstream ws + replay source, common interface
│   │   ├── state/              # sharded vessel table (value structs)
│   │   ├── geo/                # grid index, polygon tests, deadreckoning.go, intercept.go
│   │   ├── check/              # geofence.go spoof.go dark.go
│   │   ├── alert/              # bus, severity, schema, pool
│   │   ├── api/                # ws out, REST endpoints, stats
│   │   └── gen/                # firehose + scenario generator
│   ├── data/                   # zones geojson, scenario json, patrol config
│   ├── go.mod
│   └── Makefile                # run, bench, test
├── dashboard/                  # React + MapLibre GL
│   └── src/
│       ├── map/                # vessel layer, zone layer, cone layer, intercept layer
│       ├── panels/             # alert feed, HUD, latency histogram, ship drawer
│       ├── ws.js               # single ws client
│       └── App.jsx
├── docs/
│   ├── pitch/                  # ppt, script, Q&A, act1-recording.mp4
│   └── ARCHITECTURE.md
└── README.md                   # benchmark command front and center
```

## Appendix A: Rendezvous detection spec (cut, preserved for the slide and the roadmap)

Trigger: two vessels, both speed < 3 kn, within 500 m of each other for > 5 continuous minutes, outside port polygons. Severity MEDIUM. Implementation: slow-set pruning (only vessels under 3 kn enter the candidate set, hundreds not tens of thousands), grid neighbor lookup for proximity, pair-state table with continuity timers, sweep every 10-30s. Known refinement: hexagonal indexing (uber/h3-go) gives uniform 1-ring neighbor lookups and removes square-grid diagonal edge cases; adopt when this feature is built. Estimated cost: 5 team hours. Cut for scope honesty; the detection story stands on absence and deception, and transshipment detection is where incumbents are already strongest.

---

*Frozen scope, v2. Additions after hour 1 require removing something of equal size.*
