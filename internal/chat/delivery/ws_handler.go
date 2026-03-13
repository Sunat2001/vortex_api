package delivery

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/voronka/backend/internal/chat"
	"github.com/voronka/backend/internal/shared/middleware"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, restrict origins
	},
}

// WSEvent represents a WebSocket event sent to clients
type WSEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Client represents a connected WebSocket client
type Client struct {
	UserID   uuid.UUID
	UserName string
	Conn     *websocket.Conn
	Send     chan []byte
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	mu         sync.RWMutex
	clients    map[uuid.UUID]map[*Client]bool // userID -> set of clients
	register   chan *Client
	unregister chan *Client
	logger     *zap.Logger
}

// NewHub creates a new WebSocket hub
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
	}
}

// Run starts the hub's event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.UserID] == nil {
				h.clients[client.UserID] = make(map[*Client]bool)
			}
			h.clients[client.UserID][client] = true
			h.mu.Unlock()

			h.logger.Info("websocket client connected",
				zap.String("user_id", client.UserID.String()),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.UserID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.Send)
					if len(clients) == 0 {
						delete(h.clients, client.UserID)
					}
				}
			}
			h.mu.Unlock()

			h.logger.Info("websocket client disconnected",
				zap.String("user_id", client.UserID.String()),
			)
		}
	}
}

// SendToUser sends a WebSocket event to all connections of a specific user
func (h *Hub) SendToUser(userID uuid.UUID, event *WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("failed to marshal ws event", zap.Error(err))
		return
	}

	h.mu.RLock()
	clients, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	for client := range clients {
		select {
		case client.Send <- data:
		default:
			go func(c *Client) { h.unregister <- c }(client)
		}
	}
}

// Broadcast sends a WebSocket event to all connected clients
func (h *Hub) Broadcast(event *WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("failed to marshal ws event", zap.Error(err))
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, clients := range h.clients {
		for client := range clients {
			select {
			case client.Send <- data:
			default:
				go func(c *Client) { h.unregister <- c }(client)
			}
		}
	}
}

// WSHandler handles WebSocket upgrade requests
type WSHandler struct {
	hub    *Hub
	logger *zap.Logger
}

// NewWSHandler creates a new WebSocket handler
func NewWSHandler(hub *Hub, logger *zap.Logger) *WSHandler {
	return &WSHandler{
		hub:    hub,
		logger: logger,
	}
}

// RegisterRoutes registers WebSocket routes
func (h *WSHandler) RegisterRoutes(router *gin.RouterGroup, authMw gin.HandlerFunc) {
	router.GET("/ws", authMw, h.HandleWebSocket)
}

// HandleWebSocket handles GET /api/v1/ws
func (h *WSHandler) HandleWebSocket(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("failed to upgrade websocket", zap.Error(err))
		return
	}

	client := &Client{
		UserID:   userID,
		UserName: c.Query("name"), // Optional: client can pass name as query param
		Conn:     conn,
		Send:     make(chan []byte, 256),
	}

	h.hub.register <- client

	go h.writePump(client)
	go h.readPump(client)
}

// readPump pumps messages from the WebSocket connection to the hub
func (h *WSHandler) readPump(client *Client) {
	defer func() {
		h.hub.unregister <- client
		client.Conn.Close()
	}()

	client.Conn.SetReadLimit(4096)
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Error("websocket read error",
					zap.String("user_id", client.UserID.String()),
					zap.Error(err),
				)
			}
			break
		}

		// Handle incoming client messages (typing indicators, message_read)
		var event WSEvent
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}

		switch event.Type {
		case "typing":
			h.hub.Broadcast(&WSEvent{
				Type: "typing",
				Data: map[string]interface{}{
					"user_id":     client.UserID.String(),
					"sender_name": client.UserName,
					"sender_type": "agent",
					"dialog_id":   event.Data,
				},
			})
		case "message_read":
			// Forward message_read event: data should contain dialog_id and message_ids
			if data, ok := event.Data.(map[string]interface{}); ok {
				data["user_id"] = client.UserID.String()
				h.hub.Broadcast(&WSEvent{
					Type: "message_read",
					Data: data,
				})
			}
		}
	}
}

// HubNotifier adapts the Hub to implement the chat.Notifier interface.
// Lives in the delivery layer; the usecase depends only on the interface.
type HubNotifier struct {
	hub    *Hub
	logger *zap.Logger
}

// NewHubNotifier creates a new HubNotifier.
func NewHubNotifier(hub *Hub, logger *zap.Logger) *HubNotifier {
	return &HubNotifier{hub: hub, logger: logger}
}

func (n *HubNotifier) NotifyNewMessage(ctx context.Context, event *chat.NewMessageEvent) {
	wsEvent := &WSEvent{
		Type: "new_message",
		Data: event.Message.ToDTO(""),
	}
	n.sendOrBroadcast(event.AgentID, wsEvent)
}

func (n *HubNotifier) NotifyDialogUpdated(ctx context.Context, event *chat.DialogUpdatedEvent) {
	wsEvent := &WSEvent{
		Type: "dialog_updated",
		Data: map[string]interface{}{
			"dialog_id":  event.DialogID.String(),
			"event_type": string(event.EventType),
		},
	}
	n.sendOrBroadcast(event.AgentID, wsEvent)
}

func (n *HubNotifier) NotifyMessageStatusChanged(ctx context.Context, event *chat.MessageStatusEvent) {
	wsEvent := &WSEvent{
		Type: "message_status",
		Data: map[string]interface{}{
			"message_id": event.MessageID.String(),
			"dialog_id":  event.DialogID.String(),
			"status":     string(event.Status),
		},
	}
	n.sendOrBroadcast(event.AgentID, wsEvent)
}

// sendOrBroadcast sends to a specific agent or broadcasts to all if agentID is nil.
func (n *HubNotifier) sendOrBroadcast(agentID *uuid.UUID, event *WSEvent) {
	if agentID != nil {
		n.hub.SendToUser(*agentID, event)
		return
	}
	n.hub.Broadcast(event)
}

// writePump pumps messages from the hub to the WebSocket connection
func (h *WSHandler) writePump(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
