// Package weather is an optional, best-effort sea-state context layer.
//
// A background Poller fetches wave height from the Open-Meteo Marine API and
// stores the last-good reading in an in-memory Cache. The detection path only
// ever READS the cache (never the network), and only at alert-emit time, so the
// per-message hot path and the 50k throughput floor are untouched. Everything
// here fails open: if the API is unreachable the cache reports "unavailable" and
// detection behaves exactly as if weather were off. The whole subsystem is
// constructed only behind the -weather flag; the default build never touches it.
//
// This package deliberately depends on the standard library only (plus zerolog,
// already in the freeze) so it adds no new dependency and stays trivially
// deletable.
package weather

import (
	"sync"
	"time"
)

// StalenessMs bounds how old a reading may be before it is treated as
// unavailable. It is comfortably larger than the poll interval so a single
// failed fetch does not blank the layer.
const StalenessMs int64 = 30 * 60 * 1000 // 30 minutes

// Cache holds the last-good sea state for the monitored region. v1 stores a
// single regional value (the monitored zones' centroid); SeaStateAt already
// accepts lat/lon so a coarse gridded cache is a drop-in replacement later
// without changing callers. All fields are guarded by mu.
type Cache struct {
	mu        sync.RWMutex
	waveM     float64
	updatedMs int64
	ok        bool
}

// NewCache returns an empty cache that reports "unavailable" until the first
// successful fetch.
func NewCache() *Cache {
	return &Cache{}
}

// set records a fresh reading. Called by the poller after a successful fetch.
func (c *Cache) set(waveM float64, nowMs int64) {
	c.mu.Lock()
	c.waveM = waveM
	c.updatedMs = nowMs
	c.ok = true
	c.mu.Unlock()
}

// snapshot returns the stored reading and whether one has ever been recorded.
func (c *Cache) snapshot() (waveM float64, updatedMs int64, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.waveM, c.updatedMs, c.ok
}

// SeaStateAt returns the cached wave height in meters for the given position and
// whether a usable, non-stale reading exists. v1 ignores lat/lon (single
// regional value). ok is false until the first successful fetch and once the
// reading goes stale. This is called only at alert-emit time, never per message.
func (c *Cache) SeaStateAt(lat, lon float64) (float64, bool) {
	waveM, updatedMs, ok := c.snapshot()
	if !ok || time.Now().UnixMilli()-updatedMs > StalenessMs {
		return 0, false
	}
	return waveM, true
}

// StatusMsg is the additive websocket frame describing current sea state, sent
// about once a second for the dashboard badge. Frontends that predate weather
// ignore unknown "type" values, so this does not change the frozen contract.
type StatusMsg struct {
	Type        string  `json:"type"` // always "weather"
	WaveHeightM float64 `json:"wave_height_m"`
	UpdatedMs   int64   `json:"updated_ms"`
	Available   bool    `json:"available"`
}

// StatusMsg builds the current status frame. Available is true only when a
// reading exists and is not stale, so the badge degrades to "offline" on its own
// if the poller stops succeeding.
func (c *Cache) StatusMsg(nowMs int64) StatusMsg {
	waveM, updatedMs, ok := c.snapshot()
	return StatusMsg{
		Type:        "weather",
		WaveHeightM: waveM,
		UpdatedMs:   updatedMs,
		Available:   ok && nowMs-updatedMs <= StalenessMs,
	}
}
