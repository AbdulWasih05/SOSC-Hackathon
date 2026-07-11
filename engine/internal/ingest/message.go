// Package ingest defines the wire message and the batching pipeline that feeds
// the worker pool. Messages are grouped into fixed-size batches so there is one
// channel operation per batch, not per message (hot-path rule 2).
package ingest

import "time"

// epoch anchors all in-process latency stamps. time.Since(epoch) yields a
// monotonic nanosecond duration, so ingest-to-emit latency is immune to wall
// clock changes.
var epoch = time.Now()

// NowNs returns monotonic nanoseconds since process start.
func NowNs() int64 { return int64(time.Since(epoch)) }

// Message is one AIS position report as it flows through the engine. It is a
// flat value type (MMSI is uint32, hot-path rule 7). IngestNs is stamped when
// the message enters the pipeline and is read at alert emission to measure
// inline latency.
type Message struct {
	MMSI       uint32
	Lat        float64
	Lon        float64
	SpeedKn    float32
	HeadingDeg float32
	TsMs       int64
	FlagCode   uint16
	IngestNs   int64
}
