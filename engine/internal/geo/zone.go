package geo

import (
	"fmt"
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
