package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
	"github.com/liliang-cn/gosiblepkg/connection"
	"github.com/liliang-cn/gosiblepkg/logging"
	"github.com/liliang-cn/gosiblepkg/websocket"
)

// StepTrackingIntegrationDemo demonstrates how the enhanced features
// work together with the existing step tracking system
func main() {
	fmt.Println("🎯 gosible Step Tracking Integration Demo")
	fmt.Println("==========================================")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Set up integrated logging and WebSocket
	logger, wsServer := setupIntegratedSystems()
	defer logger.Close()
	defer wsServer.Stop()

	// Create connection
	conn := connection.NewLocalConnection()
	if err := conn.Connect(ctx, types.ConnectionInfo{}); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Demonstrate integration with step tracking
	demoIntegratedDeployment(ctx, conn, wsServer, logger)

	fmt.Println("\n✅ Step tracking integration demo completed!")
}

func setupIntegratedSystems() (*logging.StreamLogger, *websocket.StreamServer) {
	// Set up logging
	logger := logging.NewStreamLogger("step_integration", "integration_session")
	logger.AddConsoleOutput("text", true)
	logger.AddMemoryOutput(100)
	logger.SetLevel(logging.LevelDebug)
	logger.SetFilters(true, true, true, true)

	// Set up WebSocket server
	wsServer := websocket.NewStreamServer()
	wsServer.Start()

	fmt.Println("📡 Integrated systems ready (logging + WebSocket)")

	return logger, wsServer
}

func demoIntegratedDeployment(ctx context.Context, conn *connection.LocalConnection, wsServer *websocket.StreamServer, logger *logging.StreamLogger) {
	fmt.Println("\n🚀 Simulating Multi-Step Deployment with Full Integration")

	// Define deployment steps
	steps := []struct {
		id          string
		name        string
		description string
		duration    time.Duration
		critical    bool
	}{
		{"validate", "Validate Environment", "Check system requirements and permissions", 2 * time.Second, true},
		{"backup", "Backup Current Version", "Create backup of existing application", 3 * time.Second, true},
		{"download", "Download Package", "Download new application version", 5 * time.Second, true},
		{"extract", "Extract Files", "Extract application files from package", 2 * time.Second, true},
		{"configure", "Configure Application", "Update configuration files", 3 * time.Second, false},
		{"permissions", "Set Permissions", "Set proper file permissions", 1 * time.Second, false},
		{"start", "Start Services", "Start application services", 2 * time.Second, true},
		{"verify", "Health Check", "Verify application is running correctly", 3 * time.Second, true},
	}

	totalSteps := len(steps)
	completedSteps := []types.StepInfo{}

	fmt.Printf("📋 Starting deployment with %d steps\n", totalSteps)

	// Log deployment start
	logger.Log(logging.LevelInfo, "Deployment started", map[string]interface{}{
		"total_steps": totalSteps,
		"target":      "demo-server",
		"version":     "v2.1.0",
	})

	// Broadcast deployment start
	wsServer.BroadcastStreamEvent(types.StreamEvent{
		Type:      types.StreamStdout,
		Data:      fmt.Sprintf("🚀 Starting deployment with %d steps", totalSteps),
		Timestamp: time.Now(),
	}, "deployment")

	// Execute each step with full integration
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
				"critical":    stepDef.critical,
			},
		}

		// Log step start
		logger.LogStep(step, "deployment", "demo-server")

		// Broadcast step start
		wsServer.BroadcastStreamEvent(types.StreamEvent{
			Type:      types.StreamStepStart,
			Step:      &step,
			Timestamp: time.Now(),
		}, "deployment")

		// Show progress
		fmt.Printf("🔄 Step %d/%d: %s\n", stepNumber, totalSteps, step.Name)
		fmt.Printf("   📝 %s\n", step.Description)

		// Simulate step execution with progress updates
		executeStepWithProgress(step, stepDef.duration, wsServer, logger)

		// Complete the step
		step.Status = types.StepCompleted
		step.EndTime = time.Now()
		step.Duration = step.EndTime.Sub(step.StartTime)
		completedSteps = append(completedSteps, step)

		// Log step completion
		logger.LogStep(step, "deployment", "demo-server")

		// Broadcast step completion
		wsServer.BroadcastStreamEvent(types.StreamEvent{
			Type:      types.StreamStepEnd,
			Step:      &step,
			Timestamp: time.Now(),
		}, "deployment")

		// Update overall progress
		overallProgress := types.ProgressInfo{
			Stage:          "deployment",
			Percentage:     float64(stepNumber) / float64(totalSteps) * 100,
			Message:        fmt.Sprintf("Completed step %d/%d: %s", stepNumber, totalSteps, step.Name),
			Timestamp:      time.Now(),
			CurrentStep:    &step,
			CompletedSteps: completedSteps,
			TotalSteps:     totalSteps,
			StepNumber:     stepNumber,
		}

		// Log progress
		logger.LogProgress(overallProgress, "deployment", "demo-server")

		// Broadcast progress
		wsServer.BroadcastProgress(overallProgress, "deployment")

		fmt.Printf("   ✅ %s completed in %v\n", step.Name, step.Duration)

		// Small delay between steps
		time.Sleep(500 * time.Millisecond)
	}

	// Final deployment completion
	totalDuration := time.Since(completedSteps[0].StartTime)

	logger.Log(logging.LevelInfo, "Deployment completed successfully", map[string]interface{}{
		"total_steps":     totalSteps,
		"completed_steps": len(completedSteps),
		"total_duration":  totalDuration.String(),
		"success_rate":    "100%",
	})

	// Broadcast final completion
	wsServer.BroadcastStreamEvent(types.StreamEvent{
		Type: types.StreamDone,
		Data: "Deployment completed successfully!",
		Result: &types.Result{
			Success:    true,
			Message:    fmt.Sprintf("All %d deployment steps completed", totalSteps),
			StartTime:  completedSteps[0].StartTime,
			EndTime:    time.Now(),
			Duration:   totalDuration,
			ModuleName: "deployment",
			Data: map[string]interface{}{
				"steps":           completedSteps,
				"total_duration":  totalDuration.String(),
				"completed_steps": len(completedSteps),
			},
		},
		Timestamp: time.Now(),
	}, "deployment")

	// Show final summary
	fmt.Println("\n🎉 Deployment Summary")
	fmt.Println("=====================")
	fmt.Printf("📊 Total steps: %d/%d completed\n", len(completedSteps), totalSteps)
	fmt.Printf("⏱️  Total time: %v\n", totalDuration)
	fmt.Printf("📈 Success rate: 100%%\n")

	fmt.Println("\n📋 Step Details:")
	for i, step := range completedSteps {
		fmt.Printf("  %d. %-25s %v\n", i+1, step.Name, step.Duration)
	}
}

func executeStepWithProgress(step types.StepInfo, duration time.Duration, wsServer *websocket.StreamServer, logger *logging.StreamLogger) {
	// Simulate step execution with periodic progress updates
	updateInterval := duration / 4 // 4 progress updates per step

	for i := 1; i <= 4; i++ {
		time.Sleep(updateInterval)

		percentage := float64(i) * 25.0 // 25%, 50%, 75%, 100%

		// Create progress update
		progress := types.ProgressInfo{
			Stage:       "step_execution",
			Percentage:  percentage,
			Message:     fmt.Sprintf("Executing %s... %d%% complete", step.Name, int(percentage)),
			Timestamp:   time.Now(),
			CurrentStep: &step,
		}

		// Log progress
		logger.LogProgress(progress, "deployment", "demo-server")

		// Broadcast progress update
		wsServer.BroadcastStreamEvent(types.StreamEvent{
			Type:      types.StreamStepUpdate,
			Step:      &step,
			Progress:  &progress,
			Timestamp: time.Now(),
		}, "deployment")

		if i < 4 { // Don't show 100% here as it will be shown in completion
			fmt.Printf("   📊 %s: %.0f%% complete\n", step.Name, percentage)
		}
	}
}

func init() {
	fmt.Println(`
🎯 Step Tracking Integration Features:

✨ Demonstrated Integrations:
• Step-by-step deployment with real-time tracking
• WebSocket broadcasting of step events (start, update, end)
• Comprehensive logging of each step with metadata
• Progress tracking across entire multi-step process
• Integration of step info with streaming events
• Performance timing and duration tracking

📡 WebSocket Events Generated:
• step_start - When each step begins
• step_update - Progress updates during step execution  
• step_end - When each step completes
• progress - Overall deployment progress
• stream_event - General deployment events

📝 Logging Integration:
• Step lifecycle logging (start, progress, completion)
• Metadata tracking (step number, critical status, etc.)
• Performance metrics (duration, success rate)
• Structured logging for easy analysis

🔧 Real-World Applications:
• Web dashboard with live deployment progress
• Automated deployment pipelines with detailed tracking
• Operations monitoring with step-by-step visibility
• Error handling with precise step failure identification
• Performance analysis and optimization opportunities

This demonstrates how gosibles enhanced features provide
enterprise-grade visibility and control for complex operations.
	`)
}
