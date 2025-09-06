package modules

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/gosinble/gosinble/pkg/types"
)

// MockStreamingConnection implements StreamingConnection for testing
type MockStreamingConnection struct {
	*MockConnection
	streamEvents []types.StreamEvent
	streamDelay  time.Duration
}

func NewMockStreamingConnection() *MockStreamingConnection {
	return &MockStreamingConnection{
		MockConnection: &MockConnection{},
		streamDelay:    10 * time.Millisecond,
	}
}

func (m *MockStreamingConnection) ExecuteStream(ctx context.Context, command string, options types.ExecuteOptions) (<-chan types.StreamEvent, error) {
	eventChan := make(chan types.StreamEvent, 10)

	go func() {
		defer close(eventChan)

		// Send initial progress
		if options.StreamOutput {
			eventChan <- types.StreamEvent{
				Type: types.StreamProgress,
				Progress: &types.ProgressInfo{
					Stage:     "executing",
					Message:   "Starting command",
					Timestamp: time.Now(),
				},
				Timestamp: time.Now(),
			}

			// Simulate some output
			time.Sleep(m.streamDelay)
			eventChan <- types.StreamEvent{
				Type:      types.StreamStdout,
				Data:      "Mock output line 1",
				Timestamp: time.Now(),
			}

			time.Sleep(m.streamDelay)
			eventChan <- types.StreamEvent{
				Type:      types.StreamStdout,
				Data:      "Mock output line 2",
				Timestamp: time.Now(),
			}

			// Send final progress
			eventChan <- types.StreamEvent{
				Type: types.StreamProgress,
				Progress: &types.ProgressInfo{
					Stage:      "completed",
					Percentage: 100.0,
					Message:    "Command completed",
					Timestamp:  time.Now(),
				},
				Timestamp: time.Now(),
			}
		}

		// Send final result
		result := &types.Result{
			Success:    true,
			Changed:    true,
			Message:    "Mock command executed",
			Host:       "mock-host",
			StartTime:  time.Now().Add(-100 * time.Millisecond),
			EndTime:    time.Now(),
			Duration:   100 * time.Millisecond,
			ModuleName: "streaming_shell",
			Data: map[string]interface{}{
				"stdout":    "Mock output line 1\nMock output line 2\n",
				"stderr":    "",
				"cmd":       command,
				"exit_code": 0,
			},
		}

		eventChan <- types.StreamEvent{
			Type:      types.StreamDone,
			Result:    result,
			Timestamp: time.Now(),
		}
	}()

	return eventChan, nil
}

func TestStreamingShellModule(t *testing.T) {
	module := NewStreamingShellModule()
	ctx := context.Background()

	t.Run("BasicStreaming", func(t *testing.T) {
		conn := NewMockStreamingConnection()
		args := map[string]interface{}{
			"cmd":           "echo 'test'",
			"stream_output": true,
		}

		result, err := module.Run(ctx, conn, args)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %v", result.Error)
		}

		// Check streaming metadata
		if streamingEnabled, ok := result.Data["streaming_enabled"].(bool); !ok || !streamingEnabled {
			t.Error("Expected streaming to be enabled")
		}

		if eventCount, ok := result.Data["stream_events_received"].(int); !ok || eventCount == 0 {
			t.Errorf("Expected stream events, got %v", eventCount)
		}
	})

	t.Run("StreamingDisabled", func(t *testing.T) {
		conn := NewMockStreamingConnection()
		args := map[string]interface{}{
			"cmd":           "echo 'test'",
			"stream_output": false,
		}

		// Mock the standard Execute method
		conn.MockConnection.On("Execute", ctx, "echo 'test'", mock.Anything).Return(&types.Result{
			Success: true,
			Changed: true,
			Message: "Standard execution",
			Data: map[string]interface{}{
				"exit_code": 0,
			},
		}, nil)

		result, err := module.Run(ctx, conn, args)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %v", result.Error)
		}

		// Check that standard execution was used
		if streamingEnabled, ok := result.Data["streaming_enabled"].(bool); !ok || streamingEnabled {
			t.Error("Expected streaming to be disabled")
		}

		if executionMode, ok := result.Data["execution_mode"].(string); !ok || executionMode != "standard" {
			t.Errorf("Expected standard execution mode, got %v", executionMode)
		}
	})

	t.Run("NonStreamingConnection", func(t *testing.T) {
		conn := &MockConnection{}
		args := map[string]interface{}{
			"cmd":           "echo 'test'",
			"stream_output": true, // Request streaming but connection doesn't support it
		}

		// Mock the standard Execute method
		conn.On("Execute", ctx, "echo 'test'", mock.Anything).Return(&types.Result{
			Success: true,
			Changed: true,
			Message: "Fallback execution",
			Data: map[string]interface{}{
				"stdout":    "test\n",
				"stderr":    "",
				"exit_code": 0,
			},
		}, nil)

		result, err := module.Run(ctx, conn, args)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %v", result.Error)
		}

		// Should fall back to standard execution
		if streamingEnabled, ok := result.Data["streaming_enabled"].(bool); !ok || streamingEnabled {
			t.Error("Expected streaming to be disabled for non-streaming connection")
		}
	})

	t.Run("ValidationTests", func(t *testing.T) {
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name:    "MissingCommand",
				args:    map[string]interface{}{},
				wantErr: true,
			},
			{
				name: "ValidCommand",
				args: map[string]interface{}{
					"cmd": "echo test",
				},
				wantErr: false,
			},
			{
				name: "ValidTimeout",
				args: map[string]interface{}{
					"cmd":     "echo test",
					"timeout": 60,
				},
				wantErr: false,
			},
			{
				name: "InvalidTimeout",
				args: map[string]interface{}{
					"cmd":     "echo test",
					"timeout": -1,
				},
				wantErr: true,
			},
			{
				name: "InvalidTimeoutType",
				args: map[string]interface{}{
					"cmd":     "echo test",
					"timeout": "invalid",
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := module.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("StreamingWithTimeout", func(t *testing.T) {
		conn := NewMockStreamingConnection()
		conn.streamDelay = 200 * time.Millisecond // Longer delay
		
		args := map[string]interface{}{
			"cmd":           "sleep 1",
			"stream_output": true,
			"timeout":       1, // 1 second timeout
		}

		start := time.Now()
		result, err := module.Run(ctx, conn, args)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should complete within reasonable time
		if elapsed > 2*time.Second {
			t.Errorf("Expected quick completion, took %v", elapsed)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %v", result.Error)
		}
	})

	t.Run("Documentation", func(t *testing.T) {
		doc := module.Documentation()
		
		if doc.Name != "streaming_shell" {
			t.Errorf("Expected name 'streaming_shell', got %s", doc.Name)
		}

		if doc.Description == "" {
			t.Error("Expected non-empty description")
		}

		// Check required parameters
		if cmdParam, ok := doc.Parameters["cmd"]; !ok {
			t.Error("Expected 'cmd' parameter in documentation")
		} else if !cmdParam.Required {
			t.Error("Expected 'cmd' parameter to be required")
		}

		// Check optional parameters
		if streamParam, ok := doc.Parameters["stream_output"]; !ok {
			t.Error("Expected 'stream_output' parameter in documentation")
		} else if streamParam.Required {
			t.Error("Expected 'stream_output' parameter to be optional")
		}

		if len(doc.Examples) == 0 {
			t.Error("Expected examples in documentation")
		}

		if len(doc.Returns) == 0 {
			t.Error("Expected return values in documentation")
		}
	})
}

func TestStreamingShellModuleName(t *testing.T) {
	module := NewStreamingShellModule()
	
	if module.Name() != "streaming_shell" {
		t.Errorf("Expected name 'streaming_shell', got %s", module.Name())
	}
}

// Benchmark tests
func BenchmarkStreamingShellModule(b *testing.B) {
	module := NewStreamingShellModule()
	conn := NewMockStreamingConnection()
	conn.streamDelay = 0 // No delay for benchmarking
	ctx := context.Background()
	
	args := map[string]interface{}{
		"cmd":           "echo benchmark",
		"stream_output": true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := module.Run(ctx, conn, args)
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
	}
}

func BenchmarkStreamingShellModuleStandard(b *testing.B) {
	module := NewStreamingShellModule()
	conn := &MockConnection{}
	ctx := context.Background()

	// Mock standard execution
	conn.On("Execute", ctx, "echo benchmark", mock.Anything).Return(&types.Result{
		Success: true,
		Changed: true,
		Message: "Benchmark execution",
		Data: map[string]interface{}{
			"stdout":    "benchmark\n",
			"stderr":    "",
			"exit_code": 0,
		},
	}, nil)

	args := map[string]interface{}{
		"cmd":           "echo benchmark",
		"stream_output": false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := module.Run(ctx, conn, args)
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
	}
}