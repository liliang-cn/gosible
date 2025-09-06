package connection

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gosinble/gosinble/pkg/types"
)

func TestNewConnectionManager(t *testing.T) {
	manager := NewConnectionManager()
	if manager == nil {
		t.Fatal("NewConnectionManager returned nil")
	}

	plugins := manager.ListPlugins()
	if len(plugins) < 2 {
		t.Errorf("expected at least 2 built-in plugins, got %d", len(plugins))
	}

	// Check that built-in plugins are registered
	hasLocal := false
	hasSSH := false
	for _, plugin := range plugins {
		if plugin == ConnectionTypeLocal {
			hasLocal = true
		}
		if plugin == ConnectionTypeSSH {
			hasSSH = true
		}
	}

	if !hasLocal {
		t.Error("local connection plugin not registered")
	}
	if !hasSSH {
		t.Error("SSH connection plugin not registered")
	}
}

func TestConnectionManagerRegisterPlugin(t *testing.T) {
	manager := NewConnectionManager()

	// Register a custom plugin
	customType := ConnectionType("custom")
	manager.RegisterPlugin(customType, func() types.Connection {
		return NewLocalConnection() // Use local connection as mock
	})

	plugins := manager.ListPlugins()
	hasCustom := false
	for _, plugin := range plugins {
		if plugin == customType {
			hasCustom = true
			break
		}
	}

	if !hasCustom {
		t.Error("custom plugin not registered")
	}
}

func TestConnectionManagerCreateConnection(t *testing.T) {
	manager := NewConnectionManager()

	// Test creating local connection
	localConn, err := manager.CreateConnection(ConnectionTypeLocal)
	if err != nil {
		t.Fatalf("failed to create local connection: %v", err)
	}
	if localConn == nil {
		t.Error("created connection is nil")
	}

	// Test creating SSH connection
	sshConn, err := manager.CreateConnection(ConnectionTypeSSH)
	if err != nil {
		t.Fatalf("failed to create SSH connection: %v", err)
	}
	if sshConn == nil {
		t.Error("created SSH connection is nil")
	}

	// Test unsupported connection type
	_, err = manager.CreateConnection(ConnectionType("unsupported"))
	if err == nil {
		t.Error("expected error for unsupported connection type")
	}
}

func TestLocalConnectionConnect(t *testing.T) {
	conn := NewLocalConnection()
	
	if conn.IsConnected() {
		t.Error("connection should not be connected initially")
	}

	ctx := context.Background()
	info := types.ConnectionInfo{
		Type: "local",
		Host: "localhost",
	}

	err := conn.Connect(ctx, info)
	if err != nil {
		t.Fatalf("local connection failed: %v", err)
	}

	if !conn.IsConnected() {
		t.Error("connection should be connected after Connect()")
	}
}

func TestLocalConnectionExecute(t *testing.T) {
	conn := NewLocalConnection()
	ctx := context.Background()

	// Connect first
	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		t.Fatalf("connection failed: %v", err)
	}

	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{
			name:    "simple echo command",
			command: "echo 'hello world'",
			wantErr: false,
		},
		{
			name:    "command with exit code 0",
			command: "exit 0",
			wantErr: false,
		},
		{
			name:    "command with non-zero exit code",
			command: "exit 1",
			wantErr: false, // Command execution doesn't fail, but result.Success will be false
		},
		{
			name:    "invalid command",
			command: "this-command-does-not-exist-12345",
			wantErr: false, // Command execution doesn't fail, but result.Success will be false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := types.ExecuteOptions{}
			result, err := conn.Execute(ctx, tt.command, options)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Error("result should not be nil")
				return
			}

			if result.Host != "localhost" {
				t.Errorf("result.Host = %v, want localhost", result.Host)
			}

			if result.StartTime.IsZero() || result.EndTime.IsZero() {
				t.Error("result timestamps should be set")
			}

			if result.Duration == 0 {
				t.Error("result duration should be set")
			}

			// Check data fields
			if result.Data == nil {
				t.Error("result.Data should not be nil")
			}

			if _, exists := result.Data["stdout"]; !exists {
				t.Error("result.Data should contain stdout")
			}

			if _, exists := result.Data["stderr"]; !exists {
				t.Error("result.Data should contain stderr")
			}

			if _, exists := result.Data["cmd"]; !exists {
				t.Error("result.Data should contain cmd")
			}

			if _, exists := result.Data["exit_code"]; !exists {
				t.Error("result.Data should contain exit_code")
			}
		})
	}
}

func TestLocalConnectionExecuteWithTimeout(t *testing.T) {
	conn := NewLocalConnection()
	ctx := context.Background()

	// Connect first
	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		t.Fatalf("connection failed: %v", err)
	}

	// Test command that should timeout
	options := types.ExecuteOptions{
		Timeout: 100 * time.Millisecond,
	}

	result, err := conn.Execute(ctx, "sleep 1", options)
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// The command should have been killed due to timeout
	if result.Success {
		t.Error("expected command to fail due to timeout")
	}
}

func TestLocalConnectionExecuteWithWorkingDir(t *testing.T) {
	conn := NewLocalConnection()
	ctx := context.Background()

	// Connect first
	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		t.Fatalf("connection failed: %v", err)
	}

	// Test command with working directory
	options := types.ExecuteOptions{
		WorkingDir: "/tmp",
	}

	result, err := conn.Execute(ctx, "pwd", options)
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("command should have succeeded: %v", result.Error)
	}

	stdout, ok := result.Data["stdout"].(string)
	if !ok {
		t.Fatal("stdout should be a string")
	}

	if !strings.Contains(stdout, "/tmp") {
		t.Errorf("expected output to contain /tmp, got: %s", stdout)
	}
}

func TestLocalConnectionCopyAndFetch(t *testing.T) {
	conn := NewLocalConnection()
	ctx := context.Background()

	// Connect first
	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		t.Fatalf("connection failed: %v", err)
	}

	// Test data
	testData := "This is test data for copy/fetch operations"
	testFile := "/tmp/gosinble-test-copy-fetch"

	// Clean up function
	defer func() {
		conn.Execute(ctx, "rm -f "+testFile, types.ExecuteOptions{})
	}()

	// Test Copy
	reader := strings.NewReader(testData)
	err := conn.Copy(ctx, reader, testFile, 0644)
	if err != nil {
		t.Fatalf("Copy() failed: %v", err)
	}

	// Verify file was created
	result, err := conn.Execute(ctx, "test -f "+testFile, types.ExecuteOptions{})
	if err != nil {
		t.Fatalf("file existence check failed: %v", err)
	}
	if !result.Success {
		t.Error("file should exist after copy")
	}

	// Test Fetch
	fetchedReader, err := conn.Fetch(ctx, testFile)
	if err != nil {
		t.Fatalf("Fetch() failed: %v", err)
	}

	fetchedData := make([]byte, len(testData)+10) // Buffer larger than needed
	n, err := fetchedReader.Read(fetchedData)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("failed to read fetched data: %v", err)
	}

	fetchedContent := string(fetchedData[:n])
	if fetchedContent != testData {
		t.Errorf("fetched data doesn't match original. Expected: %q, Got: %q", testData, fetchedContent)
	}
}

func TestLocalConnectionClose(t *testing.T) {
	conn := NewLocalConnection()
	ctx := context.Background()

	// Connect first
	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		t.Fatalf("connection failed: %v", err)
	}

	if !conn.IsConnected() {
		t.Error("connection should be connected")
	}

	// Close connection
	err := conn.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	if conn.IsConnected() {
		t.Error("connection should not be connected after Close()")
	}
}

func TestLocalConnectionGetSystemInfo(t *testing.T) {
	conn := NewLocalConnection()
	ctx := context.Background()

	// Connect first
	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		t.Fatalf("connection failed: %v", err)
	}

	systemInfo, err := conn.GetSystemInfo()
	if err != nil {
		t.Fatalf("GetSystemInfo() failed: %v", err)
	}

	// Check that we got some expected fields
	expectedFields := []string{"hostname", "user", "working_directory"}
	for _, field := range expectedFields {
		if _, exists := systemInfo[field]; !exists {
			t.Errorf("system info should contain field: %s", field)
		}
	}
}

func TestConnectionManagerGetConnection(t *testing.T) {
	manager := NewConnectionManager()

	// Test local connection
	ctx := context.Background()
	info := types.ConnectionInfo{
		Type: "local",
		Host: "localhost",
	}

	conn, err := manager.GetConnection(ctx, info)
	if err != nil {
		t.Fatalf("GetConnection() failed: %v", err)
	}

	if conn == nil {
		t.Error("connection should not be nil")
	}

	if !conn.IsConnected() {
		t.Error("connection should be connected")
	}

	// Test that we can execute a command
	result, err := conn.Execute(ctx, "echo 'test'", types.ExecuteOptions{})
	if err != nil {
		t.Fatalf("command execution failed: %v", err)
	}

	if !result.Success {
		t.Error("test command should have succeeded")
	}

	// Clean up
	conn.Close()
}

// Benchmark tests
func BenchmarkLocalConnectionExecute(b *testing.B) {
	conn := NewLocalConnection()
	ctx := context.Background()

	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		b.Fatalf("connection failed: %v", err)
	}
	defer conn.Close()

	options := types.ExecuteOptions{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := conn.Execute(ctx, "echo 'benchmark test'", options)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConnectionManagerCreate(b *testing.B) {
	manager := NewConnectionManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, err := manager.CreateConnection(ConnectionTypeLocal)
		if err != nil {
			b.Fatal(err)
		}
		if conn == nil {
			b.Fatal("connection is nil")
		}
	}
}