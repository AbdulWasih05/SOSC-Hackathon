import { memo } from 'react'
import { compact } from '../theme.js'

// 6 feature summary cards displayed across the top, mirroring the
// efficiency / compliance card row in vessel monitoring platforms.
// Cards show KPIs derived from the live metrics and alert count.

function FeatureCards({ metrics, alerts }) {
  const m = metrics || {}
  const rateNum = m.rate_per_s ?? 0
  const activeNum = m.active_vessels ?? 0
  const processedNum = m.processed_total ?? 0
  const droppedNum = m.dropped_total ?? 0
  const alertsNum = m.alerts_total ?? 0

  // Derive threat level from alerts
  const recentCritical = alerts.filter((a) => a.severity === 'CRITICAL').length
  const threatLevel = recentCritical > 2 ? 'High' : recentCritical > 0 ? 'Medium' : 'Low'
  const threatClass = recentCritical > 2 ? 'warn' : recentCritical > 0 ? 'accent' : 'success'

  // Derive integrity from drop rate
  const dropRate = processedNum > 0 ? ((droppedNum / (processedNum + droppedNum)) * 100) : 0
  const integrity = dropRate < 0.1 ? '99.9%' : dropRate < 1 ? (100 - dropRate).toFixed(1) + '%' : (100 - dropRate).toFixed(0) + '%'

  return (
    <div className="cards-row">
      {/* Card 1: Throughput */}
      <div className="feature-card active">
        <span className="card-section-label">Performance</span>
        <div className="card-title">Throughput</div>
        <div className="card-value accent">{compact(rateNum)}</div>
        <div className="card-sub">
          <span className="unit">msgs/sec</span>
        </div>
      </div>

      {/* Card 2: Active Vessels */}
      <div className="feature-card">
        <div className="card-title">Active Vessels</div>
        <div className="card-value">{compact(activeNum)}</div>
        <div className="card-sub">
          <span className="unit">tracked</span>
        </div>
      </div>

      {/* Card 3: Alerts Fired */}
      <div className="feature-card">
        <div className="card-title">Alerts Fired</div>
        <div className="card-value">{compact(alertsNum)}</div>
        <div className="card-sub">
          <span className="unit">total detections</span>
        </div>
      </div>

      {/* Card 4: Threat Level */}
      <div className="feature-card">
        <span className="card-section-label">Compliance</span>
        <div className="card-title">Threat Level</div>
        <div className={`card-value ${threatClass}`}>{threatLevel}</div>
        <div className="card-sub">
          <span className="unit">{recentCritical} critical alert{recentCritical !== 1 ? 's' : ''}</span>
        </div>
      </div>

      {/* Card 5: Data Integrity */}
      <div className="feature-card">
        <div className="card-title">Data Integrity</div>
        <div className={`card-value ${droppedNum > 0 ? 'warn' : 'success'}`}>{integrity}</div>
        <div className="card-sub">
          <span className="unit">{compact(droppedNum)} dropped</span>
        </div>
      </div>

      {/* Card 6: Processed */}
      <div className="feature-card">
        <div className="card-title">Total Processed</div>
        <div className="card-value">{compact(processedNum)}</div>
        <div className="card-sub">
          <span className="unit">messages</span>
        </div>
      </div>
    </div>
  )
}

export default memo(FeatureCards)
