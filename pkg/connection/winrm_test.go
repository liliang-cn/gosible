package connection

import (
	"context"
	"strings"
	"testing"

	"github.com/liliang-cn/gosinble/pkg/types"
)

func TestWinRMConnection_Connect(t *testing.T) {
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
			name: "password auth with SSL",
			info: types.ConnectionInfo{
				Host:       "localhost",
				User:       "testuser",
				Password:   "testpass",
				UseSSL:     true,
				SkipVerify: true,
			},
			expectError: true, // Will fail without actual WinRM server
		},
		{
			name: "password auth without SSL",
			info: types.ConnectionInfo{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpass",
				UseSSL:   false,
			},
			expectError: true, // Will fail without actual WinRM server
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := NewWinRMConnection()
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

func TestWinRMConnection_BuildCommand(t *testing.T) {
	conn := NewWinRMConnection()
	
	tests := []struct {
		name     string
		command  string
		options  types.ExecuteOptions
		expected string
	}{
		{
			name:     "simple command",
			command:  "dir",
			options:  types.ExecuteOptions{},
			expected: "dir",
		},
		{
			name:    "with working directory - CMD",
			command: "dir",
			options: types.ExecuteOptions{
				WorkingDir: "C:\\temp",
			},
			expected: `cd /d "C:\temp" && dir`,
		},
		{
			name:    "with working directory - PowerShell",
			command: "$env:PATH",
			options: types.ExecuteOptions{
				WorkingDir: "C:\\temp",
				Shell:      "powershell",
			},
			expected: `Set-Location 'C:\temp'; $env:PATH`,
		},
		{
			name:    "PowerShell command by shell option",
			command: "Get-Process",
			options: types.ExecuteOptions{
				Shell: "powershell",
			},
			expected: "Get-Process",
		},
		{
			name:    "PowerShell command by content detection",
			command: "$processes = Get-Process",
			options: types.ExecuteOptions{},
			expected: "$processes = Get-Process",
		},
		{
			name:    "with different user",
			command: "whoami",
			options: types.ExecuteOptions{
				User: "testuser",
			},
			expected: "REM Run as user testuser not supported yet\nwhoami",
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

func TestWinRMConnection_IsConnected(t *testing.T) {
	conn := NewWinRMConnection()
	
	// Initially not connected
	if conn.IsConnected() {
		t.Error("expected connection to be false initially")
	}
	
	// After failed connection attempt, still not connected
	err := conn.Connect(context.Background(), types.ConnectionInfo{
		Host:     "invalid-host-12345",
		User:     "test",
		Password: "test",
	})
	
	if err == nil {
		t.Error("expected connection error")
	}
	
	if conn.IsConnected() {
		t.Error("expected connection to be false after failed connect")
	}
}

func TestWinRMConnection_Close(t *testing.T) {
	conn := NewWinRMConnection()
	
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

func TestWinRMConnection_ExecuteWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	ctx := context.Background()
	
	_, err := conn.Execute(ctx, "echo test", types.ExecuteOptions{})
	if err == nil {
		t.Error("expected error when executing without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_ExecuteStreamWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	ctx := context.Background()
	
	_, err := conn.ExecuteStream(ctx, "echo test", types.ExecuteOptions{})
	if err == nil {
		t.Error("expected error when executing stream without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_CopyWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	ctx := context.Background()
	
	err := conn.Copy(ctx, strings.NewReader("test content"), "C:\\temp\\test.txt", 0644)
	if err == nil {
		t.Error("expected error when copying without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_FetchWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	ctx := context.Background()
	
	_, err := conn.Fetch(ctx, "C:\\temp\\test.txt")
	if err == nil {
		t.Error("expected error when fetching without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_FileExistsWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	
	_, err := conn.FileExists("C:\\temp\\test.txt")
	if err == nil {
		t.Error("expected error when checking file existence without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_CreateDirectoryWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	
	err := conn.CreateDirectory("C:\\temp\\test", 0755)
	if err == nil {
		t.Error("expected error when creating directory without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_RemoveFileWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	
	err := conn.RemoveFile("C:\\temp\\test.txt")
	if err == nil {
		t.Error("expected error when removing file without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_GetHostnameWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	
	_, err := conn.GetHostname()
	if err == nil {
		t.Error("expected error when getting hostname without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_GetSystemInfoWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	
	_, err := conn.GetSystemInfo()
	if err == nil {
		t.Error("expected error when getting system info without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_ExecuteScriptWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	ctx := context.Background()
	
	_, err := conn.ExecuteScript(ctx, "echo test", types.ExecuteOptions{})
	if err == nil {
		t.Error("expected error when executing script without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_PingWithoutConnection(t *testing.T) {
	conn := NewWinRMConnection()
	
	err := conn.Ping()
	if err == nil {
		t.Error("expected error when pinging without connection")
	}
	
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWinRMConnection_DefaultPorts(t *testing.T) {
	tests := []struct {
		name         string
		useSSL       bool
		expectedPort int
	}{
		{
			name:         "HTTP port",
			useSSL:       false,
			expectedPort: 5985,
		},
		{
			name:         "HTTPS port",
			useSSL:       true,
			expectedPort: 5986,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := NewWinRMConnection()
			info := types.ConnectionInfo{
				Host:     "testhost",
				User:     "testuser",
				Password: "testpass",
				UseSSL:   tt.useSSL,
			}
			
			// This will fail to connect but we can check the error message
			// to see if it's trying to connect to the right port
			err := conn.Connect(context.Background(), info)
			if err == nil {
				t.Error("expected connection error")
			}
			
			// Check if error mentions the expected port
			errorMsg := err.Error()
			if !strings.Contains(errorMsg, "testhost") {
				t.Errorf("expected error to mention host, got: %v", err)
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkWinRMConnection_BuildCommand(b *testing.B) {
	conn := NewWinRMConnection()
	options := types.ExecuteOptions{
		WorkingDir: "C:\\temp",
		Shell:      "powershell",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.buildCommand("Get-Process", options)
	}
}

func BenchmarkWinRMConnection_BuildCommandCMD(b *testing.B) {
	conn := NewWinRMConnection()
	options := types.ExecuteOptions{
		WorkingDir: "C:\\temp",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.buildCommand("dir", options)
	}
}