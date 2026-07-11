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

Notes:
- Throughput is ~150x the 50k/s floor on this laptop. The constraint is met with
  large headroom; the story is latency discipline, not raw rate.
- Inline latency under the firehose is dominated by channel-buffer queueing
  depth (in-flight messages / rate), not per-message work. The `-buf` flag trades
  throughput for latency. Default 2*workers keeps p99 single-digit ms.
- Race detector (`-race`) needs cgo and did not run on this Windows box (no C
  compiler). Run it on the Linux bench machine before the benchmark is LOCKED.
- Sweep latency is not yet measured (dark-event sweep lands H12).
- H6 vs H4: adding the mandatory spoof check (one flat-plane DistanceM per
  message) costs ~20% throughput. This is inherent feature cost, not a
  regression bug; boring-first means the sqrt stays on the common path until
  pprof justifies a squared-distance fast path on a branch. Still ~120x the
  50k/s floor. Run-to-run variance on this laptop is high (5.3M-7.6M observed),
  so single-run deltas are noise; medians are quoted.
