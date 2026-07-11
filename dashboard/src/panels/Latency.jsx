// Latency panel. Inline (zone, spoof) and sweep (dark) are shown separately and
// labeled as such, never blurred into one "latency" claim (hot-path rule 3).
// Bars encode magnitude in one hue; p50/p99 are direct-labeled.

import { memo } from 'react'

function Bar({ label, us, max }) {
  const pct = Math.max(2, Math.min(100, (us / max) * 100))
  return (
    <div className="lat-row">
      <span className="lat-name">{label}</span>
      <span className="lat-track">
        <span className="lat-fill" style={{ width: `${pct}%` }} />
      </span>
      <span className="lat-val">{Math.round(us).toLocaleString()} µs</span>
    </div>
  )
}

function Group({ title, p50, p99, max }) {
  return (
    <div className="lat-group">
      <div className="lat-group-title">{title}</div>
      <Bar label="p50" us={p50} max={max} />
      <Bar label="p99" us={p99} max={max} />
    </div>
  )
}

function Latency({ metrics }) {
  const l = metrics?.latency_us || { inline_p50: 0, inline_p99: 0, sweep_p50: 0, sweep_p99: 0 }
  const max = Math.max(l.inline_p99, l.sweep_p99, 1)
  return (
    <div className="panel">
      <div className="panel-header">
        <span className="panel-title">Latency · µs</span>
      </div>
      <div className="panel-body">
        <Group title="Inline · Zone, Spoof" p50={l.inline_p50} p99={l.inline_p99} max={max} />
        <Group title="Sweep · Dark, 1s tick" p50={l.sweep_p50} p99={l.sweep_p99} max={max} />
      </div>
    </div>
  )
}

export default memo(Latency)
