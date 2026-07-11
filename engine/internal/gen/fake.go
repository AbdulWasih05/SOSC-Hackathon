// Package gen holds the synthetic message generators. fake.go is the H1
// contract emitter: it produces schema-valid alerts and metrics so the frontend
// can build against the frozen contracts before the real engine exists. It is
// NOT the firehose or scenario generator (those land at H8/H16) and touches no
// hot-path code.
package gen

import (
	"context"
	"fmt"
	"math"
	"time"

	"palkwatch/internal/alert"
)

// Broadcaster is the subset of api.Hub the emitter needs.
type Broadcaster interface {
	Broadcast(v any)
}

// RunFake emits one metrics frame per second and one alert every few seconds,
// cycling through the three alert kinds, until ctx is cancelled. These are
// hand-built demo values, not measurements; the real numbers come from the
// engine and benchmark.
func RunFake(ctx context.Context, b Broadcaster) {
	const alertPeriod = 3 * time.Second

	metricsTick := time.NewTicker(1 * time.Second)
	alertTick := time.NewTicker(alertPeriod)
	posTick := time.NewTicker(500 * time.Millisecond)
	defer metricsTick.Stop()
	defer alertTick.Stop()
	defer posTick.Stop()

	kinds := []string{alert.KindZone, alert.KindSpoof, alert.KindDark}
	vessels := newFakeVessels(40)

	var (
		seq       uint64
		ingested  uint64
		processed uint64
		dropped   uint64
		alerts    uint64
		ki        int
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-metricsTick.C:
			// Pretend we sustained ~52k/s this second.
			ingested += 52000
			processed += 51950
			dropped += 50
			b.Broadcast(fakeMetrics(seq, ingested, processed, dropped, alerts))
		case <-alertTick.C:
			seq++
			alerts++
			k := kinds[ki%len(kinds)]
			ki++
			b.Broadcast(fakeAlert(seq, k))
		case <-posTick.C:
			vessels.advance()
			b.Broadcast(vessels.featureCollection())
		}
	}
}

// fakeVessels drift a handful of synthetic vessels around the Gulf of Mannar so
// the frontend can build the map layer before the real engine exists.
type fakeVessels struct {
	mmsi    []uint32
	lat     []float64
	lon     []float64
	heading []float32
	speed   []float32
}

func newFakeVessels(n int) *fakeVessels {
	fv := &fakeVessels{}
	for i := 0; i < n; i++ {
		fv.mmsi = append(fv.mmsi, 419000000+uint32(i))
		fv.lat = append(fv.lat, 8.8+float64(i%10)*0.05)
		fv.lon = append(fv.lon, 79.0+float64(i%13)*0.04)
		fv.heading = append(fv.heading, float32((i*29)%360))
		fv.speed = append(fv.speed, float32(4+i%16))
	}
	return fv
}

// advance nudges each vessel one step along its heading (deterministic, no
// randomness). It bounces vessels back inside a loose bounding box.
func (fv *fakeVessels) advance() {
	for i := range fv.mmsi {
		h := float64(fv.heading[i]) * math.Pi / 180
		step := float64(fv.speed[i]) * 0.00004
		fv.lat[i] += step * math.Cos(h)
		fv.lon[i] += step * math.Sin(h)
		if fv.lat[i] < 8.6 || fv.lat[i] > 9.5 || fv.lon[i] < 78.7 || fv.lon[i] > 80.0 {
			fv.heading[i] = float32((int(fv.heading[i]) + 150) % 360)
		}
	}
}

func (fv *fakeVessels) featureCollection() alert.PositionMsg {
	feats := make([]alert.Feature, len(fv.mmsi))
	for i := range fv.mmsi {
		feats[i] = alert.NewFeature(fv.mmsi[i], fv.lat[i], fv.lon[i], fv.speed[i], fv.heading[i])
	}
	return alert.PositionMsg{Type: "positions", FC: alert.FeatureCollection{Type: "FeatureCollection", Features: feats}}
}

func fakeMetrics(seq, ingested, processed, dropped, alerts uint64) alert.Metrics {
	// Deterministic wiggle so the HUD looks alive without randomness.
	w := float64(seq % 7)
	return alert.Metrics{
		Type:           "metrics",
		IngestedTotal:  ingested,
		ProcessedTotal: processed,
		DroppedTotal:   dropped,
		RatePerS:       52000 + w*130,
		LatencyUs: alert.LatencyStats{
			InlineP50: 620 + w*40,   // ~0.6 ms
			InlineP99: 3800 + w*160, // ~4 ms, single-digit ms
			SweepP50:  240 + w*30,   // sweep-pass processing time
			SweepP99:  1450 + w*90,
		},
		ActiveVessels: 50000 + int(seq%97),
		AlertsTotal:   alerts,
	}
}

func fakeAlert(seq uint64, kind string) alert.Envelope {
	a := alert.Alert{
		ID:       fmt.Sprintf("a-%06d", seq),
		Kind:     kind,
		Severity: alert.SeverityHigh,
		MMSI:     419000000 + uint32(seq%9000),
		Name:     "SYNTHETIC VESSEL",
		TsMs:     time.Now().UnixMilli(),
		Lat:      9.05 + float64(seq%20)*0.01,
		Lon:      79.10 + float64(seq%25)*0.01,
		ZoneID:   "gulf-of-mannar-mnp",
		Detail:   map[string]any{},
	}

	switch kind {
	case alert.KindSpoof:
		a.Detail["implied_speed_kn"] = 74.2
	case alert.KindDark:
		a.Severity = alert.SeverityCritical
		a.Detail["silence_s"] = 68
		a.Cone = &alert.Cone{
			Lat:        a.Lat,
			Lon:        a.Lon,
			HeadingDeg: 210,
			SpreadDeg:  30,
			RadiusM:    1200 + float64(seq%10)*80,
		}
		a.Intercepts = []alert.Intercept{
			{PatrolID: "P1", HeadingDeg: 168, EtaS: 2460, Feasible: true},
			{PatrolID: "P2", HeadingDeg: 205, EtaS: 3900, Feasible: false},
		}
	}
	return alert.Envelope{Type: "alert", Alert: a}
}
