import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react'
import maplibregl from 'maplibre-gl'
import { HTTP_BASE } from '../ws.js'
import { conePolygon, interceptLine } from './cone.js'
import { KIND_COLOR } from '../theme.js'

// Offline style: no external tiles (the pitch runs on localhost with no venue
// wifi). Zones and vessels are drawn as GeoJSON layers over a dark canvas.
const EMPTY_STYLE = {
  version: 8,
  sources: {},
  layers: [{ id: 'bg', type: 'background', paint: { 'background-color': '#e6e2d3' } }],
}

const CENTER = [79.25, 9.0]
const EMPTY_FC = { type: 'FeatureCollection', features: [] }

// How long alert emphasis (flagged vessel, cone, intercept) stays on the map.
const CONE_TTL_MS = 15000
const FLAG_TTL_MS = 9000

const MapView = forwardRef(function MapView({ onVesselClick }, ref) {
  const containerRef = useRef(null)
  const mapRef = useRef(null)
  const readyRef = useRef(false)
  const patrolsRef = useRef({})
  const conesRef = useRef(new Map()) // id -> {feature, expires}
  const flagsRef = useRef(new Map()) // mmsi -> {feature, expires}
  const linesRef = useRef(new Map())
  const onVesselClickRef = useRef(onVesselClick)
  onVesselClickRef.current = onVesselClick

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
      // Zones from the engine (single source of truth).
      try {
        const zones = await fetch(`${HTTP_BASE}/zones`).then((r) => r.json())
        map.addSource('zones', { type: 'geojson', data: zones })
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
            'fill-opacity': [
              'match', ['get', 'type'],
              'ecological-zone', 0.15,
              'eez', 0.05,
              'coral-reef', 0.2,
              'fishing-banned', 0.2,
              'international-water', 0.1,
              0.14 // default
            ],
          },
        })
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
            'line-width': ['match', ['get', 'type'], 'eez', 1, 1.5],
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
          'circle-radius': ['interpolate', ['linear'], ['zoom'], 6, 1.5, 10, 3],
          'circle-color': '#3e5c76',
          'circle-opacity': 0.8,
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
          const props = e.features[0].properties
          if (props.mmsi && onVesselClickRef.current) {
            onVesselClickRef.current(props.mmsi)
          }
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
      map.remove()
    }
  }, [])

  useImperativeHandle(ref, () => ({
    setVessels(fc) {
      if (readyRef.current) mapRef.current.getSource('vessels')?.setData(fc)
    },
    showAlert(a) {
      if (!readyRef.current) return
      const now = Date.now()
      const color = KIND_COLOR[a.kind] || '#5b6b82'
      flagsRef.current.set(a.mmsi, {
        feature: { type: 'Feature', geometry: { type: 'Point', coordinates: [a.lon, a.lat] }, properties: { color, mmsi: a.mmsi } },
        expires: now + FLAG_TTL_MS,
      })
      if (a.cone) {
        conesRef.current.set(a.id, { feature: conePolygon(a.cone, a.id), expires: now + CONE_TTL_MS })
        ;(a.intercepts || []).forEach((ic, i) => {
          const patrol = patrolsRef.current[ic.patrol_id]
          if (patrol) linesRef.current.set(`${a.id}:${ic.patrol_id}`, { feature: interceptLine(patrol, a.cone, ic, `${a.id}:${i}`), expires: now + CONE_TTL_MS })
        })
        mapRef.current.easeTo({ center: [a.lon, a.lat], zoom: 9.2, duration: 1200 })
      }
      mapRef.current._redraw()
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

export default MapView
