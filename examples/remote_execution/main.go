package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gosinble/gosinble/pkg/connection"
	"github.com/gosinble/gosinble/pkg/types"
)

func main() {
	fmt.Println("=== Gosinble Remote Execution Examples ===")

	// Example 1: Basic SSH connection and command execution
	fmt.Println("\n1. Basic SSH Connection Example:")
	basicSSHExample()

	// Example 2: Connection pooling
	fmt.Println("\n2. Connection Pooling Example:")
	connectionPoolExample()

	// Example 3: Windows/WinRM example (commented out as it needs a Windows host)
	fmt.Println("\n3. Windows/WinRM Connection Example (demonstration):")
	winrmExample()

	// Example 4: Streaming execution
	fmt.Println("\n4. Streaming Execution Example:")
	streamingExample()

	// Example 5: File operations
	fmt.Println("\n5. File Operations Example:")
	fileOpsExample()
}

func basicSSHExample() {
	// Create SSH connection
	conn := connection.NewSSHConnection()
	
	// Connection info for a hypothetical SSH server
	info := types.ConnectionInfo{
		Type:       "ssh",
		Host:       "localhost", // Change to actual host
		Port:       22,
		User:       "testuser",
		Password:   "testpass", // In production, use keys!
		Timeout:    30 * time.Second,
	}

	ctx := context.Background()

	// Attempt to connect (will likely fail without real SSH server)
	if err := conn.Connect(ctx, info); err != nil {
		fmt.Printf("Connection failed (expected): %v\n", err)
		fmt.Println("To test this example, update the connection info with a real SSH server")
		return
	}
	defer conn.Close()

	// Execute a command
	result, err := conn.Execute(ctx, "echo 'Hello from remote host!'", types.ExecuteOptions{})
	if err != nil {
		fmt.Printf("Command execution failed: %v\n", err)
		return
	}

	fmt.Printf("Command output: %s\n", result.Data["stdout"])
	fmt.Printf("Command succeeded: %v\n", result.Success)
}

func connectionPoolExample() {
	// Create a connection manager with pooling
	config := connection.DefaultConnectionPoolConfig()
	config.MaxConnections = 5
	config.MaxIdleTime = 2 * time.Minute

	manager := connection.NewPooledConnectionManager(config)
	defer manager.Close()

	// Connection info
	info := types.ConnectionInfo{
		Type:     "ssh",
		Host:     "example.com",
		User:     "user",
		Password: "pass",
	}

	ctx := context.Background()

	// Execute commands using the pool
	for i := 0; i < 3; i++ {
		result, err := manager.ExecuteOnHost(ctx, info, fmt.Sprintf("echo 'Command %d'", i+1), types.ExecuteOptions{})
		if err != nil {
			fmt.Printf("Pool execution %d failed (expected): %v\n", i+1, err)
		} else {
			fmt.Printf("Pool execution %d: %s\n", i+1, result.Data["stdout"])
		}
	}

	// Show pool statistics
	stats := manager.Stats()
	fmt.Printf("Pool stats - Total: %d, Active: %d, Idle: %d\n", 
		stats.TotalConnections, stats.ActiveConnections, stats.IdleConnections)
}

func winrmExample() {
	// Create WinRM connection
	conn := connection.NewWinRMConnection()
	
	// Connection info for a Windows host
	info := types.ConnectionInfo{
		Type:       "winrm",
		Host:       "windows-host.example.com",
		Port:       5985, // HTTP, use 5986 for HTTPS
		User:       "Administrator",
		Password:   "password",
		UseSSL:     false,
		SkipVerify: true, // Only for testing!
		Timeout:    30 * time.Second,
	}

	ctx := context.Background()

	fmt.Println("This example demonstrates WinRM connection setup (will fail without Windows host)")
	
	if err := conn.Connect(ctx, info); err != nil {
		fmt.Printf("WinRM connection failed (expected): %v\n", err)
		return
	}
	defer conn.Close()

	// Execute PowerShell command
	result, err := conn.Execute(ctx, "Get-ComputerInfo | Select-Object WindowsProductName", types.ExecuteOptions{
		Shell: "powershell",
	})
	if err != nil {
		fmt.Printf("PowerShell command failed: %v\n", err)
		return
	}

	fmt.Printf("Windows info: %s\n", result.Data["stdout"])

	// Execute CMD command
	result, err = conn.Execute(ctx, "dir C:\\", types.ExecuteOptions{})
	if err != nil {
		fmt.Printf("CMD command failed: %v\n", err)
		return
	}

	fmt.Printf("Directory listing: %s\n", result.Data["stdout"])
}

func streamingExample() {
	// Use local connection for streaming example
	conn := connection.NewLocalConnection()
	
	info := types.ConnectionInfo{Type: "local"}
	ctx := context.Background()

	if err := conn.Connect(ctx, info); err != nil {
		fmt.Printf("Local connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	// Execute a command with streaming
	options := types.ExecuteOptions{
		StreamOutput: true,
		OutputCallback: func(line string, isStderr bool) {
			if isStderr {
				fmt.Printf("[STDERR] %s\n", line)
			} else {
				fmt.Printf("[STDOUT] %s\n", line)
			}
		},
		ProgressCallback: func(progress types.ProgressInfo) {
			fmt.Printf("[PROGRESS] %s: %.1f%% - %s\n", 
				progress.Stage, progress.Percentage, progress.Message)
		},
	}

	// Local connection supports streaming
	fmt.Println("Starting streaming command...")
	
	eventChan, err := conn.ExecuteStream(ctx, "echo 'Line 1'; sleep 1; echo 'Line 2'; sleep 1; echo 'Line 3'", options)
	if err != nil {
		fmt.Printf("Streaming failed: %v\n", err)
		return
	}

	// Process streaming events
	for event := range eventChan {
		switch event.Type {
		case types.StreamStdout:
			fmt.Printf("📤 %s\n", event.Data)
		case types.StreamStderr:
			fmt.Printf("📥 %s\n", event.Data)
		case types.StreamProgress:
			if event.Progress != nil {
				fmt.Printf("⏳ %s: %.1f%%\n", event.Progress.Stage, event.Progress.Percentage)
			}
		case types.StreamDone:
			fmt.Printf("✅ Command completed: %v\n", event.Result.Success)
		case types.StreamError:
			fmt.Printf("❌ Error: %v\n", event.Error)
		}
	}
}

func fileOpsExample() {
	// Use local connection for file operations example
	conn := connection.NewLocalConnection()
	
	info := types.ConnectionInfo{Type: "local"}
	ctx := context.Background()

	if err := conn.Connect(ctx, info); err != nil {
		fmt.Printf("Local connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	// Create a test file
	testContent := "Hello, Gosinble!\nThis is a test file.\n"
	testPath := "/tmp/gosinble-test.txt"

	fmt.Printf("Creating file: %s\n", testPath)
	err := conn.Copy(ctx, strings.NewReader(testContent), testPath, 0644)
	if err != nil {
		fmt.Printf("Failed to create file: %v\n", err)
		return
	}

	// Verify file exists
	result, err := conn.Execute(ctx, fmt.Sprintf("test -f %s && echo 'File exists' || echo 'File not found'", testPath), types.ExecuteOptions{})
	if err != nil {
		fmt.Printf("Failed to check file: %v\n", err)
		return
	}
	fmt.Printf("File check: %s\n", strings.TrimSpace(result.Data["stdout"].(string)))

	// Fetch the file back
	fmt.Println("Reading file back...")
	reader, err := conn.Fetch(ctx, testPath)
	if err != nil {
		fmt.Printf("Failed to fetch file: %v\n", err)
		return
	}

	// Read content
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	if err != nil && n == 0 {
		fmt.Printf("Failed to read file content: %v\n", err)
		return
	}

	fmt.Printf("File content:\n%s", string(buf[:n]))

	// Clean up
	_, err = conn.Execute(ctx, fmt.Sprintf("rm -f %s", testPath), types.ExecuteOptions{})
	if err != nil {
		fmt.Printf("Failed to clean up file: %v\n", err)
	} else {
		fmt.Println("Test file cleaned up")
	}
}