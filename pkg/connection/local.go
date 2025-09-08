package connection

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// LocalConnection implements the Connection interface for local execution
type LocalConnection struct {
	connected bool
	info      types.ConnectionInfo
}

// NewLocalConnection creates a new local connection
func NewLocalConnection() *LocalConnection {
	return &LocalConnection{}
}

// Connect establishes a local connection (always succeeds)
func (c *LocalConnection) Connect(ctx context.Context, info types.ConnectionInfo) error {
	c.info = info
	c.connected = true
	return nil
}

// Execute runs a command locally
func (c *LocalConnection) Execute(ctx context.Context, command string, options types.ExecuteOptions) (*types.Result, error) {
	if !c.connected {
		return nil, types.NewConnectionError("local", "not connected", nil)
	}

	startTime := time.Now()
	result := &types.Result{
		StartTime:  startTime,
		Host:       "localhost",
		ModuleName: "command",
	}

	// Create command with timeout context
	cmdCtx := ctx
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	var cmd *exec.Cmd
	if options.Sudo && options.User != "" {
		// Use sudo to run as different user
		cmd = exec.CommandContext(cmdCtx, "sudo", "-u", options.User, "sh", "-c", command)
	} else if options.User != "" {
		// Use su to run as different user
		cmd = exec.CommandContext(cmdCtx, "su", "-c", command, options.User)
	} else {
		// Run as current user
		cmd = exec.CommandContext(cmdCtx, "sh", "-c", command)
	}

	// Set working directory
	if options.WorkingDir != "" {
		cmd.Dir = options.WorkingDir
	}

	// Set environment variables
	if options.Env != nil {
		env := os.Environ()
		for k, v := range options.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	// Execute command
	output, err := cmd.CombinedOutput()
	endTime := time.Now()

	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)
	result.Data = map[string]interface{}{
		"stdout": string(output),
		"stderr": "",
		"cmd":    command,
	}

	if err != nil {
		result.Success = false
		result.Error = err
		result.Message = fmt.Sprintf("command failed: %v", err)

		// Check for exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				result.Data["exit_code"] = status.ExitStatus()
			}
		}
	} else {
		result.Success = true
		result.Message = "command executed successfully"
		result.Data["exit_code"] = 0
	}

	// Determine if anything changed (for idempotency)
	// This is a simple heuristic - in practice, modules would implement more sophisticated logic
	result.Changed = result.Success

	return result, nil
}

// ExecuteStream runs a command locally with real-time output streaming
func (c *LocalConnection) ExecuteStream(ctx context.Context, command string, options types.ExecuteOptions) (<-chan types.StreamEvent, error) {
	if !c.connected {
		return nil, types.NewConnectionError("local", "not connected", nil)
	}

	eventChan := make(chan types.StreamEvent, 100) // Buffered channel for events

	go func() {
		defer close(eventChan)

		startTime := time.Now()

		// Send initial progress
		if options.StreamOutput || options.ProgressCallback != nil {
			progress := types.ProgressInfo{
				Stage:     "executing",
				Message:   fmt.Sprintf("Starting command: %s", command),
				Timestamp: time.Now(),
			}

			if options.ProgressCallback != nil {
				options.ProgressCallback(progress)
			}

			if options.StreamOutput {
				eventChan <- types.StreamEvent{
					Type:      types.StreamProgress,
					Progress:  &progress,
					Timestamp: time.Now(),
				}
			}
		}

		// Create command with timeout context
		cmdCtx := ctx
		if options.Timeout > 0 {
			var cancel context.CancelFunc
			cmdCtx, cancel = context.WithTimeout(ctx, options.Timeout)
			defer cancel()
		}

		var cmd *exec.Cmd
		if options.Sudo && options.User != "" {
			// Use sudo to run as different user
			cmd = exec.CommandContext(cmdCtx, "sudo", "-u", options.User, "sh", "-c", command)
		} else if options.User != "" {
			// Use su to run as different user
			cmd = exec.CommandContext(cmdCtx, "su", "-c", command, options.User)
		} else {
			// Run as current user
			cmd = exec.CommandContext(cmdCtx, "sh", "-c", command)
		}

		// Set working directory
		if options.WorkingDir != "" {
			cmd.Dir = options.WorkingDir
		}

		// Set environment variables
		if options.Env != nil {
			env := os.Environ()
			for k, v := range options.Env {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
			cmd.Env = env
		}

		// Set up pipes for real-time output
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			eventChan <- types.StreamEvent{
				Type:      types.StreamError,
				Error:     types.NewConnectionError("local", "failed to create stdout pipe", err),
				Timestamp: time.Now(),
			}
			return
		}

		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			eventChan <- types.StreamEvent{
				Type:      types.StreamError,
				Error:     types.NewConnectionError("local", "failed to create stderr pipe", err),
				Timestamp: time.Now(),
			}
			return
		}

		// Start command
		if err := cmd.Start(); err != nil {
			eventChan <- types.StreamEvent{
				Type:      types.StreamError,
				Error:     types.NewConnectionError("local", "failed to start command", err),
				Timestamp: time.Now(),
			}
			return
		}

		// Set up goroutines to read output streams
		var wg sync.WaitGroup
		var stdout, stderr strings.Builder

		// Read stdout
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				line := scanner.Text()
				stdout.WriteString(line + "\n")

				// Send real-time output
				if options.StreamOutput {
					if options.OutputCallback != nil {
						options.OutputCallback(line, false)
					}

					eventChan <- types.StreamEvent{
						Type:      types.StreamStdout,
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
				stderr.WriteString(line + "\n")

				// Send real-time output
				if options.StreamOutput {
					if options.OutputCallback != nil {
						options.OutputCallback(line, true)
					}

					eventChan <- types.StreamEvent{
						Type:      types.StreamStderr,
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
			Changed:    true, // Assume changed for commands
			Message:    "Command executed",
			StartTime:  startTime,
			EndTime:    endTime,
			Duration:   endTime.Sub(startTime),
			ModuleName: "streaming_command",
			Data: map[string]interface{}{
				"stdout": stdout.String(),
				"stderr": stderr.String(),
				"cmd":    command,
			},
		}

		if err != nil {
			result.Success = false
			result.Error = err
			result.Message = fmt.Sprintf("Command failed: %v", err)

			// Check for exit code
			if exitError, ok := err.(*exec.ExitError); ok {
				if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
					result.Data["exit_code"] = status.ExitStatus()
				}
			}
		} else {
			result.Success = true
			result.Message = "Command executed successfully"
			result.Data["exit_code"] = 0
		}

		// Send final progress
		if options.StreamOutput || options.ProgressCallback != nil {
			progress := types.ProgressInfo{
				Stage:      "completed",
				Percentage: 100.0,
				Message:    "Command completed",
				Timestamp:  time.Now(),
			}

			if options.ProgressCallback != nil {
				options.ProgressCallback(progress)
			}

			if options.StreamOutput {
				eventChan <- types.StreamEvent{
					Type:      types.StreamProgress,
					Progress:  &progress,
					Timestamp: time.Now(),
				}
			}
		}

		// Send final result
		eventChan <- types.StreamEvent{
			Type:      types.StreamDone,
			Result:    result,
			Timestamp: time.Now(),
		}
	}()

	return eventChan, nil
}

// Copy transfers a file locally
func (c *LocalConnection) Copy(ctx context.Context, src io.Reader, dest string, mode int) error {
	if !c.connected {
		return types.NewConnectionError("local", "not connected", nil)
	}

	// Sanitize destination path
	dest = types.SanitizePath(dest)

	// Create destination file
	file, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return types.NewConnectionError("local", fmt.Sprintf("failed to create destination file %s", dest), err)
	}
	defer file.Close()

	// Copy data
	_, err = io.Copy(file, src)
	if err != nil {
		return types.NewConnectionError("local", fmt.Sprintf("failed to copy data to %s", dest), err)
	}

	return nil
}

// CopyWithProgress transfers a file locally with progress tracking
func (c *LocalConnection) CopyWithProgress(ctx context.Context, src io.Reader, dest string, mode int, totalSize int64, progressCallback func(progress types.ProgressInfo)) error {
	if !c.connected {
		return types.NewConnectionError("local", "not connected", nil)
	}

	// Sanitize destination path
	dest = types.SanitizePath(dest)

	// Send initial progress
	if progressCallback != nil {
		progressCallback(types.ProgressInfo{
			Stage:      "transferring",
			Percentage: 0.0,
			Message:    fmt.Sprintf("Starting file transfer to %s", dest),
			BytesTotal: totalSize,
			BytesDone:  0,
			Timestamp:  time.Now(),
		})
	}

	// Create destination file
	file, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return types.NewConnectionError("local", fmt.Sprintf("failed to create destination file %s", dest), err)
	}
	defer file.Close()

	// Create progress reader if we have a total size
	var reader io.Reader = src
	if totalSize > 0 && progressCallback != nil {
		reader = &progressReader{
			reader:           src,
			totalSize:        totalSize,
			progressCallback: progressCallback,
			dest:             dest,
		}
	}

	// Copy data with progress tracking
	bytesWritten, err := io.Copy(file, reader)
	if err != nil {
		if progressCallback != nil {
			progressCallback(types.ProgressInfo{
				Stage:      "transferring",
				Percentage: 0.0,
				Message:    fmt.Sprintf("File transfer failed: %v", err),
				BytesTotal: totalSize,
				BytesDone:  bytesWritten,
				Timestamp:  time.Now(),
			})
		}
		return types.NewConnectionError("local", fmt.Sprintf("failed to copy data to %s", dest), err)
	}

	// Send completion progress
	if progressCallback != nil {
		progressCallback(types.ProgressInfo{
			Stage:      "transferring",
			Percentage: 100.0,
			Message:    fmt.Sprintf("File transfer completed to %s", dest),
			BytesTotal: totalSize,
			BytesDone:  bytesWritten,
			Timestamp:  time.Now(),
		})
	}

	return nil
}

// progressReader wraps an io.Reader to provide transfer progress updates
type progressReader struct {
	reader           io.Reader
	totalSize        int64
	bytesRead        int64
	progressCallback func(progress types.ProgressInfo)
	dest             string
	lastUpdate       time.Time
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.bytesRead += int64(n)
		
		// Update progress every 100ms or on completion
		now := time.Now()
		if now.Sub(pr.lastUpdate) > 100*time.Millisecond || err == io.EOF {
			pr.lastUpdate = now
			
			percentage := float64(pr.bytesRead) / float64(pr.totalSize) * 100
			if percentage > 100 {
				percentage = 100
			}
			
			pr.progressCallback(types.ProgressInfo{
				Stage:      "transferring",
				Percentage: percentage,
				Message:    fmt.Sprintf("Transferring to %s: %s/%s", pr.dest, formatBytes(pr.bytesRead), formatBytes(pr.totalSize)),
				BytesTotal: pr.totalSize,
				BytesDone:  pr.bytesRead,
				Timestamp:  now,
			})
		}
	}
	return n, err
}

// formatBytes formats byte counts in human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Fetch retrieves a file locally
func (c *LocalConnection) Fetch(ctx context.Context, src string) (io.Reader, error) {
	if !c.connected {
		return nil, types.NewConnectionError("local", "not connected", nil)
	}

	// Sanitize source path
	src = types.SanitizePath(src)

	file, err := os.Open(src)
	if err != nil {
		return nil, types.NewConnectionError("local", fmt.Sprintf("failed to open source file %s", src), err)
	}

	return file, nil
}

// Close terminates the connection
func (c *LocalConnection) Close() error {
	c.connected = false
	return nil
}

// IsConnected returns true if the connection is active
func (c *LocalConnection) IsConnected() bool {
	return c.connected
}

// ExecuteScript executes a script file locally
func (c *LocalConnection) ExecuteScript(ctx context.Context, script string, options types.ExecuteOptions) (*types.Result, error) {
	// Create temporary script file
	tempFile, err := os.CreateTemp("", "gosinble-script-*.sh")
	if err != nil {
		return nil, types.NewConnectionError("local", "failed to create temp script file", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write script content
	if _, err := tempFile.WriteString(script); err != nil {
		return nil, types.NewConnectionError("local", "failed to write script content", err)
	}

	// Make script executable
	if err := tempFile.Chmod(0755); err != nil {
		return nil, types.NewConnectionError("local", "failed to make script executable", err)
	}

	// Close file before execution
	tempFile.Close()

	// Execute script
	return c.Execute(ctx, tempFile.Name(), options)
}

// GetUser returns the current user information
func (c *LocalConnection) GetUser() (*user.User, error) {
	if !c.connected {
		return nil, types.NewConnectionError("local", "not connected", nil)
	}

	return user.Current()
}

// GetWorkingDirectory returns the current working directory
func (c *LocalConnection) GetWorkingDirectory() (string, error) {
	if !c.connected {
		return "", types.NewConnectionError("local", "not connected", nil)
	}

	return os.Getwd()
}

// FileExists checks if a file exists locally
func (c *LocalConnection) FileExists(path string) (bool, error) {
	if !c.connected {
		return false, types.NewConnectionError("local", "not connected", nil)
	}

	path = types.SanitizePath(path)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// GetFileInfo returns information about a file
func (c *LocalConnection) GetFileInfo(path string) (os.FileInfo, error) {
	if !c.connected {
		return nil, types.NewConnectionError("local", "not connected", nil)
	}

	path = types.SanitizePath(path)
	return os.Stat(path)
}

// CreateDirectory creates a directory locally
func (c *LocalConnection) CreateDirectory(path string, mode os.FileMode) error {
	if !c.connected {
		return types.NewConnectionError("local", "not connected", nil)
	}

	path = types.SanitizePath(path)
	return os.MkdirAll(path, mode)
}

// RemoveFile removes a file locally
func (c *LocalConnection) RemoveFile(path string) error {
	if !c.connected {
		return types.NewConnectionError("local", "not connected", nil)
	}

	path = types.SanitizePath(path)
	return os.Remove(path)
}

// ListDirectory lists the contents of a directory
func (c *LocalConnection) ListDirectory(path string) ([]os.FileInfo, error) {
	if !c.connected {
		return nil, types.NewConnectionError("local", "not connected", nil)
	}

	path = types.SanitizePath(path)
	dir, err := os.Open(path)
	if err != nil {
		return nil, types.NewConnectionError("local", fmt.Sprintf("failed to open directory %s", path), err)
	}
	defer dir.Close()

	return dir.Readdir(-1)
}

// GetEnvironmentVariable returns the value of an environment variable
func (c *LocalConnection) GetEnvironmentVariable(name string) string {
	return os.Getenv(name)
}

// SetEnvironmentVariable sets an environment variable (only for the duration of the connection)
func (c *LocalConnection) SetEnvironmentVariable(name, value string) error {
	return os.Setenv(name, value)
}

// GetHostname returns the local hostname
func (c *LocalConnection) GetHostname() (string, error) {
	if !c.connected {
		return "", types.NewConnectionError("local", "not connected", nil)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return "", types.NewConnectionError("local", "failed to get hostname", err)
	}

	return hostname, nil
}

// GetSystemInfo returns basic system information
func (c *LocalConnection) GetSystemInfo() (map[string]interface{}, error) {
	if !c.connected {
		return nil, types.NewConnectionError("local", "not connected", nil)
	}

	info := make(map[string]interface{})

	// Get hostname
	if hostname, err := c.GetHostname(); err == nil {
		info["hostname"] = hostname
	}

	// Get current user
	if currentUser, err := c.GetUser(); err == nil {
		info["user"] = currentUser.Username
		info["uid"] = currentUser.Uid
		info["gid"] = currentUser.Gid
		info["home"] = currentUser.HomeDir
	}

	// Get working directory
	if wd, err := c.GetWorkingDirectory(); err == nil {
		info["working_directory"] = wd
	}

	// Get some environment variables
	info["path"] = c.GetEnvironmentVariable("PATH")
	info["shell"] = c.GetEnvironmentVariable("SHELL")
	info["term"] = c.GetEnvironmentVariable("TERM")

	return info, nil
}

// TestConnection tests if the local connection is working
func (c *LocalConnection) TestConnection() error {
	if !c.connected {
		return types.NewConnectionError("local", "not connected", nil)
	}

	// Try to execute a simple command
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := c.Execute(ctx, "echo 'connection test'", types.ExecuteOptions{})
	if err != nil {
		return types.NewConnectionError("local", "connection test failed", err)
	}

	if !result.Success {
		return types.NewConnectionError("local", "connection test command failed", result.Error)
	}

	stdout, ok := result.Data["stdout"].(string)
	if !ok || !strings.Contains(stdout, "connection test") {
		return types.NewConnectionError("local", "unexpected test command output", nil)
	}

	return nil
}