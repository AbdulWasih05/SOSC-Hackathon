// Single websocket client for the whole dashboard (no state library, no axios;
// native WebSocket + fetch only). Dispatches the three engine message types and
// auto-reconnects.

const host = location.hostname || 'localhost'
export const HTTP_BASE = `http://${host}:8080`
const WS_URL = `ws://${host}:8080/ws`

// connect wires the handlers and returns a disconnect function.
export function connect(handlers) {
  let ws
  let closed = false

  const open = () => {
    ws = new WebSocket(WS_URL)
    ws.onopen = () => handlers.onStatus?.('connected')
    ws.onmessage = (e) => {
      let msg
      try {
        msg = JSON.parse(e.data)
      } catch {
        return
      }
      if (msg.type === 'alert') handlers.onAlert?.(msg.alert)
      else if (msg.type === 'metrics') handlers.onMetrics?.(msg)
      else if (msg.type === 'positions') handlers.onPositions?.(msg.fc)
    }
    ws.onclose = () => {
      handlers.onStatus?.('disconnected')
      if (!closed) setTimeout(open, 1000)
    }
    ws.onerror = () => ws.close()
  }

  open()
  return () => {
    closed = true
    ws?.close()
  }
}
