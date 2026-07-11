package gen

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
)

// Danish Maritime Authority (aisdk) CSV columns, by index. The header line is
// "# Timestamp,Type of mobile,MMSI,Latitude,Longitude,Navigational status,ROT,
// SOG,COG,Heading,IMO,Callsign,Name,Ship type,...".
const (
	colTime    = 0
	colType    = 1
	colMMSI    = 2
	colLat     = 3
	colLon     = 4
	colSOG     = 7
	colCOG     = 8
	colHeading = 9
	colName    = 12
	minColumns = 13
)

// aisdkTimeLayout parses the aisdk timestamp "DD/MM/YYYY HH:MM:SS".
const aisdkTimeLayout = "02/01/2006 15:04:05"

// AISDKConfig configures replay of an aisdk CSV.
type AISDKConfig struct {
	Path  string
	Speed float64 // event seconds per wall second; <=0 means 30x
}

// RunAISDK streams a Danish AIS CSV and replays Class A/B position reports into
// out on a compressed event-time schedule (Speed x real time). Because event
// timestamps are real, the spoof implied-speed check is exact; names go to the
// cold store via setName. Streaming (never loading 3 GB into memory) keeps the
// hot path allocation-free per rule 3. It does not close out; the caller owns
// that. Returns when ctx is cancelled or the file ends.
func RunAISDK(ctx context.Context, cfg AISDKConfig, out chan<- ingest.Message, setName func(uint32, string)) error {
	speed := cfg.Speed
	if speed <= 0 {
		speed = 30
	}
	f, err := os.Open(cfg.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	wallStart := time.Now()
	var eventStart int64 = -1
	var sent, skipped uint64

	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		msg, name, eventMs, ok := parseAISDK(line)
		if !ok {
			skipped++
			continue
		}
		if eventStart < 0 {
			eventStart = eventMs
		}

		// Pace replay: hold each message until its compressed wall time. Most
		// consecutive rows share a timestamp, so this rarely sleeps.
		offsetMs := float64(eventMs-eventStart) / speed
		target := wallStart.Add(time.Duration(offsetMs * float64(time.Millisecond)))
		if d := time.Until(target); d > 2*time.Millisecond {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
			}
		}

		if name != "" {
			setName(msg.MMSI, name)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- msg:
			sent++
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	log.Info().Uint64("sent", sent).Uint64("skipped", skipped).Msg("aisdk replay reached end of file")
	return nil
}

// parseAISDK maps one CSV row to a Message. It returns ok=false for base
// stations, aids to navigation, malformed rows, and invalid positions.
func parseAISDK(line string) (ingest.Message, string, int64, bool) {
	fields := strings.Split(line, ",")
	if len(fields) < minColumns {
		return ingest.Message{}, "", 0, false
	}
	switch fields[colType] {
	case "Class A", "Class B":
	default:
		return ingest.Message{}, "", 0, false
	}

	mmsi64, err := strconv.ParseUint(strings.TrimSpace(fields[colMMSI]), 10, 32)
	if err != nil || mmsi64 == 0 {
		return ingest.Message{}, "", 0, false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(fields[colLat]), 64)
	lon, err2 := strconv.ParseFloat(strings.TrimSpace(fields[colLon]), 64)
	if err1 != nil || err2 != nil {
		return ingest.Message{}, "", 0, false
	}
	// aisdk uses 91/181 for unavailable positions; reject those and (0,0).
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 || (lat == 0 && lon == 0) {
		return ingest.Message{}, "", 0, false
	}

	t, err := time.Parse(aisdkTimeLayout, strings.TrimSpace(fields[colTime]))
	if err != nil {
		return ingest.Message{}, "", 0, false
	}

	speed := float32(parseFloatOr(fields[colSOG], 0))
	if speed < 0 || speed > 102.2 { // 1023/10 = not available
		speed = 0
	}
	heading := float32(parseFloatOr(fields[colHeading], -1))
	if heading < 0 || heading >= 360 { // 511 = not available -> fall back to course
		heading = float32(parseFloatOr(fields[colCOG], 0))
	}
	if heading < 0 || heading >= 360 {
		heading = 0
	}

	mmsi := uint32(mmsi64)
	msg := ingest.Message{
		MMSI:       mmsi,
		Lat:        lat,
		Lon:        lon,
		SpeedKn:    speed,
		HeadingDeg: heading,
		TsMs:       t.UnixMilli(),
		FlagCode:   geo.FlagFromMMSI(mmsi),
	}
	return msg, strings.TrimSpace(fields[colName]), t.UnixMilli(), true
}

func parseFloatOr(s string, def float64) float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return v
	}
	return def
}
