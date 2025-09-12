package runner

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/liliang-cn/gosible/pkg/types"
)

// HandlerManager manages task handlers for notifications
type HandlerManager struct {
	handlers       map[string]types.Task
	notifications  []string
	mu             sync.RWMutex
}

// NewHandlerManager creates a new handler manager
func NewHandlerManager() *HandlerManager {
	return &HandlerManager{
		handlers:      make(map[string]types.Task),
		notifications: make([]string, 0),
	}
}

// RegisterHandler registers a handler task
func (h *HandlerManager) RegisterHandler(handler types.Task) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if handler.Name == "" {
		return fmt.Errorf("handler must have a name")
	}
	
	// Register by name
	h.handlers[handler.Name] = handler
	
	// Also register by listen attribute if present
	if handler.Listen != "" {
		h.handlers[handler.Listen] = handler
	}
	
	return nil
}

// Notify adds a notification for a handler
func (h *HandlerManager) Notify(handlerNames []string) {
	if len(handlerNames) == 0 {
		return
	}
	
	h.mu.Lock()
	defer h.mu.Unlock()
	
	for _, name := range handlerNames {
		// Check if handler exists
		if _, exists := h.handlers[name]; exists {
			// Avoid duplicate notifications
			found := false
			for _, n := range h.notifications {
				if n == name {
					found = true
					break
				}
			}
			if !found {
				h.notifications = append(h.notifications, name)
			}
		}
	}
}

// GetPendingHandlers returns and clears all pending handler notifications
func (h *HandlerManager) GetPendingHandlers() []types.Task {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	var handlers []types.Task
	processedNames := make(map[string]bool)
	
	for _, name := range h.notifications {
		if !processedNames[name] {
			if handler, exists := h.handlers[name]; exists {
				handlers = append(handlers, handler)
				processedNames[name] = true
			}
		}
	}
	
	// Clear notifications after processing
	h.notifications = make([]string, 0)
	
	return handlers
}

// HasHandlers returns true if there are registered handlers
func (h *HandlerManager) HasHandlers() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.handlers) > 0
}

// GetHandler returns a handler by name
func (h *HandlerManager) GetHandler(name string) (types.Task, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	handler, exists := h.handlers[name]
	return handler, exists
}

// Clear clears all handlers and notifications
func (h *HandlerManager) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers = make(map[string]types.Task)
	h.notifications = make([]string, 0)
}

// ProcessHandlers executes all pending handlers
func (h *HandlerManager) ProcessHandlers(ctx context.Context, runner *TaskRunner, hosts []types.Host, vars map[string]interface{}) ([]types.Result, error) {
	handlers := h.GetPendingHandlers()
	if len(handlers) == 0 {
		return []types.Result{}, nil
	}
	
	var allResults []types.Result
	
	for _, handler := range handlers {
		// Execute handler task
		results, err := runner.Run(ctx, handler, hosts, vars)
		if err != nil {
			return allResults, fmt.Errorf("handler '%s' failed: %w", handler.Name, err)
		}
		allResults = append(allResults, results...)
	}
	
	return allResults, nil
}