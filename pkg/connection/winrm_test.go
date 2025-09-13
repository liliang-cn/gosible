package connection

import (
	"context"
	"strings"
	"testing"

	"github.com/liliang-cn/gosible/pkg/types"
)

func TestWinRMConnection_Connect(t *testing.T) {
	t.Skip("Skipping test that attempts real WinRM connections")
	
	// These tests attempt real network connections which can be slow.
	// In a real test environment, we would use a mock WinRM server or
	// test against a containerized Windows server.
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
	
	// Skip the actual connection test that would timeout
	t.Skip("Skipping test that attempts real WinRM connection")
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
	t.Skip("Skipping test that attempts real WinRM connections")
	
	// This test attempts to connect to hosts which causes DNS lookups
	// and connection timeouts. In a real test environment, we would use
	// a mock WinRM client or test against a containerized Windows server.
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