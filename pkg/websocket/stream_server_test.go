package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/liliang-cn/gosinble/pkg/types"
)

func TestNewStreamServer(t *testing.T) {
	server := NewStreamServer()
	
	if server == nil {
		t.Fatal("NewStreamServer should not return nil")
	}
	
	if server.clients == nil {
		t.Error("clients map should be initialized")
	}
	
	if server.broadcast == nil {
		t.Error("broadcast channel should be initialized")
	}
	
	if server.register == nil {
		t.Error("register channel should be initialized")
	}
	
	if server.unregister == nil {
		t.Error("unregister channel should be initialized")
	}
	
	if server.running {
		t.Error("server should not be running initially")
	}
}

func TestStreamServer_StartStop(t *testing.T) {
	server := NewStreamServer()
	
	// Test start
	server.Start()
	
	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)
	
	server.runningMux.RLock()
	running := server.running
	server.runningMux.RUnlock()
	
	if !running {
		t.Error("server should be running after Start()")
	}
	
	// Test stop
	server.Stop()
	
	server.runningMux.RLock()
	running = server.running
	server.runningMux.RUnlock()
	
	if running {
		t.Error("server should not be running after Stop()")
	}
}

func TestStreamServer_StartIdempotent(t *testing.T) {
	server := NewStreamServer()
	
	// Start multiple times
	server.Start()
	server.Start()
	server.Start()
	
	time.Sleep(10 * time.Millisecond)
	
	server.runningMux.RLock()
	running := server.running
	server.runningMux.RUnlock()
	
	if !running {
		t.Error("server should be running after multiple Start() calls")
	}
	
	server.Stop()
}

func TestStreamServer_BroadcastStreamEvent(t *testing.T) {
	server := NewStreamServer()
	// Don't start the server, just test the broadcast method directly
	
	// Create test event
	event := types.StreamEvent{
		Type: types.StreamProgress,
		Progress: &types.ProgressInfo{
			Stage:      "testing",
			Percentage: 50.0,
			Message:    "Test progress",
			Timestamp:  time.Now(),
		},
	}
	
	// Test that broadcast doesn't block
	server.BroadcastStreamEvent(event, "test_source")
	
	// Verify message was sent to broadcast channel
	select {
	case message := <-server.broadcast:
		if message.Type != MessageTypeStreamEvent {
			t.Errorf("Expected message type %s, got %s", MessageTypeStreamEvent, message.Type)
		}
		
		if message.Source != "test_source" {
			t.Errorf("Expected source 'test_source', got '%s'", message.Source)
		}
		
		if message.StreamEvent == nil {
			t.Error("StreamEvent should not be nil")
		}
		
		if message.Progress == nil {
			t.Error("Progress should not be nil for StreamProgress event")
		}
		
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected message in broadcast channel")
	}
}

func TestStreamServer_BroadcastProgress(t *testing.T) {
	server := NewStreamServer()
	// Don't start the server, just test the broadcast method directly
	
	progress := types.ProgressInfo{
		Stage:      "uploading",
		Percentage: 75.0,
		Message:    "Upload in progress",
		Timestamp:  time.Now(),
	}
	
	server.BroadcastProgress(progress, "upload_source")
	
	select {
	case message := <-server.broadcast:
		if message.Type != MessageTypeProgress {
			t.Errorf("Expected message type %s, got %s", MessageTypeProgress, message.Type)
		}
		
		if message.Source != "upload_source" {
			t.Errorf("Expected source 'upload_source', got '%s'", message.Source)
		}
		
		if message.Progress == nil {
			t.Error("Progress should not be nil")
		}
		
		if message.Progress.Percentage != 75.0 {
			t.Errorf("Expected progress 75.0, got %f", message.Progress.Percentage)
		}
		
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected message in broadcast channel")
	}
}

func TestStreamServer_GetConnectedClients(t *testing.T) {
	server := NewStreamServer()
	
	// Initially should be empty
	clients := server.GetConnectedClients()
	if len(clients) != 0 {
		t.Errorf("Expected 0 clients, got %d", len(clients))
	}
	
	// Add a mock client
	mockClient := &Client{
		id: "test_client_1",
		sessionInfo: ClientSession{
			SessionID:   "test_client_1",
			UserID:      "test_user",
			ConnectedAt: time.Now(),
			RemoteAddr:  "127.0.0.1:12345",
		},
	}
	
	server.clientsMux.Lock()
	server.clients[mockClient] = true
	server.clientsMux.Unlock()
	
	clients = server.GetConnectedClients()
	if len(clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(clients))
	}
	
	if clients[0].SessionID != "test_client_1" {
		t.Errorf("Expected session ID 'test_client_1', got '%s'", clients[0].SessionID)
	}
}

func TestStreamMessage_Serialization(t *testing.T) {
	message := StreamMessage{
		Type:      MessageTypeStreamEvent,
		Timestamp: time.Now(),
		Source:    "test_module",
		SessionID: "session_123",
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
		StreamEvent: &types.StreamEvent{
			Type: types.StreamProgress,
			Data: "test data",
		},
		Progress: &types.ProgressInfo{
			Stage:      "processing",
			Percentage: 33.3,
			Message:    "Processing items",
		},
	}
	
	// Test JSON serialization
	jsonData, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}
	
	// Test JSON deserialization
	var unmarshaled StreamMessage
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}
	
	// Verify fields
	if unmarshaled.Type != message.Type {
		t.Errorf("Type mismatch after serialization")
	}
	
	if unmarshaled.Source != message.Source {
		t.Errorf("Source mismatch after serialization")
	}
	
	if unmarshaled.SessionID != message.SessionID {
		t.Errorf("SessionID mismatch after serialization")
	}
}

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()
	
	if id1 == id2 {
		t.Error("generateSessionID should produce unique IDs")
	}
	
	if !strings.HasPrefix(id1, "ws_") {
		t.Errorf("Session ID should start with 'ws_', got '%s'", id1)
	}
	
	if !strings.HasPrefix(id2, "ws_") {
		t.Errorf("Session ID should start with 'ws_', got '%s'", id2)
	}
}

func TestGetSubscriptionKeys(t *testing.T) {
	subscriptions := map[string]bool{
		"stream_event": true,
		"progress":     true,
		"error":        true,
	}
	
	keys := getSubscriptionKeys(subscriptions)
	
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}
	
	// Convert to map for easy checking
	keyMap := make(map[string]bool)
	for _, key := range keys {
		keyMap[key] = true
	}
	
	expectedKeys := []string{"stream_event", "progress", "error"}
	for _, expectedKey := range expectedKeys {
		if !keyMap[expectedKey] {
			t.Errorf("Missing key '%s' in result", expectedKey)
		}
	}
}

func TestStreamingWebSocketAdapter(t *testing.T) {
	server := NewStreamServer()
	adapter := NewStreamingWebSocketAdapter(server, "test_adapter")
	
	if adapter == nil {
		t.Fatal("NewStreamingWebSocketAdapter should not return nil")
	}
	
	if adapter.server != server {
		t.Error("Adapter should reference the correct server")
	}
	
	if adapter.source != "test_adapter" {
		t.Errorf("Expected source 'test_adapter', got '%s'", adapter.source)
	}
}

func TestStreamingWebSocketAdapter_CreateProgressCallback(t *testing.T) {
	server := NewStreamServer()
	server.Start()
	defer server.Stop()
	
	adapter := NewStreamingWebSocketAdapter(server, "callback_test")
	callback := adapter.CreateProgressCallback()
	
	progress := types.ProgressInfo{
		Stage:      "testing",
		Percentage: 25.0,
		Message:    "Testing callback",
		Timestamp:  time.Now(),
	}
	
	// Call the callback
	callback(progress)
	
	// Verify message was broadcast
	select {
	case message := <-server.broadcast:
		if message.Type != MessageTypeProgress {
			t.Errorf("Expected message type %s, got %s", MessageTypeProgress, message.Type)
		}
		
		if message.Source != "callback_test" {
			t.Errorf("Expected source 'callback_test', got '%s'", message.Source)
		}
		
		if message.Progress.Percentage != 25.0 {
			t.Errorf("Expected progress 25.0, got %f", message.Progress.Percentage)
		}
		
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected message in broadcast channel")
	}
}

func TestStreamingWebSocketAdapter_HandleStreamEvents(t *testing.T) {
	server := NewStreamServer()
	server.Start()
	defer server.Stop()
	
	adapter := NewStreamingWebSocketAdapter(server, "event_handler")
	
	// Create events channel
	events := make(chan types.StreamEvent, 2)
	
	// Create test events
	event1 := types.StreamEvent{
		Type: types.StreamProgress,
		Data: "test event 1",
	}
	
	event2 := types.StreamEvent{
		Type: types.StreamStdout,
		Data: "test output",
	}
	
	// Send events
	events <- event1
	events <- event2
	close(events)
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	// Handle events in goroutine
	go adapter.HandleStreamEvents(ctx, events)
	
	// Verify events (we'll get them in order)
	for i := 0; i < 2; i++ {
		select {
		case message := <-server.broadcast:
			if message.Type != MessageTypeStreamEvent {
				t.Errorf("Expected message type %s, got %s", MessageTypeStreamEvent, message.Type)
			}
			
			// First event is StreamProgress, second is StreamStdout
			expectedType := types.StreamProgress
			if i == 1 {
				expectedType = types.StreamStdout
			}
			
			if message.StreamEvent.Type != expectedType {
				t.Errorf("Expected stream event type %s, got %s", expectedType, message.StreamEvent.Type)
			}
			
		case <-time.After(500 * time.Millisecond):
			t.Errorf("Expected event %d in broadcast channel", i+1)
		}
	}
}

// Integration test with actual WebSocket connection
func TestStreamServer_WebSocketIntegration(t *testing.T) {
	server := NewStreamServer()
	server.Start()
	defer server.Stop()
	
	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleWebSocket(w, r)
	}))
	defer httpServer.Close()
	
	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")
	
	// Create WebSocket client
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()
	
	// Read connection message
	var connMessage StreamMessage
	err = conn.ReadJSON(&connMessage)
	if err != nil {
		t.Fatalf("Failed to read connection message: %v", err)
	}
	
	if connMessage.Type != MessageTypeConnection {
		t.Errorf("Expected connection message, got %s", connMessage.Type)
	}
	
	// Send subscription message
	subscribeMsg := StreamMessage{
		Type: MessageTypeSubscribe,
		Data: map[string]interface{}{
			"event_types": []interface{}{"stream_event", "progress"},
		},
	}
	
	err = conn.WriteJSON(subscribeMsg)
	if err != nil {
		t.Fatalf("Failed to send subscription message: %v", err)
	}
	
	// Read subscription confirmation
	var subResponse StreamMessage
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	err = conn.ReadJSON(&subResponse)
	if err != nil {
		t.Fatalf("Failed to read subscription response: %v", err)
	}
	
	if subResponse.Type != MessageTypeSubscribe {
		t.Errorf("Expected subscription response, got %s", subResponse.Type)
	}
	
	// Broadcast a test message
	testProgress := types.ProgressInfo{
		Stage:      "integration_test",
		Percentage: 50.0,
		Message:    "Integration test progress",
		Timestamp:  time.Now(),
	}
	
	server.BroadcastProgress(testProgress, "integration_test")
	
	// Read broadcasted message
	var broadcastMessage StreamMessage
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	err = conn.ReadJSON(&broadcastMessage)
	if err != nil {
		t.Fatalf("Failed to read broadcast message: %v", err)
	}
	
	if broadcastMessage.Type != MessageTypeProgress {
		t.Errorf("Expected progress message, got %s", broadcastMessage.Type)
	}
	
	if broadcastMessage.Progress.Percentage != 50.0 {
		t.Errorf("Expected progress 50.0, got %f", broadcastMessage.Progress.Percentage)
	}
}

// Benchmark tests
func BenchmarkStreamServer_BroadcastStreamEvent(b *testing.B) {
	server := NewStreamServer()
	server.Start()
	defer server.Stop()
	
	event := types.StreamEvent{
		Type: types.StreamProgress,
		Data: "benchmark test",
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		server.BroadcastStreamEvent(event, "benchmark")
		
		// Drain the channel to avoid blocking
		select {
		case <-server.broadcast:
		default:
		}
	}
}

func BenchmarkGenerateSessionID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateSessionID()
	}
}

func BenchmarkStreamMessage_JSONMarshal(b *testing.B) {
	message := StreamMessage{
		Type:      MessageTypeStreamEvent,
		Timestamp: time.Now(),
		Source:    "benchmark_test",
		Data: map[string]interface{}{
			"test_key": "test_value",
			"number":   123,
		},
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(message)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}