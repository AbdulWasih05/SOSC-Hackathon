# Benchmark log

Every hot-path change records its measured number here with the commit hash. A
regression greater than 15% blocks merge (self-review item 6). Numbers are
quick-bench runs (10s) unless marked LOCKED (the 60s methodology run, H18).

Machine: Windows 11, 8 CPUs, Go 1.26.5, GOMAXPROCS=8. The methodology run for
the slide happens on the team's weakest Linux laptop at H18.

| Commit | Milestone | Dur | Buffer | msgs/sec | inline p50 | inline p99 | dropped | notes |
|--------|-----------|-----|--------|----------|-----------|-----------|---------|-------|
| 9eb6f2f | H4 | 10s | 16 (2*workers) | 7,609,906 | 1536 us | 6144 us | 0 | first engine number; geofence fires ~670k alerts |
| 9eb6f2f | H4 | 10s | 4 | 7,062,556 | 2 us | 3072 us | 0 | smaller buffer, lower latency, ~7% less throughput |
| 6622e0f | H6 (v0-boring) | 8s x3 | 16 | ~6,000,000 (median) | 1536 us | 6144 us | 0 | spoof check added; median of 3 runs 5.68M/5.99M/6.00M |
| 114dec9 | H12 | 8s x3 | 16 | ~9,000,000 (median) | 768 us | 3072 us | 0 | dark sweep added; runs 8.77M/9.47M/9.00M. Sweep latency p50 ~1.5ms, p99 3-6ms over 98k vessels |
| 616dee1 | H16/H18 | 60s | 16 | 8,523,519 | 1536 us | 3072 us | 0 | 60s sustained methodology run. 511,432,704 processed, 0 dropped, 4.58M alerts. Sweep p99 6144us. Headline number (Windows dev laptop; real LOCKED run on the Linux bench machine) |
| dcf4e53 | pre-risk-engine | 10s | 16 | 8,126,083 | 768 us | 3072 us | 0 | BASELINE before the risk-scoring engine. Tag `pre-risk-engine` (escape pod). 15% regression floor for risk slices = 6,907,170 msgs/sec. Sweep p50 3072us / p99 6144us over 98,064 vessels. |
| risk-engine | real-data bench | 10s | 16 | 8,681,066 | 768 us | 3072 us | 0 | REAL Danish AIS (`make bench`): 5M-message in-memory sample of aisdk-2025-02-27.csv, 2,396 distinct real vessels, 8,125 alerts. Real message shapes looped at full speed, not a replay rate. ~174x the 50k floor. The synthetic firehose (98k vessels) is now `make bench-firehose`. |
| risk-engine | P0 risk engine | 10s | 16 | 9,023,753 | 768 us | 3072 us | 0 | After the P0 risk-scoring engine. Real-data bench, 2,396 vessels. Risk is off in the bench (cmd/bench does not wire it); the only hot-path delta is a per-alert nil-check in the inline checks, so no regression (within run-to-run variance of the 8.68M baseline). Scoring is a 5s sweep over the scored active set, off the per-message path. |
| a97e9e3 | merge (risk + fishing) | 10s | 16 | 4,652,456 | 1536 us | 6144 us | 0 | REGRESSION after merging origin/master. The fishing-pattern detector (Trawling/Longlining/Purse Seining) runs PER MESSAGE in the hot path (FishingPattern over a per-vessel ring buffer, unconditional), roughly halving throughput (9.0M -> 4.65M) and doubling inline latency (768->1536us p50). Not caused by the risk engine (5s off-path sweep). Still 93x the 50k floor. Recommend moving fishing detection off the per-message path (sample or sweep) to restore headroom; tracked as a follow-up. |
| f03b4ce+wt | fix pass (zone-gated fishing) | 10s | 16 | 2,896,612 | 3072 us | 12288 us | 0 | CONTAMINATED RUN, do not compare: live engine + browser + vite dev server were running on the same laptop during the fix pass. Firehose source (go run ./cmd/bench -d 10s). FishingPattern is now gated on zones where fishing would be illegal (e701c65 + merge), but the firehose box sits largely inside the India EEZ with foreign runners, so the gate skips little there; the a97e9e3 regression stands for the synthetic bench until fishing moves off the per-message path or the gate narrows to MPA-only. Still 58x the 50k floor under load. Re-run clean (make bench and make bench-firehose, nothing else running) before quoting any number. |

Notes:
- Throughput is ~150x the 50k/s floor on this laptop. The constraint is met with
  large headroom; the story is latency discipline, not raw rate.
- Inline latency under the firehose is dominated by channel-buffer queueing
  depth (in-flight messages / rate), not per-message work. The `-buf` flag trades
  throughput for latency. Default 2*workers keeps p99 single-digit ms.
- Race detector (`-race`) needs cgo and did not run on this Windows box (no C
  compiler). Run it on the Linux bench machine before the benchmark is LOCKED.
- Sweep latency (H12+) is the cost of one full-table dark scan of ~98k vessels:
  p50 ~1.5ms, p99 3-6ms, far inside the 1s sweep tick. This is scan cost, not
  detection latency; dark detection latency is bounded by the 1s tick plus the
  silence threshold and is never a millisecond claim.
- H6 vs H4: adding the mandatory spoof check (one flat-plane DistanceM per
  message) costs ~20% throughput. This is inherent feature cost, not a
  regression bug; boring-first means the sqrt stays on the common path until
  pprof justifies a squared-distance fast path on a branch. Still ~120x the
  50k/s floor. Run-to-run variance on this laptop is high (5.3M-7.6M observed),
  so single-run deltas are noise; medians are quoted.
