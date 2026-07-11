// The backend emits the dead-reckoning cone as a scalar (origin, heading,
// spread, radius); the frontend draws the polygon (hot-path rule 5). Flat-plane
// projection, matching the engine's geo helper.

const M_PER_DEG_LAT = 111320

function project(lat, lon, headingDeg, distM) {
  const h = (headingDeg * Math.PI) / 180
  const dLat = (distM * Math.cos(h)) / M_PER_DEG_LAT
  const dLon = (distM * Math.sin(h)) / (M_PER_DEG_LAT * Math.cos((lat * Math.PI) / 180))
  return [lon + dLon, lat + dLat]
}

// conePolygon returns a GeoJSON Polygon feature for a cone scalar.
export function conePolygon(cone, id, steps = 28) {
  const { lat, lon, heading_deg, spread_deg, radius_m } = cone
  const half = spread_deg / 2
  const ring = [[lon, lat]]
  for (let i = 0; i <= steps; i++) {
    const ang = heading_deg - half + (spread_deg * i) / steps
    ring.push(project(lat, lon, ang, radius_m))
  }
  ring.push([lon, lat])
  return { type: 'Feature', id, geometry: { type: 'Polygon', coordinates: [ring] }, properties: {} }
}

// interceptLine returns a line from a patrol to the cone origin, tagged with the
// solution so the layer can style feasible vs infeasible and label the ETA.
export function interceptLine(patrol, cone, ic, id) {
  return {
    type: 'Feature',
    id,
    geometry: { type: 'LineString', coordinates: [[patrol.lon, patrol.lat], [cone.lon, cone.lat]] },
    properties: {
      patrol_id: ic.patrol_id,
      feasible: ic.feasible,
      eta_min: Math.round(ic.eta_s / 60),
    },
  }
}
