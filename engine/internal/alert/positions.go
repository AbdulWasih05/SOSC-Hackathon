package alert

// Vessel position feed. A GeoJSON FeatureCollection for a single MapLibre
// GeoJSONSource, broadcast at most twice a second regardless of ingest rate.
// Names are not included here (rule 1: the cold map is touched only on alert
// emission); the frontend joins names from the alert feed.

// PositionMsg is the websocket frame carrying a batch of vessel positions.
type PositionMsg struct {
	Type string            `json:"type"` // always "positions"
	FC   FeatureCollection `json:"fc"`
}

// FeatureCollection is a GeoJSON FeatureCollection of vessel points.
type FeatureCollection struct {
	Type     string    `json:"type"` // always "FeatureCollection"
	Features []Feature `json:"features"`
}

// Feature is one vessel as a GeoJSON point feature.
type Feature struct {
	Type       string      `json:"type"` // always "Feature"
	Geometry   Geometry    `json:"geometry"`
	Properties VesselProps `json:"properties"`
}

// Geometry is a GeoJSON Point. Coordinates are [lon, lat] per the spec.
type Geometry struct {
	Type        string     `json:"type"` // always "Point"
	Coordinates [2]float64 `json:"coordinates"`
}

// VesselProps are the per-vessel properties the map layer needs. RiskScore,
// RiskTier and Factors are additive risk-engine fields (omitempty): they are set
// only for scored vessels when the risk engine is on, so the map colors dots by
// tier and the Score Breakdown Drawer reads factors without a separate endpoint.
type VesselProps struct {
	MMSI       uint32   `json:"mmsi"`
	SpeedKn    float32  `json:"speed_kn"`
	HeadingDeg float32  `json:"heading_deg"`
	RiskScore  int      `json:"risk_score,omitempty"`
	RiskTier   string   `json:"risk_tier,omitempty"`
	Factors    []Factor `json:"factors,omitempty"`
}

// NewFeature builds a point feature for a vessel.
func NewFeature(mmsi uint32, lat, lon float64, speedKn, headingDeg float32) Feature {
	return Feature{
		Type:     "Feature",
		Geometry: Geometry{Type: "Point", Coordinates: [2]float64{lon, lat}},
		Properties: VesselProps{
			MMSI:       mmsi,
			SpeedKn:    speedKn,
			HeadingDeg: headingDeg,
		},
	}
}
