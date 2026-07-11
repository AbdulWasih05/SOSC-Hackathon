// Package api hosts the dashboard websocket. This is the only outbound channel
// the engine has (no SMS, email, webhooks, or authority routing; see CLAUDE.md
// out-of-scope list). One hub fans a JSON frame out to every connected client.
package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// Hub tracks connected dashboard clients and broadcasts JSON frames to all of
// them. gorilla/websocket forbids concurrent writes to one connection, so every
// write is serialized under mu; at dashboard broadcast rates (metrics 1/s,
// positions <=2/s, alerts on demand) this is not a bottleneck.
type Hub struct {
	mu       sync.Mutex
	clients  map[*websocket.Conn]struct{}
	upgrader websocket.Upgrader
}

// NewHub returns an empty hub ready to accept clients.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			// Localhost demo tool: any origin may connect.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

// HandleWS upgrades an HTTP request to a websocket and registers the client.
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("ws upgrade failed")
		return
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	n := len(h.clients)
	h.mu.Unlock()
	log.Info().Str("remote", r.RemoteAddr).Int("clients", n).Msg("ws client connected")

	// Drain reads so we notice disconnects; the dashboard sends nothing.
	go func() {
		defer h.remove(conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (h *Hub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	if _, ok := h.clients[conn]; ok {
		delete(h.clients, conn)
		conn.Close()
		log.Info().Int("clients", len(h.clients)).Msg("ws client disconnected")
	}
	h.mu.Unlock()
}

// Broadcast marshals v to JSON and writes it to every connected client. Clients
// whose write fails are dropped.
func (h *Hub) Broadcast(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error().Err(err).Msg("broadcast marshal failed")
		return
	}
	h.mu.Lock()
	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			delete(h.clients, conn)
			conn.Close()
		}
	}
	h.mu.Unlock()
}
