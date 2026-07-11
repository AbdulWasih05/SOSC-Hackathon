// Package alert defines the frozen websocket contracts between the engine and
// the dashboard. These structs are a field-for-field mirror of the JSON schemas
// in CLAUDE.md. Changing any tag or field requires explicit human approval.
package alert

// Alert kinds. MMSI is uint32 everywhere (hot-path rule 7). The last two are
// risk-engine tier-transition alerts (P0), added additively: a pre-risk frontend
// ignores kinds it does not know.
const (
	KindZone             = "ZONE_VIOLATION"
	KindSpoof            = "SPOOF_TELEPORT"
	KindDark             = "DARK_EVENT"
	KindTrawling         = "TRAWLING"
	KindLonglining       = "LONGLINING"
	KindPurseSeining     = "PURSE_SEINING"
	KindIllegalSuspected = "ILLEGAL_FISHING_SUSPECTED"
	KindBoarding         = "BOARDING_RECOMMENDED"
)

// Severities.
const (
	SeverityHigh     = "HIGH"
	SeverityCritical = "CRITICAL"
)

// Envelope is the top-level websocket frame for a single alert.
type Envelope struct {
	Type  string `json:"type"` // always "alert"
	Alert Alert  `json:"alert"`
}

// Alert is one raised alert. cone and intercepts are populated only for
// DARK_EVENT; detail carries kind-specific fields (e.g. implied_speed_kn).
// score and factors are populated only on risk-engine tier-transition alerts
// (ILLEGAL_FISHING_SUSPECTED, BOARDING_RECOMMENDED); both are additive and
// omitempty, so the pre-risk contract is unchanged for the other kinds.
type Alert struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Severity   string         `json:"severity"`
	MMSI       uint32         `json:"mmsi"`
	Name       string         `json:"name"`
	TsMs       int64          `json:"ts_ms"`
	Lat        float64        `json:"lat"`
	Lon        float64        `json:"lon"`
	ZoneID     string         `json:"zone_id"`
	Detail     map[string]any `json:"detail"`
	Cone       *Cone          `json:"cone,omitempty"`
	Intercepts []Intercept    `json:"intercepts,omitempty"`
	Score      int            `json:"score,omitempty"`
	Factors    []Factor       `json:"factors,omitempty"`
}

// Factor is one line of a risk-score breakdown: a readable, decayed contribution
// to the 0-100 suspicion score. code is a stable machine key (ZONE, DARK,
// SPOOF), label is the human string, points is the decayed contribution, and
// ts_ms is when the underlying event was last observed.
type Factor struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Points int    `json:"points"`
	TsMs   int64  `json:"ts_ms"`
}

// Cone is the dead-reckoning uncertainty cone as a scalar (hot-path rule 5).
// The backend emits origin, heading, spread and radius; the frontend draws the
// polygon.
type Cone struct {
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	HeadingDeg float64 `json:"heading_deg"`
	SpreadDeg  float64 `json:"spread_deg"`
	RadiusM    float64 `json:"radius_m"`
}

// Intercept is the closing geometry for one stationed patrol asset.
type Intercept struct {
	PatrolID   string  `json:"patrol_id"`
	HeadingDeg float64 `json:"heading_deg"`
	EtaS       float64 `json:"eta_s"`
	Feasible   bool    `json:"feasible"`
}

// Metrics is the once-per-second telemetry frame. risk_sweep_us and
// scored_vessels are additive risk-engine fields (omitempty): they are 0 and
// omitted when the risk engine is off, so the pre-risk metrics frame is
// unchanged.
type Metrics struct {
	Type           string       `json:"type"` // always "metrics"
	IngestedTotal  uint64       `json:"ingested_total"`
	ProcessedTotal uint64       `json:"processed_total"`
	DroppedTotal   uint64       `json:"dropped_total"`
	RatePerS       float64      `json:"rate_per_s"`
	LatencyUs      LatencyStats `json:"latency_us"`
	ActiveVessels  int          `json:"active_vessels"`
	AlertsTotal    uint64       `json:"alerts_total"`
	RiskSweepUs    float64      `json:"risk_sweep_us,omitempty"`
	ScoredVessels  int          `json:"scored_vessels,omitempty"`
}

// LatencyStats reports inline and sweep latency percentiles in microseconds.
// Inline covers ZONE_VIOLATION and SPOOF_TELEPORT; sweep covers the 1s
// DARK_EVENT pass. The two are always reported separately so the millisecond
// claim is never blurred across the inline/sweep boundary.
type LatencyStats struct {
	InlineP50 float64 `json:"inline_p50"`
	InlineP99 float64 `json:"inline_p99"`
	SweepP50  float64 `json:"sweep_p50"`
	SweepP99  float64 `json:"sweep_p99"`
}
