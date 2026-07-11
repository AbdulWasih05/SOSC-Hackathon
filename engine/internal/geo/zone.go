package geo

import (
	"fmt"
	"math"
	"os"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
)

// Flag/country codes. Kept small so they fit a uint16 field in the hot vessel
// struct (no strings on the hot path).
const (
	CountryUnknown uint16 = 0
	CountryIN      uint16 = 1
	CountryLK      uint16 = 2
)

// CountryCode maps a two-letter code to the compact numeric code.
func CountryCode(s string) uint16 {
	switch s {
	case "IN":
		return CountryIN
	case "LK":
		return CountryLK
	default:
		return CountryUnknown
	}
}

// Zone kinds.
const (
	KindMPA = "mpa"
	KindEEZ = "eez"
)

// Zone is a named restricted area or EEZ segment. Poly is in planar lon/lat;
// point-in-polygon uses orb's planar test, consistent with the flat-plane rule
// at this scale.
type Zone struct {
	ID          string
	Name        string
	Kind        string
	CountryCode uint16
	Poly        orb.Polygon
	bound       orb.Bound
}

// Contains reports whether the point (lon, lat) lies inside the zone polygon.
// The bounding-box pretest rejects the common far-away case without a polygon
// scan.
func (z *Zone) Contains(lon, lat float64) bool {
	p := orb.Point{lon, lat}
	if !z.bound.Contains(p) {
		return false
	}
	return planar.PolygonContains(z.Poly, p)
}

// NewZone builds a zone from a polygon. Used by tests and any programmatic zone
// source; LoadZones is the file-backed path.
func NewZone(id, name, kind string, country uint16, poly orb.Polygon) *Zone {
	return &Zone{
		ID:          id,
		Name:        name,
		Kind:        kind,
		CountryCode: country,
		Poly:        poly,
		bound:       poly.Bound(),
	}
}

// DistanceNm returns 0 if (lat, lon) is inside the zone, else the minimum
// flat-plane distance in nautical miles from the point to the zone boundary.
// Used by the dark sweep to keep open-ocean coverage gaps from alerting.
func (z *Zone) DistanceNm(lat, lon float64) float64 {
	if z.Contains(lon, lat) {
		return 0
	}
	cosLat := math.Cos(lat * degToRad)
	minM := math.MaxFloat64
	for _, ring := range z.Poly {
		for i := 0; i+1 < len(ring); i++ {
			// Project the segment endpoints to local east/north meters about
			// the query point, which sits at the origin.
			ax := (ring[i][0] - lon) * metersPerDegLat * cosLat
			ay := (ring[i][1] - lat) * metersPerDegLat
			bx := (ring[i+1][0] - lon) * metersPerDegLat * cosLat
			by := (ring[i+1][1] - lat) * metersPerDegLat
			if d := pointSegDistM(0, 0, ax, ay, bx, by); d < minM {
				minM = d
			}
		}
	}
	return MetersToNm(minM)
}

// pointSegDistM is the distance from point (px,py) to segment (ax,ay)-(bx,by).
func pointSegDistM(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

// LoadZones reads a GeoJSON FeatureCollection of Polygon zones. Each feature's
// properties supply id, name, type (mpa|eez), and optional country.
func LoadZones(path string) ([]*Zone, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fc, err := geojson.UnmarshalFeatureCollection(data)
	if err != nil {
		return nil, fmt.Errorf("parse zones: %w", err)
	}

	var zones []*Zone
	for _, f := range fc.Features {
		var poly orb.Polygon
		switch g := f.Geometry.(type) {
		case orb.Polygon:
			poly = g
		case orb.MultiPolygon:
			if len(g) == 0 {
				continue
			}
			poly = g[0]
		default:
			continue
		}
		z := &Zone{
			ID:          propString(f.Properties, "id"),
			Name:        propString(f.Properties, "name"),
			Kind:        propString(f.Properties, "type"),
			CountryCode: CountryCode(propString(f.Properties, "country")),
			Poly:        poly,
			bound:       poly.Bound(),
		}
		zones = append(zones, z)
	}
	if len(zones) == 0 {
		return nil, fmt.Errorf("no polygon zones in %s", path)
	}
	return zones, nil
}

func propString(p geojson.Properties, key string) string {
	if v, ok := p[key].(string); ok {
		return v
	}
	return ""
}
