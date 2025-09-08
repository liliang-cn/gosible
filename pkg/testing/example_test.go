package testing

import (
	"context"
	"fmt"
	"testing"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// ExampleModule demonstrates how to create a testable module
type ExampleModule struct {
	*MockBaseModule
}

// MockBaseModule provides a minimal base module implementation for testing
type MockBaseModule struct {
	name string
	doc  types.ModuleDoc
}

func NewMockBaseModule(name string, doc types.ModuleDoc) *MockBaseModule {
	return &MockBaseModule{name: name, doc: doc}
}

func (m *MockBaseModule) Name() string                               { return m.name }
func (m *MockBaseModule) Documentation() types.ModuleDoc            { return m.doc }
func (m *MockBaseModule) Capabilities() *types.ModuleCapability     { return types.DefaultCapabilities() }
func (m *MockBaseModule) Validate(args map[string]interface{}) error { return nil }

func NewExampleModule() *ExampleModule {
	doc := types.ModuleDoc{
		Name:        "example",
		Description: "Example module for testing demonstration",
		Parameters: map[string]types.ParamDoc{
			"message": {
				Description: "Message to echo",
				Required:    true,
				Type:        "string",
			},
			"repeat": {
				Description: "Number of times to repeat",
				Required:    false,
				Type:        "int",
				Default:     1,
			},
		},
	}

	base := NewMockBaseModule("example", doc)
	return &ExampleModule{MockBaseModule: base}
}

// Validate implements proper validation for the example module
func (m *ExampleModule) Validate(args map[string]interface{}) error {
	// Check required parameters
	if _, ok := args["message"]; !ok {
		return fmt.Errorf("message parameter is required")
	}
	
	// Check parameter types
	if repeat, ok := args["repeat"]; ok {
		if _, ok := repeat.(int); !ok {
			return fmt.Errorf("repeat parameter must be an integer")
		}
	}
	
	return nil
}

// Run implements a simple echo module
func (m *ExampleModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	message, ok := args["message"].(string)
	if !ok || message == "" {
		return &types.Result{
			Success: false,
			Error:   fmt.Errorf("message parameter is required"),
		}, nil
	}

	repeat := 1
	if r, ok := args["repeat"]; ok {
		if ri, ok := r.(int); ok {
			repeat = ri
		}
	}

	// Execute echo command
	var fullMessage string
	for i := 0; i < repeat; i++ {
		if i > 0 {
			fullMessage += " "
		}
		fullMessage += message
	}

	cmd := fmt.Sprintf("echo '%s'", fullMessage)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return &types.Result{
			Success: false,
			Error:   err,
		}, nil
	}

	return &types.Result{
		Success: result.Success,
		Changed: true, // Echo always produces output
		Message: fmt.Sprintf("Echoed message %d time(s)", repeat),
		Data: map[string]interface{}{
			"command": cmd,
			"output":  result.Message,
		},
	}, nil
}

// TestExampleModule demonstrates the testing framework
func TestExampleModule(t *testing.T) {
	module := NewExampleModule()
	helper := NewModuleTestHelper(t, module)

	t.Run("BasicEcho", func(t *testing.T) {
		// Setup: expect the echo command
		helper.GetConnection().ExpectCommand("echo 'hello world'", &CommandResponse{
			Stdout:   "hello world",
			ExitCode: 0,
		})

		// Execute
		result := helper.Execute(map[string]interface{}{
			"message": "hello world",
		}, false, false)

		// Verify
		helper.AssertSuccess(result)
		helper.AssertChanged(result)
		// Verify command was executed (using connection verification)

		if result.Data["command"] != "echo 'hello world'" {
			t.Errorf("Expected command in data, got %v", result.Data["command"])
		}
	})

	t.Run("RepeatMessage", func(t *testing.T) {
		helper.Reset()

		helper.GetConnection().ExpectCommand("echo 'test test test'", &CommandResponse{
			Stdout:   "test test test",
			ExitCode: 0,
		})

		result := helper.Execute(map[string]interface{}{
			"message": "test",
			"repeat":  3,
		}, false, false)

		helper.AssertSuccess(result)
		if !stringContains(result.Message, "3 time(s)") {
			t.Errorf("Expected message to mention 3 times, got: %s", result.Message)
		}
	})

	t.Run("MissingMessage", func(t *testing.T) {
		helper.Reset()

		err := helper.ExecuteExpectingError(map[string]interface{}{
			"repeat": 2,
		})

		if err == nil || !stringContains(err.Error(), "message parameter is required") {
			t.Error("Expected error about missing message parameter")
		}
	})
}

// TestExampleModuleBatchCases demonstrates batch test case execution
func TestExampleModuleBatchCases(t *testing.T) {
	module := NewExampleModule()
	helper := NewModuleTestHelper(t, module)

	testCases := []TestCase{
		{
			Name: "SimpleEcho",
			Args: map[string]interface{}{
				"message": "hello",
			},
			Setup: func(h *ModuleTestHelper) {
				h.GetConnection().ExpectCommand("echo 'hello'", &CommandResponse{
					Stdout:   "hello",
					ExitCode: 0,
				})
			},
			Assertions: func(h *ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "1 time(s)")
			},
		},
		{
			Name: "CommandFailure",
			Args: map[string]interface{}{
				"message": "fail",
			},
			Setup: func(h *ModuleTestHelper) {
				h.GetConnection().ExpectCommand("echo 'fail'", &CommandResponse{
					Stderr:   "command not found",
					ExitCode: 127,
				})
			},
			Assertions: func(h *ModuleTestHelper, result *types.Result) {
				h.AssertFailure(result)
			},
		},
		{
			Name: "ValidationFailure",
			Args: map[string]interface{}{
				"repeat": "invalid",
			},
			ExpectError: true,
		},
	}

	helper.RunTestCases(testCases)
}

// TestMockConnectionFeatures demonstrates MockConnection capabilities
func TestMockConnectionFeatures(t *testing.T) {
	conn := NewMockConnection(t)

	t.Run("BasicExpectations", func(t *testing.T) {
		// Setup expectations
		conn.ExpectCommand("ls", &CommandResponse{
			Stdout:   "file1.txt\nfile2.txt",
			ExitCode: 0,
		})

		conn.ExpectCommandRegex("cat .*", &CommandResponse{
			Stdout:   "file content",
			ExitCode: 0,
		})

		// Test execution
		ctx := context.Background()
		connInfo := types.ConnectionInfo{Type: "mock", Host: "test-host"}
		
		if err := conn.Connect(ctx, connInfo); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}

		// Execute expected commands
		result1, err := conn.Execute(ctx, "ls", types.ExecuteOptions{})
		if err != nil {
			t.Errorf("ls command failed: %v", err)
		}
		if result1.Message != "file1.txt\nfile2.txt" {
			t.Errorf("Unexpected ls output: %s", result1.Message)
		}

		result2, err := conn.Execute(ctx, "cat somefile.txt", types.ExecuteOptions{})
		if err != nil {
			t.Errorf("cat command failed: %v", err)
		}
		if result2.Message != "file content" {
			t.Errorf("Unexpected cat output: %s", result2.Message)
		}

		// Verify all expectations were met
		if err := conn.VerifyAllExpectationsMet(); err != nil {
			t.Errorf("Expectations not met: %v", err)
		}
	})

	t.Run("OrderedExecution", func(t *testing.T) {
		conn.Reset()
		conn.EnableStrictOrder()

		conn.ExpectCommandOrder("first", 1, &CommandResponse{ExitCode: 0})
		conn.ExpectCommandOrder("second", 2, &CommandResponse{ExitCode: 0})

		ctx := context.Background()
		conn.Connect(ctx, types.ConnectionInfo{})

		// Execute in order
		conn.Execute(ctx, "first", types.ExecuteOptions{})
		conn.Execute(ctx, "second", types.ExecuteOptions{})

		// Verify execution order
		order := conn.GetExecutionOrder()
		if len(order) != 2 || order[0] != "first" || order[1] != "second" {
			t.Errorf("Unexpected execution order: %v", order)
		}
	})

	t.Run("EnvironmentVariables", func(t *testing.T) {
		conn.Reset()

		conn.ExpectCommandWithEnv("deploy", map[string]string{
			"ENV": "production",
		}, &CommandResponse{ExitCode: 0})

		ctx := context.Background()
		conn.Connect(ctx, types.ConnectionInfo{})

		// This should match
		result1, err := conn.Execute(ctx, "deploy", types.ExecuteOptions{
			Env: map[string]string{"ENV": "production"},
		})
		if err != nil || !result1.Success {
			t.Error("Expected command with correct env to succeed")
		}

		// This should not match (uses default response)
		conn.SetDefaultCommandResponse(&CommandResponse{ExitCode: 1})
		result2, err := conn.Execute(ctx, "deploy", types.ExecuteOptions{
			Env: map[string]string{"ENV": "development"},
		})
		if result2.Success {
			t.Error("Expected command with wrong env to use default response")
		}
	})
}

// TestMockFileSystemFeatures demonstrates MockFileSystem capabilities
func TestMockFileSystemFeatures(t *testing.T) {
	fs := NewMockFileSystem(t)

	t.Run("BasicOperations", func(t *testing.T) {
		// Create files and directories
		err := fs.AddDir("/tmp", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		err = fs.AddFile("/tmp/test.txt", []byte("hello world"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Read file
		content, err := fs.ReadFile("/tmp/test.txt")
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(content) != "hello world" {
			t.Errorf("Unexpected file content: %s", string(content))
		}

		// Check existence
		if !fs.Exists("/tmp/test.txt") {
			t.Error("File should exist")
		}
		if !fs.IsDir("/tmp") {
			t.Error("Directory should exist")
		}

		// List directory
		files, err := fs.ReadDir("/tmp")
		if err != nil {
			t.Fatalf("Failed to read directory: %v", err)
		}
		if len(files) != 1 || files[0].Name() != "test.txt" {
			t.Errorf("Unexpected directory contents: %v", files)
		}
	})

	t.Run("ErrorSimulation", func(t *testing.T) {
		fs.Reset()
		fs.SimulatePermissionErrors()

		// Files with "restricted" in path should fail
		_, err := fs.ReadFile("/restricted/secret.txt")
		if err == nil {
			t.Error("Expected permission error for restricted file")
		}

		fs.SimulateIOErrors()
		
		// Files with "broken" in path should fail
		_, err = fs.ReadFile("/tmp/broken-disk.txt")
		if err == nil {
			t.Error("Expected IO error for broken file")
		}
	})

	t.Run("OperationTracking", func(t *testing.T) {
		fs.Reset()

		fs.AddFile("/test.txt", []byte("content"), 0644)
		fs.ReadFile("/test.txt")
		fs.WriteFile("/test2.txt", []byte("content2"), 0644)
		fs.ReadFile("/test2.txt") // Add another read operation

		operations := fs.GetOperations()
		if len(operations) < 3 {
			t.Errorf("Expected at least 3 operations, got %d", len(operations))
		}

		counts := fs.GetOperationsCount()
		if counts["write"] < 2 {
			t.Errorf("Expected at least 2 write operations, got %d", counts["write"])
		}
		if counts["read"] < 1 {
			t.Errorf("Expected at least 1 read operation, got %d", counts["read"])
		}
	})
}

// Helper function for string containment check  
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}