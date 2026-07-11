package check

import "palkwatch/internal/geo"

// Spoof thresholds (CLAUDE.md alert taxonomy).
const (
	SpoofMaxSpeedKn  = 60.0   // implied speed between consecutive fixes
	SpoofDupNm       = 50.0   // duplicate MMSI seen this far apart...
	SpoofDupWindowMs = 60_000 // ...within this window
	spoofSpeedCapKn  = 9999.0 // reported when dt <= 0 (teleport at same/backwards ts)
)

// SpoofTeleport reports whether the jump from a previous fix to the current one
// is physically impossible, and the implied speed to report in the alert
// detail. Two triggers, both reducing to flat-plane distance vs time:
//   - the same MMSI appears more than 50 nm apart within a 60 s window
//     (covers duplicate-identity spoofing, including a same-timestamp jump)
//   - the implied speed between consecutive fixes exceeds 60 kn
//
// Distance uses the shared equirectangular helper (hot-path rule 8); this runs
// once per message and is the hottest distance calc in the system.
func SpoofTeleport(prevLat, prevLon float64, prevTsMs int64, lat, lon float64, tsMs int64) (bool, float64) {
	dtMs := tsMs - prevTsMs
	distM := geo.DistanceM(prevLat, prevLon, lat, lon)

	if dtMs >= 0 && dtMs <= SpoofDupWindowMs && geo.MetersToNm(distM) > SpoofDupNm {
		return true, impliedKn(distM, dtMs)
	}
	if dtMs > 0 {
		if spd := impliedKn(distM, dtMs); spd > SpoofMaxSpeedKn {
			return true, spd
		}
	}
	return false, 0
}

// impliedKn is the implied speed in knots for a jump of distM meters over dtMs
// milliseconds. A non-positive interval means a teleport at the same or an
// earlier timestamp; we report a finite cap rather than infinity so the alert
// detail stays JSON-encodable.
func impliedKn(distM float64, dtMs int64) float64 {
	secs := float64(dtMs) / 1000.0
	if secs <= 0 {
		return spoofSpeedCapKn
	}
	return geo.MPSToKnots(distM / secs)
}
