package status

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/ryanjdillon/symphony/internal/orchestrator"
)

// Hub manages WebSocket clients and broadcasts state updates.
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
	logger  *slog.Logger
}

type wsClient struct {
	conn   *websocket.Conn
	send   chan []byte
	cancel context.CancelFunc
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*wsClient]struct{}),
		logger:  slog.Default(),
	}
}

// Run processes client lifecycle. Call in a goroutine.
func (h *Hub) Run() {
	select {}
}

// Broadcast sends a state snapshot to all connected clients.
func (h *Hub) Broadcast(snap *orchestrator.StateSnapshot) {
	msg := wsMessage{
		Type: "state_update",
		Data: statePayload(snap),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Warn("failed to marshal ws message", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

type wsMessage struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

// HandleWebSocket upgrades an HTTP connection to a WebSocket.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		h.logger.Warn("ws upgrade failed", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	client := &wsClient{
		conn:   conn,
		send:   make(chan []byte, 64),
		cancel: cancel,
	}

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	h.logger.Info("ws client connected", "clients", h.clientCount())

	go h.writePump(ctx, client)
	h.readPump(ctx, client)
}

func (h *Hub) readPump(ctx context.Context, client *wsClient) {
	defer func() {
		client.cancel()
		h.removeClient(client)
		_ = client.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		var msg wsMessage
		if err := wsjson.Read(ctx, client.conn, &msg); err != nil {
			return
		}

		if msg.Type == "refresh" {
			h.logger.Debug("ws refresh requested")
		}
	}
}

func (h *Hub) writePump(ctx context.Context, client *wsClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-client.send:
			if !ok {
				return
			}
			if err := client.conn.Write(ctx, websocket.MessageText, data); err != nil {
				return
			}
		case <-ticker.C:
			if err := client.conn.Ping(ctx); err != nil {
				return
			}
		}
	}
}

func (h *Hub) removeClient(client *wsClient) {
	h.mu.Lock()
	delete(h.clients, client)
	close(client.send)
	h.mu.Unlock()
	h.logger.Info("ws client disconnected", "clients", h.clientCount())
}

func (h *Hub) clientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
