package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gosinble/gosinble/pkg/types"
)

// StreamServer handles WebSocket connections for streaming gosinble events
type StreamServer struct {
	upgrader    websocket.Upgrader
	clients     map[*Client]bool
	clientsMux  sync.RWMutex
	broadcast   chan StreamMessage
	register    chan *Client
	unregister  chan *Client
	running     bool
	runningMux  sync.RWMutex
}

// Client represents a WebSocket client connection
type Client struct {
	conn        *websocket.Conn
	send        chan StreamMessage
	server      *StreamServer
	id          string
	sessionInfo ClientSession
	lastPing    time.Time
	subscriptions map[string]bool // Event type subscriptions
}

// ClientSession contains client session information
type ClientSession struct {
	UserID      string            `json:"user_id,omitempty"`
	SessionID   string            `json:"session_id"`
	ConnectedAt time.Time         `json:"connected_at"`
	UserAgent   string            `json:"user_agent,omitempty"`
	RemoteAddr  string            `json:"remote_addr"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// StreamMessage represents a message sent over WebSocket
type StreamMessage struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source,omitempty"`    // Module or connection source
	SessionID string                 `json:"session_id,omitempty"` // Client session ID
	Data      map[string]interface{} `json:"data,omitempty"`
	
	// Gosinble-specific event data
	StreamEvent *types.StreamEvent `json:"stream_event,omitempty"`
	StepInfo    *types.StepInfo    `json:"step_info,omitempty"`
	Progress    *types.ProgressInfo `json:"progress,omitempty"`
}

// MessageType constants
const (
	MessageTypeStreamEvent = "stream_event"
	MessageTypeProgress    = "progress"
	MessageTypeStep        = "step"
	MessageTypeError       = "error"
	MessageTypeConnection  = "connection"
	MessageTypeSubscribe   = "subscribe"
	MessageTypeUnsubscribe = "unsubscribe"
	MessageTypePing        = "ping"
	MessageTypePong        = "pong"
)

// NewStreamServer creates a new WebSocket stream server
func NewStreamServer() *StreamServer {
	return &StreamServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clients:    make(map[*Client]bool),
		broadcast:  make(chan StreamMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Start begins the WebSocket server message processing
func (s *StreamServer) Start() {
	s.runningMux.Lock()
	if s.running {
		s.runningMux.Unlock()
		return
	}
	s.running = true
	s.runningMux.Unlock()

	go s.run()
	log.Println("WebSocket stream server started")
}

// Stop shuts down the WebSocket server
func (s *StreamServer) Stop() {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if !s.running {
		return
	}
	
	s.running = false
	
	// Close all client connections
	s.clientsMux.Lock()
	for client := range s.clients {
		select {
		case <-client.send:
			// Channel already closed
		default:
			close(client.send)
		}
		client.conn.Close()
	}
	s.clientsMux.Unlock()

	log.Println("WebSocket stream server stopped")
}

// HandleWebSocket handles WebSocket upgrade and client management
func (s *StreamServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	sessionID := generateSessionID()
	client := &Client{
		conn:          conn,
		send:          make(chan StreamMessage, 256),
		server:        s,
		id:            sessionID,
		lastPing:      time.Now(),
		subscriptions: make(map[string]bool),
		sessionInfo: ClientSession{
			SessionID:   sessionID,
			ConnectedAt: time.Now(),
			UserAgent:   r.UserAgent(),
			RemoteAddr:  r.RemoteAddr,
			Metadata:    make(map[string]string),
		},
	}

	// Extract user info from query params or headers
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		client.sessionInfo.UserID = userID
	}

	s.register <- client

	// Start client goroutines
	go client.writePump()
	go client.readPump()

	log.Printf("Client connected: %s from %s", sessionID, r.RemoteAddr)
}

// BroadcastStreamEvent broadcasts a gosinble stream event to all clients
func (s *StreamServer) BroadcastStreamEvent(event types.StreamEvent, source string) {
	message := StreamMessage{
		Type:        MessageTypeStreamEvent,
		Timestamp:   time.Now(),
		Source:      source,
		StreamEvent: &event,
	}

	// Add specific data based on event type
	switch event.Type {
	case types.StreamProgress:
		if event.Progress != nil {
			message.Progress = event.Progress
		}
	case types.StreamStepStart, types.StreamStepUpdate, types.StreamStepEnd:
		if event.Step != nil {
			message.StepInfo = event.Step
		}
	}

	s.broadcast <- message
}

// BroadcastProgress broadcasts progress information
func (s *StreamServer) BroadcastProgress(progress types.ProgressInfo, source string) {
	message := StreamMessage{
		Type:      MessageTypeProgress,
		Timestamp: time.Now(),
		Source:    source,
		Progress:  &progress,
	}

	s.broadcast <- message
}

// BroadcastToClient sends a message to a specific client
func (s *StreamServer) BroadcastToClient(clientID string, message StreamMessage) {
	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()

	for client := range s.clients {
		if client.id == clientID {
			select {
			case client.send <- message:
			default:
				// Client buffer full, skip
			}
			break
		}
	}
}

// GetConnectedClients returns information about connected clients
func (s *StreamServer) GetConnectedClients() []ClientSession {
	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()

	sessions := make([]ClientSession, 0, len(s.clients))
	for client := range s.clients {
		sessions = append(sessions, client.sessionInfo)
	}

	return sessions
}

// run handles the main server loop
func (s *StreamServer) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-s.register:
			s.clientsMux.Lock()
			s.clients[client] = true
			s.clientsMux.Unlock()

			// Send connection confirmation
			welcomeMsg := StreamMessage{
				Type:      MessageTypeConnection,
				Timestamp: time.Now(),
				SessionID: client.id,
				Data: map[string]interface{}{
					"status":     "connected",
					"session_id": client.id,
					"server_time": time.Now().Unix(),
				},
			}
			client.send <- welcomeMsg

		case client := <-s.unregister:
			s.clientsMux.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				select {
				case <-client.send:
					// Channel already closed
				default:
					close(client.send)
				}
			}
			s.clientsMux.Unlock()
			log.Printf("Client disconnected: %s", client.id)

		case message := <-s.broadcast:
			s.clientsMux.RLock()
			for client := range s.clients {
				// Check if client is subscribed to this message type
				if len(client.subscriptions) > 0 && !client.subscriptions[message.Type] {
					continue
				}

				select {
				case client.send <- message:
				default:
					// Client buffer full, disconnect client
					delete(s.clients, client)
					select {
					case <-client.send:
						// Channel already closed
					default:
						close(client.send)
					}
				}
			}
			s.clientsMux.RUnlock()

		case <-ticker.C:
			// Periodic cleanup and ping
			s.cleanupClients()
		}

		s.runningMux.RLock()
		if !s.running {
			s.runningMux.RUnlock()
			break
		}
		s.runningMux.RUnlock()
	}
}

// cleanupClients removes inactive clients
func (s *StreamServer) cleanupClients() {
	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()

	now := time.Now()
	for client := range s.clients {
		if now.Sub(client.lastPing) > 60*time.Second {
			delete(s.clients, client)
			select {
			case <-client.send:
				// Channel already closed
			default:
				close(client.send)
			}
			client.conn.Close()
			log.Printf("Client timeout: %s", client.id)
		}
	}
}

// Client methods

// readPump handles reading from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.lastPing = time.Now()
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.lastPing = time.Now()
		c.handleMessage(messageBytes)
	}
}

// writePump handles writing to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages from the client
func (c *Client) handleMessage(messageBytes []byte) {
	var message StreamMessage
	if err := json.Unmarshal(messageBytes, &message); err != nil {
		log.Printf("Invalid message from client %s: %v", c.id, err)
		return
	}

	switch message.Type {
	case MessageTypeSubscribe:
		c.handleSubscribe(message)
	case MessageTypeUnsubscribe:
		c.handleUnsubscribe(message)
	case MessageTypePong:
		c.lastPing = time.Now()
	}
}

// handleSubscribe handles subscription requests
func (c *Client) handleSubscribe(message StreamMessage) {
	if eventTypes, ok := message.Data["event_types"].([]interface{}); ok {
		for _, eventType := range eventTypes {
			if eventTypeStr, ok := eventType.(string); ok {
				c.subscriptions[eventTypeStr] = true
			}
		}
	}

	// Send confirmation
	response := StreamMessage{
		Type:      MessageTypeSubscribe,
		Timestamp: time.Now(),
		SessionID: c.id,
		Data: map[string]interface{}{
			"status":      "subscribed",
			"event_types": getSubscriptionKeys(c.subscriptions),
		},
	}
	c.send <- response
}

// handleUnsubscribe handles unsubscription requests
func (c *Client) handleUnsubscribe(message StreamMessage) {
	if eventTypes, ok := message.Data["event_types"].([]interface{}); ok {
		for _, eventType := range eventTypes {
			if eventTypeStr, ok := eventType.(string); ok {
				delete(c.subscriptions, eventTypeStr)
			}
		}
	}

	// Send confirmation
	response := StreamMessage{
		Type:      MessageTypeUnsubscribe,
		Timestamp: time.Now(),
		SessionID: c.id,
		Data: map[string]interface{}{
			"status":      "unsubscribed",
			"event_types": getSubscriptionKeys(c.subscriptions),
		},
	}
	c.send <- response
}

// Helper functions

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("ws_%d_%d", time.Now().UnixNano(), time.Now().Unix())
}

// getSubscriptionKeys returns the keys of a subscription map
func getSubscriptionKeys(subscriptions map[string]bool) []string {
	keys := make([]string, 0, len(subscriptions))
	for key := range subscriptions {
		keys = append(keys, key)
	}
	return keys
}

// StreamingWebSocketAdapter adapts gosinble streaming to WebSocket
type StreamingWebSocketAdapter struct {
	server *StreamServer
	source string
}

// NewStreamingWebSocketAdapter creates a new adapter
func NewStreamingWebSocketAdapter(server *StreamServer, source string) *StreamingWebSocketAdapter {
	return &StreamingWebSocketAdapter{
		server: server,
		source: source,
	}
}

// HandleStreamEvents processes a channel of stream events and broadcasts them
func (a *StreamingWebSocketAdapter) HandleStreamEvents(ctx context.Context, events <-chan types.StreamEvent) {
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			a.server.BroadcastStreamEvent(event, a.source)

		case <-ctx.Done():
			return
		}
	}
}

// CreateProgressCallback creates a progress callback that broadcasts to WebSocket
func (a *StreamingWebSocketAdapter) CreateProgressCallback() func(progress types.ProgressInfo) {
	return func(progress types.ProgressInfo) {
		a.server.BroadcastProgress(progress, a.source)
	}
}