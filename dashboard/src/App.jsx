import { useCallback, useEffect, useRef, useState } from 'react'
import MapView from './map/MapView.jsx'
import Hud from './panels/Hud.jsx'
import AlertFeed from './panels/AlertFeed.jsx'
import Latency from './panels/Latency.jsx'
import { connect } from './ws.js'

export default function App() {
  const [metrics, setMetrics] = useState(null)
  const [alerts, setAlerts] = useState([])
  const [status, setStatus] = useState('connecting')
  const mapRef = useRef(null)

  const onPositions = useCallback((fc) => mapRef.current?.setVessels(fc), [])
  const onAlert = useCallback((a) => {
    setAlerts((prev) => [a, ...prev].slice(0, 40))
    mapRef.current?.showAlert(a)
  }, [])

  useEffect(() => connect({ onMetrics: setMetrics, onAlert, onPositions, onStatus: setStatus }), [onAlert, onPositions])

  return (
    <div className="app">
      <MapView ref={mapRef} />
      <div className="overlay">
        <header className="topbar">
          <div className="brand">
            PALK WATCH
            <span className={`status ${status}`} title={status} />
          </div>
          <Hud metrics={metrics} />
        </header>
        <aside className="side">
          <AlertFeed alerts={alerts} />
          <Latency metrics={metrics} />
        </aside>
      </div>
    </div>
  )
}
