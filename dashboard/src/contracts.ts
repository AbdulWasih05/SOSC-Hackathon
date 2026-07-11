// Frozen websocket contracts. Field-for-field mirror of
// engine/internal/alert/schema.go and the JSON schemas in CLAUDE.md.
// Changing any field requires explicit human approval.

export type AlertKind = "ZONE_VIOLATION" | "SPOOF_TELEPORT" | "DARK_EVENT";
export type Severity = "HIGH" | "CRITICAL";

// Dead-reckoning cone as a scalar. The backend emits origin/heading/spread/
// radius; the frontend draws the polygon (hot-path rule 5).
export interface Cone {
  lat: number;
  lon: number;
  heading_deg: number;
  spread_deg: number;
  radius_m: number;
}

export interface Intercept {
  patrol_id: string;
  heading_deg: number;
  eta_s: number;
  feasible: boolean;
}

export interface Alert {
  id: string;
  kind: AlertKind;
  severity: Severity;
  mmsi: number;
  name: string;
  ts_ms: number;
  lat: number;
  lon: number;
  zone_id: string;
  detail: Record<string, unknown>;
  cone?: Cone; // DARK_EVENT only
  intercepts?: Intercept[]; // DARK_EVENT only
}

export interface AlertMessage {
  type: "alert";
  alert: Alert;
}

// Inline covers zone + spoof; sweep covers the 1s dark-event pass. Reported
// separately so the millisecond claim is never blurred across the boundary.
export interface LatencyStats {
  inline_p50: number;
  inline_p99: number;
  sweep_p50: number;
  sweep_p99: number;
}

export interface MetricsMessage {
  type: "metrics";
  ingested_total: number;
  processed_total: number;
  dropped_total: number;
  rate_per_s: number;
  latency_us: LatencyStats;
  active_vessels: number;
  alerts_total: number;
}

// Vessel positions: a GeoJSON FeatureCollection for one MapLibre GeoJSONSource,
// sent at most twice a second. Names are not included; join them from alerts.
export interface VesselProps {
  mmsi: number;
  speed_kn: number;
  heading_deg: number;
}

export interface VesselFeature {
  type: "Feature";
  geometry: { type: "Point"; coordinates: [number, number] }; // [lon, lat]
  properties: VesselProps;
}

export interface FeatureCollection {
  type: "FeatureCollection";
  features: VesselFeature[];
}

export interface PositionMessage {
  type: "positions";
  fc: FeatureCollection;
}

export type ServerMessage = AlertMessage | MetricsMessage | PositionMessage;
