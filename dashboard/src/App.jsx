import { useCallback, useEffect, useRef, useState } from 'react'
import MapView from './map/MapView.jsx'
import Hud from './panels/Hud.jsx'
import AlertFeed from './panels/AlertFeed.jsx'
import Latency from './panels/Latency.jsx'
import FeatureCards from './panels/FeatureCards.jsx'
import VesselDetails from './panels/VesselDetails.jsx'
import { connect } from './ws.js'

export default function App() {
  const [metrics, setMetrics] = useState(null)
  const [alerts, setAlerts] = useState([])
  const [status, setStatus] = useState('connecting')
  const [rightTab, setRightTab] = useState('details')
  const [selectedMMSI, setSelectedMMSI] = useState(null)
  const mapRef = useRef(null)

  const onPositions = useCallback((fc) => mapRef.current?.setVessels(fc), [])
  const onAlert = useCallback((a) => {
    setAlerts((prev) => [a, ...prev].slice(0, 40))
    mapRef.current?.showAlert(a)
  }, [])

  useEffect(() => connect({ onMetrics: setMetrics, onAlert, onPositions, onStatus: setStatus }), [onAlert, onPositions])

  const statusLabel = status === 'connected' ? 'Live' : status === 'disconnected' ? 'Offline' : 'Connecting'

  return (
    <div className="app">
      {/* ── Top Header Bar ── */}
      <header className="topbar">
        <div className="brand">
          <img src="/logo.png" alt="Reef Watchers" className="brand-icon" style={{ width: '28px', height: '28px', marginRight: '12px', objectFit: 'contain' }} />
          Reef Watchers
        </div>
        <div className="topbar-sep" />
        <div className="topbar-meta">
          <span className="topbar-route">Current Location: Palk Strait · Gulf of Mannar</span>
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
        <div className="left-panel">
          <AlertFeed alerts={alerts} />
        </div>

        {/* Center: Map */}
        <div className="map-container">
          <MapView ref={mapRef} onVesselClick={(mmsi) => {
            setSelectedMMSI(mmsi)
            setRightTab('vessel')
          }} />
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
        <div className="right-panel">
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
              <VesselDetails mmsi={selectedMMSI} alerts={alerts} />
            ) : null}
          </div>
        </div>
      </div>
    </div>
  )
}
