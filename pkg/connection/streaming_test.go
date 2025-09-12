package connection

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// TestLocalConnectionStreaming tests the streaming functionality for LocalConnection
func TestLocalConnectionStreaming(t *testing.T) {
	conn := NewLocalConnection()
	ctx := context.Background()

	// Connect
	err := conn.Connect(ctx, types.ConnectionInfo{})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	t.Run("BasicStreaming", func(t *testing.T) {
		options := types.ExecuteOptions{
			StreamOutput: true,
			Timeout:      10 * time.Second,
		}

		events, err := conn.ExecuteStream(ctx, "echo 'Hello'; sleep 0.1; echo 'World'", options)
		if err != nil {
			t.Fatalf("ExecuteStream failed: %v", err)
		}

		var stdout []string
		var progressEvents []types.ProgressInfo
		var finalResult *types.Result

		for event := range events {
			switch event.Type {
			case types.StreamStdout:
				stdout = append(stdout, event.Data)
			case types.StreamProgress:
				if event.Progress != nil {
					progressEvents = append(progressEvents, *event.Progress)
				}
			case types.StreamDone:
				finalResult = event.Result
			case types.StreamError:
				t.Fatalf("Unexpected error event: %v", event.Error)
			}
		}

		// Verify output
		if len(stdout) < 2 {
			t.Errorf("Expected at least 2 stdout lines, got %d", len(stdout))
		}
		if !contains(stdout, "Hello") {
			t.Errorf("Expected 'Hello' in stdout, got %v", stdout)
		}
		if !contains(stdout, "World") {
			t.Errorf("Expected 'World' in stdout, got %v", stdout)
		}

		// Verify progress events
		if len(progressEvents) < 2 {
			t.Errorf("Expected at least 2 progress events, got %d", len(progressEvents))
		}

		// Verify final result
		if finalResult == nil {
			t.Fatal("Expected final result, got nil")
		}
		if !finalResult.Success {
			t.Errorf("Expected successful result, got error: %v", finalResult.Error)
		}
		if stdout, ok := finalResult.Data["stdout"].(string); !ok || stdout == "" {
			t.Error("Expected non-empty stdout in final result data")
		}
	})

	t.Run("StreamingWithCallbacks", func(t *testing.T) {
		var callbackLines []string
		var progressUpdates []types.ProgressInfo

		options := types.ExecuteOptions{
			StreamOutput: true,
			OutputCallback: func(line string, isStderr bool) {
				callbackLines = append(callbackLines, line)
			},
			ProgressCallback: func(progress types.ProgressInfo) {
				progressUpdates = append(progressUpdates, progress)
			},
		}

		events, err := conn.ExecuteStream(ctx, "echo 'Test callback'; echo 'Another line'", options)
		if err != nil {
			t.Fatalf("ExecuteStream failed: %v", err)
		}

		// Consume events
		for event := range events {
			if event.Type == types.StreamError {
				t.Fatalf("Unexpected error: %v", event.Error)
			}
		}

		// Verify callbacks were called
		if len(callbackLines) == 0 {
			t.Error("Expected output callback to be called")
		}
		if len(progressUpdates) == 0 {
			t.Error("Expected progress callback to be called")
		}
	})

	t.Run("StreamingErrorHandling", func(t *testing.T) {
		options := types.ExecuteOptions{
			StreamOutput: true,
		}

		events, err := conn.ExecuteStream(ctx, "exit 1", options)
		if err != nil {
			t.Fatalf("ExecuteStream failed: %v", err)
		}

		var finalResult *types.Result
		for event := range events {
			if event.Type == types.StreamDone {
				finalResult = event.Result
			}
		}

		if finalResult == nil {
			t.Fatal("Expected final result")
		}
		if finalResult.Success {
			t.Error("Expected command to fail, but it succeeded")
		}
	})

	t.Run("StreamingWithTimeout", func(t *testing.T) {
		options := types.ExecuteOptions{
			StreamOutput: true,
			Timeout:      100 * time.Millisecond,
		}

		start := time.Now()
		events, err := conn.ExecuteStream(ctx, "sleep 5", options)
		if err != nil {
			t.Fatalf("ExecuteStream failed: %v", err)
		}

		var finalResult *types.Result
		for event := range events {
			if event.Type == types.StreamDone {
				finalResult = event.Result
			}
		}

		elapsed := time.Since(start)
		if elapsed > 2*time.Second {
			t.Errorf("Expected timeout to occur quickly, took %v", elapsed)
		}

		if finalResult == nil {
			t.Fatal("Expected final result")
		}
		if finalResult.Success {
			t.Error("Expected command to fail due to timeout")
		}
	})

	t.Run("NonStreamingMode", func(t *testing.T) {
		// Test that streaming works even when StreamOutput is false
		options := types.ExecuteOptions{
			StreamOutput: false, // Disable streaming events
		}

		events, err := conn.ExecuteStream(ctx, "echo 'Hello World'", options)
		if err != nil {
			t.Fatalf("ExecuteStream failed: %v", err)
		}

		var eventTypes []types.StreamEventType
		var finalResult *types.Result

		for event := range events {
			eventTypes = append(eventTypes, event.Type)
			if event.Type == types.StreamDone {
				finalResult = event.Result
			}
		}

		// Should only get done event when streaming is disabled
		if len(eventTypes) > 1 {
			t.Errorf("Expected minimal events with StreamOutput=false, got %v", eventTypes)
		}

		if finalResult == nil {
			t.Fatal("Expected final result")
		}
		if !finalResult.Success {
			t.Errorf("Expected success, got error: %v", finalResult.Error)
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		options := types.ExecuteOptions{
			StreamOutput: true,
		}

		events, err := conn.ExecuteStream(cancelCtx, "sleep 10", options)
		if err != nil {
			t.Fatalf("ExecuteStream failed: %v", err)
		}

		// Cancel context after a short delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		var finalResult *types.Result
		start := time.Now()
		
		for event := range events {
			if event.Type == types.StreamDone {
				finalResult = event.Result
				break
			}
		}

		elapsed := time.Since(start)
		if elapsed > 2*time.Second {
			t.Errorf("Expected quick cancellation, took %v", elapsed)
		}

		if finalResult != nil && finalResult.Success {
			t.Error("Expected command to be cancelled")
		}
	})
}

// TestStreamEventTypes tests the different stream event types
func TestStreamEventTypes(t *testing.T) {
	tests := []struct {
		name     string
		eventType types.StreamEventType
		expected string
	}{
		{"Stdout", types.StreamStdout, "stdout"},
		{"Stderr", types.StreamStderr, "stderr"},
		{"Progress", types.StreamProgress, "progress"},
		{"Done", types.StreamDone, "done"},
		{"Error", types.StreamError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.eventType) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.eventType))
			}
		})
	}
}

// TestProgressInfo tests the ProgressInfo structure
func TestProgressInfo(t *testing.T) {
	progress := types.ProgressInfo{
		Stage:      "testing",
		Percentage: 50.0,
		Message:    "Test progress",
		BytesTotal: 1000,
		BytesDone:  500,
		Timestamp:  time.Now(),
	}

	if progress.Stage != "testing" {
		t.Errorf("Expected stage 'testing', got %s", progress.Stage)
	}
	if progress.Percentage != 50.0 {
		t.Errorf("Expected percentage 50.0, got %f", progress.Percentage)
	}
	if progress.Message != "Test progress" {
		t.Errorf("Expected message 'Test progress', got %s", progress.Message)
	}
}

// TestStreamEvent tests the StreamEvent structure
func TestStreamEvent(t *testing.T) {
	now := time.Now()
	progress := &types.ProgressInfo{
		Stage:     "testing",
		Message:   "Test message",
		Timestamp: now,
	}

	event := types.StreamEvent{
		Type:      types.StreamProgress,
		Data:      "test data",
		Progress:  progress,
		Timestamp: now,
	}

	if event.Type != types.StreamProgress {
		t.Errorf("Expected type %s, got %s", types.StreamProgress, event.Type)
	}
	if event.Data != "test data" {
		t.Errorf("Expected data 'test data', got %s", event.Data)
	}
	if event.Progress == nil {
		t.Error("Expected progress to be set")
	} else if event.Progress.Stage != "testing" {
		t.Errorf("Expected progress stage 'testing', got %s", event.Progress.Stage)
	}
}

// TestStreamingConnectionInterface tests that connections implement the interface
func TestStreamingConnectionInterface(t *testing.T) {
	var conn types.StreamingConnection

	// Test that LocalConnection implements StreamingConnection
	localConn := NewLocalConnection()
	conn = localConn
	if conn == nil {
		t.Error("LocalConnection should implement StreamingConnection interface")
	}

	// Test that SSHConnection implements StreamingConnection
	sshConn := NewSSHConnection()
	conn = sshConn
	if conn == nil {
		t.Error("SSHConnection should implement StreamingConnection interface")
	}
}

// Benchmark tests for streaming performance
func BenchmarkLocalConnectionStreaming(b *testing.B) {
	conn := NewLocalConnection()
	ctx := context.Background()

	err := conn.Connect(ctx, types.ConnectionInfo{})
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	options := types.ExecuteOptions{
		StreamOutput: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		events, err := conn.ExecuteStream(ctx, "echo 'benchmark test'", options)
		if err != nil {
			b.Fatalf("ExecuteStream failed: %v", err)
		}

		// Consume all events
		for range events {
		}
	}
}

// Test helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.Contains(s, item) {
			return true
		}
	}
	return false
}