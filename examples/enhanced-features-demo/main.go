package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
	"github.com/liliang-cn/gosiblepkg/connection"
	"github.com/liliang-cn/gosiblepkg/logging"
	"github.com/liliang-cn/gosiblepkg/modules"
	"github.com/liliang-cn/gosiblepkg/websocket"
)

// EnhancedFeaturesDemo demonstrates all the enhanced gosiblefeatures:
// 1. File transfer progress tracking
// 2. WebSocket streaming for real-time web UI updates
// 3. Comprehensive logging integration
func main() {
	fmt.Println("🚀 gosible Enhanced Features Demo")
	fmt.Println("===================================")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up comprehensive logging
	logger := setupLogging()
	defer logger.Close()

	// Start WebSocket server for real-time updates
	webSocketServer := setupWebSocketServer()
	defer webSocketServer.Stop()

	// Create connection with progress support
	conn := connection.NewLocalConnection()
	if err := conn.Connect(ctx, types.ConnectionInfo{}); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Demo 1: File transfer with progress tracking
	fmt.Println("\n📁 Demo 1: Enhanced File Transfer with Progress")
	demoEnhancedFileTransfer(ctx, conn, webSocketServer, logger)

	// Demo 2: WebSocket integration for real-time updates
	fmt.Println("\n🌐 Demo 2: WebSocket Real-Time Streaming")
	demoWebSocketStreaming(ctx, conn, webSocketServer, logger)

	// Demo 3: Comprehensive logging integration
	fmt.Println("\n📝 Demo 3: Comprehensive Logging Integration")
	demoLoggingIntegration(ctx, conn, logger)

	// Show final statistics
	showFinalStats(logger)

	fmt.Println("\n✅ Demo completed successfully!")
	fmt.Println("Check the log files and WebSocket console for detailed output")
}

func setupLogging() *logging.StreamLogger {
	fmt.Println("Setting up comprehensive logging...")

	logger := logging.NewStreamLogger("enhanced_demo", "demo_session_001")

	// Add file logging
	if err := logger.AddFileOutput("./demo_logs.json"); err != nil {
		log.Printf("Failed to add file output: %v", err)
	}

	// Add console logging with colors
	logger.AddConsoleOutput("text", true)

	// Add memory logging for statistics
	memOutput := logger.AddMemoryOutput(500)
	if memOutput == nil {
		log.Printf("Failed to add memory output")
	}

	// Set debug level for detailed logging
	logger.SetLevel(logging.LevelDebug)

	// Enable all log types
	logger.SetFilters(true, true, true, true)

	logger.Log(logging.LevelInfo, "Logging system initialized", map[string]interface{}{
		"outputs": []string{"file", "console", "memory"},
		"level":   "debug",
		"session": "demo_session_001",
	})

	return logger
}

func setupWebSocketServer() *websocket.StreamServer {
	fmt.Println("Starting WebSocket server for real-time updates...")

	server := websocket.NewStreamServer()
	server.Start()

	// Set up HTTP handler for WebSocket connections
	http.HandleFunc("/ws", server.HandleWebSocket)

	// Start HTTP server in background
	go func() {
		fmt.Println("WebSocket server listening on :8080/ws")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Printf("WebSocket HTTP server failed: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	return server
}

func demoEnhancedFileTransfer(ctx context.Context, conn *connection.LocalConnection, wsServer *websocket.StreamServer, logger *logging.StreamLogger) {
	// Create enhanced copy module
	copyModule := modules.NewEnhancedCopyModule()

	// Create test content
	testContent := `
# Enhanced gosible Demo File
This file demonstrates the enhanced file transfer capabilities of gosible

Features demonstrated:
- Real-time progress tracking
- WebSocket streaming to web clients  
- Comprehensive logging integration
- Backup functionality
- Performance metrics

Generated at: ` + time.Now().Format(time.RFC3339) + `
	`

	// Enhanced copy with all features enabled
	args := map[string]interface{}{
		"content":       strings.TrimSpace(testContent),
		"dest":          "./demo_output.txt",
		"mode":          "0644",
		"backup":        true,
		"show_progress": true,
	}

	fmt.Println("  🔄 Copying file with progress tracking...")

	// Log the operation start
	logger.Log(logging.LevelInfo, "Starting enhanced file copy", map[string]interface{}{
		"module":      "enhanced_copy",
		"destination": args["dest"],
		"has_backup":  args["backup"],
		"mode":        args["mode"],
	})

	result, err := copyModule.Run(ctx, conn, args)
	if err != nil {
		logger.Log(logging.LevelError, "File copy failed", map[string]interface{}{
			"error": err.Error(),
		})
		fmt.Printf("  ❌ Copy failed: %v\n", err)
		return
	}

	// Log successful completion
	logger.Log(logging.LevelInfo, "Enhanced file copy completed", map[string]interface{}{
		"success":        result.Success,
		"file_size":      result.Data["file_size"],
		"transfer_time":  result.Data["transfer_time"],
		"average_speed":  result.Data["average_speed"],
		"backup_created": result.Data["backup_created"],
		"backup_path":    result.Data["backup_path"],
	})

	// Broadcast completion to WebSocket clients
	wsServer.BroadcastStreamEvent(types.StreamEvent{
		Type:      types.StreamDone,
		Data:      fmt.Sprintf("File copy completed: %s", result.Message),
		Result:    result,
		Timestamp: time.Now(),
	}, "enhanced_copy")

	fmt.Printf("  ✅ Copy completed successfully!\n")
	fmt.Printf("  📊 File size: %v bytes\n", result.Data["file_size"])
	fmt.Printf("  ⏱️  Transfer time: %v\n", result.Data["transfer_time"])
	fmt.Printf("  🚀 Average speed: %v\n", result.Data["average_speed"])

	if backupCreated, ok := result.Data["backup_created"].(bool); ok && backupCreated {
		fmt.Printf("  💾 Backup created: %v\n", result.Data["backup_path"])
	}
}

func demoWebSocketStreaming(ctx context.Context, conn *connection.LocalConnection, wsServer *websocket.StreamServer, logger *logging.StreamLogger) {
	fmt.Println("  🌐 Broadcasting real-time events to WebSocket clients...")

	// Show connected clients
	clients := wsServer.GetConnectedClients()
	fmt.Printf("  👥 Connected WebSocket clients: %d\n", len(clients))

	// Create some demo events
	events := []types.StreamEvent{
		{
			Type:      types.StreamStdout,
			Data:      "Starting demo streaming process...",
			Timestamp: time.Now(),
		},
		{
			Type: types.StreamProgress,
			Progress: &types.ProgressInfo{
				Stage:      "initializing",
				Percentage: 25.0,
				Message:    "Initializing streaming demo",
				Timestamp:  time.Now(),
			},
			Timestamp: time.Now(),
		},
		{
			Type:      types.StreamStdout,
			Data:      "Processing demo data...",
			Timestamp: time.Now(),
		},
		{
			Type: types.StreamProgress,
			Progress: &types.ProgressInfo{
				Stage:      "processing",
				Percentage: 75.0,
				Message:    "Processing streaming data",
				Timestamp:  time.Now(),
			},
			Timestamp: time.Now(),
		},
		{
			Type:      types.StreamStdout,
			Data:      "Demo streaming completed successfully!",
			Timestamp: time.Now(),
		},
	}

	// Stream events with delays for demonstration
	for i, event := range events {
		fmt.Printf("  📡 Broadcasting event %d/%d: %s\n", i+1, len(events), event.Type)

		// Log the event
		logger.LogStreamEvent(event, "demo_streaming", "localhost")

		// Broadcast to WebSocket clients
		wsServer.BroadcastStreamEvent(event, "demo_streaming")

		// Add delay for realistic streaming
		time.Sleep(500 * time.Millisecond)
	}

	// Final completion broadcast
	wsServer.BroadcastStreamEvent(types.StreamEvent{
		Type: types.StreamDone,
		Data: "WebSocket streaming demo completed",
		Result: &types.Result{
			Success: true,
			Message: "All demo events broadcasted successfully",
			EndTime: time.Now(),
		},
		Timestamp: time.Now(),
	}, "demo_streaming")

	fmt.Println("  ✅ WebSocket streaming demo completed!")
}

func demoLoggingIntegration(ctx context.Context, conn *connection.LocalConnection, logger *logging.StreamLogger) {
	fmt.Println("  📝 Demonstrating comprehensive logging features...")

	// Demo different log levels
	logLevels := []struct {
		level   logging.LogLevel
		message string
		fields  map[string]interface{}
	}{
		{
			level:   logging.LevelDebug,
			message: "Debug information for troubleshooting",
			fields: map[string]interface{}{
				"debug_data": "detailed_info",
				"trace_id":   "demo_trace_001",
			},
		},
		{
			level:   logging.LevelInfo,
			message: "System operation completed normally",
			fields: map[string]interface{}{
				"operation": "demo_logging",
				"status":    "success",
			},
		},
		{
			level:   logging.LevelWarn,
			message: "Non-critical warning occurred",
			fields: map[string]interface{}{
				"warning_type": "performance",
				"threshold":    "80%",
			},
		},
		{
			level:   logging.LevelError,
			message: "Simulated error for demonstration",
			fields: map[string]interface{}{
				"error_code":  "DEMO_001",
				"component":   "logging_demo",
				"recoverable": true,
			},
		},
	}

	for i, logEntry := range logLevels {
		fmt.Printf("  📄 Logging %s message %d/%d\n", logEntry.level.String(), i+1, len(logLevels))
		logger.Log(logEntry.level, logEntry.message, logEntry.fields)
		time.Sleep(200 * time.Millisecond)
	}

	// Demo step logging
	step := types.StepInfo{
		ID:          "demo_step_001",
		Name:        "Demonstration Step",
		Description: "Shows step logging capabilities",
		Status:      types.StepCompleted,
		StartTime:   time.Now().Add(-2 * time.Second),
		EndTime:     time.Now(),
		Metadata: map[string]interface{}{
			"demo_type": "logging_integration",
			"completed": true,
		},
	}

	fmt.Println("  📋 Logging step information...")
	logger.LogStep(step, "logging_demo", "localhost")

	// Demo progress logging
	progress := types.ProgressInfo{
		Stage:      "demonstration",
		Percentage: 100.0,
		Message:    "Logging integration demo completed",
		Timestamp:  time.Now(),
	}

	fmt.Println("  📊 Logging progress information...")
	logger.LogProgress(progress, "logging_demo", "localhost")

	fmt.Println("  ✅ Logging integration demo completed!")
}

func showFinalStats(logger *logging.StreamLogger) {
	fmt.Println("\n📈 Final Demo Statistics")
	fmt.Println("========================")

	// Try to get memory output for statistics
	// Note: This would require access to the memory output instance
	// For demo purposes, we'll show what would be available

	fmt.Printf("🗃️  Log entries: Available in './demo_logs.json'\n")
	fmt.Printf("📊 Memory buffer: Available via API\n")
	fmt.Printf("🎯 Log levels: DEBUG, INFO, WARN, ERROR demonstrated\n")
	fmt.Printf("🔄 Event types: Stream events, progress, steps logged\n")
	fmt.Printf("🌐 WebSocket: Real-time broadcasting demonstrated\n")
	fmt.Printf("📁 File transfer: Progress tracking demonstrated\n")

	// Check if log file was created
	if _, err := os.Stat("./demo_logs.json"); err == nil {
		fmt.Printf("✅ Log file created successfully\n")
	} else {
		fmt.Printf("❌ Log file not found: %v\n", err)
	}

	// Check if demo output file was created
	if _, err := os.Stat("./demo_output.txt"); err == nil {
		fmt.Printf("✅ Demo output file created successfully\n")
	} else {
		fmt.Printf("❌ Demo output file not found: %v\n", err)
	}
}

// Instructions for running the demo
func init() {
	fmt.Println(`
🔧 Demo Setup Instructions:
1. Run: go run examples/enhanced_features_demo.go
2. Open browser to ws://localhost:8080/ws for WebSocket connection
3. Monitor real-time events in browser console
4. Check './demo_logs.json' for comprehensive logging
5. Review './demo_output.txt' for file transfer results

📖 Features Demonstrated:
• File transfer progress tracking with real-time updates
• WebSocket streaming for live web dashboard updates  
• Multi-output logging (file, console, memory)
• Step-by-step operation tracking
• Performance metrics and timing information
• Backup functionality with error handling
• Integration between all enhanced components

🌐 WebSocket Test Client:
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received:', data.type, data);
};
ws.send(JSON.stringify({
  type: 'subscribe',
  data: { event_types: ['stream_event', 'progress', 'connection'] }
}));
	`)
}
