package logging

import (
	"context"
	"testing"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

func TestNewStreamLogger(t *testing.T) {
	logger := NewStreamLogger("test_source", "test_session")
	
	if logger == nil {
		t.Fatal("NewStreamLogger should not return nil")
	}
	
	if logger.level != LevelInfo {
		t.Errorf("Expected log level Info, got %s", logger.level)
	}
	
	if logger.sessionID != "test_session" {
		t.Errorf("Expected session ID 'test_session', got '%s'", logger.sessionID)
	}
	
	if logger.source != "test_source" {
		t.Errorf("Expected source 'test_source', got '%s'", logger.source)
	}
	
	logger.Close()
}

func TestStreamLogger_SetLevel(t *testing.T) {
	logger := NewStreamLogger("test", "session")
	defer logger.Close()
	
	logger.SetLevel(LevelError)
	if logger.level != LevelError {
		t.Errorf("Expected level Error, got %s", logger.level)
	}
	
	logger.SetLevel(LevelDebug)
	if logger.level != LevelDebug {
		t.Errorf("Expected level Debug, got %s", logger.level)
	}
}

func TestStreamLogger_AddConsoleOutput(t *testing.T) {
	logger := NewStreamLogger("test", "session")
	defer logger.Close()
	
	// This should not panic
	logger.AddConsoleOutput("json", false)
	logger.AddConsoleOutput("text", true)
	
	// Test that we can log to console outputs
	logger.Log(LevelInfo, "Test message", map[string]interface{}{"key": "value"})
}

func TestStreamLogger_AddMemoryOutput(t *testing.T) {
	logger := NewStreamLogger("test", "session")
	defer logger.Close()
	
	memOutput := logger.AddMemoryOutput(10)
	if memOutput == nil {
		t.Fatal("AddMemoryOutput should return a memory output")
	}
	
	// Log some messages
	logger.Log(LevelInfo, "Test message 1", nil)
	logger.Log(LevelWarn, "Test message 2", nil)
	
	// Force flush to ensure entries are written to outputs
	logger.Flush()
	
	entries := memOutput.GetEntries()
	if len(entries) < 2 {
		t.Errorf("Expected at least 2 entries, got %d", len(entries))
	}
}

func TestStreamLogger_LogStreamEvent(t *testing.T) {
	logger := NewStreamLogger("test", "session")
	logger.SetLevel(LevelDebug) // Set to debug level to capture StreamStdout events
	memOutput := logger.AddMemoryOutput(10)
	defer logger.Close()
	
	event := types.StreamEvent{
		Type:      types.StreamStdout,
		Data:      "Test output",
		Timestamp: time.Now(),
	}
	
	logger.LogStreamEvent(event, "test_task", "localhost")
	
	// Force flush to ensure entries are written to outputs
	logger.Flush()
	
	entries := memOutput.GetEntries()
	if len(entries) == 0 {
		t.Error("Expected at least one log entry")
	}
	
	// Check that the entry contains stream event data
	found := false
	for _, entry := range entries {
		if entry.StreamEvent != nil && entry.StreamEvent.Type == types.StreamStdout {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected to find stream event log entry")
	}
}

func TestStreamLogger_LogProgress(t *testing.T) {
	logger := NewStreamLogger("test", "session")
	memOutput := logger.AddMemoryOutput(10)
	defer logger.Close()
	
	progress := types.ProgressInfo{
		Stage:      "uploading",
		Percentage: 75.0,
		Message:    "Upload in progress",
		Timestamp:  time.Now(),
	}
	
	logger.LogProgress(progress, "test_task", "localhost")
	
	// Force flush to ensure entries are written to outputs
	logger.Flush()
	
	entries := memOutput.GetEntries()
	if len(entries) == 0 {
		t.Error("Expected at least one log entry")
	}
	
	// Check that progress data is logged
	found := false
	for _, entry := range entries {
		if entry.Progress != nil && entry.Progress.Percentage == 75.0 {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected to find progress log entry")
	}
}

func TestStreamLogger_LogStep(t *testing.T) {
	logger := NewStreamLogger("test", "session")
	memOutput := logger.AddMemoryOutput(10)
	defer logger.Close()
	
	step := types.StepInfo{
		ID:          "test_step",
		Name:        "Test Step",
		Description: "A test step",
		Status:      types.StepCompleted,
		StartTime:   time.Now().Add(-time.Second),
		EndTime:     time.Now(),
	}
	
	logger.LogStep(step, "test_task", "localhost")
	
	// Force flush to ensure entries are written to outputs
	logger.Flush()
	
	entries := memOutput.GetEntries()
	if len(entries) == 0 {
		t.Error("Expected at least one log entry")
	}
	
	// Check that step data is logged
	found := false
	for _, entry := range entries {
		if entry.StepInfo != nil && entry.StepInfo.ID == "test_step" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected to find step log entry")
	}
}

func TestStreamLogger_SetFilters(t *testing.T) {
	logger := NewStreamLogger("test", "session")
	logger.SetLevel(LevelDebug) // Set to debug level to capture all events
	defer logger.Close()
	
	// This should not panic
	logger.SetFilters(true, true, false, true)
	
	// Verify filtering works by adding memory output and checking logs
	memOutput := logger.AddMemoryOutput(10)
	
	// Log different types
	event := types.StreamEvent{Type: types.StreamStdout, Data: "output"}
	progress := types.ProgressInfo{Percentage: 50.0}
	step := types.StepInfo{ID: "step1", Status: types.StepRunning}
	
	logger.LogStreamEvent(event, "task", "host")
	logger.LogProgress(progress, "task", "host") 
	logger.LogStep(step, "task", "host")
	
	// Force flush to ensure entries are written to outputs
	logger.Flush()
	
	entries := memOutput.GetEntries()
	// With output filtered out, should have step and progress but not output
	if len(entries) != 2 {
		t.Errorf("Expected exactly 2 entries with current filters (step and progress), got %d", len(entries))
	}
}

func TestStreamLogger_SetEnabled(t *testing.T) {
	logger := NewStreamLogger("test", "session")
	memOutput := logger.AddMemoryOutput(10)
	defer logger.Close()
	
	// Disable logging
	logger.SetEnabled(false)
	logger.Log(LevelInfo, "This should not be logged", nil)
	
	time.Sleep(20 * time.Millisecond)
	
	entries := memOutput.GetEntries()
	if len(entries) > 0 {
		t.Error("No entries should be logged when disabled")
	}
	
	// Re-enable logging
	logger.SetEnabled(true)
	logger.Log(LevelInfo, "This should be logged", nil)
	
	// Force flush to ensure entries are written to outputs
	logger.Flush()
	
	entries = memOutput.GetEntries()
	if len(entries) == 0 {
		t.Error("Entry should be logged when enabled")
	}
}

func TestMemoryLogOutput_GetEntries(t *testing.T) {
	output := &MemoryLogOutput{
		maxSize: 5,
	}
	
	// Add some entries
	for i := 0; i < 3; i++ {
		entry := LogEntry{
			Level:     LevelInfo,
			Message:   "Test message",
			Timestamp: time.Now(),
		}
		output.Write(entry)
	}
	
	entries := output.GetEntries()
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
	
	// Test buffer overflow
	for i := 0; i < 5; i++ {
		entry := LogEntry{
			Level:     LevelInfo,
			Message:   "Overflow message",
			Timestamp: time.Now(),
		}
		output.Write(entry)
	}
	
	entries = output.GetEntries()
	if len(entries) != 5 { // Should be capped at maxSize
		t.Errorf("Expected 5 entries (maxSize), got %d", len(entries))
	}
	
	// Clear entries
	output.Clear()
	entries = output.GetEntries()
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(entries))
	}
}

func TestStreamingLoggerAdapter(t *testing.T) {
	logger := NewStreamLogger("test", "session")
	logger.SetLevel(LevelDebug) // Set to debug level to capture StreamStdout events
	memOutput := logger.AddMemoryOutput(10)
	defer logger.Close()
	
	adapter := NewStreamingLoggerAdapter(logger, "test_task", "localhost")
	if adapter == nil {
		t.Fatal("NewStreamingLoggerAdapter should not return nil")
	}
	
	// Test HandleStreamEvents with context
	events := make(chan types.StreamEvent, 1)
	event := types.StreamEvent{
		Type: types.StreamStdout,
		Data: "Test output",
	}
	events <- event
	close(events)
	
	ctx := context.Background()
	go adapter.HandleStreamEvents(ctx, events)
	
	time.Sleep(100 * time.Millisecond)
	
	// Force flush to ensure entries are written to outputs
	logger.Flush()
	
	entries := memOutput.GetEntries()
	if len(entries) == 0 {
		t.Error("Expected log entry from HandleStreamEvents")
	}
	
	// Test CreateProgressCallback
	callback := adapter.CreateProgressCallback()
	if callback == nil {
		t.Fatal("CreateProgressCallback should not return nil")
	}
	
	progress := types.ProgressInfo{
		Percentage: 25.0,
		Message:    "Test progress",
	}
	
	callback(progress)
	
	time.Sleep(50 * time.Millisecond)
	
	// Force flush to ensure entries are written to outputs
	logger.Flush()
	
	entries = memOutput.GetEntries()
	if len(entries) < 2 {
		t.Error("Expected additional log entry from progress callback")
	}
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
	}
	
	for _, test := range tests {
		result := test.level.String()
		if result != test.expected {
			t.Errorf("Level %d should return '%s', got '%s'", int(test.level), test.expected, result)
		}
	}
}

// Benchmark tests
func BenchmarkStreamLogger_Log(b *testing.B) {
	logger := NewStreamLogger("benchmark", "session")
	logger.AddMemoryOutput(1000)
	defer logger.Close()
	
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		logger.Log(LevelInfo, "Benchmark message", fields)
	}
}

func BenchmarkMemoryLogOutput_Write(b *testing.B) {
	output := &MemoryLogOutput{maxSize: 1000}
	
	entry := LogEntry{
		Level:     LevelInfo,
		Message:   "Benchmark message",
		Timestamp: time.Now(),
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		output.Write(entry)
	}
}