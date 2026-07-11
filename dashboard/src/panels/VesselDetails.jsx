import { memo } from 'react'
import { KIND_LABEL } from '../theme.js'

function VesselDetails({ mmsi, alerts }) {
  if (!mmsi) return <div className="feed-empty">No vessel selected</div>
  
  const vesselAlerts = alerts.filter(a => a.mmsi === mmsi)
  const name = vesselAlerts.length > 0 ? vesselAlerts[0].name : 'Unknown Vessel'
  
  const hasCritical = vesselAlerts.some(a => a.severity === 'CRITICAL')
  const hasHigh = vesselAlerts.some(a => a.severity === 'HIGH')
  const risk = hasCritical ? 'HIGH' : hasHigh ? 'MEDIUM' : vesselAlerts.length > 0 ? 'LOW' : 'NONE'
  const riskClass = risk.toLowerCase()
  
  return (
    <div className="vessel-details">
      <div className="panel-title" style={{ marginBottom: 16 }}>Vessel Information</div>
      <table className="stats-table" style={{ marginBottom: 24 }}>
        <tbody>
          <tr>
            <td>MMSI</td>
            <td style={{ textAlign: 'right', fontWeight: 'bold' }}>{mmsi}</td>
          </tr>
          <tr>
            <td>Name</td>
            <td style={{ textAlign: 'right', fontWeight: 'bold' }}>{name}</td>
          </tr>
          <tr>
            <td>Threat Level</td>
            <td style={{ textAlign: 'right' }}>
              <span className={`risk-badge ${riskClass === 'none' ? 'low' : riskClass}`}>{risk}</span>
            </td>
          </tr>
        </tbody>
      </table>

      <div className="panel-title" style={{ marginBottom: 12 }}>Suspicious Activities ({vesselAlerts.length})</div>
      {vesselAlerts.length === 0 ? (
        <div className="feed-empty" style={{ border: '1px solid var(--border)', background: 'var(--surface-alt)' }}>No suspicious activities logged.</div>
      ) : (
        <div className="feed-list" style={{ gap: '8px' }}>
          {vesselAlerts.map(a => (
            <div key={a.id} className="feed-row" style={{ border: '1px solid var(--border)', background: 'var(--surface)' }}>
              <div className="feed-body">
                <div className="feed-head" style={{ marginBottom: '6px' }}>
                  <span className="feed-kind" style={{ color: 'var(--ink)' }}>{KIND_LABEL[a.kind] || a.kind}</span>
                  <span className="feed-crit" style={{ background: a.severity === 'CRITICAL' ? 'var(--crit)' : 'var(--warn)' }}>
                    {a.severity}
                  </span>
                </div>
                <div className="feed-detail" style={{ marginTop: '0', fontSize: '11px', fontWeight: 'bold', color: 'var(--ink-2)' }}>
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
