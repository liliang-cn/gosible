// Real-Time Output Implementation for Gosinble
// This demonstrates how to add streaming output capability

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/gosinble/gosinble/pkg/types"
)

// Extended ExecuteOptions with streaming support
type StreamingExecuteOptions struct {
	types.ExecuteOptions
	
	// NEW: Real-time streaming options
	StreamOutput     bool                              // Enable streaming
	OutputCallback   func(line string, isStderr bool) // Real-time callback  
	ProgressCallback func(progress ProgressInfo)       // Progress updates
}

// Progress information structure
type ProgressInfo struct {
	Stage       string    // "connecting", "executing", "transferring"
	Percentage  float64   // 0-100
	Message     string    // Current operation
	BytesTotal  int64     // For file transfers
	BytesDone   int64     // For file transfers
	Timestamp   time.Time
}

// Stream event types
type StreamEventType string

const (
	StreamStdout   StreamEventType = "stdout"
	StreamStderr   StreamEventType = "stderr"
	StreamProgress StreamEventType = "progress"
	StreamDone     StreamEventType = "done" 
	StreamError    StreamEventType = "error"
)

// Stream event structure
type StreamEvent struct {
	Type      StreamEventType
	Data      string
	Progress  *ProgressInfo
	Result    *types.Result
	Error     error
	Timestamp time.Time
}

// Enhanced connection interface
type StreamingConnection interface {
	types.Connection
	
	// NEW: Streaming execution method
	ExecuteStream(ctx context.Context, command string, options StreamingExecuteOptions) (<-chan StreamEvent, error)
}

// Implementation for LocalConnection with streaming
type StreamingLocalConnection struct {
	connected bool
	info      types.ConnectionInfo
}

func NewStreamingLocalConnection() *StreamingLocalConnection {
	return &StreamingLocalConnection{}
}

func (c *StreamingLocalConnection) Connect(ctx context.Context, info types.ConnectionInfo) error {
	c.info = info
	c.connected = true
	return nil
}

func (c *StreamingLocalConnection) Close() error {
	c.connected = false
	return nil
}

func (c *StreamingLocalConnection) IsConnected() bool {
	return c.connected
}

// Standard Execute method (compatibility)
func (c *StreamingLocalConnection) Execute(ctx context.Context, command string, options types.ExecuteOptions) (*types.Result, error) {
	// Convert to streaming options and collect all output
	streamOpts := StreamingExecuteOptions{
		ExecuteOptions: options,
		StreamOutput:   false,
	}
	
	events, err := c.ExecuteStream(ctx, command, streamOpts)
	if err != nil {
		return nil, err
	}
	
	// Collect all events and return final result
	for event := range events {
		if event.Type == StreamDone {
			return event.Result, nil
		} else if event.Type == StreamError {
			return nil, event.Error
		}
	}
	
	return nil, fmt.Errorf("no result received")
}

// NEW: Streaming execution method
func (c *StreamingLocalConnection) ExecuteStream(ctx context.Context, command string, options StreamingExecuteOptions) (<-chan StreamEvent, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}
	
	eventChan := make(chan StreamEvent, 100) // Buffered channel
	
	go func() {
		defer close(eventChan)
		
		startTime := time.Now()
		
		// Send progress update
		if options.ProgressCallback != nil || options.StreamOutput {
			progress := ProgressInfo{
				Stage:     "executing",
				Message:   fmt.Sprintf("Starting command: %s", command),
				Timestamp: time.Now(),
			}
			
			if options.ProgressCallback != nil {
				options.ProgressCallback(progress)
			}
			
			eventChan <- StreamEvent{
				Type:      StreamProgress,
				Progress:  &progress,
				Timestamp: time.Now(),
			}
		}
		
		// Create command
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		
		// Set up pipes for real-time output
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			eventChan <- StreamEvent{
				Type:      StreamError,
				Error:     fmt.Errorf("failed to create stdout pipe: %v", err),
				Timestamp: time.Now(),
			}
			return
		}
		
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			eventChan <- StreamEvent{
				Type:      StreamError,
				Error:     fmt.Errorf("failed to create stderr pipe: %v", err),
				Timestamp: time.Now(),
			}
			return
		}
		
		// Start command
		if err := cmd.Start(); err != nil {
			eventChan <- StreamEvent{
				Type:      StreamError,
				Error:     fmt.Errorf("failed to start command: %v", err),
				Timestamp: time.Now(),
			}
			return
		}
		
		// Set up goroutines to read output streams
		var wg sync.WaitGroup
		var stdout, stderr string
		
		// Read stdout
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				line := scanner.Text()
				stdout += line + "\n"
				
				// Send real-time output
				if options.StreamOutput {
					if options.OutputCallback != nil {
						options.OutputCallback(line, false)
					}
					
					eventChan <- StreamEvent{
						Type:      StreamStdout,
						Data:      line,
						Timestamp: time.Now(),
					}
				}
			}
		}()
		
		// Read stderr  
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				line := scanner.Text()
				stderr += line + "\n"
				
				// Send real-time output
				if options.StreamOutput {
					if options.OutputCallback != nil {
						options.OutputCallback(line, true)
					}
					
					eventChan <- StreamEvent{
						Type:      StreamStderr,
						Data:      line,
						Timestamp: time.Now(),
					}
				}
			}
		}()
		
		// Wait for command completion
		err = cmd.Wait()
		wg.Wait() // Wait for output readers to finish
		
		endTime := time.Now()
		
		// Create result
		result := &types.Result{
			Host:       "localhost",
			Success:    err == nil,
			Changed:    true, // Assume changed for demo
			Message:    "Command executed",
			Data: map[string]interface{}{
				"stdout": stdout,
				"stderr": stderr,
			},
			StartTime:  startTime,
			EndTime:    endTime,
			Duration:   endTime.Sub(startTime),
			ModuleName: "streaming_command",
		}
		
		if err != nil {
			result.Error = err
			result.Message = fmt.Sprintf("Command failed: %v", err)
		}
		
		// Send final progress
		if options.ProgressCallback != nil || options.StreamOutput {
			progress := ProgressInfo{
				Stage:      "completed",
				Percentage: 100.0,
				Message:    "Command completed",
				Timestamp:  time.Now(),
			}
			
			if options.ProgressCallback != nil {
				options.ProgressCallback(progress)
			}
			
			eventChan <- StreamEvent{
				Type:      StreamProgress,
				Progress:  &progress,
				Timestamp: time.Now(),
			}
		}
		
		// Send final result
		eventChan <- StreamEvent{
			Type:      StreamDone,
			Result:    result,
			Timestamp: time.Now(),
		}
	}()
	
	return eventChan, nil
}

// Implement remaining Connection interface methods (stubs)
func (c *StreamingLocalConnection) Copy(ctx context.Context, src io.Reader, dest string, mode int) error {
	return fmt.Errorf("not implemented")
}

func (c *StreamingLocalConnection) Fetch(ctx context.Context, src string) (io.Reader, error) {
	return nil, fmt.Errorf("not implemented")
}

// Usage example
func main() {
	fmt.Println("🚀 Real-Time Output Streaming Example")
	
	conn := NewStreamingLocalConnection()
	ctx := context.Background()
	
	// Connect
	if err := conn.Connect(ctx, types.ConnectionInfo{}); err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	
	// Example 1: Long-running command with real-time output
	fmt.Println("\n📡 Example 1: Real-time streaming")
	
	options := StreamingExecuteOptions{
		ExecuteOptions: types.ExecuteOptions{
			Timeout: 30 * time.Second,
		},
		StreamOutput: true,
		OutputCallback: func(line string, isStderr bool) {
			if isStderr {
				fmt.Printf("🔴 [STDERR] %s\n", line)
			} else {
				fmt.Printf("🟢 [STDOUT] %s\n", line)
			}
		},
		ProgressCallback: func(progress ProgressInfo) {
			fmt.Printf("📊 [PROGRESS] %.1f%% - %s (%s)\n", 
				progress.Percentage, progress.Message, progress.Stage)
		},
	}
	
	// Run a command that produces incremental output
	events, err := conn.ExecuteStream(ctx, "for i in {1..5}; do echo \"Processing item $i\"; sleep 1; done", options)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	
	fmt.Println("⏳ Streaming output:")
	for event := range events {
		switch event.Type {
		case StreamStdout:
			fmt.Printf("   📤 %s\n", event.Data)
		case StreamStderr:
			fmt.Printf("   ❌ %s\n", event.Data)
		case StreamProgress:
			fmt.Printf("   📈 %s\n", event.Progress.Message)
		case StreamDone:
			fmt.Printf("   ✅ Command completed successfully!\n")
			fmt.Printf("   ⏱️  Duration: %v\n", event.Result.Duration)
		case StreamError:
			fmt.Printf("   💥 Error: %v\n", event.Error)
		}
	}
	
	// Example 2: File copy with progress
	fmt.Println("\n📁 Example 2: File operation with progress")
	
	copyOptions := StreamingExecuteOptions{
		ExecuteOptions: types.ExecuteOptions{
			Timeout: 60 * time.Second,
		},
		StreamOutput: true,
		ProgressCallback: func(progress ProgressInfo) {
			if progress.BytesTotal > 0 {
				fmt.Printf("📊 Copying: %.1f%% (%d/%d bytes)\n",
					progress.Percentage, progress.BytesDone, progress.BytesTotal)
			} else {
				fmt.Printf("📊 %s\n", progress.Message)
			}
		},
	}
	
	events, err = conn.ExecuteStream(ctx, "cp /dev/urandom /tmp/testfile & sleep 2; pkill dd; echo 'Copy operation simulated'", copyOptions)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	
	for event := range events {
		if event.Type == StreamDone {
			fmt.Printf("✅ File operation completed\n")
			break
		}
	}
	
	fmt.Println("\n🎯 Key Benefits:")
	fmt.Println("- ✅ Real-time output streaming")
	fmt.Println("- ✅ Progress callbacks for long operations")
	fmt.Println("- ✅ Non-blocking execution")
	fmt.Println("- ✅ Better user experience")
	fmt.Println("- ✅ Compatible with existing interfaces")
}

// Example module using streaming output
type StreamingShellModule struct {
	name string
}

func NewStreamingShellModule() *StreamingShellModule {
	return &StreamingShellModule{name: "streaming_shell"}
}

func (m *StreamingShellModule) Name() string { return m.name }

func (m *StreamingShellModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	cmd := args["cmd"].(string)
	
	// Check if connection supports streaming
	if streamConn, ok := conn.(StreamingConnection); ok {
		fmt.Println("🚀 Using streaming execution...")
		
		options := StreamingExecuteOptions{
			ExecuteOptions: types.ExecuteOptions{
				Timeout: 300 * time.Second,
			},
			StreamOutput: true,
			OutputCallback: func(line string, isStderr bool) {
				if isStderr {
					fmt.Printf("[%s] STDERR: %s\n", m.name, line)
				} else {
					fmt.Printf("[%s] STDOUT: %s\n", m.name, line)
				}
			},
			ProgressCallback: func(progress ProgressInfo) {
				fmt.Printf("[%s] Progress: %s\n", m.name, progress.Message)
			},
		}
		
		events, err := streamConn.ExecuteStream(ctx, cmd, options)
		if err != nil {
			return nil, err
		}
		
		// Process events and return final result
		for event := range events {
			if event.Type == StreamDone {
				return event.Result, nil
			} else if event.Type == StreamError {
				return nil, event.Error
			}
		}
	}
	
	// Fallback to standard execution
	fmt.Println("📡 Using standard execution...")
	return conn.Execute(ctx, cmd, types.ExecuteOptions{})
}

func (m *StreamingShellModule) Validate(args map[string]interface{}) error {
	if _, exists := args["cmd"]; !exists {
		return fmt.Errorf("cmd parameter is required")
	}
	return nil
}

func (m *StreamingShellModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "streaming_shell",
		Description: "Execute shell commands with real-time output streaming",
		Parameters: map[string]types.ParamDoc{
			"cmd": {
				Description: "Command to execute",
				Required:    true,
				Type:        "string",
			},
		},
		Examples: []string{
			"- name: Long running command\n  streaming_shell:\n    cmd: 'make install'",
		},
	}
}