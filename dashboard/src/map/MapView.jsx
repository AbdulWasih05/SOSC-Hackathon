import { forwardRef, memo, useEffect, useImperativeHandle, useRef } from 'react'
import maplibregl from 'maplibre-gl'
import { HTTP_BASE } from '../ws.js'
import { conePolygon, interceptLine } from './cone.js'
import { KIND_COLOR } from '../theme.js'

// Offline style: no external tiles (the pitch runs on localhost with no venue
// wifi). Zones and vessels are drawn as GeoJSON layers over a dark canvas.
// Light nautical theme: water is a pale sea blue, land (added on load) is the
// warm parchment tone, so the two read as distinct.
const EMPTY_STYLE = {
  version: 8,
  sources: {},
  layers: [{ id: 'bg', type: 'background', paint: { 'background-color': '#bcd4de' } }],
}

const CENTER = [79.25, 9.0]
const EMPTY_FC = { type: 'FeatureCollection', features: [] }

// bboxOf returns [minLon,minLat,maxLon,maxLat] over one geometry's coordinates.
function bboxOf(coords) {
  let minLon = Infinity, minLat = Infinity, maxLon = -Infinity, maxLat = -Infinity
  const visit = (c) => {
    if (typeof c[0] === 'number') {
      const [lon, lat] = c
      if (lon < minLon) minLon = lon
      if (lat < minLat) minLat = lat
      if (lon > maxLon) maxLon = lon
      if (lat > maxLat) maxLat = lat
      return
    }
    c.forEach(visit)
  }
  visit(coords)
  return [minLon, minLat, maxLon, maxLat]
}

// CLUSTER_DEG is the centroid-distance threshold (degrees) for grouping zones
// into one regional cluster. A real regional zone set (e.g. the North Sea's 3
// zones, or Gulf of Mannar's 6) sits within a couple of degrees of itself;
// unrelated far-flung zones (placeholder/reference data from other parts of
// the world) sit tens of degrees from anything else. 10 degrees comfortably
// separates the two without needing to hardcode any region.
const CLUSTER_DEG = 10

// zonesBounds returns [[minLon,minLat],[maxLon,maxLat]] over the single
// largest cluster of mutually-nearby zones, so the map centers on whichever
// region the engine actually serves (North Sea for the live feed, Gulf of
// Mannar for the scenario) even when the zone response also carries unrelated
// reference zones from elsewhere in the world. Fitting to every zone's raw
// bounding box would zoom out to fit them all, shrinking the real region (and
// every vessel in it) to a few pixels.
function zonesBounds(fc) {
  const feats = (fc.features || []).filter((f) => f.geometry?.coordinates)
  if (feats.length === 0) return null

  const boxes = feats.map((f) => bboxOf(f.geometry.coordinates))
  const centroids = boxes.map(([minLon, minLat, maxLon, maxLat]) => [(minLon + maxLon) / 2, (minLat + maxLat) / 2])

  // Union-find over centroid proximity: cheap at this scale (a handful of
  // zones), so a plain O(n^2) adjacency pass is fine.
  const parent = centroids.map((_, i) => i)
  const find = (i) => (parent[i] === i ? i : (parent[i] = find(parent[i])))
  const union = (a, b) => { const ra = find(a), rb = find(b); if (ra !== rb) parent[ra] = rb }
  for (let i = 0; i < centroids.length; i++) {
    for (let j = i + 1; j < centroids.length; j++) {
      const dLon = centroids[i][0] - centroids[j][0]
      const dLat = centroids[i][1] - centroids[j][1]
      if (Math.hypot(dLon, dLat) <= CLUSTER_DEG) union(i, j)
    }
  }

  const clusterSizes = new Map()
  for (let i = 0; i < centroids.length; i++) {
    const root = find(i)
    clusterSizes.set(root, (clusterSizes.get(root) || 0) + 1)
  }
  let bestRoot = 0, bestSize = 0
  for (const [root, size] of clusterSizes) if (size > bestSize) { bestRoot = root; bestSize = size }

  let minLon = Infinity, minLat = Infinity, maxLon = -Infinity, maxLat = -Infinity
  for (let i = 0; i < boxes.length; i++) {
    if (find(i) !== bestRoot) continue
    const [bMinLon, bMinLat, bMaxLon, bMaxLat] = boxes[i]
    if (bMinLon < minLon) minLon = bMinLon
    if (bMinLat < minLat) minLat = bMinLat
    if (bMaxLon > maxLon) maxLon = bMaxLon
    if (bMaxLat > maxLat) maxLat = bMaxLat
  }
  return Number.isFinite(minLon) ? [[minLon, minLat], [maxLon, maxLat]] : null
}

// How long alert emphasis (flagged vessel, cone, intercept) stays on the map.
const CONE_TTL_MS = 15000
const FLAG_TTL_MS = 9000

const MapView = forwardRef(function MapView({ onVesselClick, onVesselData, selectedMMSI }, ref) {
  const containerRef = useRef(null)
  const mapRef = useRef(null)
  const readyRef = useRef(false)
  const patrolsRef = useRef({})
  const conesRef = useRef(new Map()) // id -> {feature, expires}
  const flagsRef = useRef(new Map()) // mmsi -> {feature, expires}
  const linesRef = useRef(new Map())
  const onVesselClickRef = useRef(onVesselClick)
  onVesselClickRef.current = onVesselClick
  const onVesselDataRef = useRef(onVesselData)
  onVesselDataRef.current = onVesselData
  // Latest position frame (real JS objects, so nested `factors` stays an array;
  // MapLibre would JSON-stringify it if read back through feature.properties).
  const latestFCRef = useRef(null)
  const selectedRef = useRef(selectedMMSI)
  selectedRef.current = selectedMMSI

  // Look up a vessel's full properties (including the factors array) from the
  // latest frame. Stable across renders via the refs it closes over.
  const findVesselProps = (mmsi) => {
    const feats = latestFCRef.current?.features
    if (!feats) return null
    for (const f of feats) if (Number(f.properties?.mmsi) === mmsi) return f.properties
    return null
  }
  // Highest-frequency path: vessel position frames. Coalesce them to at most one
  // GeoJSON setData per animation frame so a burst never queues redundant work.
  const pendingVesselsRef = useRef(null)
  const rafRef = useRef(0)
  // Highlight marker: a custom HTML element placed at the selected vessel.
  const highlightMarkerRef = useRef(null)
  const highlightTimerRef  = useRef(null)
  // Pending retry timers for the engine-served zones/patrols config.
  const staticRetryTimersRef = useRef([])

  useEffect(() => {
    const map = new maplibregl.Map({
      container: containerRef.current,
      style: EMPTY_STYLE,
      center: CENTER,
      zoom: 8.2,
      attributionControl: false,
    })
    mapRef.current = map
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'bottom-right')
    // Fullscreen targets the whole map-container (this div's parent), not just
    // the canvas, so the legend and connection notice stay visible in
    // fullscreen too.
    map.addControl(new maplibregl.FullscreenControl({ container: containerRef.current.parentElement }), 'bottom-left')

    map.on('load', () => {
      // Offline land/terrain overview: a local low-resolution coastline drawn
      // under everything else. No external tiles (stays offline, no venue wifi),
      // just a static GeoJSON bundled with the dashboard so land reads as land.
      try {
        map.addSource('land', { type: 'geojson', data: `${import.meta.env.BASE_URL}land.geojson` })
        // Land is the warm parchment tone over the pale-blue sea background.
        map.addLayer({ id: 'land-fill', type: 'fill', source: 'land', paint: { 'fill-color': '#e6e2d3', 'fill-opacity': 1 } })
        map.addLayer({ id: 'land-line', type: 'line', source: 'land', paint: { 'line-color': '#c2b48f', 'line-width': 0.8 } })
      } catch {
        /* land is decorative; map still works without it */
      }

      // Zones and patrols come from the engine (single source of truth), which
      // may not be up yet or may restart mid-demo. The websocket reconnects on
      // its own, so these fetches must retry too: a one-shot fetch left the map
      // with no protected areas until a manual refresh. Retries stop on success
      // or unmount. Layers insert beneath the vessel dots, so late arrival
      // cannot cover vessels or alert emphasis.
      const loadZones = async () => {
        const zones = await fetch(`${HTTP_BASE}/zones`).then((r) => r.json())
        map.addSource('zones', { type: 'geojson', data: zones })
        // Center on the served region (North Sea live feed, or Gulf of Mannar).
        const b = zonesBounds(zones)
        if (b) map.fitBounds(b, { padding: 70, duration: 0, maxZoom: 10 })
        map.addLayer({
          id: 'zone-fill',
          type: 'fill',
          source: 'zones',
          paint: {
            'fill-color': [
              'match', ['get', 'type'],
              'ecological-zone', '#1a8a3f',
              'eez', '#3987e5',
              'coral-reef', '#ff7b00',
              'fishing-banned', '#e60000',
              'international-water', '#8a2be2',
              '#e66767' // default
            ],
            // Projector-legible fill: bumped from the original values (which
            // washed out on screen) while keeping EEZ the subtlest wash so it
            // doesn't compete with the named protected-area types.
            'fill-opacity': [
              'match', ['get', 'type'],
              'ecological-zone', 0.30,
              'eez', 0.10,
              'coral-reef', 0.35,
              'fishing-banned', 0.35,
              'international-water', 0.20,
              0.30 // default
            ],
          },
        }, 'vessel-dot')
        map.addLayer({
          id: 'zone-line',
          type: 'line',
          source: 'zones',
          paint: {
            'line-color': [
              'match', ['get', 'type'],
              'ecological-zone', '#1a8a3f',
              'eez', '#3987e5',
              'coral-reef', '#ff7b00',
              'fishing-banned', '#e60000',
              'international-water', '#8a2be2',
              '#e66767' // default
            ],
            'line-width': ['match', ['get', 'type'], 'eez', 1.5, 2.5],
            'line-opacity': 0.9,
          },
        }, 'vessel-dot')
      }
      const loadPatrols = async () => {
        const doc = await fetch(`${HTTP_BASE}/patrols`).then((r) => r.json())
        const feats = (doc.patrols || []).map((p) => {
          patrolsRef.current[p.id] = p
          return {
            type: 'Feature',
            geometry: { type: 'Point', coordinates: [p.lon, p.lat] },
            properties: { id: p.id },
          }
        })
        map.addSource('patrols', { type: 'geojson', data: { type: 'FeatureCollection', features: feats } })
        map.addLayer({
          id: 'patrol-dot',
          type: 'circle',
          source: 'patrols',
          paint: {
            'circle-radius': 6,
            'circle-color': '#1a2332',
            'circle-stroke-color': '#ffffff',
            'circle-stroke-width': 2,
          },
        }, 'vessel-dot')
      }
      const retryUntilLoaded = (load) => {
        load().catch(() => {
          const t = setTimeout(() => retryUntilLoaded(load), 2000)
          staticRetryTimersRef.current.push(t)
        })
      }

      // Vessel field.
      map.addSource('vessels', { type: 'geojson', data: EMPTY_FC })
      map.addLayer({
        id: 'vessel-dot',
        type: 'circle',
        source: 'vessels',
        paint: {
          // Color by risk tier (P0): scored vessels stand out from the slate
          // field; unscored vessels keep the default slate.
          'circle-color': [
            'match', ['get', 'risk_tier'],
            'CRITICAL', '#e11d1d',
            'HIGH', '#e8791a',
            'ELEVATED', '#d99000',
            'LOW', '#3e5c76',
            /* default (no risk_tier) */ '#3e5c76',
          ],
          // Scored vessels draw a touch larger so the eye finds them.
          'circle-radius': [
            'interpolate', ['linear'], ['zoom'],
            6, ['match', ['get', 'risk_tier'], 'CRITICAL', 4, 'HIGH', 3.5, 'ELEVATED', 3, 1.5],
            10, ['match', ['get', 'risk_tier'], 'CRITICAL', 8, 'HIGH', 7, 'ELEVATED', 6, 3],
          ],
          'circle-stroke-color': '#0a1128',
          'circle-stroke-width': ['case', ['has', 'risk_tier'], 1.5, 0],
          'circle-opacity': 0.85,
        },
      })

      // Intercept vectors (patrol -> dark vessel). Feasible solid green,
      // infeasible dashed red.
      map.addSource('intercepts', { type: 'geojson', data: EMPTY_FC })
      map.addLayer({
        id: 'intercept-line',
        type: 'line',
        source: 'intercepts',
        paint: {
          // line-dasharray is not data-driven in MapLibre, so feasibility is
          // encoded by color (green vs red), not dash pattern.
          'line-color': ['case', ['get', 'feasible'], '#0ca30c', '#e66767'],
          'line-width': 2,
          'line-opacity': 0.9,
        },
      })

      // Dead-reckoning cones (drawn from the scalar).
      map.addSource('cones', { type: 'geojson', data: EMPTY_FC })
      map.addLayer({
        id: 'cone-fill',
        type: 'fill',
        source: 'cones',
        paint: { 'fill-color': '#e66767', 'fill-opacity': 0.22 },
      })
      map.addLayer({
        id: 'cone-line',
        type: 'line',
        source: 'cones',
        paint: { 'line-color': '#e66767', 'line-width': 1.5 },
      })

      // Flagged (recently alerted) vessels, colored by kind, drawn on top.
      map.addSource('flags', { type: 'geojson', data: EMPTY_FC })
      map.addLayer({
        id: 'flag-dot',
        type: 'circle',
        source: 'flags',
        paint: {
          'circle-radius': 7,
          'circle-color': ['get', 'color'],
          'circle-stroke-color': '#0a1128',
          'circle-stroke-width': 2,
        },
      })

      const handleVesselClick = (e) => {
        if (e.features.length > 0) {
          const mmsi = Number(e.features[0].properties.mmsi)
          if (!mmsi) return
          onVesselClickRef.current?.(mmsi)
          // Push the full props (with the factors array intact) from the raw
          // frame, not the click event, which JSON-stringifies nested fields.
          onVesselDataRef.current?.(findVesselProps(mmsi))
        }
      }

      map.on('click', 'vessel-dot', handleVesselClick)
      map.on('click', 'flag-dot', handleVesselClick)
      
      const setPointer = () => { map.getCanvas().style.cursor = 'pointer' }
      const resetPointer = () => { map.getCanvas().style.cursor = '' }
      
      map.on('mouseenter', 'vessel-dot', setPointer)
      map.on('mouseleave', 'vessel-dot', resetPointer)
      map.on('mouseenter', 'flag-dot', setPointer)
      map.on('mouseleave', 'flag-dot', resetPointer)

      readyRef.current = true

      // Kick off the engine-config loads only now that 'vessel-dot' exists
      // (their layers insert beneath it).
      retryUntilLoaded(loadZones)
      retryUntilLoaded(loadPatrols)
    })

    // Expire emphasis layers on a timer.
    const sweep = setInterval(() => {
      if (!readyRef.current) return
      const now = Date.now()
      let changed = false
      for (const [id, v] of conesRef.current) if (v.expires < now) { conesRef.current.delete(id); changed = true }
      for (const [id, v] of linesRef.current) if (v.expires < now) { linesRef.current.delete(id); changed = true }
      for (const [id, v] of flagsRef.current) if (v.expires < now) { flagsRef.current.delete(id); changed = true }
      if (changed) redraw()
    }, 1000)

    const redraw = () => {
      const map = mapRef.current
      if (!map || !readyRef.current) return
      map.getSource('cones')?.setData({ type: 'FeatureCollection', features: [...conesRef.current.values()].map((v) => v.feature) })
      map.getSource('intercepts')?.setData({ type: 'FeatureCollection', features: [...linesRef.current.values()].map((v) => v.feature) })
      map.getSource('flags')?.setData({ type: 'FeatureCollection', features: [...flagsRef.current.values()].map((v) => v.feature) })
    }
    mapRef.current._redraw = redraw

    return () => {
      clearInterval(sweep)
      staticRetryTimersRef.current.forEach(clearTimeout)
      if (rafRef.current) cancelAnimationFrame(rafRef.current)
      if (highlightTimerRef.current) clearTimeout(highlightTimerRef.current)
      if (highlightMarkerRef.current) highlightMarkerRef.current.remove()
      map.remove()
    }
  }, [])

  useImperativeHandle(ref, () => ({
    setVessels(fc) {
      if (!readyRef.current) return
      // Store the latest frame and flush on the next animation frame, collapsing
      // any frames that arrive in between into a single setData.
      pendingVesselsRef.current = fc
      if (rafRef.current) return
      rafRef.current = requestAnimationFrame(() => {
        rafRef.current = 0
        const fc = pendingVesselsRef.current
        pendingVesselsRef.current = null
        if (!fc) return
        mapRef.current?.getSource('vessels')?.setData(fc)
        latestFCRef.current = fc
        // Keep the open drawer live: push the selected vessel's fresh score.
        if (selectedRef.current) {
          onVesselDataRef.current?.(findVesselProps(Number(selectedRef.current)))
        }
      })
    },
    showAlert(a) {
      if (!readyRef.current) return
      const now = Date.now()
      const color = KIND_COLOR[a.kind] || '#5b6b82'
      const isHighOrCrit = a.severity === 'HIGH' || a.severity === 'CRITICAL'
      
      const weight = { CRITICAL: 3, HIGH: 2, MEDIUM: 1, LOW: 0 }
      const newWeight = weight[a.severity] || 0
      const existing = flagsRef.current.get(a.mmsi)
      const existingWeight = existing ? (weight[existing.severity] || 0) : -1

      if (newWeight >= existingWeight) {
        flagsRef.current.set(a.mmsi, {
          feature: { type: 'Feature', geometry: { type: 'Point', coordinates: [a.lon, a.lat] }, properties: { color, mmsi: a.mmsi } },
          expires: isHighOrCrit ? Infinity : now + FLAG_TTL_MS,
          severity: a.severity,
        })
      }
      if (a.cone) {
        conesRef.current.set(a.id, { feature: conePolygon(a.cone, a.id), expires: isHighOrCrit ? Infinity : now + CONE_TTL_MS })
        ;(a.intercepts || []).forEach((ic, i) => {
          const patrol = patrolsRef.current[ic.patrol_id]
          if (patrol) linesRef.current.set(`${a.id}:${ic.patrol_id}`, { feature: interceptLine(patrol, a.cone, ic, `${a.id}:${i}`), expires: isHighOrCrit ? Infinity : now + CONE_TTL_MS })
        })
        mapRef.current.easeTo({ center: [a.lon, a.lat], zoom: 9.2, duration: 1200 })
      }
      mapRef.current._redraw()
    },
    flyTo(lat, lon) {
      if (!readyRef.current) return
      mapRef.current?.easeTo({ center: [lon, lat], zoom: 10, duration: 900 })
    },
    highlightVessel(lat, lon, color) {
      if (!readyRef.current) return
      // Remove any existing highlight marker
      if (highlightMarkerRef.current) highlightMarkerRef.current.remove()
      if (highlightTimerRef.current) clearTimeout(highlightTimerRef.current)

      // Build a custom HTML pulse ring and place it on the map
      const el = document.createElement('div')
      el.className = 'vessel-highlight-marker'
      el.style.setProperty('--hcolor', color || '#3987e5')

      highlightMarkerRef.current = new maplibregl.Marker({ element: el, anchor: 'center' })
        .setLngLat([lon, lat])
        .addTo(mapRef.current)

      // Auto-remove after 10 seconds
      highlightTimerRef.current = setTimeout(() => {
        if (highlightMarkerRef.current) {
          highlightMarkerRef.current.remove()
          highlightMarkerRef.current = null
        }
      }, 10000)
    },
  }))

  return (
    <div style={{ position: 'relative', width: '100%', height: '100%' }}>
      <div className="map" ref={containerRef} />
      
      {/* Legend Overlay */}
      <div className="map-legend">
        <div className="legend-title">Zones Legend</div>
        <div className="legend-item">
          <span className="legend-color" style={{ background: '#3987e5', opacity: 0.7 }}></span>
          EEZ Borders
        </div>
        <div className="legend-item">
          <span className="legend-color" style={{ background: '#8a2be2', opacity: 0.7 }}></span>
          International Waters
        </div>
        <div className="legend-item">
          <span className="legend-color" style={{ background: '#1a8a3f', opacity: 0.7 }}></span>
          Ecological Zone
        </div>
        <div className="legend-item">
          <span className="legend-color" style={{ background: '#ff7b00', opacity: 0.7 }}></span>
          Coral Reefs
        </div>
        <div className="legend-item">
          <span className="legend-color" style={{ background: '#e60000', opacity: 0.7 }}></span>
          Fishing Banned
        </div>
      </div>
    </div>
  )
})

export default memo(MapView)
