import { KIND_COLOR, KIND_LABEL } from '../theme.js'

function detailLine(a) {
  if (a.kind === 'SPOOF_TELEPORT') return `implied ${Math.round(a.detail?.implied_speed_kn ?? 0)} kn`
  if (a.kind === 'DARK_EVENT') return `silent ${Math.round(a.detail?.silence_s ?? 0)} s · ${a.zone_id}`
  return a.zone_id
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

function Row({ a }) {
  const color = KIND_COLOR[a.kind] || '#8e95a2'
  return (
    <div className="feed-row">
      <div className="feed-indicator" style={{ background: color }} />
      <div className="feed-body">
        <div className="feed-head">
          <span className="feed-kind" style={{ color }}>
            {KIND_LABEL[a.kind] || a.kind}
          </span>
          <span className="feed-name">{a.name || `MMSI ${a.mmsi}`}</span>
          {a.severity === 'CRITICAL' && <span className="feed-crit">CRITICAL</span>}
        </div>
        <div className="feed-detail">{detailLine(a)}</div>
        <Intercepts list={a.intercepts} />
      </div>
    </div>
  )
}

export default function AlertFeed({ alerts }) {
  return (
    <div className="panel feed">
      <div className="panel-header">
        <span className="panel-title">Alerts</span>
        {alerts.length > 0 && (
          <span className="panel-title-count">{alerts.length}</span>
        )}
      </div>
      <div className="feed-list">
        {alerts.length === 0 ? (
          <div className="feed-empty">No alerts detected yet</div>
        ) : (
          alerts.map((a) => <Row key={a.id} a={a} />)
        )}
      </div>
    </div>
  )
}
