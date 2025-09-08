package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// LogLevel represents the severity of a log entry
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       LogLevel               `json:"level"`
	Message     string                 `json:"message"`
	Source      string                 `json:"source,omitempty"`      // Module or connection source
	SessionID   string                 `json:"session_id,omitempty"`  // Execution session
	TaskName    string                 `json:"task_name,omitempty"`   // Task name
	Host        string                 `json:"host,omitempty"`        // Target host
	Duration    time.Duration          `json:"duration,omitempty"`    // Operation duration
	Fields      map[string]interface{} `json:"fields,omitempty"`      // Additional structured data
	
	// Gosinble-specific fields
	StreamEvent *types.StreamEvent `json:"stream_event,omitempty"`
	StepInfo    *types.StepInfo    `json:"step_info,omitempty"`
	Progress    *types.ProgressInfo `json:"progress,omitempty"`
	
	// Error information
	Error       string `json:"error,omitempty"`
	StackTrace  string `json:"stack_trace,omitempty"`
}

// StreamLogger handles logging for gosinble streaming operations
type StreamLogger struct {
	mu          sync.RWMutex
	outputs     []LogOutput
	level       LogLevel
	sessionID   string
	source      string
	enabled     bool
	
	// Filtering options
	includeSteps     bool
	includeProgress  bool
	includeOutput    bool
	includeErrors    bool
	
	// Buffering options
	bufferSize      int
	flushInterval   time.Duration
	buffer          []LogEntry
	bufferMu        sync.Mutex
	flushTicker     *time.Ticker
	stopFlush       chan bool
}

// LogOutput represents a log destination
type LogOutput interface {
	Write(entry LogEntry) error
	Close() error
}

// FileLogOutput writes logs to a file
type FileLogOutput struct {
	file     *os.File
	encoder  *json.Encoder
	filePath string
	mu       sync.Mutex
}

// ConsoleLogOutput writes logs to console
type ConsoleLogOutput struct {
	writer io.Writer
	format string // "json" or "text"
	colors bool
}

// MemoryLogOutput stores logs in memory (useful for testing)
type MemoryLogOutput struct {
	entries []LogEntry
	mu      sync.RWMutex
	maxSize int
}

// NewStreamLogger creates a new stream logger
func NewStreamLogger(source, sessionID string) *StreamLogger {
	logger := &StreamLogger{
		outputs:         make([]LogOutput, 0),
		level:           LevelInfo,
		sessionID:       sessionID,
		source:          source,
		enabled:         true,
		includeSteps:    true,
		includeProgress: true,
		includeOutput:   true,
		includeErrors:   true,
		bufferSize:      100,
		flushInterval:   5 * time.Second,
		buffer:          make([]LogEntry, 0),
		stopFlush:       make(chan bool),
	}
	
	// Start buffer flushing
	logger.startFlushTimer()
	
	return logger
}

// AddFileOutput adds a file output to the logger
func (l *StreamLogger) AddFileOutput(filePath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}
	
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	
	output := &FileLogOutput{
		file:     file,
		encoder:  json.NewEncoder(file),
		filePath: filePath,
	}
	
	l.outputs = append(l.outputs, output)
	return nil
}

// AddConsoleOutput adds console output to the logger
func (l *StreamLogger) AddConsoleOutput(format string, colors bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	output := &ConsoleLogOutput{
		writer: os.Stdout,
		format: format,
		colors: colors,
	}
	
	l.outputs = append(l.outputs, output)
}

// AddMemoryOutput adds memory output to the logger (useful for testing)
func (l *StreamLogger) AddMemoryOutput(maxSize int) *MemoryLogOutput {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	output := &MemoryLogOutput{
		entries: make([]LogEntry, 0),
		maxSize: maxSize,
	}
	
	l.outputs = append(l.outputs, output)
	return output
}

// SetLevel sets the minimum log level
func (l *StreamLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetFilters configures what types of events to log
func (l *StreamLogger) SetFilters(steps, progress, output, errors bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.includeSteps = steps
	l.includeProgress = progress
	l.includeOutput = output
	l.includeErrors = errors
}

// Enable/Disable logging
func (l *StreamLogger) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// Log writes a log entry
func (l *StreamLogger) Log(level LogLevel, message string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}
	
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    l.source,
		SessionID: l.sessionID,
		Fields:    fields,
	}
	
	l.writeEntry(entry)
}

// LogStreamEvent logs a gosinble stream event
func (l *StreamLogger) LogStreamEvent(event types.StreamEvent, taskName, host string) {
	if !l.enabled {
		return
	}
	
	// Apply filters
	switch event.Type {
	case types.StreamStepStart, types.StreamStepUpdate, types.StreamStepEnd:
		if !l.includeSteps {
			return
		}
	case types.StreamProgress:
		if !l.includeProgress {
			return
		}
	case types.StreamStdout, types.StreamStderr:
		if !l.includeOutput {
			return
		}
	case types.StreamError:
		if !l.includeErrors {
			return
		}
	}
	
	level := l.getLogLevelForEvent(event)
	if !l.shouldLog(level) {
		return
	}
	
	message := l.formatEventMessage(event)
	
	entry := LogEntry{
		Timestamp:   time.Now(),
		Level:       level,
		Message:     message,
		Source:      l.source,
		SessionID:   l.sessionID,
		TaskName:    taskName,
		Host:        host,
		StreamEvent: &event,
		Fields: map[string]interface{}{
			"event_type": string(event.Type),
		},
	}
	
	// Add specific fields based on event type
	if event.Progress != nil {
		entry.Progress = event.Progress
		entry.Fields["percentage"] = event.Progress.Percentage
		entry.Fields["stage"] = event.Progress.Stage
	}
	
	if event.Step != nil {
		entry.StepInfo = event.Step
		entry.Fields["step_id"] = event.Step.ID
		entry.Fields["step_status"] = string(event.Step.Status)
		if event.Step.Duration > 0 {
			entry.Duration = event.Step.Duration
		}
	}
	
	if event.Error != nil {
		entry.Error = event.Error.Error()
	}
	
	l.writeEntry(entry)
}

// LogProgress logs progress information
func (l *StreamLogger) LogProgress(progress types.ProgressInfo, taskName, host string) {
	if !l.enabled || !l.includeProgress {
		return
	}
	
	level := LevelInfo
	if !l.shouldLog(level) {
		return
	}
	
	message := fmt.Sprintf("Progress: %.1f%% - %s", progress.Percentage, progress.Message)
	
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    l.source,
		SessionID: l.sessionID,
		TaskName:  taskName,
		Host:      host,
		Progress:  &progress,
		Fields: map[string]interface{}{
			"percentage": progress.Percentage,
			"stage":      progress.Stage,
			"step_number": progress.StepNumber,
			"total_steps": progress.TotalSteps,
		},
	}
	
	l.writeEntry(entry)
}

// LogStep logs step information
func (l *StreamLogger) LogStep(step types.StepInfo, taskName, host string) {
	if !l.enabled || !l.includeSteps {
		return
	}
	
	level := l.getLogLevelForStep(step)
	if !l.shouldLog(level) {
		return
	}
	
	message := fmt.Sprintf("Step %s (%s): %s", step.ID, step.Status, step.Name)
	
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    l.source,
		SessionID: l.sessionID,
		TaskName:  taskName,
		Host:      host,
		StepInfo:  &step,
		Duration:  step.Duration,
		Fields: map[string]interface{}{
			"step_id":     step.ID,
			"step_status": string(step.Status),
			"step_name":   step.Name,
		},
	}
	
	l.writeEntry(entry)
}

// Close closes the logger and all outputs
func (l *StreamLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Stop flush timer
	if l.flushTicker != nil {
		l.flushTicker.Stop()
		close(l.stopFlush)
	}
	
	// Flush remaining buffer (without additional locking since we already have the lock)
	l.flushUnsafe()
	
	// Close all outputs
	var lastErr error
	for _, output := range l.outputs {
		if err := output.Close(); err != nil {
			lastErr = err
		}
	}
	
	return lastErr
}

// Private methods

func (l *StreamLogger) shouldLog(level LogLevel) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabled && level >= l.level
}

func (l *StreamLogger) writeEntry(entry LogEntry) {
	l.bufferMu.Lock()
	defer l.bufferMu.Unlock()
	
	l.buffer = append(l.buffer, entry)
	
	// Flush if buffer is full
	if len(l.buffer) >= l.bufferSize {
		l.flush()
	}
}

// Flush forces a flush of the buffer to all outputs
func (l *StreamLogger) Flush() {
	l.flush()
}

func (l *StreamLogger) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.flushUnsafe()
}

func (l *StreamLogger) flushUnsafe() {
	if len(l.buffer) == 0 {
		return
	}
	
	// We assume the caller has already acquired the necessary lock
	for _, output := range l.outputs {
		for _, entry := range l.buffer {
			output.Write(entry)
		}
	}
	
	l.buffer = l.buffer[:0]
}

func (l *StreamLogger) startFlushTimer() {
	l.flushTicker = time.NewTicker(l.flushInterval)
	go func() {
		for {
			select {
			case <-l.flushTicker.C:
				l.bufferMu.Lock()
				l.flush()
				l.bufferMu.Unlock()
			case <-l.stopFlush:
				return
			}
		}
	}()
}

func (l *StreamLogger) getLogLevelForEvent(event types.StreamEvent) LogLevel {
	switch event.Type {
	case types.StreamError:
		return LevelError
	case types.StreamStderr:
		return LevelWarn
	case types.StreamStepStart, types.StreamStepEnd:
		return LevelInfo
	default:
		return LevelDebug
	}
}

func (l *StreamLogger) getLogLevelForStep(step types.StepInfo) LogLevel {
	switch step.Status {
	case types.StepFailed:
		return LevelError
	case types.StepSkipped:
		return LevelWarn
	case types.StepCompleted:
		return LevelInfo
	default:
		return LevelDebug
	}
}

func (l *StreamLogger) formatEventMessage(event types.StreamEvent) string {
	switch event.Type {
	case types.StreamStdout:
		return fmt.Sprintf("Output: %s", event.Data)
	case types.StreamStderr:
		return fmt.Sprintf("Error Output: %s", event.Data)
	case types.StreamProgress:
		if event.Progress != nil {
			return fmt.Sprintf("Progress: %.1f%% - %s", event.Progress.Percentage, event.Progress.Message)
		}
		return "Progress Update"
	case types.StreamStepStart:
		if event.Step != nil {
			return fmt.Sprintf("Step Started: %s", event.Step.Name)
		}
		return "Step Started"
	case types.StreamStepEnd:
		if event.Step != nil {
			return fmt.Sprintf("Step Completed: %s (%s)", event.Step.Name, event.Step.Status)
		}
		return "Step Completed"
	case types.StreamDone:
		return "Execution Completed"
	case types.StreamError:
		if event.Error != nil {
			return fmt.Sprintf("Error: %v", event.Error)
		}
		return "Error Occurred"
	default:
		return fmt.Sprintf("Stream Event: %s", event.Type)
	}
}

// LogOutput implementations

// FileLogOutput methods
func (f *FileLogOutput) Write(entry LogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.encoder.Encode(entry)
}

func (f *FileLogOutput) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Close()
}

// ConsoleLogOutput methods
func (c *ConsoleLogOutput) Write(entry LogEntry) error {
	if c.format == "json" {
		encoder := json.NewEncoder(c.writer)
		return encoder.Encode(entry)
	}
	
	// Text format
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	level := entry.Level.String()
	
	if c.colors {
		level = c.colorizeLevel(level)
	}
	
	line := fmt.Sprintf("%s [%s] [%s] %s", timestamp, level, entry.Source, entry.Message)
	if entry.Host != "" {
		line += fmt.Sprintf(" (host: %s)", entry.Host)
	}
	if entry.Duration > 0 {
		line += fmt.Sprintf(" (duration: %v)", entry.Duration)
	}
	
	_, err := fmt.Fprintln(c.writer, line)
	return err
}

func (c *ConsoleLogOutput) Close() error {
	return nil
}

func (c *ConsoleLogOutput) colorizeLevel(level string) string {
	if !c.colors {
		return level
	}
	
	const (
		colorReset  = "\033[0m"
		colorRed    = "\033[31m"
		colorYellow = "\033[33m"
		colorBlue   = "\033[34m"
		colorGray   = "\033[37m"
	)
	
	switch level {
	case "ERROR", "FATAL":
		return colorRed + level + colorReset
	case "WARN":
		return colorYellow + level + colorReset
	case "INFO":
		return colorBlue + level + colorReset
	case "DEBUG":
		return colorGray + level + colorReset
	default:
		return level
	}
}

// MemoryLogOutput methods
func (m *MemoryLogOutput) Write(entry LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.entries = append(m.entries, entry)
	
	// Keep only the last maxSize entries
	if len(m.entries) > m.maxSize {
		m.entries = m.entries[1:]
	}
	
	return nil
}

func (m *MemoryLogOutput) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = nil
	return nil
}

func (m *MemoryLogOutput) GetEntries() []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	entries := make([]LogEntry, len(m.entries))
	copy(entries, m.entries)
	return entries
}

func (m *MemoryLogOutput) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = m.entries[:0]
}

// StreamingLoggerAdapter adapts gosinble streaming to logging
type StreamingLoggerAdapter struct {
	logger   *StreamLogger
	taskName string
	host     string
}

// NewStreamingLoggerAdapter creates a new adapter
func NewStreamingLoggerAdapter(logger *StreamLogger, taskName, host string) *StreamingLoggerAdapter {
	return &StreamingLoggerAdapter{
		logger:   logger,
		taskName: taskName,
		host:     host,
	}
}

// HandleStreamEvents processes a channel of stream events and logs them
func (a *StreamingLoggerAdapter) HandleStreamEvents(ctx context.Context, events <-chan types.StreamEvent) {
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			a.logger.LogStreamEvent(event, a.taskName, a.host)

		case <-ctx.Done():
			return
		}
	}
}

// CreateProgressCallback creates a progress callback that logs to the logger
func (a *StreamingLoggerAdapter) CreateProgressCallback() func(progress types.ProgressInfo) {
	return func(progress types.ProgressInfo) {
		a.logger.LogProgress(progress, a.taskName, a.host)
	}
}