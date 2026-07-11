// Validated categorical palette (dataviz skill: passes lightness band, chroma,
// CVD 35.9, contrast on the navy surface). One hue per alert kind, fixed order.
export const KIND_COLOR = {
  ZONE_VIOLATION: '#3987e5', // blue
  SPOOF_TELEPORT: '#c98500', // yellow
  DARK_EVENT: '#e66767', // red
  TRAWLING: '#9d4edd', // purple
  LONGLINING: '#20b2aa', // teal
  PURSE_SEINING: '#ff007f', // pink
  ILLEGAL_FISHING_SUSPECTED: '#e8791a', // orange (risk HIGH)
  BOARDING_RECOMMENDED: '#e11d1d', // red (risk CRITICAL)
}

export const KIND_LABEL = {
  ZONE_VIOLATION: 'ZONE',
  SPOOF_TELEPORT: 'SPOOF',
  DARK_EVENT: 'DARK',
  TRAWLING: 'TRAWL',
  LONGLINING: 'LONGLINE',
  PURSE_SEINING: 'SEINE',
  ILLEGAL_FISHING_SUSPECTED: 'SUSPECTED',
  BOARDING_RECOMMENDED: 'BOARD',
}

// Risk tier colors (P0): LOW muted slate, ELEVATED amber, HIGH orange, CRITICAL
// red. One ramp, used for vessel dots and the Score Breakdown Drawer badge.
export const TIER_COLOR = {
  LOW: '#3e5c76',
  ELEVATED: '#d99000',
  HIGH: '#e8791a',
  CRITICAL: '#e11d1d',
}

// tierColor returns the ramp color for a tier, defaulting to the LOW slate.
export function tierColor(tier) {
  return TIER_COLOR[tier] || TIER_COLOR.LOW
}

// isAlertTab decides which feed events count as actionable Alerts (the left
// tab, the Alerts Fired card, and the mobile badge all share this): only
// ZONE_VIOLATION, at HIGH or CRITICAL severity. Everything else (dark events,
// spoof, fishing patterns, tier-transition escalations) is a Log.
export function isAlertTab(a) {
  return a.kind === 'ZONE_VIOLATION' && (a.severity === 'CRITICAL' || a.severity === 'HIGH')
}

// Compact number format: 8,523,519 -> 8.5M.
export function compact(n) {
  if (n == null) return '—'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'k'
  return String(Math.round(n))
}
