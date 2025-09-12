package connection

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/liliang-cn/gosible/pkg/types"
)

// SSHConnection implements the Connection interface for SSH connections
type SSHConnection struct {
	client    *ssh.Client
	connected bool
	info      types.ConnectionInfo
}

// NewSSHConnection creates a new SSH connection
func NewSSHConnection() *SSHConnection {
	return &SSHConnection{}
}

// Connect establishes an SSH connection to the target host
func (c *SSHConnection) Connect(ctx context.Context, info types.ConnectionInfo) error {
	c.info = info

	// Set default timeout if not specified
	timeout := info.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Set default port if not specified
	port := info.Port
	if port == 0 {
		port = 22
	}

	// Create SSH client configuration
	config := &ssh.ClientConfig{
		User:            info.User,
		Timeout:         timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // In production, implement proper host key verification
	}

	// Add authentication methods
	// Priority 1: Password authentication
	if info.Password != "" {
		config.Auth = append(config.Auth, ssh.Password(info.Password))
	}

	// Priority 2: Private key from configuration
	if info.PrivateKey != "" {
		signer, err := c.parsePrivateKey(info.PrivateKey)
		if err != nil {
			return types.NewConnectionError(info.Host, "failed to parse private key", err)
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	// Priority 3: Try default SSH keys only if no other auth method is provided
	if len(config.Auth) == 0 {
		if signers, err := c.loadDefaultKeys(); err == nil && len(signers) > 0 {
			config.Auth = append(config.Auth, ssh.PublicKeys(signers...))
		}
	}

	// If still no auth methods, return error
	if len(config.Auth) == 0 {
		return types.NewConnectionError(info.Host, "no authentication method provided", nil)
	}

	// Establish connection
	address := fmt.Sprintf("%s:%d", info.Host, port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return types.NewConnectionError(info.Host, fmt.Sprintf("failed to connect to %s", address), err)
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

// Execute runs a command on the remote host via SSH
func (c *SSHConnection) Execute(ctx context.Context, command string, options types.ExecuteOptions) (*types.Result, error) {
	if !c.connected {
		return nil, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	startTime := time.Now()
	result := &types.Result{
		StartTime:  startTime,
		Host:       c.info.Host,
		ModuleName: "command",
	}

	// Create SSH session
	session, err := c.client.NewSession()
	if err != nil {
		return nil, types.NewConnectionError(c.info.Host, "failed to create SSH session", err)
	}
	defer session.Close()

	// Set up command with options
	fullCommand := c.buildCommand(command, options)

	// Set up output capture
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Set environment variables
	if options.Env != nil {
		for k, v := range options.Env {
			session.Setenv(k, v)
		}
	}

	// Execute command with timeout
	done := make(chan error, 1)
	go func() {
		done <- session.Run(fullCommand)
	}()

	var execErr error
	if options.Timeout > 0 {
		select {
		case execErr = <-done:
		case <-time.After(options.Timeout):
			session.Signal(ssh.SIGKILL)
			execErr = types.ErrTimeout
		case <-ctx.Done():
			session.Signal(ssh.SIGKILL)
			execErr = ctx.Err()
		}
	} else {
		select {
		case execErr = <-done:
		case <-ctx.Done():
			session.Signal(ssh.SIGKILL)
			execErr = ctx.Err()
		}
	}

	endTime := time.Now()
	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)
	result.Data = map[string]interface{}{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
		"cmd":    fullCommand,
	}

	if execErr != nil {
		result.Success = false
		result.Error = execErr
		result.Message = fmt.Sprintf("command failed: %v", execErr)

		// Extract exit code from SSH error
		if exitError, ok := execErr.(*ssh.ExitError); ok {
			result.Data["exit_code"] = exitError.ExitStatus()
		} else {
			result.Data["exit_code"] = -1
		}
	} else {
		result.Success = true
		result.Message = "command executed successfully"
		result.Data["exit_code"] = 0
	}

	// Determine if anything changed (simple heuristic)
	result.Changed = result.Success

	return result, nil
}

// ExecuteStream runs a command on the remote host with real-time output streaming
func (c *SSHConnection) ExecuteStream(ctx context.Context, command string, options types.ExecuteOptions) (<-chan types.StreamEvent, error) {
	if !c.connected {
		return nil, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	eventChan := make(chan types.StreamEvent, 100) // Buffered channel for events

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

		// Create SSH session
		session, err := c.client.NewSession()
		if err != nil {
			eventChan <- types.StreamEvent{
				Type:      types.StreamError,
				Error:     types.NewConnectionError(c.info.Host, "failed to create SSH session", err),
				Timestamp: time.Now(),
			}
			return
		}
		defer session.Close()

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

		// Set up command with options
		fullCommand := c.buildCommand(command, options)

		// Set up pipes for real-time output
		stdoutPipe, err := session.StdoutPipe()
		if err != nil {
			eventChan <- types.StreamEvent{
				Type:      types.StreamError,
				Error:     types.NewConnectionError(c.info.Host, "failed to create stdout pipe", err),
				Timestamp: time.Now(),
			}
			return
		}

		stderrPipe, err := session.StderrPipe()
		if err != nil {
			eventChan <- types.StreamEvent{
				Type:      types.StreamError,
				Error:     types.NewConnectionError(c.info.Host, "failed to create stderr pipe", err),
				Timestamp: time.Now(),
			}
			return
		}

		// Set environment variables
		if options.Env != nil {
			for k, v := range options.Env {
				session.Setenv(k, v)
			}
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

		// Start command execution
		done := make(chan error, 1)
		go func() {
			done <- session.Run(fullCommand)
		}()

		var execErr error
		if options.Timeout > 0 {
			select {
			case execErr = <-done:
			case <-time.After(options.Timeout):
				session.Signal(ssh.SIGKILL)
				execErr = types.ErrTimeout
			case <-ctx.Done():
				session.Signal(ssh.SIGKILL)
				execErr = ctx.Err()
			}
		} else {
			select {
			case execErr = <-done:
			case <-ctx.Done():
				session.Signal(ssh.SIGKILL)
				execErr = ctx.Err()
			}
		}

		// Wait for output readers to finish
		wg.Wait()
		endTime := time.Now()

		// Create result
		result := &types.Result{
			Host:       c.info.Host,
			Success:    execErr == nil,
			Changed:    true, // Assume changed for commands
			Message:    "Command executed",
			StartTime:  startTime,
			EndTime:    endTime,
			Duration:   endTime.Sub(startTime),
			ModuleName: "streaming_command",
			Data: map[string]interface{}{
				"stdout": stdout.String(),
				"stderr": stderr.String(),
				"cmd":    fullCommand,
			},
		}

		if execErr != nil {
			result.Success = false
			result.Error = execErr
			result.Message = fmt.Sprintf("Command failed: %v", execErr)

			// Extract exit code from SSH error
			if exitError, ok := execErr.(*ssh.ExitError); ok {
				result.Data["exit_code"] = exitError.ExitStatus()
			} else {
				result.Data["exit_code"] = -1
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

// Copy transfers a file to the remote host via chunked base64 encoding
func (c *SSHConnection) Copy(ctx context.Context, src io.Reader, dest string, mode int) error {
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

	// Sanitize destination path
	dest = types.SanitizePath(dest)

	// Create destination directory if needed
	destDir := filepath.Dir(dest)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", destDir)
	if _, err := c.Execute(ctx, mkdirCmd, types.ExecuteOptions{}); err != nil {
		return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to create directory %s", destDir), err)
	}

	// Use a temporary file for transfer
	tempFile := fmt.Sprintf("/tmp/gosible_copy_%d.b64", time.Now().Unix())

	// Split encoded data into chunks (to avoid command line length limits)
	chunkSize := 64000 // Safe size for command line
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]

		// Append chunk to temp file
		appendCmd := fmt.Sprintf("echo -n '%s' >> %s", chunk, tempFile)
		if i == 0 {
			// First chunk, create new file
			appendCmd = fmt.Sprintf("echo -n '%s' > %s", chunk, tempFile)
		}

		result, err := c.Execute(ctx, appendCmd, types.ExecuteOptions{})
		if err != nil || !result.Success {
			// Clean up temp file on error
			c.Execute(ctx, fmt.Sprintf("rm -f %s", tempFile), types.ExecuteOptions{})
			return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to transfer chunk to %s", dest), err)
		}
	}

	// Decode and move to final destination with correct permissions
	finalCmd := fmt.Sprintf("base64 -d < %s > %s && rm -f %s && chmod %04o %s",
		tempFile, dest, tempFile, mode, dest)

	result, err := c.Execute(ctx, finalCmd, types.ExecuteOptions{})
	if err != nil {
		// Clean up temp file on error
		c.Execute(ctx, fmt.Sprintf("rm -f %s", tempFile), types.ExecuteOptions{})
		return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to decode and install file to %s", dest), err)
	}

	if !result.Success {
		// Clean up temp file on error
		c.Execute(ctx, fmt.Sprintf("rm -f %s", tempFile), types.ExecuteOptions{})
		return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to copy file to %s: %v", dest, result.Error), nil)
	}

	return nil
}

// Fetch retrieves a file from the remote host via SCP
func (c *SSHConnection) Fetch(ctx context.Context, src string) (io.Reader, error) {
	if !c.connected {
		return nil, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	// Create SSH session for cat command (simpler than SCP for fetching)
	session, err := c.client.NewSession()
	if err != nil {
		return nil, types.NewConnectionError(c.info.Host, "failed to create SSH session for fetch", err)
	}
	defer session.Close()

	// Use cat to read the file
	src = types.SanitizePath(src)
	var output bytes.Buffer
	session.Stdout = &output

	if err := session.Run(fmt.Sprintf("cat %s", src)); err != nil {
		return nil, types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to fetch %s", src), err)
	}

	return &output, nil
}

// Close terminates the SSH connection
func (c *SSHConnection) Close() error {
	if c.client != nil {
		err := c.client.Close()
		c.client = nil
		c.connected = false
		return err
	}
	c.connected = false
	return nil
}

// IsConnected returns true if the SSH connection is active
func (c *SSHConnection) IsConnected() bool {
	return c.connected && c.client != nil
}

// buildCommand builds the full command string with options
func (c *SSHConnection) buildCommand(command string, options types.ExecuteOptions) string {
	var parts []string

	// Change directory if specified
	if options.WorkingDir != "" {
		parts = append(parts, fmt.Sprintf("cd %s", options.WorkingDir))
	}

	// Set up sudo if requested
	if options.Sudo && options.User != "" {
		command = fmt.Sprintf("sudo -u %s %s", options.User, command)
	} else if options.User != "" {
		command = fmt.Sprintf("su -c '%s' %s", command, options.User)
	}

	parts = append(parts, command)
	return strings.Join(parts, " && ")
}

// parsePrivateKey parses a private key string
func (c *SSHConnection) parsePrivateKey(privateKey string) (ssh.Signer, error) {
	// Try to parse as file path first
	if _, err := os.Stat(privateKey); err == nil {
		keyData, err := ioutil.ReadFile(privateKey)
		if err != nil {
			return nil, err
		}
		return ssh.ParsePrivateKey(keyData)
	}

	// Parse as key content
	return ssh.ParsePrivateKey([]byte(privateKey))
}

// loadDefaultKeys loads default SSH keys from the user's .ssh directory
func (c *SSHConnection) loadDefaultKeys() ([]ssh.Signer, error) {
	var signers []ssh.Signer

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	defaultKeys := []string{"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519"}

	for _, keyName := range defaultKeys {
		keyPath := filepath.Join(sshDir, keyName)
		if _, err := os.Stat(keyPath); err != nil {
			continue // Key doesn't exist, skip
		}

		keyData, err := ioutil.ReadFile(keyPath)
		if err != nil {
			continue // Can't read key, skip
		}

		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			continue // Can't parse key, skip
		}

		signers = append(signers, signer)
	}

	return signers, nil
}

// testConnection tests if the SSH connection is working
func (c *SSHConnection) testConnection(ctx context.Context) error {
	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	var output bytes.Buffer
	session.Stdout = &output

	if err := session.Run("echo 'connection test'"); err != nil {
		return err
	}

	if !strings.Contains(output.String(), "connection test") {
		return fmt.Errorf("unexpected test output: %s", output.String())
	}

	return nil
}

// FileExists checks if a file exists on the remote host
func (c *SSHConnection) FileExists(path string) (bool, error) {
	if !c.connected {
		return false, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	path = types.SanitizePath(path)
	result, err := c.Execute(context.Background(), fmt.Sprintf("test -f %s", path), types.ExecuteOptions{})
	if err != nil {
		return false, err
	}

	return result.Success, nil
}

// CreateDirectory creates a directory on the remote host
func (c *SSHConnection) CreateDirectory(path string, mode int) error {
	if !c.connected {
		return types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	path = types.SanitizePath(path)
	command := fmt.Sprintf("mkdir -p %s && chmod %o %s", path, mode, path)
	result, err := c.Execute(context.Background(), command, types.ExecuteOptions{})
	if err != nil {
		return err
	}

	if !result.Success {
		return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to create directory %s", path), result.Error)
	}

	return nil
}

// RemoveFile removes a file on the remote host
func (c *SSHConnection) RemoveFile(path string) error {
	if !c.connected {
		return types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	path = types.SanitizePath(path)
	result, err := c.Execute(context.Background(), fmt.Sprintf("rm -f %s", path), types.ExecuteOptions{})
	if err != nil {
		return err
	}

	if !result.Success {
		return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to remove file %s", path), result.Error)
	}

	return nil
}

// GetHostname returns the remote hostname
func (c *SSHConnection) GetHostname() (string, error) {
	if !c.connected {
		return "", types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	result, err := c.Execute(context.Background(), "hostname", types.ExecuteOptions{})
	if err != nil {
		return "", err
	}

	if !result.Success {
		return "", types.NewConnectionError(c.info.Host, "failed to get hostname", result.Error)
	}

	hostname := strings.TrimSpace(result.Data["stdout"].(string))
	return hostname, nil
}

// GetSystemInfo returns basic system information from the remote host
func (c *SSHConnection) GetSystemInfo() (map[string]interface{}, error) {
	if !c.connected {
		return nil, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	info := make(map[string]interface{})

	// Get hostname
	if hostname, err := c.GetHostname(); err == nil {
		info["hostname"] = hostname
	}

	// Get current user
	if result, err := c.Execute(context.Background(), "whoami", types.ExecuteOptions{}); err == nil && result.Success {
		info["user"] = strings.TrimSpace(result.Data["stdout"].(string))
	}

	// Get working directory
	if result, err := c.Execute(context.Background(), "pwd", types.ExecuteOptions{}); err == nil && result.Success {
		info["working_directory"] = strings.TrimSpace(result.Data["stdout"].(string))
	}

	// Get OS information
	if result, err := c.Execute(context.Background(), "uname -a", types.ExecuteOptions{}); err == nil && result.Success {
		info["uname"] = strings.TrimSpace(result.Data["stdout"].(string))
	}

	return info, nil
}

// ExecuteScript executes a script on the remote host
func (c *SSHConnection) ExecuteScript(ctx context.Context, script string, options types.ExecuteOptions) (*types.Result, error) {
	if !c.connected {
		return nil, types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	// Create a temporary script file on the remote host
	tempPath := fmt.Sprintf("/tmp/gosiblescript-%d.sh", time.Now().UnixNano())

	// Upload script content
	if err := c.Copy(ctx, strings.NewReader(script), tempPath, 0755); err != nil {
		return nil, types.NewConnectionError(c.info.Host, "failed to upload script", err)
	}

	// Execute script
	result, err := c.Execute(ctx, tempPath, options)

	// Clean up temporary file (best effort)
	c.RemoveFile(tempPath)

	return result, err
}

// Ping tests connectivity to the remote host
func (c *SSHConnection) Ping() error {
	if !c.connected {
		return types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	// Test with a simple command
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := c.Execute(ctx, "echo 'pong'", types.ExecuteOptions{})
	if err != nil {
		return types.NewConnectionError(c.info.Host, "ping failed", err)
	}

	if !result.Success || !strings.Contains(result.Data["stdout"].(string), "pong") {
		return types.NewConnectionError(c.info.Host, "ping command failed", result.Error)
	}

	return nil
}

// PortForward creates a port forward from local to remote host
func (c *SSHConnection) PortForward(localAddr, remoteAddr string) error {
	if !c.connected {
		return types.NewConnectionError(c.info.Host, "not connected", nil)
	}

	// Listen on local address
	localListener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return types.NewConnectionError(c.info.Host, fmt.Sprintf("failed to listen on %s", localAddr), err)
	}

	go func() {
		defer localListener.Close()
		for {
			localConn, err := localListener.Accept()
			if err != nil {
				return
			}

			go func(local net.Conn) {
				defer local.Close()

				// Dial remote address through SSH
				remote, err := c.client.Dial("tcp", remoteAddr)
				if err != nil {
					return
				}
				defer remote.Close()

				// Copy data between connections
				go io.Copy(remote, local)
				io.Copy(local, remote)
			}(localConn)
		}
	}()

	return nil
}
