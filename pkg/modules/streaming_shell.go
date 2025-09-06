package modules

import (
	"context"
	"fmt"
	"time"

	"github.com/gosinble/gosinble/pkg/types"
)

// StreamingShellModule implements shell command execution with real-time streaming
type StreamingShellModule struct {
	BaseModule
}

// NewStreamingShellModule creates a new streaming shell module instance
func NewStreamingShellModule() *StreamingShellModule {
	return &StreamingShellModule{
		BaseModule: BaseModule{
			name: "streaming_shell",
		},
	}
}

// Run executes the streaming shell module
func (m *StreamingShellModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	cmd := m.GetStringArg(args, "cmd", "")
	if cmd == "" {
		return nil, fmt.Errorf("streaming_shell: cmd parameter cannot be empty")
	}

	// Get streaming options
	streamOutput := m.GetBoolArg(args, "stream_output", true)
	showProgress := m.GetBoolArg(args, "show_progress", true)
	timeoutInt, err := m.GetIntArg(args, "timeout", 300)
	if err != nil {
		return nil, fmt.Errorf("streaming_shell: invalid timeout value: %v", err)
	}
	timeout := time.Duration(timeoutInt) * time.Second

	// Check if connection supports streaming
	if streamConn, ok := conn.(types.StreamingConnection); ok && streamOutput {
		return m.executeWithStreaming(ctx, streamConn, cmd, timeout, showProgress)
	}

	// Fallback to standard execution
	return m.executeStandard(ctx, conn, cmd, timeout)
}

// executeWithStreaming uses the streaming execution method
func (m *StreamingShellModule) executeWithStreaming(ctx context.Context, conn types.StreamingConnection, cmd string, timeout time.Duration, showProgress bool) (*types.Result, error) {
	var outputLines []string
	var errorLines []string
	progressCount := 0

	options := types.ExecuteOptions{
		StreamOutput: true,
		Timeout:      timeout,
		OutputCallback: func(line string, isStderr bool) {
			if isStderr {
				errorLines = append(errorLines, line)
			} else {
				outputLines = append(outputLines, line)
			}
		},
		ProgressCallback: func(progress types.ProgressInfo) {
			progressCount++
			if showProgress {
				m.LogInfo("Progress: %s - %s", progress.Stage, progress.Message)
			}
		},
	}

	events, err := conn.ExecuteStream(ctx, cmd, options)
	if err != nil {
		return nil, fmt.Errorf("streaming_shell: failed to start streaming execution: %v", err)
	}

	// Process events
	var finalResult *types.Result
	streamEventCount := 0
	
	for event := range events {
		streamEventCount++
		
		switch event.Type {
		case types.StreamStdout:
			m.LogInfo("STDOUT: %s", event.Data)
		case types.StreamStderr:
			m.LogWarn("STDERR: %s", event.Data)
		case types.StreamProgress:
			if event.Progress != nil {
				m.LogInfo("Progress: %.1f%% - %s", event.Progress.Percentage, event.Progress.Message)
			}
		case types.StreamDone:
			finalResult = event.Result
		case types.StreamError:
			return nil, fmt.Errorf("streaming_shell: streaming execution failed: %v", event.Error)
		}
	}

	if finalResult == nil {
		return nil, fmt.Errorf("streaming_shell: no final result received from streaming execution")
	}

	// Enhance result with streaming metadata
	if finalResult.Data == nil {
		finalResult.Data = make(map[string]interface{})
	}
	finalResult.Data["streaming_enabled"] = true
	finalResult.Data["stream_events_received"] = streamEventCount
	finalResult.Data["progress_updates"] = progressCount
	finalResult.Data["output_lines"] = len(outputLines)
	finalResult.Data["error_lines"] = len(errorLines)

	return finalResult, nil
}

// executeStandard uses the standard execution method as fallback
func (m *StreamingShellModule) executeStandard(ctx context.Context, conn types.Connection, cmd string, timeout time.Duration) (*types.Result, error) {
	options := types.ExecuteOptions{
		Timeout: timeout,
	}

	result, err := conn.Execute(ctx, cmd, options)
	if err != nil {
		return nil, fmt.Errorf("streaming_shell: standard execution failed: %v", err)
	}

	// Add metadata to indicate non-streaming execution
	if result.Data == nil {
		result.Data = make(map[string]interface{})
	}
	result.Data["streaming_enabled"] = false
	result.Data["execution_mode"] = "standard"

	return result, nil
}

// Validate checks if the module arguments are valid
func (m *StreamingShellModule) Validate(args map[string]interface{}) error {
	// Required fields
	required := []string{"cmd"}
	if err := m.ValidateRequired(args, required); err != nil {
		return err
	}

	// Validate timeout if provided
	if _, exists := args["timeout"]; exists {
		if timeoutInt, err := m.GetIntArg(args, "timeout", 300); err != nil {
			return fmt.Errorf("streaming_shell: invalid timeout value: %v", err)
		} else if timeoutInt <= 0 {
			return fmt.Errorf("streaming_shell: timeout must be positive, got %d", timeoutInt)
		}
	}

	return nil
}

// Documentation returns the module documentation
func (m *StreamingShellModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "streaming_shell",
		Description: "Execute shell commands with real-time output streaming support",
		Parameters: map[string]types.ParamDoc{
			"cmd": {
				Description: "Shell command to execute",
				Required:    true,
				Type:        "string",
			},
			"stream_output": {
				Description: "Enable real-time output streaming",
				Required:    false,
				Type:        "boolean",
				Default:     "true",
			},
			"show_progress": {
				Description: "Show progress updates during execution",
				Required:    false,
				Type:        "boolean",
				Default:     "true",
			},
			"timeout": {
				Description: "Command timeout in seconds",
				Required:    false,
				Type:        "integer",
				Default:     "300",
			},
		},
		Examples: []string{
			"- name: Run long command with streaming\n  streaming_shell:\n    cmd: 'make install'",
			"- name: Build with progress\n  streaming_shell:\n    cmd: 'npm run build'\n    timeout: 600",
			"- name: Deploy with streaming disabled\n  streaming_shell:\n    cmd: 'docker deploy'\n    stream_output: false",
		},
		Returns: map[string]string{
			"streaming_enabled":      "Whether streaming was used",
			"stream_events_received": "Number of stream events processed",
			"progress_updates":       "Number of progress updates received",
			"output_lines":          "Number of output lines captured",
			"error_lines":           "Number of error lines captured",
		},
	}
}