package gen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
)

// scenarioBaseTs is the AIS event-time epoch for scenario frames. Frame TMs are
// offsets from it; consecutive frames' event times drive the spoof check.
const scenarioBaseTs int64 = 1720000000000

// ScenarioVessel names a vessel and its flag for the cold store and geofence.
type ScenarioVessel struct {
	MMSI uint32 `json:"mmsi"`
	Name string `json:"name"`
	Flag string `json:"flag"`
}

// ScenarioFrame is one scripted AIS position for a vessel at time TMs (ms from
// scenario start).
type ScenarioFrame struct {
	TMs        int64   `json:"t_ms"`
	MMSI       uint32  `json:"mmsi"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	SpeedKn    float32 `json:"speed_kn"`
	HeadingDeg float32 `json:"heading_deg"`
}

// Scenario is a deterministic demo timeline. A vessel that simply stops having
// frames goes dark; the sweeper detects the absence.
type Scenario struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Speed       float64          `json:"speed"` // playback multiplier; 0/absent means 1x
	Vessels     []ScenarioVessel `json:"vessels"`
	Frames      []ScenarioFrame  `json:"frames"`
}

// LoadScenario reads and validates a scenario file.
func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sc Scenario
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("parse scenario: %w", err)
	}
	if len(sc.Frames) == 0 {
		return nil, fmt.Errorf("scenario %s has no frames", path)
	}
	if sc.Speed <= 0 {
		sc.Speed = 1
	}
	sort.SliceStable(sc.Frames, func(i, j int) bool { return sc.Frames[i].TMs < sc.Frames[j].TMs })
	return &sc, nil
}

// FlagCodes maps each vessel MMSI to its compact flag code.
func (sc *Scenario) FlagCodes() map[uint32]uint16 {
	m := make(map[uint32]uint16, len(sc.Vessels))
	for _, v := range sc.Vessels {
		m[v.MMSI] = geo.CountryCode(v.Flag)
	}
	return m
}

// Play emits the scenario's frames into out on schedule (wall-clock offset =
// TMs / Speed), then holds until ctx is cancelled so dark events can mature
// after their vessel stops transmitting. It does not close out; the caller
// closes it once Play returns.
func (sc *Scenario) Play(ctx context.Context, out chan<- ingest.Message) {
	flags := sc.FlagCodes()
	start := time.Now()
	for _, f := range sc.Frames {
		due := start.Add(time.Duration(float64(f.TMs)/sc.Speed) * time.Millisecond)
		if d := time.Until(due); d > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(d):
			}
		}
		msg := ingest.Message{
			MMSI:       f.MMSI,
			Lat:        f.Lat,
			Lon:        f.Lon,
			SpeedKn:    f.SpeedKn,
			HeadingDeg: f.HeadingDeg,
			TsMs:       scenarioBaseTs + f.TMs,
			FlagCode:   flags[f.MMSI],
		}
		select {
		case <-ctx.Done():
			return
		case out <- msg:
		}
	}
	<-ctx.Done() // keep the engine alive so the dark sweep can fire
}
