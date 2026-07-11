import { memo } from 'react'
import { KIND_LABEL, tierColor } from '../theme.js'

// Score Breakdown Drawer: the product's face. Click a vessel, see its 0-100 IUU
// suspicion score, its tier, and the exact factor list, every point a readable
// fact. The score never says "illegal: yes/no"; it ranks suspicion with visible
// evidence so patrols prioritize and authorities verify.
function VesselDetails({ mmsi, alerts, vesselData }) {
  if (!mmsi) return <div className="feed-empty">No vessel selected</div>

  const vesselAlerts = alerts.filter((a) => a.mmsi === mmsi)
  const name = vesselAlerts.length > 0 ? vesselAlerts[0].name : 'Unknown Vessel'

  const scored = vesselData && vesselData.risk_tier
  const score = scored ? vesselData.risk_score || 0 : null
  const tier = scored ? vesselData.risk_tier : null
  const factors = (scored && vesselData.factors) || []
  const color = scored ? tierColor(tier) : '#3e5c76'

  return (
    <div className="vessel-details">
      {/* ── Score header ── */}
      {scored ? (
        <div
          className="risk-score-card"
          style={{
            display: 'flex', alignItems: 'center', gap: 16, marginBottom: 18,
            padding: '16px', borderRadius: 10,
            border: `1px solid ${color}`, background: `${color}18`,
          }}
        >
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 72 }}>
            <div style={{ fontSize: 40, fontWeight: 800, lineHeight: 1, color }}>{score}</div>
            <div style={{ fontSize: 10, letterSpacing: 1, color: 'var(--ink-2)', marginTop: 2 }}>/ 100</div>
          </div>
          <div style={{ flex: 1 }}>
            <span
              style={{
                display: 'inline-block', padding: '3px 10px', borderRadius: 5,
                fontSize: 12, fontWeight: 700, letterSpacing: 0.5,
                color: '#fff', background: color,
              }}
            >
              {tier}
            </span>
            <div style={{ fontSize: 11, color: 'var(--ink-2)', marginTop: 8, lineHeight: 1.4 }}>
              IUU suspicion score. Evidence attached, not a verdict.
            </div>
          </div>
        </div>
      ) : (
        <div className="feed-empty" style={{ marginBottom: 18, border: '1px solid var(--border)', background: 'var(--surface-alt)' }}>
          No risk score yet. Enable the risk engine (-risk) and wait for the first sweep.
        </div>
      )}

      {/* ── Vessel identity ── */}
      <table className="stats-table" style={{ marginBottom: 20 }}>
        <tbody>
          <tr>
            <td>MMSI</td>
            <td style={{ textAlign: 'right', fontWeight: 'bold' }}>{mmsi}</td>
          </tr>
          <tr>
            <td>Name</td>
            <td style={{ textAlign: 'right', fontWeight: 'bold' }}>{name}</td>
          </tr>
        </tbody>
      </table>

      {/* ── Factor breakdown ── */}
      <div className="panel-title" style={{ marginBottom: 12 }}>Score Breakdown ({factors.length})</div>
      {factors.length === 0 ? (
        <div className="feed-empty" style={{ border: '1px solid var(--border)', background: 'var(--surface-alt)' }}>
          No contributing factors.
        </div>
      ) : (
        <div className="feed-list" style={{ gap: 8 }}>
          {factors.map((f) => (
            <div key={f.code} className="feed-row" style={{ border: '1px solid var(--border)', background: 'var(--surface)', display: 'block' }}>
              <div className="feed-body">
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 6 }}>
                  <span className="feed-kind" style={{ color: 'var(--ink)' }}>{f.label}</span>
                  <span style={{ fontWeight: 800, color, fontSize: 15 }}>+{f.points}</span>
                </div>
                {/* Contribution bar (points out of 100). */}
                <div style={{ height: 4, borderRadius: 2, background: 'var(--surface-alt)', overflow: 'hidden', marginBottom: 6 }}>
                  <div style={{ height: '100%', width: `${Math.min(100, f.points)}%`, background: color }} />
                </div>
                <div style={{ fontSize: 10, fontWeight: 700, color: 'var(--ink-2)' }}>
                  {f.code} · {f.ts_ms ? new Date(f.ts_ms).toLocaleTimeString() : ''}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* ── Raw alert history ── */}
      <div className="panel-title" style={{ margin: '20px 0 12px' }}>Alert History ({vesselAlerts.length})</div>
      {vesselAlerts.length === 0 ? (
        <div className="feed-empty" style={{ border: '1px solid var(--border)', background: 'var(--surface-alt)' }}>No alerts logged.</div>
      ) : (
        <div className="feed-list" style={{ gap: 8 }}>
          {vesselAlerts.map((a) => (
            <div key={a.id} className="feed-row" style={{ border: '1px solid var(--border)', background: 'var(--surface)' }}>
              <div className="feed-body">
                <div className="feed-head" style={{ marginBottom: 6 }}>
                  <span className="feed-kind" style={{ color: 'var(--ink)' }}>{KIND_LABEL[a.kind] || a.kind}</span>
                  <span className="feed-crit" style={{ background: a.severity === 'CRITICAL' ? 'var(--crit)' : 'var(--warn)' }}>
                    {a.severity}
                  </span>
                </div>
                <div className="feed-detail" style={{ marginTop: 0, fontSize: 11, fontWeight: 'bold', color: 'var(--ink-2)' }}>
                  {new Date(a.ts_ms).toLocaleTimeString()}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default memo(VesselDetails)
