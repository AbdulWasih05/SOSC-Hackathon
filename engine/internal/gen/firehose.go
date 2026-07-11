package gen

import (
	"math"

	"palkwatch/internal/geo"
	"palkwatch/internal/ingest"
)

// Firehose pre-generates `total` messages in memory for the benchmark. It mixes
// a large field of static background vessels (bounded distinct MMSIs, so the
// state table stays realistic) with a handful of runner vessels that hop across
// the Gulf of Mannar MPA boundary, so the geofence fires a steady, modest alert
// stream during the run. No network, no allocation in the measured loop; the
// bench loops over this slice.
func Firehose(total int) []ingest.Message {
	const (
		runnerEvery   = 50     // one in fifty messages is a runner
		numRunners    = 64     // distinct runner vessels
		numBackground = 100000 // distinct background vessels
	)
	const baseTs int64 = 1720000000000

	msgs := make([]ingest.Message, 0, total)
	runnerOccur := make([]int, numRunners)

	for i := 0; i < total; i++ {
		if i%runnerEvery == 0 {
			r := (i / runnerEvery) % numRunners
			occ := runnerOccur[r]
			runnerOccur[r]++
			msgs = append(msgs, runnerMsg(r, occ, baseTs))
			continue
		}
		msgs = append(msgs, backgroundMsg(i%numBackground, baseTs))
	}
	return msgs
}

// runnerMsg alternates a runner between a point just outside the MPA (south of
// the 8.90 boundary) and one just inside it. Fixes are spaced 600s apart so the
// ~11 km hop implies ~36 kn, staying under the 60 kn spoof threshold. The
// runner sits inside the Indian EEZ at both points, so only the MPA bit toggles
// and repeated entries raise ZONE_VIOLATION.
func runnerMsg(r, occ int, baseTs int64) ingest.Message {
	lat := 8.85 // outside
	if occ%2 == 0 {
		lat = 8.95 // inside
	}
	lon := 79.10 + float64(r)*0.002 // stays within the MPA's 79.00-79.30 lon band
	return ingest.Message{
		MMSI:       420000000 + uint32(r),
		Lat:        lat,
		Lon:        lon,
		SpeedKn:    8,
		HeadingDeg: 0,
		TsMs:       baseTs + int64(occ)*600000,
		FlagCode:   geo.CountryLK,
	}
}

// backgroundMsg places a static vessel deterministically across the wider Palk
// region. Position depends only on the vessel id, so repeats do not move (no
// spurious transitions or teleports).
func backgroundMsg(vid int, baseTs int64) ingest.Message {
	lat := 8.5 + math.Mod(float64(vid)*0.000173, 1.4)  // 8.5 .. 9.9
	lon := 78.5 + math.Mod(float64(vid)*0.000239, 2.2) // 78.5 .. 80.7
	flag := geo.CountryIN
	if vid%3 == 0 {
		flag = geo.CountryLK
	}
	return ingest.Message{
		MMSI:       419000000 + uint32(vid),
		Lat:        lat,
		Lon:        lon,
		SpeedKn:    float32(2 + vid%22),
		HeadingDeg: float32(vid % 360),
		TsMs:       baseTs,
		FlagCode:   flag,
	}
}
