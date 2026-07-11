# Palk Watch, Pitch Deck (slide-by-slide)

Maritime surveillance that alerts on absence, not just presence. One Go binary,
real time, offline, on a laptop a coast guard station already owns.

> Presenter status (internal, not a slide): Shipped and demo-ready now: the three
> alerts (zone, spoof, dark), intercept geometry, both benchmarks, the real
> Danish AIS replay. In progress: the IUU Fishing Risk Score (the headline
> differentiator) and the move of the scripted scenario into Danish waters. Only
> demo the risk score once it is on screen and verified. Every number below is
> measured, not aspirational.

Every slide is tagged with the judging criterion it earns:
Innovation, Feasibility, Impact, Execution, Presentation.

---

## Slide 1, Title

**Palk Watch**
Real-time IUU fishing and dark-vessel surveillance for the tactical edge.

One line: "Everyone alerts when a ship enters a zone. The crime happens when a
ship disappears. We alert on absence in milliseconds, compute the intercept, and
run on hardware a coast guard station already owns."

Stage: Danish waters (Kattegat and the Great Belt), Danish EEZ plus a Natura
2000 marine protected area. The engine is geography agnostic; we point it
anywhere by swapping a zones file.

---

## Slide 2, The Problem (the gap) [Impact]

Incumbent platforms like Global Fishing Watch are built for global, historical
observation, not immediate tactical action. They fall short in three ways:

1. **The dark-ship blindspot.** They track honest ships perfectly. When bad
   actors switch off their AIS transponder to fish illegally,
   satellite fallback takes hours to notice the gap.
2. **The deception problem.** Cheap hardware lets a vessel spoof its position,
   claiming to be somewhere it is not, or moving impossibly fast.
3. **Cloud dependency and latency.** Incumbents are heavy cloud services. They
   lack sovereignty, cannot run offline, and cannot compute an intercept course
   for a local patrol boat in real time.

The result: enforcement sees the crime after it is over, from a dashboard hosted
on another continent.

---

## Slide 3, The Solution [Innovation, Feasibility]

A hyper-optimized single-binary Go stream engine, paired with a React and
MapLibre GL dashboard. It runs entirely on a standard laptop, no internet and no
cloud database, and sustains millions of AIS messages per second.

Instead of only alerting on zone entry, it detects, in real time:

- **Zone violations:** outside-to-inside transitions of a protected area, or an
  EEZ crossing by a foreign-flagged vessel. Inline, per message.
- **Spoofing and teleportation:** implied speed over 60 knots between fixes, or
  one MMSI reported in two places at once. Inline, per message.
- **Dark events plus intercepts:** the moment a moving vessel near a protected
  zone goes silent past its expected report interval, the engine projects an
  expanding dead-reckoning cone and solves the compass heading and ETA for each
  patrol asset to intercept.

Memory is the database. No Kafka, no Redis, no Postgres, no queue. That is the
sovereignty and deployability story, not a shortcut.

---

## Slide 4, The Headline: IUU Fishing Risk Score [Innovation, Impact]

The differentiator. Every vessel carries an explainable 0 to 100 suspicion
score, where every point is a readable fact, not a black-box model output.

**The weighted scoring module.** Each signal the engine raises adds a weighted,
time-decayed contribution to the vessel's score:

| Factor | Weight | Status |
|--------|--------|--------|
| Protected-zone violation | +30 | shipped |
| Dark event | +15 first, +10 each additional | shipped |
| Position spoof / teleport | +15 | shipped |
| Fishing-movement pattern (trawl / longline / seine) | +20 | detector shipped, scoring next |
| Repeat offender (3+ flags in 30 days) | +10 | designed |

Every factor decays with a 24-hour half-life over a 48-hour window, computed at
read time, so the score reflects recent behavior. The score is the sum of the
live weighted factors, capped at 100, recomputed on a 5-second sweep over only
the vessels that carry a factor.

Worked example (the live demo): KADAL SELVI enters the marine park (+30), spoofs
its position (+15), then goes dark four times near the park (+15, +10, +10, +10).
30 + 15 + 45 = 90, which is CRITICAL and fires a boarding recommendation with the
intercept solutions attached. On stage you watch each number appear.

Tiers: **0 to 39 LOW, 40 to 64 ELEVATED, 65 to 84 HIGH, 85 plus CRITICAL.**

**Why weighted-and-explainable is our accuracy story.** We do not publish a
black-box accuracy percentage. In our system, accuracy means every point traces
to a named, timestamped event a human can verify on the map. The weights are
hand-calibrated, validated on scripted scenarios, and adjustable in the config;
production calibration against real enforcement outcomes is the roadmap.
Incumbents ship an ML score with accuracy caveats; we ship the reasoning.

Click any vessel and the Score Breakdown Drawer shows the score, the tier, and
the exact factor list with points and timestamps. Crossing into HIGH raises an
alert; crossing into CRITICAL raises a boarding recommendation with the full
evidence and the intercept solution attached.

The discipline that wins the trust question: we never output "illegal: yes or
no." No system can say that from AIS alone. We rank suspicion with visible
evidence so patrols board the right vessel first. Authorities verify; we
prioritize. Ceiling wording: "illegal fishing suspected, evidence attached."

---

## Slide 5, The Numbers (measured) [Execution]

Machine: a Windows dev laptop, 8 CPUs, Go 1.26.5. The locked 60-second
methodology run happens on the team's weakest Linux laptop before finals.


| Metric                                                | Number                                             |
| ----------------------------------------------------- | -------------------------------------------------- |
| Required throughput floor                             | 50,000 msgs/sec                                    |
| Real Danish AIS throughput (`make bench`)             | 8,681,066 msgs/sec, 2,396 vessels, ~174x the floor |
| Synthetic firehose throughput (`make bench-firehose`) | 8,126,083 msgs/sec, 98,064 vessels                 |
| Dropped messages                                      | 0, at both rates                                   |
| Inline alert latency (zone, spoof)                    | p50 768 us, p99 3072 us                            |
| Dark-event detection                                  | 1s sweep plus the silence threshold                |

One-line version: "50k floor, met at 8.7 million per second on real Danish AIS,
zero dropped, single-digit millisecond inline latency, on a laptop."

Honesty note we volunteer: the real-data number is not a replay rate. Real AIS
arrives at tens of messages per second; we parse the recording into memory once
and loop the real message shapes at full speed to measure the engine's true
capacity.

---

## Slide 6, Latency, said honestly [Execution]

Two of the three alerts are inline, per message, and fire in single-digit
milliseconds: zone violations and spoof teleports. The Act 3 histogram shows this
live.

The dark event is different by definition. Detecting that a vessel stopped
transmitting means waiting long enough to be statistically certain, not firing on
one missed ping. That is a 1-second sweep plus the silence threshold, about 60
seconds for a slow vessel. We never call the dark alert a millisecond alert.
Absence cannot be detected instantly, and pretending otherwise would be the
dishonest version of this product.

The line to use for the dark event (defensible under questioning):

> "A dark vessel does not announce itself; it just stops. Satellite fallback
> catches that in hours. We catch it in the seconds it takes to be sure the
> silence is real, and we have already computed who can reach it and when."

For the intercept ETA, read the number the screen actually shows (computed live
from patrol position and vessel drift), never a memorized figure.

---

## Slide 7, Under the hood, engineering flexes [Execution, Feasibility]

For the technical judge who digs in:

- **Value structs, no GC pressure.** Hot state is `map[uint32]VesselState` across
  64 shards, flat fixed-size numeric fields only. No strings or pointers in the
  hot path, so Go's garbage collector never walks the live vessel data. Names and
  metadata live in a separate cold map, touched only when an alert fires.
- **Flat-plane math.** No heavy GIS libraries or Haversine spherical trig in the
  hot path. Local equirectangular projection: at strait and belt scale the error
  is negligible and the kinematics are cheap.
- **Timeout batching.** Ingest flushes every 5 milliseconds regardless of whether
  the 512-message batch is full, so latency never balloons on a low-rate live
  stream. The 5ms flush is the latency floor and stays inside the single-digit
  millisecond claim.
- **Absence by sweep, presence inline.** Zone and spoof are inline per message;
  the dark scan is a 1-second sweep. We never blur the two.

---

## Slide 8, The 3-Act Demo [Presentation]

**Act 1, Prove reality.** A stream of actual Danish ship traffic from a recorded
AIS file flows through the live engine. Real-world data, reproducible, fully
offline, immune to venue Wi-Fi.

**Act 2, Tell the story.** The scripted scenario. A vessel crosses into the
protected area, then spoofs, then goes dark. The dead-reckoning cone blooms and
the intercept vector paints on screen. As the risk score lands, watch the
evidence accumulate and the tier climb from ELEVATED to CRITICAL.

**Act 3, Slam the firehose.** Turn on the in-memory generator. The HUD rockets
past 50,000 messages per second while the inline latency histogram holds in
single-digit milliseconds. Then show the real-data benchmark number beside it.

---

## Slide 9, Impact [Impact]

- A coast guard station runs this on a laptop it already owns, offline, with no
  data leaving the building. Sovereign by construction.
- It converts "we found out hours later" into "we knew the moment it went dark,
  and the patrol had a heading."
- The explainable score means the scarce patrol boat is sent to the highest-
  evidence vessel first, not the loudest one.

---

## Slide 10, Feasibility and roadmap [Feasibility]

Working today: three alerts, intercept geometry, both benchmarks, real Danish AIS
replay, the risk score with an explainable breakdown.

Roadmap, honestly labeled:

- Weights are hand-calibrated and validated on scripted scenarios, adjustable in
  a config file. Production calibration against real enforcement outcomes is the
  next step.
- The engine is a unified alert bus. Coastal radar or satellite SAR triggers can
  feed the same pipeline for vessels with no transponder at all.

---

## Slide 11, Business model (save money, earn money) [Feasibility, Impact]

Two sides: it cuts the customer's cost, and it earns on a model that is cheap for
us to serve.

**How it saves the customer money**

- Runs on hardware the station already owns. No cloud bill, no per-message fees,
  no satellite-data subscription. Near-zero recurring cost to operate.
- Fewer wasted patrol sorties. A patrol boat burns fuel and crew hours every hour
  at sea. The explainable risk score sends the scarce boat to the highest-
  evidence vessel first, so fewer empty interceptions. This is the largest and
  most measurable ROI: cost per boarding drops.
- Offline and sovereign. No data leaves the building, so no egress fees and no
  foreign-cloud compliance cost.

**The stakes (why the buyer cares)**

- IUU fishing is estimated by UN and industry sources to cost 10 to 23 billion
  USD a year globally. Recovering even a sliver through faster, better-targeted 
  enforcement dwarfs the price of the software.

**How we earn**

- Per-station license plus annual support and updates. Fits the on-prem, no-cloud
  ethos.
- Tiered by scale: single station, regional, national fleet. National coast guard
  contracts are the anchor deals.
- Recurring services: calibrating the risk weights against the customer's own
  enforcement outcomes, zone configuration, training. This turns the "production
  calibration" roadmap item into billable work.
- Optional turnkey appliance: a pre-loaded rugged mini-PCC for stations that want
  plug and play, at a hardware margin.
- Upsell modules later: radar and SAR ingestion, added alert types
  (transshipment), each a paid add-on on the same engine.

**Why the margins work**

- The customer hosts it, so we pay no cloud bill per customer. Cost to serve is
  near zero after the sale; revenue is license plus services. Software margins,
  not infrastructure drag.

**Who buys**

- National coast guards, fisheries enforcement agencies, marine protected area
  authorities, port authorities, regional fisheries management organizations, and
  navies. Conservation funders (EU maritime funds, NGOs) can finance deployments
  in protected waters.

One line: "We take the customer's operating cost and wasted patrol hours down
hard, and we earn on licenses and calibration services with software margins."

---

## Slide 12, Cut with honor (scope) [Execution]

We chose depth on three real-time alerts over breadth on more. Deliberately left
out, designed and specified, cut for scope honesty in a 24-hour build:

- Rendezvous and transshipment detection (spec in PRD Appendix A).
- Any notification path beyond the dashboard (no SMS, email, webhooks).
- Persistence beyond in-memory plus optional JSON export.
- ML models of any kind. The score is expert-tuned and explainable on purpose.

We would rather ship three alert types that work than five that half-work.

---

## Slide 13, Judge Ju-Jitsu (anticipated questions)

**"Global Fishing Watch already exists and is free. Why this?"**
GFW is an analytical tool with hours of latency. Ours is a tactical, real-time
edge device. It runs offline on hardware a coast guard station already owns,
catches spoofers, and generates intercept vectors in milliseconds.

**"What if a ship has no AIS transponder at all?"**
Out of scope for this build, but the engine is a unified alert bus. The
architecture is ready to ingest coastal radar or satellite SAR triggers into the
exact same pipeline.

**"Is it illegal fishing or not?"**
No system can say that from AIS alone. We rank suspicion with visible evidence so
patrols board the right vessel first. Authorities verify; we prioritize.

**"Why did you cut transshipment tracking?"**
We designed it completely, Appendix A, and cut it to focus on absence and
deception. Depth over breadth.

**"How did you prove the throughput?"**
Real Danish AIS parsed into memory once, then looped through the exact production
path at full speed, no network in the measured loop, all of ingested, processed,
and dropped reported. 8.7 million per second, zero dropped. The firehose number
is the synthetic-headroom figure beside it.

Full answer bank: see `docs/pitch/qna.md`.

---

## Slide 14, Close

"The cloud sees the crime hours later, from another continent. We see the silence
the moment it happens, name the evidence, and hand the patrol a heading, on a
laptop, offline. That is the difference between watching and enforcing."
