package runner

import (
	"context"
	"testing"
	
	"github.com/gosinble/gosinble/pkg/types"
)

func TestHandlerManager(t *testing.T) {
	hm := NewHandlerManager()
	
	// Test registering a handler
	handler1 := types.Task{
		Name:   "restart_service",
		Module: types.TypeService,
		Args:   map[string]interface{}{"name": "nginx", "state": "restarted"},
	}
	
	err := hm.RegisterHandler(handler1)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}
	
	// Test registering handler with listen attribute
	handler2 := types.Task{
		Name:   "reload_config",
		Module: types.TypeCommand,
		Args:   map[string]interface{}{"cmd": "reload config"},
		Listen: "config_changed",
	}
	
	err = hm.RegisterHandler(handler2)
	if err != nil {
		t.Fatalf("failed to register handler with listen: %v", err)
	}
	
	// Test registering handler without name (should fail)
	handler3 := types.Task{
		Module: types.TypeDebug,
		Args:   map[string]interface{}{"msg": "test"},
	}
	
	err = hm.RegisterHandler(handler3)
	if err == nil {
		t.Error("expected error when registering handler without name")
	}
	
	// Test HasHandlers
	if !hm.HasHandlers() {
		t.Error("expected HasHandlers to return true")
	}
	
	// Test GetHandler
	h, exists := hm.GetHandler("restart_service")
	if !exists {
		t.Error("expected to find handler by name")
	}
	if h.Name != "restart_service" {
		t.Errorf("expected handler name 'restart_service', got '%s'", h.Name)
	}
	
	// Test GetHandler by listen attribute
	h, exists = hm.GetHandler("config_changed")
	if !exists {
		t.Error("expected to find handler by listen attribute")
	}
	if h.Name != "reload_config" {
		t.Errorf("expected handler name 'reload_config', got '%s'", h.Name)
	}
	
	// Test Notify
	hm.Notify([]string{"restart_service"})
	
	// Test GetPendingHandlers
	pending := hm.GetPendingHandlers()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending handler, got %d", len(pending))
	}
	if pending[0].Name != "restart_service" {
		t.Errorf("expected pending handler 'restart_service', got '%s'", pending[0].Name)
	}
	
	// Test that notifications are cleared after getting pending handlers
	pending = hm.GetPendingHandlers()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending handlers after clearing, got %d", len(pending))
	}
	
	// Test multiple notifications
	hm.Notify([]string{"restart_service", "config_changed"})
	pending = hm.GetPendingHandlers()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending handlers, got %d", len(pending))
	}
	
	// Test duplicate notifications (should only run once)
	hm.Notify([]string{"restart_service"})
	hm.Notify([]string{"restart_service"})
	pending = hm.GetPendingHandlers()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending handler (no duplicates), got %d", len(pending))
	}
	
	// Test Clear
	hm.Clear()
	if hm.HasHandlers() {
		t.Error("expected HasHandlers to return false after Clear")
	}
}

func TestHandlerManagerProcessHandlers(t *testing.T) {
	hm := NewHandlerManager()
	runner := NewTaskRunner()
	
	// Register a debug handler
	handler := types.Task{
		Name:   "test_handler",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "Handler executed"},
	}
	
	err := hm.RegisterHandler(handler)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}
	
	// Notify the handler
	hm.Notify([]string{"test_handler"})
	
	// Create test hosts
	hosts := []types.Host{
		{Name: "localhost", Address: "localhost"},
	}
	
	// Process handlers
	ctx := context.Background()
	results, err := hm.ProcessHandlers(ctx, runner, hosts, nil)
	if err != nil {
		t.Fatalf("failed to process handlers: %v", err)
	}
	
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	
	if results[0].ModuleName != "debug" {
		t.Errorf("expected module 'debug', got '%s'", results[0].ModuleName)
	}
	
	// Test processing with no pending handlers
	results, err = hm.ProcessHandlers(ctx, runner, hosts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(results) != 0 {
		t.Errorf("expected 0 results when no handlers pending, got %d", len(results))
	}
}