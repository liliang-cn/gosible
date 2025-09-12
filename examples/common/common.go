// Package common provides shared utilities for examples
package common

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/liliang-cn/gosible/pkg/connection"
	"github.com/liliang-cn/gosible/pkg/inventory"
	"github.com/liliang-cn/gosible/pkg/runner"
	"github.com/liliang-cn/gosible/pkg/types"
)

// PrintHeader prints a formatted header for examples
func PrintHeader(title string) {
	fmt.Printf("\n=== %s ===\n", title)
	fmt.Println(strings.Repeat("=", len(title)+8))
	fmt.Println()
}

// PrintSubHeader prints a formatted subheader
func PrintSubHeader(title string) {
	fmt.Printf("\n--- %s ---\n", title)
}

// PrintStep prints a numbered step
func PrintStep(stepNum int, description string) {
	fmt.Printf("%d. %s\n", stepNum, description)
}

// PrintResult prints the result of an operation
func PrintResult(result *types.Result) {
	status := "✅ SUCCESS"
	if !result.Success {
		status = "❌ FAILED"
	}

	fmt.Printf("   %s: %s", status, result.Message)
	if result.Error != nil {
		fmt.Printf(" (Error: %v)", result.Error)
	}
	fmt.Println()

	if result.Changed {
		fmt.Println("   📝 Changes were made")
	}

	if result.Duration > 0 {
		fmt.Printf("   ⏱️  Duration: %v\n", result.Duration)
	}
}

// PrintError prints an error message
func PrintError(err error) {
	fmt.Printf("❌ Error: %v\n", err)
}

// PrintWarning prints a warning message
func PrintWarning(msg string) {
	fmt.Printf("⚠️  Warning: %s\n", msg)
}

// PrintInfo prints an info message
func PrintInfo(msg string) {
	fmt.Printf("ℹ️  Info: %s\n", msg)
}

// CreateSampleInventory creates a sample inventory for examples
func CreateSampleInventory() *inventory.StaticInventory {
	inv := inventory.NewStaticInventory()

	// Add sample hosts
	hosts := []types.Host{
		{
			Name:    "web1",
			Address: "192.168.1.10",
			User:    "ubuntu",
			Groups:  []string{"webservers", "production"},
			Variables: map[string]interface{}{
				"role":        "web",
				"environment": "production",
			},
		},
		{
			Name:    "web2",
			Address: "192.168.1.11",
			User:    "ubuntu",
			Groups:  []string{"webservers", "production"},
			Variables: map[string]interface{}{
				"role":        "web",
				"environment": "production",
			},
		},
		{
			Name:    "db1",
			Address: "192.168.1.20",
			User:    "postgres",
			Groups:  []string{"databases", "production"},
			Variables: map[string]interface{}{
				"role":        "database",
				"environment": "production",
			},
		},
		{
			Name:    "local",
			Address: "localhost",
			User:    os.Getenv("USER"),
			Groups:  []string{"local"},
			Variables: map[string]interface{}{
				"role":        "local",
				"environment": "development",
			},
		},
	}

	for _, host := range hosts {
		inv.AddHost(host)
	}

	// Add groups
	groups := []types.Group{
		{
			Name:  "webservers",
			Hosts: []string{"web1", "web2"},
			Variables: map[string]interface{}{
				"http_port":  80,
				"https_port": 443,
			},
		},
		{
			Name:  "databases",
			Hosts: []string{"db1"},
			Variables: map[string]interface{}{
				"db_port": 5432,
			},
		},
		{
			Name:  "production",
			Hosts: []string{"web1", "web2", "db1"},
			Variables: map[string]interface{}{
				"env":    "prod",
				"backup": true,
			},
		},
		{
			Name:  "local",
			Hosts: []string{"local"},
			Variables: map[string]interface{}{
				"env":    "dev",
				"backup": false,
			},
		},
	}

	for _, group := range groups {
		inv.AddGroup(group)
	}

	return inv
}

// CreateLocalRunner creates a runner configured for local execution
func CreateLocalRunner() *runner.TaskRunner {
	r := runner.NewTaskRunner()

	// Register common modules
	// (In a real implementation, modules would be registered here)

	return r
}

// ExecuteTaskOnLocal executes a task on the local host
func ExecuteTaskOnLocal(ctx context.Context, task types.Task) (*types.Result, error) {
	// Create local connection
	conn := connection.NewLocalConnection()

	// Connect
	info := types.ConnectionInfo{Type: "local"}
	if err := conn.Connect(ctx, info); err != nil {
		return nil, fmt.Errorf("failed to connect locally: %w", err)
	}
	defer conn.Close()

	// For this example, we'll simulate task execution with a command
	command := fmt.Sprintf("echo 'Executing task: %s'", task.Name)
	if cmd, ok := task.Args["command"].(string); ok {
		command = cmd
	}

	result, err := conn.Execute(ctx, command, types.ExecuteOptions{})
	if err != nil {
		return nil, err
	}

	// Enhance result with task information
	result.TaskName = task.Name
	result.ModuleName = string(task.Module)

	return result, nil
}

// WaitForInput waits for user to press Enter
func WaitForInput(prompt string) {
	fmt.Printf("%s (Press Enter to continue...)", prompt)
	fmt.Scanln()
}

// SafeExit exits the program with a message
func SafeExit(code int, message string) {
	if message != "" {
		if code == 0 {
			PrintInfo(message)
		} else {
			PrintError(fmt.Errorf("%s", message))
		}
	}
	os.Exit(code)
}

// RetryWithBackoff executes a function with exponential backoff
func RetryWithBackoff(ctx context.Context, maxRetries int, baseDelay time.Duration, fn func() error) error {
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			PrintInfo(fmt.Sprintf("Retrying in %v (attempt %d/%d)", delay, attempt+1, maxRetries))

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err = fn()
		if err == nil {
			return nil
		}

		PrintWarning(fmt.Sprintf("Attempt %d failed: %v", attempt+1, err))
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, err)
}

// CreateProgressCallback creates a progress callback function
func CreateProgressCallback() func(types.ProgressInfo) {
	return func(progress types.ProgressInfo) {
		fmt.Printf("📊 Progress: %s - %.1f%% - %s\n",
			progress.Stage, progress.Percentage, progress.Message)
	}
}

// CreateOutputCallback creates an output callback function
func CreateOutputCallback() func(string, bool) {
	return func(line string, isStderr bool) {
		prefix := "📤"
		if isStderr {
			prefix = "📥"
		}
		fmt.Printf("%s %s\n", prefix, line)
	}
}
