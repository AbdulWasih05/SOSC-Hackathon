package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	pollInterval   = 5 * time.Minute
	requestTimeout = 3 * time.Second
	marineBaseURL  = "https://marine-api.open-meteo.com/v1/marine"
)

// Poller refreshes a Cache from the Open-Meteo Marine API on a fixed interval.
// It is best-effort and fails open: a fetch error is logged and the last-good
// reading is retained. It does nothing unless Run is called, and the whole
// subsystem is constructed only behind the -weather flag.
type Poller struct {
	cache    *Cache
	lat, lon float64
	client   *http.Client
}

// NewPoller builds a poller for a single region point (typically the monitored
// zones' centroid) writing into cache.
func NewPoller(cache *Cache, lat, lon float64) *Poller {
	return &Poller{
		cache:  cache,
		lat:    lat,
		lon:    lon,
		client: &http.Client{Timeout: requestTimeout},
	}
}

// Run fetches once immediately, then every pollInterval, until ctx is done.
// It blocks; run it in its own goroutine.
func (p *Poller) Run(ctx context.Context) {
	p.fetchOnce(ctx)
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.fetchOnce(ctx)
		}
	}
}

func (p *Poller) fetchOnce(ctx context.Context) {
	waveM, err := p.fetch(ctx)
	if err != nil {
		if ctx.Err() == nil {
			log.Warn().Err(err).Msg("weather fetch failed; keeping last-good reading (fail-open)")
		}
		return
	}
	p.cache.set(waveM, time.Now().UnixMilli())
}

// marineResponse mirrors the flat Open-Meteo Marine "current" payload. Only
// wave_height is used; the other fields are requested for context and ignored.
type marineResponse struct {
	Current struct {
		WaveHeight      float64 `json:"wave_height"`
		WindWaveHeight  float64 `json:"wind_wave_height"`
		SwellWaveHeight float64 `json:"swell_wave_height"`
	} `json:"current"`
}

func (p *Poller) fetch(ctx context.Context) (float64, error) {
	url := fmt.Sprintf("%s?latitude=%.4f&longitude=%.4f&current=wave_height,wind_wave_height,swell_wave_height",
		marineBaseURL, p.lat, p.lon)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("open-meteo status %d", resp.StatusCode)
	}
	var mr marineResponse
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return 0, err
	}
	return mr.Current.WaveHeight, nil
}
