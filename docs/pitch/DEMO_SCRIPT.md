# Reef Watchers, Round 2 Demo Runbook

Goal for this round: show the processing engine working and show the vessel
surveillance working, across every angle the engine has, in a controlled and
reproducible way. We have three ways to feed the engine, and we use all three,
each for what it proves:

1. Recorded real Danish AIS (the 3.1 GB aisdk file): proves this runs on real
   traffic at scale, offline.
2. The scripted all-angles scenario (this document's centrepiece): proves every
   alert type and the intercept, deterministically, in about 75 seconds.
3. The in-memory firehose: proves the 50k throughput floor with a live latency
   histogram.

Everything runs on one laptop with no internet required (the weather layer is the
only optional online extra, and it fails open).

Every command is run from the `engine/` directory. The dashboard is a separate
`npm run dev` in `dashboard/`. Open the dashboard, then start one engine command
at a time. Only one engine can hold the websocket port at once, so stop the
previous one (Ctrl+C) before starting the next.

If `make` is not installed on the demo machine, use the raw `go run` command
under each `make` target below. The four you need:

```
# Act 1 real data:   go run ./cmd/palkwatch -csv data/aisdk-2025-02-27.csv
# Act 2 all-angles:  go run ./cmd/palkwatch -scenario data/scenario-denmark.json -zones data/zones-denmark.geojson -patrol data/patrol-denmark.json -risk
# Act 3 firehose:    go run ./cmd/palkwatch -firehose
# Act 3 benchmark:   go run ./cmd/bench -csv data/aisdk-2025-02-27.csv -n 5000000
```

---

## The map you are looking at

Stage is Danish waters, the Kattegat and the Great Belt. Two protected areas are
drawn on the map:

- Great Belt Natura 2000 (an MPA, closed water).
- Anholt Offshore Wind Farm (an MPA, closed to shipping).
- The Danish EEZ box around both.

Two patrol assets are stationed and drawn as markers:

- P1, HDMS Diana, near the Anholt wind farm.
- P2, HDMS Rota, near the Great Belt.

The recorded replay and the scripted scenario use the same map and the same
zones, so Act 1 and Act 2 look continuous. That is deliberate.

---

## Where alerts appear on screen (read this before presenting)

The dashboard routes only zone violations (HIGH or CRITICAL) to the ALERTS tab.
Every other alert kind (dark events, spoof teleports, trawling, and the
illegal-fishing-suspected escalation) shows in the LOGS tab. So during the demo:

- The ALERTS tab is where the protected-zone entry lands.
- The LOGS tab is the live feed of everything else, including the dark-event
  money shot.
- Click any vessel to open the Score Breakdown drawer and read its risk factors.

Do not wait for a dark event to show in the ALERTS tab. It is in LOGS by design.

---

## Act 1, Prove reality (recorded real AIS)

Command:

```
make demo
```

or push real data through the engine above the throughput floor:

```
make demo-fast
```

Say: "This is a recording of actual Danish ship traffic from the Danish Maritime
Authority, replayed through the live engine. No internet, no cloud, reproducible.
The dots are real vessels. Zone and spoof alerts you see here are the engine
reacting to real message shapes."

`make demo` replays at a watchable speed. `make demo-fast` replays at 300x to
show real data crossing the throughput floor. The recording is clean official
data, so genuine dark events are rare in it. That is the point of Act 2: we
script the disappearance we cannot rely on a clean recording to hand us.

---

## Act 2, The all-angles scenario (the centrepiece)

Command:

```
make scenario-dk
```

With the optional live sea-state layer on the fishing alert:

```
make scenario-dk-weather
```

If `make` is not available, the raw command is:

```
go run ./cmd/palkwatch -scenario data/scenario-denmark.json \
  -zones data/zones-denmark.geojson -patrol data/patrol-denmark.json -risk
```

This is a scripted timeline, about 75 seconds of wall time, that fires every
detection path in sequence. Three vessels:

- GRETHE HANSEN (Danish trawler): the protected-zone offender.
- NORTH STAR (foreign flagged): the deception-and-absence climber.
- DFDS PEARL (Danish ferry): the honest control that never alerts.

### What fires, in order, and what to say

1. NORTH STAR teleports. A SPOOF_TELEPORT appears in LOGS almost immediately, with
   an impossible implied speed in the detail. Say: "One MMSI, two positions no
   ship could travel between. That is a spoof, caught inline, per message."

2. GRETHE HANSEN crosses into the Great Belt MPA. A ZONE_VIOLATION appears in the
   ALERTS tab, severity CRITICAL. Say: "A protected-area entry. Inline, in
   milliseconds. On its own this already scores 90 and is critical, so the boat is
   worth a look before it does anything else."

3. GRETHE HANSEN works a slow weaving track inside the MPA. A TRAWLING alert
   appears in LOGS with the pattern description, for example "slow 2.0 to 4.5 kn
   with course spread 52 deg". Say: "This is not a straight transit. Slow speed
   and constant course changes is the signature of towing gear. The engine flags
   fishing behaviour only where fishing is illegal, so this is fishing where it
   should not be." If you ran `scenario-dk-weather`, add: "And the sea state is on
   the alert. Calm water here, so the behaviour is not weather induced, and the
   confidence is high. In heavy swell we would hold this lower, because swell
   makes any ship slow and weave."

4. NORTH STAR goes dark near the Anholt wind farm, repeatedly. Each DARK_EVENT
   appears in LOGS with a dead-reckoning cone and an intercept solution. HDMS
   Diana (P1) is the feasible interceptor with an ETA of roughly ten minutes; the
   far patrol is honestly marked not feasible. Say: "It stops transmitting. We do
   not wait hours for a satellite to notice. In the seconds it takes to be sure
   the silence is real, we project where it drifted and we already have the
   heading and ETA for the patrol that can reach it."

5. GRETHE HANSEN dashes out of the MPA and goes dark too. A DARK_EVENT with a cone
   near the Great Belt, this time HDMS Rota (P2) feasible at roughly eight minutes.

6. The risk scores climb. Click NORTH STAR: its score has climbed LOW to ELEVATED
   to HIGH as the spoof and the repeated dark events stacked up, and it fired an
   ILLEGAL_FISHING_SUSPECTED at the HIGH crossing. Click GRETHE HANSEN: it is
   CRITICAL, and the drawer lists the exact factors, zone plus fishing plus dark,
   each with points and a timestamp. Say: "Every point is a named, timestamped
   event a human can verify on the map. We never say illegal yes or no. We rank
   suspicion with the evidence attached, so the patrol boards the right vessel
   first."

7. Point at DFDS PEARL the whole time. Say: "The ferry has been transiting the
   whole run. It never entered a zone, never spoofed, never went dark. It never
   alerts and never scores. Clean traffic stays quiet. That is the precision
   story: we are not flooding anyone with false alarms."

### The intercept caveat, said honestly

Each dark event shows a solution per patrol asset. One is usually feasible and one
is not, because a patrol far from the event cannot catch a moving target. That is
correct behaviour, not a bug. Read the ETA the screen shows; never quote a
memorised number.

---

## Act 3, Slam the firehose (throughput)

Command:

```
make firehose
```

Say: "Same engine, now fed an in-memory generator. Watch the rate rocket past
fifty thousand messages a second while the inline latency histogram holds in
single-digit milliseconds."

Then show the measured benchmark numbers beside it:

```
make bench
```

Say: "And this is the honest number. We parse the real Danish recording into
memory once, then loop the real message shapes through the exact production path
at full speed, no network in the measured loop. Millions of messages a second,
zero dropped, on this laptop." Read the number the run prints; do not quote a
memorised figure. The latest recorded figures live in `docs/BENCH_LOG.md` and on
slide 5.

---

## Latency honesty (keep this straight under questioning)

Two of the three alerts are inline and fire in single-digit milliseconds: zone
violations and spoof teleports. The dark event is different by definition.
Detecting that a vessel stopped transmitting means waiting long enough to be sure
the silence is real: a one-second sweep plus the silence threshold, about twelve
seconds for a fast vessel in this scenario. Never call the dark alert a
millisecond alert. See `docs/pitch/latency-honesty.md`.

---

## Quick reference

| Angle | Where it shows | Command |
|-------|----------------|---------|
| Real data at scale | map + LOGS | `make demo` / `make demo-fast` |
| Zone violation | ALERTS tab (CRITICAL) | `make scenario-dk` |
| Spoof teleport | LOGS | `make scenario-dk` |
| Fishing pattern (trawl) | LOGS, with sea state if `-weather` | `make scenario-dk-weather` |
| Dark event, cone, intercept | LOGS, cone on map | `make scenario-dk` |
| Risk score climb and tiers | Score drawer (click a vessel) | `make scenario-dk` |
| No false alarms (control) | DFDS PEARL stays quiet | `make scenario-dk` |
| 50k throughput, latency HUD | metrics HUD | `make firehose`, `make bench` |

## Troubleshooting

- No zones on the map: the dashboard fetches zones from the engine, so start the
  engine, then reload. It retries automatically.
- Port already in use: an old engine is still running. Stop it, or pass a
  different `-addr`.
- Scenario shows only a spoof and nothing else: you started it without the Danish
  zones. Use `make scenario-dk` (it passes the right zone and patrol files);
  scenario mode otherwise defaults to the Gulf of Mannar zone file and your Danish
  coordinates match nothing.
- Weather badge shows offline: no internet, which is fine. The scenario runs
  identically; only the sea-state label on the fishing alert is absent.
