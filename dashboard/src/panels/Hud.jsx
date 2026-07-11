import { compact } from '../theme.js'

// Engine stats panel rendered as a table in the right sidebar,
// similar to the vessel details pane in the reference design.

export default function Hud({ metrics }) {
  const m = metrics || {}
  return (
    <div className="panel hud-section">
      <div className="panel-header">
        <span className="panel-title">Engine Metrics</span>
      </div>
      <div className="panel-body">
        <table className="stats-table">
          <tbody>
            <tr>
              <td>Throughput</td>
              <td>{compact(m.rate_per_s)}</td>
              <td>msgs/sec</td>
            </tr>
            <tr>
              <td>Processed</td>
              <td>{compact(m.processed_total)}</td>
              <td>total</td>
            </tr>
            <tr>
              <td>Active Vessels</td>
              <td>{compact(m.active_vessels)}</td>
              <td>tracked</td>
            </tr>
            <tr>
              <td>Alerts</td>
              <td>{compact(m.alerts_total)}</td>
              <td>total</td>
            </tr>
            <tr>
              <td>Ingested</td>
              <td>{compact(m.ingested_total)}</td>
              <td>total</td>
            </tr>
            <tr>
              <td>Dropped</td>
              <td style={m.dropped_total > 0 ? { color: 'var(--crit)' } : undefined}>
                {compact(m.dropped_total)}
              </td>
              <td>{m.dropped_total > 0 ? '⚠' : ''}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  )
}
