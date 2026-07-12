import { useCallback, useEffect, useRef, useState } from 'react'
import MapView from './map/MapView.jsx'
import Hud from './panels/Hud.jsx'
import AlertFeed from './panels/AlertFeed.jsx'
import Latency from './panels/Latency.jsx'
import FeatureCards from './panels/FeatureCards.jsx'
import VesselDetails from './panels/VesselDetails.jsx'
import LandingPage from './LandingPage.jsx'
import { connect } from './ws.js'
import { KIND_COLOR } from './theme.js'

export default function App() {
  const [view, setView] = useState('landing') // 'landing' | 'dashboard'

  if (view === 'landing') {
    return <LandingPage onEnter={() => setView('dashboard')} />
  }

  return <Dashboard />
}

function Dashboard() {
  const [metrics, setMetrics] = useState(null)
  const [alerts, setAlerts] = useState([])
  const [status, setStatus] = useState('connecting')
  const [rightTab, setRightTab] = useState('details')
  const [selectedMMSI, setSelectedMMSI] = useState(null)
  const [selectedVesselData, setSelectedVesselData] = useState(null)
  // Mobile bottom nav: 'map' | 'alerts' | 'stats'
  const [mobileTab, setMobileTab] = useState('map')
  const mapRef = useRef(null)

  const onPositions = useCallback((fc) => mapRef.current?.setVessels(fc), [])
  const onAlert = useCallback((a) => {
    setAlerts((prev) => {
      const next = [a, ...prev]
      const kept = []
      let otherCount = 0
      for (const item of next) {
        if (item.severity === 'HIGH' || item.severity === 'CRITICAL') {
          kept.push(item)
        } else if (otherCount < 40) {
          kept.push(item)
          otherCount++
        }
      }
      return kept
    })
    mapRef.current?.showAlert(a)
  }, [])
  // Stable so memo(MapView) is not defeated by a fresh closure every render.
  const onVesselClick = useCallback((mmsi) => {
    setSelectedMMSI(mmsi)
    setRightTab('vessel')
    setMobileTab('stats')
  }, [])

  // Fly the map to a clicked alert's vessel position and switch mobile to map.
  const onAlertClick = useCallback((a) => {
    const color = KIND_COLOR[a.kind] || '#3987e5'
    mapRef.current?.flyTo(a.lat, a.lon)
    mapRef.current?.highlightVessel(a.lat, a.lon, color)
    setMobileTab('map')
  }, [])
  // Live risk data (score, tier, factors) for the selected vessel, pushed by
  // MapView from the raw position frame so the Score Breakdown Drawer stays live.
  const onVesselData = useCallback((props) => setSelectedVesselData(props), [])

  useEffect(() => connect({ onMetrics: setMetrics, onAlert, onPositions, onStatus: setStatus }), [onAlert, onPositions])

  const statusLabel = status === 'connected' ? 'Live' : status === 'disconnected' ? 'Offline' : 'Connecting'

  return (
    <div className="app">
      {/* ── Top Header Bar ── */}
      <header className="topbar">
        <div className="brand">
          <img src="/logo.svg" alt="Reef Watchers" className="brand-icon" style={{ width: '32px', height: '32px', marginRight: '12px', objectFit: 'contain', borderRadius: '50%' }} />
          Reef Watchers
        </div>
        <div className="topbar-sep" />
        <div className="topbar-meta">
          <span className="topbar-route">Current Location: Danish Waters</span>
          <span className="topbar-tag">Real-Time Engine</span>
          <span className="topbar-tag">AIS Surveillance</span>
        </div>
        <div className="topbar-right">
          <span className="topbar-update">
            Last update <strong>{status === 'connected' ? 'live' : '—'}</strong>
          </span>
          <span className={`status-badge ${status}`}>
            <span className="status-dot" />
            {statusLabel}
          </span>
        </div>
      </header>

      {/* ── 6 Feature Cards ── */}
      <FeatureCards metrics={metrics} alerts={alerts} />

      {/* ── Main Content: Left Panel | Map | Right Panel ── */}
      <div className="main-content">
        {/* Left panel: Alert feed */}
        <div className={`left-panel${mobileTab === 'alerts' ? ' mobile-panel-visible' : ''}`}>
          <AlertFeed alerts={alerts} onAlertClick={onAlertClick} />
        </div>

        {/* Center: Map */}
        <div className="map-container">
          <MapView ref={mapRef} onVesselClick={onVesselClick} onVesselData={onVesselData} selectedMMSI={selectedMMSI} />
          <MapLegend />

          {status !== 'connected' && (
            <div className="map-notice">
              <div className="notice-icon">i</div>
              <div className="notice-text">
                <strong>Waiting for engine connection</strong><br />
                Start the Go engine on port 8080 to stream live vessel data.
              </div>
            </div>
          )}
        </div>

        {/* Right panel: Details & Latency */}
        <div className={`right-panel${mobileTab === 'stats' ? ' mobile-panel-visible' : ''}`}>
          <div className="right-panel-tabs">
            <div
              className={`right-tab${rightTab === 'details' ? ' active' : ''}`}
              onClick={() => setRightTab('details')}
            >
              Engine Stats
            </div>
            <div
              className={`right-tab${rightTab === 'latency' ? ' active' : ''}`}
              onClick={() => setRightTab('latency')}
            >
              Latency
            </div>
            {selectedMMSI && (
              <div
                className={`right-tab${rightTab === 'vessel' ? ' active' : ''}`}
                onClick={() => setRightTab('vessel')}
              >
                Vessel
              </div>
            )}
          </div>
          <div className="right-panel-body">
            {rightTab === 'details' ? (
              <Hud metrics={metrics} />
            ) : rightTab === 'latency' ? (
              <Latency metrics={metrics} />
            ) : selectedMMSI ? (
              <VesselDetails mmsi={selectedMMSI} alerts={alerts} vesselData={selectedVesselData} />
            ) : null}
          </div>
        </div>
      </div>

      {/* ── Mobile Bottom Navigation Bar ── */}
      <nav className="mobile-nav">
        <button
          className={`mobile-nav-tab${mobileTab === 'map' ? ' active' : ''}`}
          onClick={() => setMobileTab('map')}
        >
          <span className="mobile-nav-icon">🗺</span>
          Map
        </button>
        <button
          className={`mobile-nav-tab${mobileTab === 'alerts' ? ' active' : ''}`}
          onClick={() => setMobileTab('alerts')}
        >
          <span className="mobile-nav-icon">⚠</span>
          Alerts{alerts.filter(a => a.kind === 'DARK_EVENT').length > 0
            ? ` (${alerts.filter(a => a.kind === 'DARK_EVENT').length})`
            : ''}
        </button>
        <button
          className={`mobile-nav-tab${mobileTab === 'stats' ? ' active' : ''}`}
          onClick={() => setMobileTab('stats')}
        >
          <span className="mobile-nav-icon">📊</span>
          Stats
        </button>
      </nav>
    </div>
  )
}

// Static key for the map marks: zones (the coloured squares), alert kinds
// (flagged vessel dots), and assets. Pure presentation, no engine data.
function MapLegend() {
  return (
    <div className="map-legend">
      <div className="legend-title">Legend</div>
      <div className="legend-group">
        <span className="legend-label">Zones</span>
        <div className="legend-row">
          <span className="legend-swatch sq" style={{ background: 'rgba(230,103,103,0.18)', borderColor: '#e66767' }} />
          Restricted / MPA
        </div>
        <div className="legend-row">
          <span className="legend-swatch sq" style={{ background: 'rgba(57,135,229,0.10)', borderColor: '#3987e5' }} />
          EEZ · national waters
        </div>
      </div>
      <div className="legend-group">
        <span className="legend-label">Alerts</span>
        <div className="legend-row"><span className="legend-swatch dot" style={{ background: '#3987e5' }} />Zone violation</div>
        <div className="legend-row"><span className="legend-swatch dot" style={{ background: '#c98500' }} />Spoof / teleport</div>
        <div className="legend-row"><span className="legend-swatch dot" style={{ background: '#e66767' }} />Dark event</div>
        <div className="legend-row"><span className="legend-swatch dot" style={{ background: '#9d4edd' }} />Trawling pattern</div>
        <div className="legend-row"><span className="legend-swatch dot" style={{ background: '#20b2aa' }} />Longlining pattern</div>
        <div className="legend-row"><span className="legend-swatch dot" style={{ background: '#ff007f' }} />Purse seining loop</div>
      </div>
      <div className="legend-group">
        <span className="legend-label">Assets</span>
        <div className="legend-row"><span className="legend-swatch dot ring" style={{ background: '#1a2332' }} />Patrol vessel</div>
        <div className="legend-row"><span className="legend-swatch dot" style={{ background: '#3e5c76' }} />Vessel</div>
        <div className="legend-row"><span className="legend-swatch line" style={{ background: '#0ca30c' }} />Intercept · feasible</div>
      </div>
    </div>
  )
}
