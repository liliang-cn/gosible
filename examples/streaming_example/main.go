// Complete example demonstrating real-time output streaming in gosinble
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
	"github.com/liliang-cn/gosinble/pkg/connection"
	"github.com/liliang-cn/gosinble/pkg/inventory"
	"github.com/liliang-cn/gosinble/pkg/runner"
)

func main() {
	fmt.Println("🚀 Gosinble Real-Time Streaming Example")
	fmt.Println("======================================")

	ctx := context.Background()

	// Example 1: Basic streaming with LocalConnection
	fmt.Println("\n📡 Example 1: Basic Local Streaming")
	if err := basicLocalStreamingExample(ctx); err != nil {
		log.Printf("Basic streaming example failed: %v", err)
	}

	// Example 2: Advanced streaming with callbacks
	fmt.Println("\n🔄 Example 2: Advanced Streaming with Callbacks")
	if err := advancedStreamingExample(ctx); err != nil {
		log.Printf("Advanced streaming example failed: %v", err)
	}

	// Example 3: Task runner with streaming
	fmt.Println("\n⚙️  Example 3: Task Runner with Streaming")
	if err := taskRunnerStreamingExample(ctx); err != nil {
		log.Printf("Task runner streaming example failed: %v", err)
	}

	// Example 4: Multiple commands with progress tracking
	fmt.Println("\n📊 Example 4: Multiple Commands with Progress")
	if err := multiCommandStreamingExample(ctx); err != nil {
		log.Printf("Multi-command streaming example failed: %v", err)
	}

	// Example 5: Error handling and timeouts
	fmt.Println("\n⚠️  Example 5: Error Handling and Timeouts")
	if err := errorHandlingExample(ctx); err != nil {
		log.Printf("Error handling example failed: %v", err)
	}

	fmt.Println("\n✅ All examples completed!")
}

// basicLocalStreamingExample demonstrates basic streaming functionality
func basicLocalStreamingExample(ctx context.Context) error {
	conn := connection.NewLocalConnection()
	if err := conn.Connect(ctx, types.ConnectionInfo{}); err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Check if connection supports streaming
	// LocalConnection implements StreamingConnection interface
	var streamConn types.StreamingConnection = conn

	fmt.Println("   Running: for i in {1..3}; do echo \"Line $i\"; sleep 0.5; done")

	options := types.ExecuteOptions{
		StreamOutput: true,
		Timeout:      10 * time.Second,
	}

	events, err := streamConn.ExecuteStream(ctx, "for i in {1..3}; do echo \"Line $i\"; sleep 0.5; done", options)
	if err != nil {
		return fmt.Errorf("ExecuteStream failed: %v", err)
	}

	for event := range events {
		switch event.Type {
		case types.StreamStdout:
			fmt.Printf("   📤 %s\n", event.Data)
		case types.StreamStderr:
			fmt.Printf("   ❌ %s\n", event.Data)
		case types.StreamProgress:
			if event.Progress != nil {
				fmt.Printf("   📈 %s\n", event.Progress.Message)
			}
		case types.StreamDone:
			fmt.Printf("   ✅ Command completed! (Duration: %v)\n", event.Result.Duration)
		case types.StreamError:
			return fmt.Errorf("stream error: %v", event.Error)
		}
	}

	return nil
}

// advancedStreamingExample demonstrates streaming with callbacks
func advancedStreamingExample(ctx context.Context) error {
	conn := connection.NewLocalConnection()
	if err := conn.Connect(ctx, types.ConnectionInfo{}); err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	var streamConn types.StreamingConnection = conn

	var outputLines []string
	var progressUpdates []types.ProgressInfo

	options := types.ExecuteOptions{
		StreamOutput: true,
		Timeout:      15 * time.Second,
		OutputCallback: func(line string, isStderr bool) {
			prefix := "STDOUT"
			if isStderr {
				prefix = "STDERR"
			}
			fmt.Printf("   🔸 [%s] %s\n", prefix, line)
			outputLines = append(outputLines, line)
		},
		ProgressCallback: func(progress types.ProgressInfo) {
			fmt.Printf("   📊 Progress: %.1f%% - %s (%s)\n", 
				progress.Percentage, progress.Message, progress.Stage)
			progressUpdates = append(progressUpdates, progress)
		},
	}

	fmt.Println("   Running: echo 'Starting...'; for i in {1..5}; do echo \"Processing item $i\"; sleep 0.3; done; echo 'Done!'")

	events, err := streamConn.ExecuteStream(ctx, "echo 'Starting...'; for i in {1..5}; do echo \"Processing item $i\"; sleep 0.3; done; echo 'Done!'", options)
	if err != nil {
		return fmt.Errorf("ExecuteStream failed: %v", err)
	}

	// Just consume the events, callbacks handle the output
	var finalResult *types.Result
	for event := range events {
		if event.Type == types.StreamDone {
			finalResult = event.Result
		}
	}

	if finalResult != nil {
		fmt.Printf("   📋 Summary: %d output lines, %d progress updates\n", 
			len(outputLines), len(progressUpdates))
	}

	return nil
}

// taskRunnerStreamingExample demonstrates using streaming with the task runner
func taskRunnerStreamingExample(ctx context.Context) error {
	// Create inventory
	inv := inventory.NewStaticInventory()
	localhost := &types.Host{
		Name:    "localhost",
		Address: "127.0.0.1",
		User:    "root",
		Variables: map[string]interface{}{
			"env": "development",
		},
	}
	inv.AddHost(*localhost)

	// Create task runner
	taskRunner := runner.NewTaskRunner()

	// Create streaming-aware tasks
	tasks := []types.Task{
		{
			Name:   "Test connectivity with streaming",
			Module: types.TypePing,
			Args:   map[string]interface{}{},
		},
		{
			Name:   "Create test directory",
			Module: types.TypeFile,
			Args: map[string]interface{}{
				"path":  "/tmp/gosinble-streaming-test",
				"state": types.StateDirectory,
				"mode":  "0755",
			},
		},
		{
			Name:   "Run streaming shell command",
			Module: "streaming_shell", // Use our new streaming shell module
			Args: map[string]interface{}{
				"cmd":           "echo 'Task runner streaming test'; for i in {1..3}; do echo \"Task step $i\"; sleep 0.2; done",
				"stream_output": true,
				"show_progress": true,
			},
		},
		{
			Name:   "Clean up test directory",
			Module: types.TypeFile,
			Args: map[string]interface{}{
				"path":  "/tmp/gosinble-streaming-test",
				"state": types.StateAbsent,
			},
		},
	}

	// Execute tasks
	fmt.Println("   Executing tasks with streaming support...")
	
	for i, task := range tasks {
		fmt.Printf("   \n   [%d/4] %s\n", i+1, task.Name)
		
		hosts, _ := inv.GetHosts("all")
		results, err := taskRunner.Run(ctx, task, hosts, nil)
		if err != nil {
			log.Printf("   ❌ Task failed: %v", err)
			continue
		}

		for _, result := range results {
			if result.Success {
				status := "✨ OK"
				if result.Changed {
					status = "✅ Changed"
				}
				fmt.Printf("   %s: %s\n", status, result.Message)
				
				// Show streaming metadata if available
				if streamingEnabled, ok := result.Data["streaming_enabled"].(bool); ok && streamingEnabled {
					if eventCount, ok := result.Data["stream_events_received"].(int); ok {
						fmt.Printf("   📡 Streaming: %d events received\n", eventCount)
					}
				}
			} else {
				fmt.Printf("   ❌ Failed: %v\n", result.Error)
			}
		}
	}

	return nil
}

// multiCommandStreamingExample demonstrates multiple parallel streaming commands
func multiCommandStreamingExample(ctx context.Context) error {
	conn := connection.NewLocalConnection()
	if err := conn.Connect(ctx, types.ConnectionInfo{}); err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	var streamConn types.StreamingConnection = conn

	commands := []struct {
		name string
		cmd  string
	}{
		{"Counter A", "for i in {1..3}; do echo \"Counter A: $i\"; sleep 0.4; done"},
		{"Counter B", "for i in {1..2}; do echo \"Counter B: $i\"; sleep 0.6; done"},
		{"List Files", "echo 'Listing files:'; ls -la /tmp | head -5"},
	}

	fmt.Println("   Running multiple commands with streaming...")

	for _, command := range commands {
		fmt.Printf("\n   🔸 %s:\n", command.name)

		options := types.ExecuteOptions{
			StreamOutput: true,
			Timeout:      10 * time.Second,
			OutputCallback: func(line string, isStderr bool) {
				fmt.Printf("     📤 %s\n", line)
			},
		}

		events, err := streamConn.ExecuteStream(ctx, command.cmd, options)
		if err != nil {
			log.Printf("     ❌ Failed: %v", err)
			continue
		}

		for event := range events {
			switch event.Type {
			case types.StreamProgress:
				if event.Progress != nil && event.Progress.Stage == "completed" {
					fmt.Printf("     ✅ %s completed\n", command.name)
				}
			case types.StreamDone:
				break
			case types.StreamError:
				fmt.Printf("     ❌ Error: %v\n", event.Error)
			}
		}
	}

	return nil
}

// errorHandlingExample demonstrates error handling and timeouts with streaming
func errorHandlingExample(ctx context.Context) error {
	conn := connection.NewLocalConnection()
	if err := conn.Connect(ctx, types.ConnectionInfo{}); err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	var streamConn types.StreamingConnection = conn

	// Test 1: Command that fails
	fmt.Println("   Test 1: Command that fails")
	options1 := types.ExecuteOptions{
		StreamOutput: true,
		Timeout:      5 * time.Second,
	}

	events1, err := streamConn.ExecuteStream(ctx, "echo 'This will fail'; exit 1", options1)
	if err != nil {
		return fmt.Errorf("ExecuteStream failed: %v", err)
	}

	for event := range events1 {
		switch event.Type {
		case types.StreamStdout:
			fmt.Printf("     📤 %s\n", event.Data)
		case types.StreamDone:
			if event.Result.Success {
				fmt.Printf("     ❌ Expected failure but command succeeded\n")
			} else {
				fmt.Printf("     ✅ Command failed as expected: %v\n", event.Result.Error)
			}
		}
	}

	// Test 2: Command with timeout
	fmt.Println("\n   Test 2: Command with timeout")
	options2 := types.ExecuteOptions{
		StreamOutput: true,
		Timeout:      1 * time.Second, // Short timeout
	}

	start := time.Now()
	events2, err := streamConn.ExecuteStream(ctx, "echo 'Starting long command'; sleep 5; echo 'Should not reach here'", options2)
	if err != nil {
		return fmt.Errorf("ExecuteStream failed: %v", err)
	}

	for event := range events2 {
		switch event.Type {
		case types.StreamStdout:
			fmt.Printf("     📤 %s\n", event.Data)
		case types.StreamDone:
			elapsed := time.Since(start)
			if elapsed > 3*time.Second {
				fmt.Printf("     ❌ Timeout didn't work, took %v\n", elapsed)
			} else {
				fmt.Printf("     ✅ Timeout worked correctly, took %v\n", elapsed)
				if !event.Result.Success {
					fmt.Printf("     ℹ️  Error: %v\n", event.Result.Error)
				}
			}
		}
	}

	// Test 3: Context cancellation
	fmt.Println("\n   Test 3: Context cancellation")
	cancelCtx, cancel := context.WithCancel(ctx)
	
	options3 := types.ExecuteOptions{
		StreamOutput: true,
	}

	events3, err := streamConn.ExecuteStream(cancelCtx, "echo 'Starting'; sleep 10; echo 'Should not reach here'", options3)
	if err != nil {
		return fmt.Errorf("ExecuteStream failed: %v", err)
	}

	// Cancel after short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		fmt.Printf("     🛑 Cancelling context...\n")
		cancel()
	}()

	start = time.Now()
	for event := range events3 {
		switch event.Type {
		case types.StreamStdout:
			fmt.Printf("     📤 %s\n", event.Data)
		case types.StreamDone:
			elapsed := time.Since(start)
			if elapsed > 2*time.Second {
				fmt.Printf("     ❌ Cancellation didn't work, took %v\n", elapsed)
			} else {
				fmt.Printf("     ✅ Cancellation worked, took %v\n", elapsed)
			}
		}
	}

	return nil
}

// Helper function to register streaming modules (if needed)
func init() {
	// This would be done in the module registry in a real application
	// For demo purposes, we're showing how streaming modules could be registered
	fmt.Println("🔧 Streaming modules initialized")
}