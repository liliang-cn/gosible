package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/gosinble/gosinble/pkg/types"
	"github.com/gosinble/gosinble/pkg/inventory"
	"github.com/gosinble/gosinble/pkg/modules"
	"github.com/gosinble/gosinble/pkg/runner"
)

// This example demonstrates the difference between CLI and Library usage
func main() {
	fmt.Println("=== CLI vs Library Comparison ===\n")

	// Example task: Install nginx on web servers
	
	fmt.Println("1. Using CLI (subprocess call from Go):")
	fmt.Println("----------------------------------------")
	useCLI()
	
	fmt.Println("\n2. Using Library (direct Go API):")
	fmt.Println("----------------------------------")
	useLibrary()
}

// useCLI demonstrates calling gosinble as a CLI tool
func useCLI() {
	// This is NOT recommended from Go code!
	// Shows what happens when you shell out to CLI
	
	cmd := exec.Command("gosinble", 
		"-i", "inventory.yml",
		"-m", "package",
		"-a", "name=nginx state=present",
		"webservers")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Only get exit code and text output
		fmt.Printf("CLI Error: %v\n", err)
		fmt.Printf("Output: %s\n", output)
		return
	}
	
	// Output is just text, need to parse if you want structured data
	fmt.Printf("CLI Output: %s\n", output)
}

// useLibrary demonstrates using gosinble as a library
func useLibrary() {
	ctx := context.Background()
	
	// Create inventory programmatically
	inv := inventory.New()
	inv.AddHost(inventory.HostEntry{
		Name:    "web1.example.com",
		Address: "192.168.1.10",
		Groups:  []string{"webservers"},
	})
	inv.AddHost(inventory.HostEntry{
		Name:    "web2.example.com", 
		Address: "192.168.1.11",
		Groups:  []string{"webservers"},
	})
	
	// Create task with type-safe fields
	task := common.Task{
		Name:   "Install nginx",
		Module: common.TypePackage,
		Args: map[string]interface{}{
			"name":  "nginx",
			"state": "present",
		},
	}
	
	// Create runner
	taskRunner := runner.NewTaskRunner()
	
	// Get hosts (with proper error handling)
	hosts, err := inv.GetHosts("webservers")
	if err != nil {
		log.Fatalf("Failed to get hosts: %v", err)
	}
	
	// Execute with structured results
	results, err := taskRunner.Run(ctx, task, hosts, nil)
	if err != nil {
		// Proper error handling with context
		log.Fatalf("Task execution failed: %v", err)
	}
	
	// Process structured results
	for _, result := range results {
		fmt.Printf("Host: %s\n", result.Host)
		fmt.Printf("  Success: %v\n", result.Success)
		fmt.Printf("  Changed: %v\n", result.Changed)
		if result.Error != nil {
			fmt.Printf("  Error: %v\n", result.Error)
		}
		
		// Can access structured data
		if data, ok := result.Data["installed_version"]; ok {
			fmt.Printf("  Installed Version: %v\n", data)
		}
	}
}

// Advanced library usage showing features not available via CLI
func advancedLibraryFeatures() {
	ctx := context.Background()
	runner := runner.NewTaskRunner()
	
	// 1. Custom module registration (not possible via CLI)
	customModule := &CustomModule{
		name: "my_custom",
	}
	runner.RegisterModule(customModule)
	
	// 2. Event callbacks (not available in CLI)
	runner.SetEventCallback(func(event common.Event) {
		switch event.Type {
		case common.EventTaskStart:
			fmt.Printf("[EVENT] Task started: %s on %s\n", event.Task, event.Host)
		case common.EventTaskComplete:
			fmt.Printf("[EVENT] Task completed: %s on %s\n", event.Task, event.Host)
		}
	})
	
	// 3. Progress callbacks for long operations
	runner.SetProgressCallback(func(progress common.ProgressInfo) {
		fmt.Printf("Progress: %.1f%% - %s\n", progress.Percentage, progress.Message)
	})
	
	// 4. Custom connection management
	connMgr := runner.GetConnectionManager()
	connMgr.SetConnectionTTL(60 * time.Minute)
	connMgr.SetMaxConnections(100)
	
	// 5. Programmatic variable management
	varMgr := runner.GetVarManager()
	varMgr.SetVar("environment", "production")
	varMgr.SetVar("deploy_version", "1.2.3")
	
	// 6. Context with timeout (graceful cancellation)
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	
	// Use all these features together
	results, err := runner.Run(ctxWithTimeout, task, hosts, varMgr.GetVars())
	if err != nil {
		// Check if it was a timeout
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Println("Operation timed out")
		}
	}
}

// CustomModule example - only possible with library
type CustomModule struct {
	name string
}

func (m *CustomModule) Name() string {
	return m.name
}

func (m *CustomModule) Run(ctx context.Context, conn common.Connection, args map[string]interface{}) (*common.Result, error) {
	// Custom implementation
	return &common.Result{
		Success: true,
		Changed: false,
		Message: "Custom module executed",
	}, nil
}

func (m *CustomModule) Validate(args map[string]interface{}) error {
	// Custom validation
	return nil
}

func (m *CustomModule) Documentation() common.ModuleDoc {
	return common.ModuleDoc{
		Name:        m.name,
		Description: "Custom module for specific business logic",
	}
}