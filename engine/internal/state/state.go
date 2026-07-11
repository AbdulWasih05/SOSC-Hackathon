// Package state holds the sharded in-memory vessel table. Memory is the
// database (no Redis, no Postgres). The hot struct is flat value semantics
// (hot-path rule 1): fixed-size numeric fields only, no strings/slices/pointers,
// so the GC mark phase never walks the hot data. Names and other metadata live
// in the cold store, touched only on alert emission.
package state

import (
	"sync"
)

// shardCount is fixed at 64 (hot-path rule 1). MMSI hashes select a shard.
const shardCount = 64

// VesselState is the flat per-vessel record. All fields are fixed-size numerics.
// ZoneMask is a bitset of the zones the vessel was last known to be inside (bit
// i == zone index i), which makes outside-to-inside transitions a cheap bit op.
// LastTsMs is the AIS event time (used for implied-speed spoof checks);
// LastSeenNs is the monotonic time the engine last processed a fix for this
// vessel (used by the dark sweep to measure silence, immune to event-time skew).
type VesselState struct {
	LastLat    float64
	LastLon    float64
	LastTsMs   int64
	LastSeenNs int64
	SpeedKn    float32
	HeadingDeg float32
	FlagCode   uint16
	ZoneMask   uint32
}

type shard struct {
	mu sync.RWMutex
	m  map[uint32]VesselState
}

// Shards is the 64-way sharded vessel table.
type Shards struct {
	shards [shardCount]shard
}

// New returns an initialized table.
func New() *Shards {
	s := &Shards{}
	for i := range s.shards {
		s.shards[i].m = make(map[uint32]VesselState)
	}
	return s
}

func (s *Shards) shardFor(mmsi uint32) *shard {
	// Knuth multiplicative hash, low bits select the shard.
	h := mmsi * 2654435761
	return &s.shards[h&(shardCount-1)]
}

// Load returns the vessel's state and whether it was present.
func (s *Shards) Load(mmsi uint32) (VesselState, bool) {
	sh := s.shardFor(mmsi)
	sh.mu.RLock()
	v, ok := sh.m[mmsi]
	sh.mu.RUnlock()
	return v, ok
}

// Update atomically reads the current state and writes the value returned by
// fn, under a single shard lock. This keeps read-decide-write races out of the
// geofence and spoof checks when two messages for one MMSI land on different
// workers. It returns the previous state and whether it existed.
func (s *Shards) Update(mmsi uint32, fn func(prev VesselState, existed bool) VesselState) (VesselState, bool) {
	sh := s.shardFor(mmsi)
	sh.mu.Lock()
	prev, existed := sh.m[mmsi]
	sh.m[mmsi] = fn(prev, existed)
	sh.mu.Unlock()
	return prev, existed
}

// Len returns the number of tracked vessels across all shards.
func (s *Shards) Len() int {
	n := 0
	for i := range s.shards {
		s.shards[i].mu.RLock()
		n += len(s.shards[i].m)
		s.shards[i].mu.RUnlock()
	}
	return n
}

// ForEach calls fn for every vessel under a read lock per shard. Used by the
// dark-event sweeper (H12). fn must not call back into Shards.
func (s *Shards) ForEach(fn func(mmsi uint32, v VesselState)) {
	for i := range s.shards {
		s.shards[i].mu.RLock()
		for mmsi, v := range s.shards[i].m {
			fn(mmsi, v)
		}
		s.shards[i].mu.RUnlock()
	}
}

// Cold is the cold metadata store (vessel names). Separate from the hot table
// so names never sit in the value structs. Touched only on alert emission.
type Cold struct {
	mu    sync.RWMutex
	names map[uint32]string
}

// NewCold returns an empty cold store.
func NewCold() *Cold {
	return &Cold{names: make(map[uint32]string)}
}

// SetName records a vessel name.
func (c *Cold) SetName(mmsi uint32, name string) {
	c.mu.Lock()
	c.names[mmsi] = name
	c.mu.Unlock()
}

// Name returns the vessel name, or "" if unknown.
func (c *Cold) Name(mmsi uint32) string {
	c.mu.RLock()
	n := c.names[mmsi]
	c.mu.RUnlock()
	return n
}
