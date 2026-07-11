package gen

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
)

// aisStreamURL is the aisstream.io realtime websocket endpoint (wss only).
const aisStreamURL = "wss://stream.aisstream.io/v0/stream"

// Reconnect backoff. aisstream.io returns 429 if connections are opened too
// often, so the floor is deliberately several seconds, not milliseconds.
const (
	liveBackoffMin = 5 * time.Second
	liveBackoffMax = 60 * time.Second
)

// LiveConfig configures the aisstream.io feed. BoundingBox is [[lat,lon],[lat,lon]]
// (aisstream orders coordinates latitude-first, unlike GeoJSON).
type LiveConfig struct {
	APIKey      string
	BoundingBox [][]float64
}

// aisEnvelope is the aisstream.io wrapper: a message type, decoded metadata, and
// the raw type-specific payload.
type aisEnvelope struct {
	MessageType string          `json:"MessageType"`
	MetaData    aisMetaData     `json:"MetaData"`
	Message     map[string]json.RawMessage `json:"Message"`
}

type aisMetaData struct {
	MMSI     int     `json:"MMSI"`
	ShipName string  `json:"ShipName"`
	Lat      float64 `json:"latitude"`
	Lon      float64 `json:"longitude"`
	TimeUTC  string  `json:"time_utc"`
}

type aisPositionReport struct {
	UserID      int     `json:"UserID"`
	Sog         float64 `json:"Sog"`
	Cog         float64 `json:"Cog"`
	TrueHeading int     `json:"TrueHeading"`
	Latitude    float64 `json:"Latitude"`
	Longitude   float64 `json:"Longitude"`
}

type aisShipStatic struct {
	UserID int    `json:"UserID"`
	Name   string `json:"Name"`
}

// LoadAPIKey resolves the aisstream key: env var AISSTREAM_API_KEY or APIKey
// first, then an APIKey=... line in engine/.env (or ./.env). Never logged.
func LoadAPIKey() string {
	if k := os.Getenv("AISSTREAM_API_KEY"); k != "" {
		return strings.TrimSpace(k)
	}
	if k := os.Getenv("APIKey"); k != "" {
		return strings.TrimSpace(k)
	}
	for _, p := range []string{".env", "engine/.env", "../.env"} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		var key string
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if strings.HasPrefix(line, "APIKey=") {
				key = strings.TrimSpace(strings.TrimPrefix(line, "APIKey="))
				break
			}
		}
		f.Close()
		if key != "" {
			return key
		}
	}
	return ""
}

// RunLive streams live AIS into out until ctx is cancelled, reconnecting with
// backoff. setName records vessel names (cold store) from ShipStaticData. It
// does not close out; the caller owns that. Names and the key are never on the
// hot Message struct (rule 1) or in per-message logs (rule 4).
func RunLive(ctx context.Context, cfg LiveConfig, out chan<- ingest.Message, setName func(uint32, string)) {
	backoff := liveBackoffMin
	for {
		if ctx.Err() != nil {
			return
		}
		n, err := runLiveOnce(ctx, cfg, out, setName)
		if ctx.Err() != nil {
			return
		}
		if n > 0 {
			backoff = liveBackoffMin // healthy session; reset backoff
		}
		log.Warn().Err(err).Int("messages", n).Dur("retry_in", backoff).Msg("live feed disconnected, reconnecting")
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > liveBackoffMax {
			backoff = liveBackoffMax
		}
	}
}

// runLiveOnce holds a single websocket session and returns how many messages it
// forwarded and the error that ended it.
func runLiveOnce(ctx context.Context, cfg LiveConfig, out chan<- ingest.Message, setName func(uint32, string)) (int, error) {
	c, _, err := websocket.DefaultDialer.DialContext(ctx, aisStreamURL, nil)
	if err != nil {
		return 0, err
	}
	defer c.Close()

	sub := map[string]any{
		"APIKey":             cfg.APIKey,
		"BoundingBoxes":      [][][]float64{cfg.BoundingBox},
		"FilterMessageTypes": []string{"PositionReport", "ShipStaticData"},
	}
	if err := c.WriteJSON(sub); err != nil {
		return 0, err
	}
	log.Info().Msg("live feed connected: subscribed to aisstream.io")

	// Close the socket when ctx is cancelled so ReadMessage unblocks.
	go func() {
		<-ctx.Done()
		_ = c.Close()
	}()

	n := 0
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			return n, err
		}
		var env aisEnvelope
		if json.Unmarshal(raw, &env) != nil {
			continue
		}
		switch env.MessageType {
		case "ShipStaticData":
			var s aisShipStatic
			if p := env.Message["ShipStaticData"]; p != nil && json.Unmarshal(p, &s) == nil {
				name := strings.TrimSpace(s.Name)
				if name != "" && s.UserID > 0 {
					setName(uint32(s.UserID), name)
				}
			}
		case "PositionReport":
			var pr aisPositionReport
			if p := env.Message["PositionReport"]; p == nil || json.Unmarshal(p, &pr) != nil {
				continue
			}
			msg, ok := toMessage(env.MetaData, pr)
			if !ok {
				continue
			}
			// The position metadata carries the ship name too, so the alert feed
			// has a name immediately without waiting for a ShipStaticData frame.
			if name := strings.TrimSpace(env.MetaData.ShipName); name != "" {
				setName(msg.MMSI, name)
			}
			select {
			case <-ctx.Done():
				return n, ctx.Err()
			case out <- msg:
				n++
			}
		}
	}
}

// toMessage maps an aisstream PositionReport to the engine's flat Message.
// Sentinel values (Sog 102.3 = n/a, TrueHeading 511 = n/a) are cleaned up, and
// the flag is derived from the MMSI MID. TsMs is receive time.
func toMessage(md aisMetaData, pr aisPositionReport) (ingest.Message, bool) {
	mmsi := pr.UserID
	if mmsi <= 0 {
		mmsi = md.MMSI
	}
	if mmsi <= 0 {
		return ingest.Message{}, false
	}
	lat, lon := pr.Latitude, pr.Longitude
	if lat == 0 && lon == 0 {
		lat, lon = md.Lat, md.Lon
	}
	// AIS reports positions of 91/181 when unavailable; reject them.
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 || (lat == 0 && lon == 0) {
		return ingest.Message{}, false
	}

	speed := float32(pr.Sog)
	if pr.Sog >= 102.3 || pr.Sog < 0 { // 1023 raw / 10 = not available
		speed = 0
	}
	heading := float32(pr.TrueHeading)
	if pr.TrueHeading < 0 || pr.TrueHeading >= 360 { // 511 = not available
		heading = float32(pr.Cog)
	}
	if heading < 0 || heading >= 360 {
		heading = 0
	}

	return ingest.Message{
		MMSI:       uint32(mmsi),
		Lat:        lat,
		Lon:        lon,
		SpeedKn:    speed,
		HeadingDeg: heading,
		TsMs:       time.Now().UnixMilli(),
		FlagCode:   geo.FlagFromMMSI(uint32(mmsi)),
	}, true
}
