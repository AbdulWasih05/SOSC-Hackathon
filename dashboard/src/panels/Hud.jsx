import { compact } from '../theme.js'

// Stat-tile row (dataviz: a headline number is not a chart). Numbers wear text
// tokens and tabular figures; the rate is the hero.
function Tile({ label, value, hero, warn }) {
  return (
    <div className={`tile${hero ? ' hero' : ''}`}>
      <div className={`val${warn ? ' warn' : ''}`}>{value}</div>
      <div className="lbl">{label}</div>
    </div>
  )
}

export default function Hud({ metrics }) {
  const m = metrics || {}
  return (
    <div className="hud">
      <Tile label="msgs / sec" value={compact(m.rate_per_s)} hero />
      <Tile label="processed" value={compact(m.processed_total)} />
      <Tile label="active vessels" value={compact(m.active_vessels)} />
      <Tile label="alerts" value={compact(m.alerts_total)} />
      <Tile label="dropped" value={compact(m.dropped_total)} warn={m.dropped_total > 0} />
    </div>
  )
}
