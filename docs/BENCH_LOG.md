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

Notes:
- Throughput is ~150x the 50k/s floor on this laptop. The constraint is met with
  large headroom; the story is latency discipline, not raw rate.
- Inline latency under the firehose is dominated by channel-buffer queueing
  depth (in-flight messages / rate), not per-message work. The `-buf` flag trades
  throughput for latency. Default 2*workers keeps p99 single-digit ms.
- Race detector (`-race`) needs cgo and did not run on this Windows box (no C
  compiler). Run it on the Linux bench machine before the benchmark is LOCKED.
- Sweep latency is not yet measured (dark-event sweep lands H12).
