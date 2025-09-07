package connection

import (
	"context"
	"strings"
	"testing"

	"github.com/gosinble/gosinble/pkg/types"
)

func TestSSHConnection_Connect(t *testing.T) {
	tests := []struct {
		name        string
		info        types.ConnectionInfo
		expectError bool
	}{
		{
			name: "invalid host",
			info: types.ConnectionInfo{
				Host:     "invalid-host-12345",
				User:     "testuser",
				Password: "testpass",
			},
			expectError: true,
		},
		{
			name: "no auth method",
			info: types.ConnectionInfo{
				Host: "localhost",
				User: "testuser",
			},
			expectError: true,
		},
		{
			name: "password auth",
			info: types.ConnectionInfo{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpass",
			},
			expectError: true, // Will fail without actual SSH server
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := NewSSHConnection()
			ctx := context.Background()
			
			err := conn.Connect(ctx, tt.info)
			
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestSSHConnection_BuildCommand(t *testing.T) {
	conn := NewSSHConnection()
	
	tests := []struct {
		name     string
		command  string
		options  types.ExecuteOptions
		expected string
	}{
		{
			name:     "simple command",
			command:  "ls -la",
			options:  types.ExecuteOptions{},
			expected: "ls -la",
		},
		{
			name:    "with working directory",
			command: "ls -la",
			options: types.ExecuteOptions{
				WorkingDir: "/tmp",
			},
			expected: "cd /tmp && ls -la",
		},
		{
			name:    "with sudo user",
			command: "systemctl start nginx",
			options: types.ExecuteOptions{
				Sudo: true,
				User: "root",
			},
			expected: "sudo -u root systemctl start nginx",
		},
		{
			name:    "with su user",
			command: "whoami",
			options: types.ExecuteOptions{
				User: "nginx",
			},
			expected: "su -c 'whoami' nginx",
		},
		{
			name:    "complex command",
			command: "echo 'test'",
			options: types.ExecuteOptions{
				WorkingDir: "/home/user",
				Sudo:       true,
				User:       "root",
			},
			expected: "cd /home/user && sudo -u root echo 'test'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conn.buildCommand(tt.command, tt.options)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSSHConnection_ParsePrivateKey(t *testing.T) {
	conn := NewSSHConnection()
	
	// Test invalid key
	_, err := conn.parsePrivateKey("invalid key content")
	if err == nil {
		t.Error("expected error for invalid key")
	}
	
	// Test non-existent file path
	_, err = conn.parsePrivateKey("/non/existent/key/file")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestSSHConnection_LoadDefaultKeys(t *testing.T) {
	conn := NewSSHConnection()
	
	// This will likely return empty signers in test environment
	signers, err := conn.loadDefaultKeys()
	
	// Should not error, even if no keys are found
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	// Signers can be empty in test environment
	if signers == nil {
		t.Error("expected non-nil signers slice")
	}
}

func TestSSHConnection_IsConnected(t *testing.T) {
	conn := NewSSHConnection()
	
	// Initially not connected
	if conn.IsConnected() {
		t.Error("expected connection to be false initially")
	}
	
	// After failed connection attempt, still not connected
	err := conn.Connect(context.Background(), types.ConnectionInfo{
		Host: "invalid-host-12345",
		User: "test",
		Password: "test",
	})
	
	if err == nil {
		t.Error("expected connection error")
	}
	
	if conn.IsConnected() {
		t.Error("expected connection to be false after failed connect")
	}
}

func TestSSHConnection_Close(t *testing.T) {
	conn := NewSSHConnection()
	
	// Should not error when closing unconnected connection
	err := conn.Close()
	if err != nil {
		t.Errorf("unexpected error closing unconnected connection: %v", err)
	}
	
	// Should mark as disconnected
	if conn.IsConnected() {
		t.Error("expected connection to be false after close")
	}
}

func TestSSHConnection_ExecuteWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	
	_, err := conn.Execute(ctx, "echo test", types.ExecuteOptions{})
	if err == nil {
		t.Error("expected error when executing without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_ExecuteStreamWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	
	_, err := conn.ExecuteStream(ctx, "echo test", types.ExecuteOptions{})
	if err == nil {
		t.Error("expected error when executing stream without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_CopyWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	
	err := conn.Copy(ctx, strings.NewReader("test content"), "/tmp/test", 0644)
	if err == nil {
		t.Error("expected error when copying without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_FetchWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	
	_, err := conn.Fetch(ctx, "/tmp/test")
	if err == nil {
		t.Error("expected error when fetching without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_FileExistsWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	
	_, err := conn.FileExists("/tmp/test")
	if err == nil {
		t.Error("expected error when checking file existence without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_CreateDirectoryWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	
	err := conn.CreateDirectory("/tmp/test", 0755)
	if err == nil {
		t.Error("expected error when creating directory without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_RemoveFileWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	
	err := conn.RemoveFile("/tmp/test")
	if err == nil {
		t.Error("expected error when removing file without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_GetHostnameWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	
	_, err := conn.GetHostname()
	if err == nil {
		t.Error("expected error when getting hostname without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_GetSystemInfoWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	
	_, err := conn.GetSystemInfo()
	if err == nil {
		t.Error("expected error when getting system info without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_ExecuteScriptWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	
	_, err := conn.ExecuteScript(ctx, "echo test", types.ExecuteOptions{})
	if err == nil {
		t.Error("expected error when executing script without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_PingWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	
	err := conn.Ping()
	if err == nil {
		t.Error("expected error when pinging without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSSHConnection_PortForwardWithoutConnection(t *testing.T) {
	conn := NewSSHConnection()
	
	err := conn.PortForward("localhost:8080", "localhost:80")
	if err == nil {
		t.Error("expected error when port forwarding without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

// Benchmark tests for performance
func BenchmarkSSHConnection_BuildCommand(b *testing.B) {
	conn := NewSSHConnection()
	options := types.ExecuteOptions{
		WorkingDir: "/tmp",
		User:       "testuser",
		Sudo:       true,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.buildCommand("echo test", options)
	}
}

func BenchmarkSSHConnection_LoadDefaultKeys(b *testing.B) {
	conn := NewSSHConnection()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.loadDefaultKeys()
	}
}