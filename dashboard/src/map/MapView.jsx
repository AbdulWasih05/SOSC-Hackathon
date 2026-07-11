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

// zonesBounds returns [[minLon,minLat],[maxLon,maxLat]] over every zone polygon
// so the map centers on whichever region the engine serves (North Sea for the
// live feed, Gulf of Mannar for the scenario), with no hardcoded region.
function zonesBounds(fc) {
  let minLon = Infinity, minLat = Infinity, maxLon = -Infinity, maxLat = -Infinity
  const visit = (coords) => {
    if (typeof coords[0] === 'number') {
      const [lon, lat] = coords
      if (lon < minLon) minLon = lon
      if (lat < minLat) minLat = lat
      if (lon > maxLon) maxLon = lon
      if (lat > maxLat) maxLat = lat
      return
    }
    coords.forEach(visit)
  }
  for (const f of fc.features || []) if (f.geometry?.coordinates) visit(f.geometry.coordinates)
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

    map.on('load', async () => {
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

      // Zones from the engine (single source of truth).
      try {
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
            'fill-color': ['match', ['get', 'type'], 'mpa', '#e66767', /* eez */ '#3987e5'],
            'fill-opacity': ['match', ['get', 'type'], 'mpa', 0.14, 0.05],
          },
        })
        map.addLayer({
          id: 'zone-line',
          type: 'line',
          source: 'zones',
          paint: {
            'line-color': ['match', ['get', 'type'], 'mpa', '#e66767', '#3987e5'],
            'line-width': ['match', ['get', 'type'], 'mpa', 1.5, 1],
            'line-opacity': 0.7,
          },
        })
      } catch {
        /* zones optional; map still works */
      }

      // Patrol assets.
      try {
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
        })
      } catch {
        /* patrols optional */
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

  return <div className="map" ref={containerRef} />
})

export default memo(MapView)
