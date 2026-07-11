# Judge Q&A — Palk Watch

Answers for the judging checkpoints. Judges are recent alumni engineers: they
reward working demos, honest benchmarks, and clean repos; they punish enterprise
cosplay and unverifiable claims. Every answer here is one we can back with code
or a measured number. If we cannot back it, we do not say it.

No em dashes. Say the honest number.

---

## Key numbers (say these)

Machine: Windows 11 dev laptop, 8 CPUs, Go 1.26.5. The locked 60s methodology
run happens on the team's weakest Linux laptop before the final slide.

| Metric | Number | Notes |
|--------|--------|-------|
| Throughput floor | 50,000 msgs/sec | The problem-statement requirement |
| Real-data throughput | 8,681,066 msgs/sec | `make bench`, real Danish AIS (aisdk), 2,396 distinct vessels, ~174x the floor |
| Synthetic throughput | 8,126,083 msgs/sec | `make bench-firehose`, 98,064 distinct vessels (fuller state table) |
| Dropped messages | 0 | At both benchmark rates |
| Inline alert latency | p50 768 us, p99 3072 us | Zone + spoof, per message (single-digit ms) |
| Dark-event detection | 1s sweep + silence threshold | Absence, never a millisecond claim; ~60s for a slow vessel |
| Sweep scan cost | p50 3072 us, p99 6144 us | Cost of one full-table dark scan over ~98k vessels, far inside the 1s tick |

One-line version: "50k floor, met at 8.7 million per second on real Danish AIS,
zero dropped, single-digit millisecond inline latency, on a laptop."

---

## The benchmark

**Q: How did you test the 50,000/sec throughput claim?**

`make bench` runs the real engine against **real Danish AIS data**. We parse the
aisdk recording (a 3 GB Danish Maritime Authority file) into memory once before
the timer starts, then loop those real messages through the exact production
path: batched ingest (512-message batches, 5ms flush) into an 8-worker pool,
inline geofence and spoof checks per message, and the 1-second dark sweep. No
network and no disk are in the measured loop, so we are timing the engine, not
the OS socket stack. Important: this is not a replay rate. Real AIS arrives at
tens of messages/sec; we loop the real message shapes at full speed to measure
the engine's actual capacity.

**Q: What did it measure?**

8,681,066 messages/sec sustained over 10 seconds on real Danish AIS (Windows 11,
8 CPUs, Go 1.26.5), 2,396 distinct real vessels, 0 dropped, inline latency p50
768 microseconds / p99 3072 microseconds. That is roughly 174x the 50,000/sec
floor, on real data. The 60-second locked methodology run lands the headline
number on the team's weakest Linux laptop.

**Q: Why not just replay the real data at real speed?**

Because that measures the data's arrival rate, not the engine. Real AIS is tens
of messages/sec; even a 300x demo replay is only about 55,000/sec. To prove the
engine's ceiling honestly you have to saturate it, so we loop the real messages
in memory as fast as the workers consume them.

**Q: Do you have a synthetic number too?**

Yes. `make bench-firehose` runs a synthetic feed of about 98,000 distinct
vessels crafted to keep the geofence firing. It sustains a similar rate against a
much fuller state table (8.1M/sec, 98k vessels). We report both: the real-data
number proves the engine handles real message shapes, the firehose number proves
it holds up with a far larger concurrent vessel table.

**Q: Is that a burst peak?**

No. The rate is processed-count divided by full elapsed time over the whole
window, not a cherry-picked spike. The bench prints ingested, processed, and
dropped counts every run so the number is auditable.

**Q: Why should we trust it?**

The bench prints its own machine header (Go version, OS, CPU count, worker
count, buffer depth) and all three counters (ingested / processed / dropped)
every run. It is the same code path the live demo uses. BENCH_LOG.md records
every run with its commit hash; a regression over 15 percent blocks merge.

**Honest caveat we volunteer:** the race detector needs a C compiler and has not
run on this Windows box. It runs on the Linux bench machine before the number is
marked LOCKED.

---

## Latency (the trap question)

**Q: You said millisecond alerts. Is that true for all three?**

For two of them, yes. Zone violations and spoof teleports are checked inline,
per message, and fire in single-digit milliseconds (p99 around 3ms, visible live
on the histogram). Dark events are different by definition: detecting that a
vessel STOPPED transmitting means waiting long enough to be statistically
certain it went silent, not on one missed ping. That is a 1-second sweep plus
the silence threshold (about 60 seconds for a slow vessel). We never call the
dark alert a millisecond alert. Absence cannot be detected instantly, and
pretending otherwise would be the dishonest version of this product.

**Q: Why 6x the reporting interval before you call it dark?**

AIS reporting cadence is defined by speed class (10s at 0 to 14 kn, 6s at 14 to
23 kn, 2s above 23 kn). Six missed intervals is the line between a normal gap
and a vessel that has gone dark. On the real feed we raise it to 10x because
community AIS coverage is gappy and we refuse to cry wolf.

---

## Architecture and constraints

**Q: Where is your database?**

Memory is the database. One Go process, no Kafka, no Redis, no Postgres. This is
deliberate, not a shortcut: a coast guard station can run this on hardware it
already owns, offline, with no cloud dependency and no data leaving the building.
Sovereignty and deployability are the pitch, not a limitation.

**Q: How do you hold 98,000 vessels in memory without choking?**

Vessel state is a flat value struct (fixed-size numeric fields only, no strings
or pointers) held in sharded maps. Names and metadata live in a separate cold
map touched only when an alert fires. The hot loop does zero per-message
allocation and zero per-message logging; counters are atomic and aggregated once
per second.

**Q: What happens under overload?**

We measure and report dropped messages honestly. At benchmark rates we drop 0.
The ingest path batches with a mandatory 5ms flush timer so latency stays
bounded even when a batch is not full.

---

## The risk score (post-P0, once shipped)

**Q: Does this tell me a ship is fishing illegally?**

No system can say that from AIS alone, and we do not claim it. We output a 0 to
100 suspicion score where every point is a readable fact: a zone violation, a
dark event, a spoof, a fishing-speed movement pattern. Authorities verify; we
prioritize which vessel a patrol boards first. Tiers are LOW, ELEVATED, HIGH,
CRITICAL. The ceiling wording we ever use is "illegal fishing suspected,
evidence attached."

**Q: How are the weights set?**

Hand-calibrated and validated against scripted scenarios, adjustable in a config
file. Production calibration against real enforcement outcomes is the roadmap. We
are explicit that this is expert-tuned, not learned, which is exactly why every
point is explainable. Incumbents ship ML scores with accuracy caveats; we ship a
score you can read line by line.

**Q: Why a 5-second sweep and not instant?**

Scoring is a judgment cycle, not a reflex. It runs on a 5-second sweep over only
the vessels that already have a nonzero factor, so it adds nothing to the
per-message hot path. The benchmark proves the engine still holds full ingest
rate with scoring active.

---

## Scope honesty

**Q: Why not detect transshipment / rendezvous?**

Designed and spec'd (PRD Appendix A), cut for scope honesty in a 24-hour build.
We would rather ship three alert types that work than five that half-work. The
cut slide lists everything we deliberately left out.

**Q: What is real vs scripted in the demo?**

Act 1 is real recorded Danish AIS data flowing through the live engine. Act 2 is
a scripted scenario so the story beats (zone entry, going dark, intercept) land
on cue; the coordinates and timings are honest AIS event times, so the implied
speeds and alerts are real engine output, not faked overlays. Act 3 is the
firehose stress feed for the throughput moment.
