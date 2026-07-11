package ingest

import (
	"sync"
	"time"

	"palkwatch/internal/alert"
)

// BatchSize is the number of messages per batch. One channel op per batch keeps
// channel contention irrelevant at 50k/s (hot-path rule 2).
const BatchSize = 512

// FlushInterval is the mandatory timeout flush. At low message rates (scenario
// acts run at single-digit msgs/sec) a batch would otherwise never fill and
// inline latency would explode. 5ms, never longer: at demo rates the flush
// timer is the latency floor and must stay inside the single-digit-ms claim.
const FlushInterval = 5 * time.Millisecond

// BatchHandler processes one batch. Each worker gets its own handler instance so
// per-worker scratch state needs no locking.
type BatchHandler interface {
	Handle(batch []Message)
}

// Pipeline batches messages and fans them to a worker pool over one channel.
type Pipeline struct {
	batchCh  chan *[]Message
	pool     sync.Pool
	counters *alert.Counters
	workers  int
	wg       sync.WaitGroup
}

// New returns a pipeline with the given worker count and channel buffer depth.
func New(counters *alert.Counters, workers, buffer int) *Pipeline {
	return &Pipeline{
		batchCh:  make(chan *[]Message, buffer),
		counters: counters,
		workers:  workers,
		pool: sync.Pool{New: func() any {
			s := make([]Message, 0, BatchSize)
			return &s
		}},
	}
}

func (p *Pipeline) getBuf() *[]Message {
	b := p.pool.Get().(*[]Message)
	*b = (*b)[:0]
	return b
}

// Start launches the worker pool. makeHandler is called once per worker so each
// worker owns private scratch. Call the returned stop function, or close the
// input via RunFirehose/RunSource, to drain.
func (p *Pipeline) Start(makeHandler func() BatchHandler) {
	for i := 0; i < p.workers; i++ {
		h := makeHandler()
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for b := range p.batchCh {
				h.Handle(*b)
				p.pool.Put(b)
			}
		}()
	}
}

// Wait blocks until all workers have drained the channel after it is closed.
func (p *Pipeline) Wait() { p.wg.Wait() }

// RunFirehose feeds pre-generated messages as fast as the workers consume, for
// the given duration, looping over msgs. This is the benchmark producer: no
// per-message channel op, only per-batch sends, so it measures true processing
// throughput under backpressure (the send blocks when workers fall behind).
// It closes the channel when done. Returns the number of messages submitted.
func (p *Pipeline) RunFirehose(msgs []Message, dur time.Duration) uint64 {
	deadline := time.Now().Add(dur)
	var ingested uint64
	i := 0
	buf := p.getBuf()
	for time.Now().Before(deadline) {
		for len(*buf) < BatchSize {
			m := msgs[i]
			i++
			if i == len(msgs) {
				i = 0
			}
			m.IngestNs = NowNs()
			*buf = append(*buf, m)
			ingested++
		}
		p.counters.Ingested.Add(uint64(len(*buf)))
		p.batchCh <- buf
		buf = p.getBuf()
	}
	if len(*buf) > 0 {
		p.counters.Ingested.Add(uint64(len(*buf)))
		p.batchCh <- buf
	}
	close(p.batchCh)
	return ingested
}

// RunSource is the live/scenario producer. It batches messages arriving on src
// and flushes on batch-full OR the mandatory 5ms ticker, so a partly filled
// batch is never held longer than FlushInterval. Closes the batch channel when
// src closes.
func (p *Pipeline) RunSource(src <-chan Message) {
	ticker := time.NewTicker(FlushInterval)
	defer ticker.Stop()
	buf := p.getBuf()
	flush := func() {
		if len(*buf) == 0 {
			return
		}
		p.counters.Ingested.Add(uint64(len(*buf)))
		p.batchCh <- buf
		buf = p.getBuf()
	}
	for {
		select {
		case m, ok := <-src:
			if !ok {
				flush()
				close(p.batchCh)
				return
			}
			m.IngestNs = NowNs()
			*buf = append(*buf, m)
			if len(*buf) == BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
