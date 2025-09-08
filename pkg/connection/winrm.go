package connection

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/masterzen/winrm"
	"github.com/liliang-cn/gosinble/pkg/types"
)

// WinRMConnection implements the Connection interface for Windows Remote Management
type WinRMConnection struct {
	client    *winrm.Client
	connected bool
	info      types.ConnectionInfo
}

// NewWinRMConnection creates a new WinRM connection
func NewWinRMConnection() *WinRMConnection {
	return &WinRMConnection{}
}

// Connect establishes a WinRM connection to the target Windows host
func (c *WinRMConnection) Connect(ctx context.Context, info types.ConnectionInfo) error {
	c.info = info

	// Set default port if not specified
	port := info.Port
	if port == 0 {
		if info.UseSSL {
			port = 5986 // HTTPS
		} else {
			port = 5985 // HTTP
		}
	}

	// Create endpoint
	endpoint := winrm.NewEndpoint(info.Host, port, info.UseSSL, info.SkipVerify, nil, nil, nil, 0)

	// Set authentication
	params := winrm.DefaultParameters
	if info.User != "" && info.Password != "" {
		params.TransportDecorator = func() winrm.Transporter {
			return &winrm.ClientNTLM{}
		}
	}

	// Create client
	client, err := winrm.NewClientWithParameters(endpoint, info.User, info.Password, params)
	if err != nil {
		return types.NewConnectionError(info.Host, "failed to create WinRM client", err)
	}

	c.client = client
	c.connected = true

	// Test connection
	if err := c.testConnection(ctx); err != nil {
		c.Close()
		return types.NewConnectionError(info.Host, "connection test failed", err)
	}

	return nil
}

// Execute runs a command on the remote Windows host via WinRM
func (c *WinRMConnection) Execute(ctx context.Context, command string, options types.ExecuteOptions) (*types.Result, error) {
	if !c.connected {
		return nil, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	startTime := time.Now()
	result := &types.Result{
		StartTime:  startTime,
		Host:       c.info.Host,
		ModuleName: "command",
	}

	// Build command with options
	fullCommand := c.buildCommand(command, options)

	// Create shell
	shell, err := c.client.CreateShell()
	if err != nil {
		return nil, types.NewConnectionError(c.info.Host, "failed to create WinRM shell", err)
	}
	defer shell.Close()

	// Execute command
	var stdout, stderr bytes.Buffer
	cmd, err := shell.ExecuteWithContext(ctx, fullCommand)
	var exitCode int
	if err == nil {
		// Copy output from command
		io.Copy(&stdout, cmd.Stdout)
		io.Copy(&stderr, cmd.Stderr)
		cmd.Wait()
		exitCode = cmd.ExitCode()
	} else {
		exitCode = -1
	}
	
	endTime := time.Now()
	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)
	result.Data = map[string]interface{}{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"cmd":       fullCommand,
		"exit_code": exitCode,
	}

	if err != nil {
		result.Success = false
		result.Error = err
		result.Message = fmt.Sprintf("command failed: %v", err)
	} else if exitCode != 0 {
		result.Success = false
		result.Message = fmt.Sprintf("command exited with code %d", exitCode)
	} else {
		result.Success = true
		result.Message = "command executed successfully"
	}

	result.Changed = result.Success

	return result, nil
}

// ExecuteStream runs a command with real-time output streaming
func (c *WinRMConnection) ExecuteStream(ctx context.Context, command string, options types.ExecuteOptions) (<-chan types.StreamEvent, error) {
	if !c.connected {
		return nil, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	eventChan := make(chan types.StreamEvent, 100)

	go func() {
		defer close(eventChan)

		startTime := time.Now()

		// Send initial progress
		if options.StreamOutput || options.ProgressCallback != nil {
			progress := types.ProgressInfo{
				Stage:     "connecting",
				Message:   fmt.Sprintf("Connecting to %s", c.info.Host),
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

		// Build command
		fullCommand := c.buildCommand(command, options)

		// Create shell
		shell, err := c.client.CreateShell()
		if err != nil {
			eventChan <- types.StreamEvent{
				Type:      types.StreamError,
				Error:     types.NewConnectionError(c.info.Host, "failed to create WinRM shell", err),
				Timestamp: time.Now(),
			}
			return
		}
		defer shell.Close()

		// Send execution progress
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

		// Execute command
		var stdout, stderr bytes.Buffer
		cmd, err := shell.ExecuteWithContext(ctx, fullCommand)
		if err != nil {
			eventChan <- types.StreamEvent{
				Type:      types.StreamError,
				Error:     types.NewConnectionError(c.info.Host, "failed to execute command", err),
				Timestamp: time.Now(),
			}
			return
		}

		// Read output in real-time
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Read from command output
					buffer := make([]byte, 1024)
					n, err := cmd.Stdout.Read(buffer)
					if n > 0 {
						output := string(buffer[:n])
						stdout.WriteString(output)

						if options.StreamOutput {
							if options.OutputCallback != nil {
								options.OutputCallback(output, false)
							}

							eventChan <- types.StreamEvent{
								Type:      types.StreamStdout,
								Data:      output,
								Timestamp: time.Now(),
							}
						}
					}
					if err == io.EOF {
						break
					}
				}
			}
		}()

		// Read stderr in real-time
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					buffer := make([]byte, 1024)
					n, err := cmd.Stderr.Read(buffer)
					if n > 0 {
						output := string(buffer[:n])
						stderr.WriteString(output)

						if options.StreamOutput {
							if options.OutputCallback != nil {
								options.OutputCallback(output, true)
							}

							eventChan <- types.StreamEvent{
								Type:      types.StreamStderr,
								Data:      output,
								Timestamp: time.Now(),
							}
						}
					}
					if err == io.EOF {
						break
					}
				}
			}
		}()

		// Wait for command to complete
		cmd.Wait()
		exitCode := cmd.ExitCode()
		endTime := time.Now()

		// Create result
		result := &types.Result{
			Host:       c.info.Host,
			Success:    exitCode == 0,
			Changed:    true,
			Message:    "Command executed",
			StartTime:  startTime,
			EndTime:    endTime,
			Duration:   endTime.Sub(startTime),
			ModuleName: "streaming_command",
			Data: map[string]interface{}{
				"stdout":    stdout.String(),
				"stderr":    stderr.String(),
				"cmd":       fullCommand,
				"exit_code": exitCode,
			},
		}

		if exitCode != 0 {
			result.Success = false
			result.Message = fmt.Sprintf("Command exited with code %d", exitCode)
		} else {
			result.Success = true
			result.Message = "Command executed successfully"
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

// Copy transfers a file to the remote Windows host
func (c *WinRMConnection) Copy(ctx context.Context, src io.Reader, dest string, mode int) error {
	if !c.connected {
		return types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	// Read source data
	data, err := ioutil.ReadAll(src)
	if err != nil {
		return types.NewConnectionError(c.info.Host, "failed to read source data", err)
	}

	// Encode data as base64
	encoded := base64.StdEncoding.EncodeToString(data)

	// PowerShell script to decode and write file
	script := fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$dest = "%s"
$content = "%s"
$bytes = [System.Convert]::FromBase64String($content)
$dir = Split-Path -Parent $dest
if (!(Test-Path $dir)) {
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
}
[System.IO.File]::WriteAllBytes($dest, $bytes)
`, dest, encoded)

	// Execute PowerShell script
	result, err := c.Execute(ctx, script, types.ExecuteOptions{
		Shell: "powershell",
	})
	if err != nil {
		return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to copy file to %s", dest), err)
	}

	if !result.Success {
		return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to copy file to %s: %v", dest, result.Error), nil)
	}

	return nil
}

// Fetch retrieves a file from the remote Windows host
func (c *WinRMConnection) Fetch(ctx context.Context, src string) (io.Reader, error) {
	if !c.connected {
		return nil, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	// PowerShell script to read and encode file
	script := fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$src = "%s"
if (!(Test-Path $src)) {
    throw "File not found: $src"
}
$bytes = [System.IO.File]::ReadAllBytes($src)
[System.Convert]::ToBase64String($bytes)
`, src)

	// Execute PowerShell script
	result, err := c.Execute(ctx, script, types.ExecuteOptions{
		Shell: "powershell",
	})
	if err != nil {
		return nil, types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to fetch %s", src), err)
	}

	if !result.Success {
		return nil, types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to fetch %s: %v", src, result.Error), nil)
	}

	// Decode base64 content
	encoded := strings.TrimSpace(result.Data["stdout"].(string))
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, types.NewConnectionError(c.info.Host, "failed to decode file content", err)
	}

	return bytes.NewReader(decoded), nil
}

// Close terminates the WinRM connection
func (c *WinRMConnection) Close() error {
	c.connected = false
	c.client = nil
	return nil
}

// IsConnected returns true if the WinRM connection is active
func (c *WinRMConnection) IsConnected() bool {
	return c.connected && c.client != nil
}

// buildCommand builds the full command string with options
func (c *WinRMConnection) buildCommand(command string, options types.ExecuteOptions) string {
	// Handle shell option
	if options.Shell == "powershell" || strings.HasPrefix(command, "$") {
		// PowerShell command
		if options.WorkingDir != "" {
			command = fmt.Sprintf("Set-Location '%s'; %s", options.WorkingDir, command)
		}
	} else {
		// CMD command
		if options.WorkingDir != "" {
			command = fmt.Sprintf("cd /d \"%s\" && %s", options.WorkingDir, command)
		}
	}

	// Handle run as user (requires appropriate privileges)
	if options.User != "" && options.User != c.info.User {
		// This would require more complex handling with scheduled tasks or PSExec
		// For now, we'll add a comment indicating this limitation
		command = fmt.Sprintf("REM Run as user %s not supported yet\n%s", options.User, command)
	}

	return command
}

// testConnection tests if the WinRM connection is working
func (c *WinRMConnection) testConnection(ctx context.Context) error {
	result, err := c.Execute(ctx, "echo 'connection test'", types.ExecuteOptions{})
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("connection test failed")
	}

	if !strings.Contains(result.Data["stdout"].(string), "connection test") {
		return fmt.Errorf("unexpected test output: %s", result.Data["stdout"].(string))
	}

	return nil
}

// FileExists checks if a file exists on the remote Windows host
func (c *WinRMConnection) FileExists(path string) (bool, error) {
	if !c.connected {
		return false, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	script := fmt.Sprintf("Test-Path '%s'", path)
	result, err := c.Execute(context.Background(), script, types.ExecuteOptions{
		Shell: "powershell",
	})
	if err != nil {
		return false, err
	}

	output := strings.TrimSpace(result.Data["stdout"].(string))
	return strings.EqualFold(output, "true"), nil
}

// CreateDirectory creates a directory on the remote Windows host
func (c *WinRMConnection) CreateDirectory(path string, mode int) error {
	if !c.connected {
		return types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	script := fmt.Sprintf(`
if (!(Test-Path '%s')) {
    New-Item -ItemType Directory -Path '%s' -Force | Out-Null
}
`, path, path)

	result, err := c.Execute(context.Background(), script, types.ExecuteOptions{
		Shell: "powershell",
	})
	if err != nil {
		return err
	}

	if !result.Success {
		return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to create directory %s", path), result.Error)
	}

	return nil
}

// RemoveFile removes a file on the remote Windows host
func (c *WinRMConnection) RemoveFile(path string) error {
	if !c.connected {
		return types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	script := fmt.Sprintf("Remove-Item -Path '%s' -Force -ErrorAction SilentlyContinue", path)
	result, err := c.Execute(context.Background(), script, types.ExecuteOptions{
		Shell: "powershell",
	})
	if err != nil {
		return err
	}

	if !result.Success {
		return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to remove file %s", path), result.Error)
	}

	return nil
}

// GetHostname returns the remote Windows hostname
func (c *WinRMConnection) GetHostname() (string, error) {
	if !c.connected {
		return "", types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	result, err := c.Execute(context.Background(), "$env:COMPUTERNAME", types.ExecuteOptions{
		Shell: "powershell",
	})
	if err != nil {
		return "", err
	}

	if !result.Success {
		return "", types.NewConnectionError(c.info.Host, "failed to get hostname", result.Error)
	}

	hostname := strings.TrimSpace(result.Data["stdout"].(string))
	return hostname, nil
}

// GetSystemInfo returns basic system information from the remote Windows host
func (c *WinRMConnection) GetSystemInfo() (map[string]interface{}, error) {
	if !c.connected {
		return nil, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	info := make(map[string]interface{})

	// Get hostname
	if hostname, err := c.GetHostname(); err == nil {
		info["hostname"] = hostname
	}

	// Get current user
	if result, err := c.Execute(context.Background(), "$env:USERNAME", types.ExecuteOptions{
		Shell: "powershell",
	}); err == nil && result.Success {
		info["user"] = strings.TrimSpace(result.Data["stdout"].(string))
	}

	// Get working directory
	if result, err := c.Execute(context.Background(), "Get-Location", types.ExecuteOptions{
		Shell: "powershell",
	}); err == nil && result.Success {
		info["working_directory"] = strings.TrimSpace(result.Data["stdout"].(string))
	}

	// Get OS information
	script := `
$os = Get-WmiObject Win32_OperatingSystem
"$($os.Caption) $($os.Version) $($os.OSArchitecture)"
`
	if result, err := c.Execute(context.Background(), script, types.ExecuteOptions{
		Shell: "powershell",
	}); err == nil && result.Success {
		info["os"] = strings.TrimSpace(result.Data["stdout"].(string))
	}

	return info, nil
}

// ExecuteScript executes a script on the remote Windows host
func (c *WinRMConnection) ExecuteScript(ctx context.Context, script string, options types.ExecuteOptions) (*types.Result, error) {
	if !c.connected {
		return nil, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	// Determine script type based on content or options
	if options.Shell == "powershell" || strings.Contains(script, "$") {
		// PowerShell script - execute directly
		return c.Execute(ctx, script, options)
	}

	// Batch script - create temp file and execute
	tempPath := fmt.Sprintf("%%TEMP%%\\gosinble-script-%d.bat", time.Now().UnixNano())
	
	// Upload script
	if err := c.Copy(ctx, strings.NewReader(script), tempPath, 0755); err != nil {
		return nil, types.NewConnectionError(c.info.Host, "failed to upload script", err)
	}

	// Execute script
	result, err := c.Execute(ctx, tempPath, options)

	// Clean up temporary file (best effort)
	c.RemoveFile(tempPath)

	return result, err
}

// Ping tests connectivity to the remote Windows host
func (c *WinRMConnection) Ping() error {
	if !c.connected {
		return types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := c.Execute(ctx, "echo pong", types.ExecuteOptions{})
	if err != nil {
		return types.NewConnectionError(c.info.Host, "ping failed", err)
	}

	if !result.Success || !strings.Contains(result.Data["stdout"].(string), "pong") {
		return types.NewConnectionError(c.info.Host, "ping command failed", result.Error)
	}

	return nil
}