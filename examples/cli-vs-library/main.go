package main

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/gosinble/gosinble/examples/common"
	"github.com/gosinble/gosinble/pkg/types"
)

// This example demonstrates the difference between CLI and Library usage
func main() {
	fmt.Println("=== CLI vs Library Comparison ===")

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
	_ = common.CreateSampleInventory()
	
	// Create task with type-safe fields
	task := types.Task{
		Name:   "Install nginx",
		Module: types.TypePackage,
		Args: map[string]interface{}{
			"name":  "nginx",
			"state": "present",
		},
	}
	
	// Execute task on local host for demonstration
	result, err := common.ExecuteTaskOnLocal(ctx, task)
	if err != nil {
		fmt.Printf("Task execution failed: %v\n", err)
		return
	}
	
	// Rich, structured result object
	common.PrintResult(result)
	
	fmt.Println("\n--- Library Advantages ---")
	fmt.Printf("✅ Type-safe API (no string parsing)\n")
	fmt.Printf("✅ Rich error handling\n")
	fmt.Printf("✅ Structured results\n")
	fmt.Printf("✅ No subprocess overhead\n")
	fmt.Printf("✅ Direct memory sharing\n")
	fmt.Printf("✅ Programmatic control\n")
}