import { memo, useState } from 'react'
import { KIND_COLOR, KIND_LABEL, isAlertTab } from '../theme.js'

function detailLine(a) {
  if (a.kind === 'SPOOF_TELEPORT') return `implied ${Math.round(a.detail?.implied_speed_kn ?? 0)} kn`
  if (a.kind === 'DARK_EVENT') return `silent ${Math.round(a.detail?.silence_s ?? 0)} s · ${a.zone_id}`
  if (a.detail?.pattern) return a.detail.pattern
  return a.zone_id || ''
}

function Intercepts({ list }) {
  if (!list?.length) return null
  return (
    <div className="ic-list">
      {list.map((ic) => (
        <span key={ic.patrol_id} className={`ic${ic.feasible ? '' : ' infeasible'}`}>
          {ic.patrol_id} {Math.round(ic.eta_s / 60)}m {ic.feasible ? '✓' : '✕'}
        </span>
      ))}
    </div>
  )
}

function Row({ a, onAlertClick }) {
  const color = KIND_COLOR[a.kind] || '#8e95a2'
  return (
    <div
      className="feed-row"
      style={{ cursor: 'pointer' }}
      onClick={() => onAlertClick?.(a)}
    >
      <div className="feed-indicator" style={{ background: color }} />
      <div className="feed-body">
        <div className="feed-head">
          <span className="feed-kind" style={{ color }}>
            {KIND_LABEL[a.kind] || a.kind}
          </span>
          <span className="feed-name">{a.name || `MMSI ${a.mmsi}`}</span>
          {a.severity === 'CRITICAL' && <span className="feed-crit">CRITICAL</span>}
          {a.severity === 'MEDIUM' && <span className="feed-log-badge">LOG</span>}
        </div>
        <div className="feed-detail">{detailLine(a)}</div>
        <Intercepts list={a.intercepts} />
      </div>
    </div>
  )
}

function EmptyState({ tab }) {
  return (
    <div className="feed-empty">
      {tab === 'alerts'
        ? 'No zone violations detected'
        : 'No observational logs yet'}
    </div>
  )
}

function AlertFeed({ alerts, onAlertClick }) {
  const [tab, setTab] = useState('alerts')

  const alertItems = alerts.filter(isAlertTab)
  const logItems   = alerts  // Logs = full audit trail of every event
  const active     = tab === 'alerts' ? alertItems : logItems

  return (
    <div className="panel feed">
      {/* Tab header */}
      <div className="feed-tabs">
        <button
          className={`feed-tab${tab === 'alerts' ? ' active' : ''}`}
          onClick={() => setTab('alerts')}
        >
          Alerts
          {alertItems.length > 0 && (
            <span className="feed-tab-badge feed-tab-badge--alert">{alertItems.length}</span>
          )}
        </button>
        <button
          className={`feed-tab${tab === 'logs' ? ' active' : ''}`}
          onClick={() => setTab('logs')}
        >
          Logs
          {logItems.length > 0 && (
            <span className="feed-tab-badge">{logItems.length}</span>
          )}
        </button>
      </div>

      {/* Feed list */}
      <div className="feed-list">
        {active.length === 0 ? (
          <EmptyState tab={tab} />
        ) : (
          active.map((a) => <Row key={a.id} a={a} onAlertClick={onAlertClick} />)
        )}
      </div>
    </div>
  )
}

export default memo(AlertFeed)
