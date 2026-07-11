// Validated categorical palette (dataviz skill: passes lightness band, chroma,
// CVD 35.9, contrast on the navy surface). One hue per alert kind, fixed order.
export const KIND_COLOR = {
  ZONE_VIOLATION: '#3987e5', // blue
  SPOOF_TELEPORT: '#c98500', // yellow
  DARK_EVENT: '#e66767', // red
  TRAWLING: '#9d4edd', // purple
  LONGLINING: '#20b2aa', // teal
  PURSE_SEINING: '#ff007f', // pink
}

export const KIND_LABEL = {
  ZONE_VIOLATION: 'ZONE',
  SPOOF_TELEPORT: 'SPOOF',
  DARK_EVENT: 'DARK',
  TRAWLING: 'TRAWL',
  LONGLINING: 'LONGLINE',
  PURSE_SEINING: 'SEINE',
}

// Compact number format: 8,523,519 -> 8.5M.
export function compact(n) {
  if (n == null) return '—'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'k'
  return String(Math.round(n))
}
