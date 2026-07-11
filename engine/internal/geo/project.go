// Package geo holds the flat-plane projection, zone polygons, and spatial grid.
// Every local distance uses an equirectangular projection (hot-path rule 8):
// scale longitude by cos(latitude), treat coordinates as 2D Cartesian meters.
// Haversine and GIS distance libraries appear nowhere here.
package geo

import "math"

const (
	// metersPerDegLat is the WGS84 mean meters per degree of latitude. At Gulf
	// of Mannar scale (tens of nautical miles) treating this as constant is
	// negligibly wrong and dramatically cheaper than spherical trig.
	metersPerDegLat = 111320.0
	metersPerNm     = 1852.0
	degToRad        = math.Pi / 180.0
)

// DistanceM returns the flat-plane distance in meters between two lat/lon
// points. Longitude is scaled by the cosine of the mean latitude. This is the
// hottest distance calc in the system (spoof check runs it per message).
func DistanceM(lat1, lon1, lat2, lon2 float64) float64 {
	latMeanRad := (lat1 + lat2) * 0.5 * degToRad
	dLat := (lat2 - lat1) * metersPerDegLat
	dLon := (lon2 - lon1) * metersPerDegLat * math.Cos(latMeanRad)
	return math.Hypot(dLat, dLon)
}

// MetersToNm converts meters to nautical miles.
func MetersToNm(m float64) float64 { return m / metersPerNm }

// NmToMeters converts nautical miles to meters.
func NmToMeters(nm float64) float64 { return nm * metersPerNm }

// KnotsToMPS converts knots to meters per second.
func KnotsToMPS(kn float64) float64 { return kn * metersPerNm / 3600.0 }

// MPSToKnots converts meters per second to knots.
func MPSToKnots(mps float64) float64 { return mps * 3600.0 / metersPerNm }
