package geo

import "math"

// DeadReckon returns the position reached from (lat, lon) after traveling
// distanceM meters along a compass heading (degrees, 0 = north, 90 = east),
// using the flat-plane projection (hot-path rule 8). Used to project a dark
// vessel's likely current position from its last known fix.
func DeadReckon(lat, lon, headingDeg, distanceM float64) (float64, float64) {
	h := headingDeg * degToRad
	north := distanceM * math.Cos(h)
	east := distanceM * math.Sin(h)
	dLat := north / metersPerDegLat
	dLon := east / (metersPerDegLat * math.Cos(lat*degToRad))
	return lat + dLat, lon + dLon
}
