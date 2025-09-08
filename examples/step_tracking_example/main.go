// Comprehensive example demonstrating step tracking in gosinble
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
	"github.com/liliang-cn/gosinble/pkg/connection"
	"github.com/liliang-cn/gosinble/pkg/inventory"
	"github.com/liliang-cn/gosinble/pkg/modules"
	"github.com/liliang-cn/gosinble/pkg/runner"
)

func main() {
	fmt.Println("🚀 Gosinble Step Tracking Example")
	fmt.Println("=================================")

	ctx := context.Background()

	// Example 1: Basic step tracking
	fmt.Println("\n📋 Example 1: Basic Step Tracking")
	if err := basicStepTrackingExample(ctx); err != nil {
		log.Printf("Basic step tracking failed: %v", err)
	}

	// Example 2: Deployment with detailed steps
	fmt.Println("\n🚀 Example 2: Deployment with Step Tracking")
	if err := deploymentStepTrackingExample(ctx); err != nil {
		log.Printf("Deployment step tracking failed: %v", err)
	}

	// Example 3: Multi-step task with progress
	fmt.Println("\n⚙️  Example 3: Multi-Step Task Runner")
	if err := multiStepTaskRunnerExample(ctx); err != nil {
		log.Printf("Multi-step task runner failed: %v", err)
	}

	// Example 4: Step failure and recovery
	fmt.Println("\n⚠️  Example 4: Step Failure Handling")
	if err := stepFailureExample(ctx); err != nil {
		log.Printf("Step failure example failed: %v", err)
	}

	fmt.Println("\n✅ All step tracking examples completed!")
}

// basicStepTrackingExample demonstrates basic step tracking functionality
func basicStepTrackingExample(ctx context.Context) error {
	conn := connection.NewLocalConnection()
	if err := conn.Connect(ctx, types.ConnectionInfo{}); err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	var streamConn types.StreamingConnection = conn

	// Define steps manually
	steps := []struct {
		id          string
		name        string
		description string
		command     string
	}{
		{
			id:          "check_system",
			name:        "System Check",
			description: "Check system information and requirements",
			command:     "echo 'Checking system...' && uname -a && sleep 0.5",
		},
		{
			id:          "prepare_env",
			name:        "Prepare Environment",
			description: "Set up environment variables and directories",
			command:     "echo 'Preparing environment...' && mkdir -p /tmp/step-example && sleep 0.3",
		},
		{
			id:          "run_task",
			name:        "Execute Main Task",
			description: "Run the primary task operation",
			command:     "echo 'Running main task...' && for i in {1..3}; do echo \"Task step $i\"; sleep 0.2; done",
		},
		{
			id:          "cleanup",
			name:        "Cleanup",
			description: "Clean up temporary files and resources",
			command:     "echo 'Cleaning up...' && rm -rf /tmp/step-example && echo 'Cleanup completed'",
		},
	}

	totalSteps := len(steps)
	completedSteps := make([]types.StepInfo, 0)

	fmt.Printf("   Starting %d-step process...\n", totalSteps)

	for i, stepDef := range steps {
		stepNumber := i + 1
		
		// Create step info
		step := types.StepInfo{
			ID:          stepDef.id,
			Name:        stepDef.name,
			Description: stepDef.description,
			Status:      types.StepRunning,
			StartTime:   time.Now(),
			Metadata: map[string]interface{}{
				"step_number": stepNumber,
				"total_steps": totalSteps,
				"command":     stepDef.command,
			},
		}

		fmt.Printf("\n   🔄 Step %d/%d: %s\n", stepNumber, totalSteps, step.Name)
		fmt.Printf("      📝 %s\n", step.Description)

		// Execute step with progress tracking
		progressCallback := func(progress types.ProgressInfo) {
			if progress.CurrentStep != nil {
				fmt.Printf("      📊 Progress: %.1f%% - %s\n", progress.Percentage, progress.Message)
			}
		}

		options := types.ExecuteOptions{
			StreamOutput:     true,
			ProgressCallback: progressCallback,
			Timeout:          10 * time.Second,
		}

		events, err := streamConn.ExecuteStream(ctx, stepDef.command, options)
		if err != nil {
			step.Status = types.StepFailed
			step.EndTime = time.Now()
			step.Duration = step.EndTime.Sub(step.StartTime)
			fmt.Printf("      ❌ Step failed: %v\n", err)
			return fmt.Errorf("step '%s' failed: %v", step.Name, err)
		}

		// Process events
		for event := range events {
			switch event.Type {
			case types.StreamStdout:
				fmt.Printf("      📤 %s\n", event.Data)
			case types.StreamStderr:
				fmt.Printf("      ⚠️  %s\n", event.Data)
			case types.StreamDone:
				if event.Result != nil && event.Result.Success {
					step.Status = types.StepCompleted
				} else {
					step.Status = types.StepFailed
				}
				step.EndTime = time.Now()
				step.Duration = step.EndTime.Sub(step.StartTime)
			case types.StreamError:
				step.Status = types.StepFailed
				step.EndTime = time.Now()
				step.Duration = step.EndTime.Sub(step.StartTime)
				return fmt.Errorf("step '%s' failed: %v", step.Name, event.Error)
			}
		}

		if step.Status == types.StepCompleted {
			fmt.Printf("      ✅ %s completed in %v\n", step.Name, step.Duration)
		}

		completedSteps = append(completedSteps, step)
	}

	// Summary
	fmt.Printf("\n   📊 Summary:\n")
	fmt.Printf("      ✅ %d steps completed\n", len(completedSteps))
	
	totalDuration := time.Since(completedSteps[0].StartTime)
	fmt.Printf("      ⏱️  Total time: %v\n", totalDuration)

	return nil
}

// deploymentStepTrackingExample demonstrates deployment with step tracking
func deploymentStepTrackingExample(ctx context.Context) error {
	// Create deployment module
	deploymentModule := modules.NewDeploymentModule()
	
	conn := connection.NewLocalConnection()
	if err := conn.Connect(ctx, types.ConnectionInfo{}); err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	fmt.Println("   Deploying sample application with step tracking...")

	args := map[string]interface{}{
		"app_name":            "sample-app",
		"version":             "v2.1.0",
		"deploy_path":         "/tmp/deployments",
		"health_check":        true,
		"rollback_on_failure": true,
	}

	result, err := deploymentModule.Run(ctx, conn, args)
	if err != nil {
		return fmt.Errorf("deployment failed: %v", err)
	}

	if result.Success {
		fmt.Printf("   📊 Deployment Summary:\n")
		if appName, ok := result.Data["app_name"].(string); ok {
			fmt.Printf("      📦 App: %s\n", appName)
		}
		if version, ok := result.Data["app_version"].(string); ok {
			fmt.Printf("      🏷️  Version: %s\n", version)
		}
		if totalSteps, ok := result.Data["total_steps"].(int); ok {
			fmt.Printf("      📋 Total Steps: %d\n", totalSteps)
		}
		if deployTime, ok := result.Data["deployment_time"].(string); ok {
			fmt.Printf("      ⏱️  Duration: %s\n", deployTime)
		}
	}

	return nil
}

// multiStepTaskRunnerExample demonstrates task runner with step tracking
func multiStepTaskRunnerExample(ctx context.Context) error {
	// Create inventory
	inv := inventory.NewStaticInventory()
	localhost := &types.Host{
		Name:    "localhost",
		Address: "127.0.0.1",
		User:    "root",
	}
	inv.AddHost(*localhost)

	// Create task runner
	taskRunner := runner.NewTaskRunner()

	// Define tasks with step-like structure
	tasks := []types.Task{
		{
			Name:   "Initialize project",
			Module: "streaming_shell",
			Args: map[string]interface{}{
				"cmd":           "echo 'Initializing project...' && mkdir -p /tmp/multi-step-project && echo 'Project initialized'",
				"stream_output": true,
			},
		},
		{
			Name:   "Install dependencies",
			Module: "streaming_shell", 
			Args: map[string]interface{}{
				"cmd":           "echo 'Installing dependencies...' && for dep in dep1 dep2 dep3; do echo \"Installing $dep\"; sleep 0.3; done",
				"stream_output": true,
			},
		},
		{
			Name:   "Build application",
			Module: "streaming_shell",
			Args: map[string]interface{}{
				"cmd":           "echo 'Building application...' && echo 'Compiling source...' && sleep 1 && echo 'Build completed'",
				"stream_output": true,
			},
		},
		{
			Name:   "Run tests",
			Module: "streaming_shell",
			Args: map[string]interface{}{
				"cmd":           "echo 'Running tests...' && for test in unit integration e2e; do echo \"Running $test tests\"; sleep 0.4; done && echo 'All tests passed'",
				"stream_output": true,
			},
		},
		{
			Name:   "Package application",
			Module: "streaming_shell",
			Args: map[string]interface{}{
				"cmd":           "echo 'Creating package...' && echo 'Compressing files...' && sleep 0.5 && echo 'Package created'",
				"stream_output": true,
			},
		},
	}

	totalTasks := len(tasks)
	fmt.Printf("   Executing %d tasks...\n", totalTasks)

	for i, task := range tasks {
		taskNumber := i + 1
		fmt.Printf("\n   🔄 Task %d/%d: %s\n", taskNumber, totalTasks, task.Name)

		hosts, _ := inv.GetHosts("all")
		results, err := taskRunner.Run(ctx, task, hosts, nil)
		if err != nil {
			fmt.Printf("      ❌ Task failed: %v\n", err)
			return fmt.Errorf("task '%s' failed: %v", task.Name, err)
		}

		for _, result := range results {
			if result.Success {
				status := "✨ OK"
				if result.Changed {
					status = "✅ Changed"
				}
				fmt.Printf("      %s: %s\n", status, result.Message)
				
				// Show progress information
				progress := float64(taskNumber) / float64(totalTasks) * 100
				fmt.Printf("      📊 Overall Progress: %.1f%% (%d/%d tasks)\n", progress, taskNumber, totalTasks)
			} else {
				fmt.Printf("      ❌ Failed: %v\n", result.Error)
				return fmt.Errorf("task '%s' failed: %v", task.Name, result.Error)
			}
		}
	}

	fmt.Printf("\n   ✅ All tasks completed successfully!\n")
	return nil
}

// stepFailureExample demonstrates step failure handling
func stepFailureExample(ctx context.Context) error {
	conn := connection.NewLocalConnection()
	if err := conn.Connect(ctx, types.ConnectionInfo{}); err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	var streamConn types.StreamingConnection = conn

	// Define steps with one that will fail
	steps := []struct {
		id          string
		name        string
		description string
		command     string
		critical    bool
	}{
		{
			id:          "step1",
			name:        "Successful Step",
			description: "This step will succeed",
			command:     "echo 'Step 1 success' && sleep 0.2",
			critical:    true,
		},
		{
			id:          "step2", 
			name:        "Failing Step",
			description: "This step will fail but is non-critical",
			command:     "echo 'Step 2 attempting...' && sleep 0.2 && exit 1",
			critical:    false,
		},
		{
			id:          "step3",
			name:        "Recovery Step",
			description: "This step will recover and continue",
			command:     "echo 'Step 3 recovering...' && sleep 0.2 && echo 'Recovered successfully'",
			critical:    true,
		},
	}

	totalSteps := len(steps)
	successfulSteps := 0
	failedSteps := 0

	fmt.Printf("   Testing failure handling with %d steps...\n", totalSteps)

	for i, stepDef := range steps {
		stepNumber := i + 1
		
		step := types.StepInfo{
			ID:          stepDef.id,
			Name:        stepDef.name,
			Description: stepDef.description,
			Status:      types.StepRunning,
			StartTime:   time.Now(),
			Metadata: map[string]interface{}{
				"critical":    stepDef.critical,
				"step_number": stepNumber,
				"command":     stepDef.command,
			},
		}

		fmt.Printf("\n   🔄 Step %d/%d: %s", stepNumber, totalSteps, step.Name)
		if stepDef.critical {
			fmt.Printf(" (Critical)")
		}
		fmt.Printf("\n      📝 %s\n", step.Description)

		options := types.ExecuteOptions{
			StreamOutput: true,
			Timeout:      5 * time.Second,
		}

		events, err := streamConn.ExecuteStream(ctx, stepDef.command, options)
		if err != nil {
			step.Status = types.StepFailed
			step.EndTime = time.Now()
			step.Duration = step.EndTime.Sub(step.StartTime)
			failedSteps++

			if stepDef.critical {
				fmt.Printf("      💥 Critical step failed: %v\n", err)
				return fmt.Errorf("critical step '%s' failed: %v", step.Name, err)
			} else {
				fmt.Printf("      ⚠️  Non-critical step failed: %v\n", err)
				fmt.Printf("      🔄 Continuing with next step...\n")
				continue
			}
		}

		// Process events
		stepSuccess := false
		for event := range events {
			switch event.Type {
			case types.StreamStdout:
				fmt.Printf("      📤 %s\n", event.Data)
			case types.StreamStderr:
				fmt.Printf("      ⚠️  %s\n", event.Data)
			case types.StreamDone:
				if event.Result != nil && event.Result.Success {
					step.Status = types.StepCompleted
					stepSuccess = true
					successfulSteps++
				} else {
					step.Status = types.StepFailed
					failedSteps++
				}
				step.EndTime = time.Now()
				step.Duration = step.EndTime.Sub(step.StartTime)
			case types.StreamError:
				step.Status = types.StepFailed
				step.EndTime = time.Now()
				step.Duration = step.EndTime.Sub(step.StartTime)
				failedSteps++

				if stepDef.critical {
					return fmt.Errorf("critical step '%s' failed: %v", step.Name, event.Error)
				} else {
					fmt.Printf("      ⚠️  Non-critical step failed: %v\n", event.Error)
					fmt.Printf("      🔄 Continuing with next step...\n")
				}
			}
		}

		if stepSuccess {
			fmt.Printf("      ✅ %s completed in %v\n", step.Name, step.Duration)
		} else if !stepDef.critical {
			fmt.Printf("      ⏭️  Skipped failed non-critical step\n")
		}
	}

	// Summary
	fmt.Printf("\n   📊 Failure Handling Summary:\n")
	fmt.Printf("      ✅ Successful steps: %d\n", successfulSteps)
	fmt.Printf("      ❌ Failed steps: %d\n", failedSteps)
	fmt.Printf("      📋 Total steps: %d\n", totalSteps)
	
	if failedSteps > 0 && successfulSteps > 0 {
		fmt.Printf("      🎯 Successfully demonstrated graceful failure handling\n")
	}

	return nil
}

// Helper function to create deployment module (placeholder)
// In a real implementation, this would be the actual DeploymentModule
func createMockDeploymentModule() interface{} {
	return struct {
		name string
	}{
		name: "deployment",
	}
}