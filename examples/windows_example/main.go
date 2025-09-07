package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/gosinble/gosinble/pkg/connection"
	"github.com/gosinble/gosinble/pkg/types"
)

func main() {
	fmt.Println("=== Gosinble Windows Compatibility Example ===")
	fmt.Printf("Running on: %s/%s\n\n", runtime.GOOS, runtime.GOARCH)

	// Demonstrate that gosinble can run ON Windows machines
	demonstrateLocalExecution()
	
	// Demonstrate connecting TO Windows machines from any OS
	demonstrateWinRMConnection()
	
	// Show cross-platform file path handling
	demonstratePathHandling()
}

func demonstrateLocalExecution() {
	fmt.Println("1. Local Execution on Current OS:")
	fmt.Println("==================================")
	
	// Create local connection (works on any OS)
	conn := connection.NewLocalConnection()
	ctx := context.Background()
	
	info := types.ConnectionInfo{Type: "local"}
	if err := conn.Connect(ctx, info); err != nil {
		fmt.Printf("Failed to connect locally: %v\n", err)
		return
	}
	defer conn.Close()
	
	// Execute OS-appropriate commands
	var commands []string
	if runtime.GOOS == "windows" {
		commands = []string{
			"echo Running on Windows",
			"ver",
			"whoami",
			"cd",
		}
	} else {
		commands = []string{
			"echo Running on Unix-like system",
			"uname -a",
			"whoami",
			"pwd",
		}
	}
	
	for _, cmd := range commands {
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			fmt.Printf("Command failed: %v\n", err)
			continue
		}
		
		output := result.Data["stdout"].(string)
		fmt.Printf("$ %s\n%s", cmd, output)
	}
}

func demonstrateWinRMConnection() {
	fmt.Println("\n2. WinRM Connection to Windows Hosts:")
	fmt.Println("=====================================")
	
	// Show how to connect to Windows from any OS (including Windows)
	conn := connection.NewWinRMConnection()
	
	info := types.ConnectionInfo{
		Type:       "winrm",
		Host:       "windows-server.example.com",
		Port:       5985, // HTTP WinRM
		User:       "Administrator",
		Password:   "SecurePassword123",
		UseSSL:     false,
		SkipVerify: true,
		Timeout:    30 * time.Second,
	}
	
	fmt.Println("This demonstrates WinRM client setup (connection will fail without real Windows host)")
	fmt.Printf("Target: %s:%d (SSL: %v)\n", info.Host, info.Port, info.UseSSL)
	fmt.Printf("User: %s\n", info.User)
	
	ctx := context.Background()
	
	// This will fail without a real Windows host, but shows the setup
	if err := conn.Connect(ctx, info); err != nil {
		fmt.Printf("Connection failed (expected without real host): %v\n", err)
	} else {
		defer conn.Close()
		
		// Example Windows commands
		commands := []struct {
			cmd   string
			shell string
			desc  string
		}{
			{"Get-ComputerInfo", "powershell", "PowerShell system info"},
			{"dir C:\\", "", "CMD directory listing"},
			{"$PSVersionTable", "powershell", "PowerShell version"},
		}
		
		for _, cmdInfo := range commands {
			fmt.Printf("\nExecuting %s: %s\n", cmdInfo.desc, cmdInfo.cmd)
			
			options := types.ExecuteOptions{}
			if cmdInfo.shell != "" {
				options.Shell = cmdInfo.shell
			}
			
			result, err := conn.Execute(ctx, cmdInfo.cmd, options)
			if err != nil {
				fmt.Printf("Failed: %v\n", err)
			} else {
				fmt.Printf("Success: %v\n", result.Success)
				if result.Data["stdout"] != nil {
					fmt.Printf("Output: %s\n", result.Data["stdout"])
				}
			}
		}
	}
}

func demonstratePathHandling() {
	fmt.Println("\n3. Cross-Platform Path Handling:")
	fmt.Println("================================")
	
	// Show path handling examples
	paths := map[string][]string{
		"Unix/Linux": {
			"/etc/hosts",
			"/tmp/test.txt",
			"/home/user/.bashrc",
		},
		"Windows": {
			"C:\\Windows\\System32\\drivers\\etc\\hosts",
			"C:\\temp\\test.txt",
			"C:\\Users\\user\\.bashrc",
		},
	}
	
	fmt.Printf("Current OS: %s\n", runtime.GOOS)
	
	for osType, pathList := range paths {
		fmt.Printf("\n%s paths:\n", osType)
		for _, path := range pathList {
			fmt.Printf("  • %s\n", path)
		}
	}
	
	// Show connection type selection logic
	fmt.Println("\nConnection Type Selection:")
	
	testConnInfo := []types.ConnectionInfo{
		{Type: "ssh", Host: "linux-server"},
		{Type: "winrm", Host: "windows-server"},
		{Type: "local"},
	}
	
	for _, info := range testConnInfo {
		var connType string
		switch info.Type {
		case "winrm":
			connType = "WinRM (Windows)"
		case "ssh":
			connType = "SSH (Unix/Linux)"
		case "local":
			connType = fmt.Sprintf("Local (%s)", runtime.GOOS)
		default:
			connType = "Unknown"
		}
		
		fmt.Printf("  • %s -> %s\n", info.Host, connType)
		if info.IsWindows() {
			fmt.Printf("    └─ Uses Windows-specific features\n")
		}
	}
}