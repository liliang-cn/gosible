package modules

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/liliang-cn/gosinble/pkg/types"
)

func TestDeploymentModule(t *testing.T) {
	module := NewDeploymentModule()
	ctx := context.Background()

	t.Run("BasicDeployment", func(t *testing.T) {
		conn := NewMockStreamingConnection()
		args := map[string]interface{}{
			"app_name":    "test-app",
			"version":     "v1.0.0",
			"deploy_path": "/tmp/test-deploy",
		}

		result, err := module.Run(ctx, conn, args)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %v", result.Error)
		}

		// Check deployment metadata
		if appName, ok := result.Data["app_name"].(string); !ok || appName != "test-app" {
			t.Errorf("Expected app_name 'test-app', got %v", appName)
		}

		if steps, ok := result.Data["steps"].([]types.StepInfo); !ok {
			t.Error("Expected steps information in result")
		} else if len(steps) == 0 {
			t.Error("Expected non-empty steps array")
		}
	})

	t.Run("DeploymentWithHealthCheck", func(t *testing.T) {
		conn := NewMockStreamingConnection()
		args := map[string]interface{}{
			"app_name":     "webapp",
			"version":      "v2.0.0",
			"health_check": true,
		}

		result, err := module.Run(ctx, conn, args)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %v", result.Error)
		}

		// Should have more steps due to health check
		if steps, ok := result.Data["steps"].([]types.StepInfo); ok {
			foundHealthCheck := false
			for _, step := range steps {
				if step.ID == "health_check" {
					foundHealthCheck = true
					break
				}
			}
			if !foundHealthCheck {
				t.Error("Expected health check step when health_check=true")
			}
		}
	})

	t.Run("StandardExecution", func(t *testing.T) {
		conn := &MockConnection{}
		args := map[string]interface{}{
			"app_name": "standard-app",
			"version":  "v1.0.0",
		}

		// Mock standard execution
		conn.On("Execute", ctx, mock.AnythingOfType("string"), mock.Anything).Return(&types.Result{
			Success: true,
			Changed: true,
			Message: "Standard deployment",
			Data: map[string]interface{}{
				"stdout":    "Deployment completed\n",
				"exit_code": 0,
			},
		}, nil)

		result, err := module.Run(ctx, conn, args)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %v", result.Error)
		}

		// Should indicate standard execution mode
		if executionMode, ok := result.Data["execution_mode"].(string); !ok || executionMode != "standard" {
			t.Errorf("Expected execution_mode 'standard', got %v", executionMode)
		}
	})

	t.Run("ValidationTests", func(t *testing.T) {
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name:    "MissingAppName",
				args:    map[string]interface{}{},
				wantErr: true,
			},
			{
				name: "ValidArgs",
				args: map[string]interface{}{
					"app_name": "valid-app",
				},
				wantErr: false,
			},
			{
				name: "InvalidDeployPath",
				args: map[string]interface{}{
					"app_name":    "test-app",
					"deploy_path": "relative/path", // Should be absolute
				},
				wantErr: true,
			},
			{
				name: "ValidDeployPath",
				args: map[string]interface{}{
					"app_name":    "test-app",
					"deploy_path": "/absolute/path",
				},
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := module.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("Documentation", func(t *testing.T) {
		doc := module.Documentation()

		if doc.Name != "deployment" {
			t.Errorf("Expected name 'deployment', got %s", doc.Name)
		}

		if doc.Description == "" {
			t.Error("Expected non-empty description")
		}

		// Check required parameters
		if appNameParam, ok := doc.Parameters["app_name"]; !ok {
			t.Error("Expected 'app_name' parameter in documentation")
		} else if !appNameParam.Required {
			t.Error("Expected 'app_name' parameter to be required")
		}

		// Check optional parameters
		if versionParam, ok := doc.Parameters["version"]; !ok {
			t.Error("Expected 'version' parameter in documentation")
		} else if versionParam.Required {
			t.Error("Expected 'version' parameter to be optional")
		}

		if len(doc.Examples) == 0 {
			t.Error("Expected examples in documentation")
		}

		if len(doc.Returns) == 0 {
			t.Error("Expected return values in documentation")
		}
	})
}

func TestDeploymentModuleName(t *testing.T) {
	module := NewDeploymentModule()

	if module.Name() != "deployment" {
		t.Errorf("Expected name 'deployment', got %s", module.Name())
	}
}

// TestStepInfo tests the StepInfo structure
func TestStepInfo(t *testing.T) {
	now := time.Now()
	step := types.StepInfo{
		ID:          "test-step",
		Name:        "Test Step",
		Description: "A test step for validation",
		Status:      types.StepRunning,
		StartTime:   now,
		EndTime:     now.Add(time.Second),
		Duration:    time.Second,
		Metadata: map[string]interface{}{
			"critical": true,
			"command":  "echo test",
		},
	}

	if step.ID != "test-step" {
		t.Errorf("Expected ID 'test-step', got %s", step.ID)
	}

	if step.Name != "Test Step" {
		t.Errorf("Expected name 'Test Step', got %s", step.Name)
	}

	if step.Status != types.StepRunning {
		t.Errorf("Expected status %s, got %s", types.StepRunning, step.Status)
	}

	if step.Duration != time.Second {
		t.Errorf("Expected duration %v, got %v", time.Second, step.Duration)
	}

	if critical, ok := step.Metadata["critical"].(bool); !ok || !critical {
		t.Error("Expected metadata 'critical' to be true")
	}
}

// TestStepStatus tests step status constants
func TestStepStatus(t *testing.T) {
	tests := []struct {
		status   types.StepStatus
		expected string
	}{
		{types.StepPending, "pending"},
		{types.StepRunning, "running"},
		{types.StepCompleted, "completed"},
		{types.StepFailed, "failed"},
		{types.StepSkipped, "skipped"},
		{types.StepCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.status))
			}
		})
	}
}

// TestProgressInfoWithSteps tests ProgressInfo with step tracking
func TestProgressInfoWithSteps(t *testing.T) {
	currentStep := &types.StepInfo{
		ID:          "current",
		Name:        "Current Step",
		Description: "Currently running step",
		Status:      types.StepRunning,
		StartTime:   time.Now(),
	}

	completedSteps := []types.StepInfo{
		{
			ID:          "completed1",
			Name:        "Completed Step 1",
			Status:      types.StepCompleted,
			StartTime:   time.Now().Add(-2 * time.Second),
			EndTime:     time.Now().Add(-1 * time.Second),
			Duration:    time.Second,
		},
		{
			ID:          "completed2",
			Name:        "Completed Step 2",
			Status:      types.StepCompleted,
			StartTime:   time.Now().Add(-1 * time.Second),
			EndTime:     time.Now(),
			Duration:    time.Second,
		},
	}

	progress := types.ProgressInfo{
		Stage:           "deploying",
		Percentage:      60.0,
		Message:         "Deploying application",
		Timestamp:       time.Now(),
		CurrentStep:     currentStep,
		CompletedSteps:  completedSteps,
		TotalSteps:      5,
		StepNumber:      3,
	}

	if progress.Stage != "deploying" {
		t.Errorf("Expected stage 'deploying', got %s", progress.Stage)
	}

	if progress.Percentage != 60.0 {
		t.Errorf("Expected percentage 60.0, got %f", progress.Percentage)
	}

	if progress.CurrentStep == nil {
		t.Error("Expected current step to be set")
	} else if progress.CurrentStep.ID != "current" {
		t.Errorf("Expected current step ID 'current', got %s", progress.CurrentStep.ID)
	}

	if len(progress.CompletedSteps) != 2 {
		t.Errorf("Expected 2 completed steps, got %d", len(progress.CompletedSteps))
	}

	if progress.TotalSteps != 5 {
		t.Errorf("Expected 5 total steps, got %d", progress.TotalSteps)
	}

	if progress.StepNumber != 3 {
		t.Errorf("Expected step number 3, got %d", progress.StepNumber)
	}
}

// TestStreamEventWithSteps tests StreamEvent with step information
func TestStreamEventWithSteps(t *testing.T) {
	step := &types.StepInfo{
		ID:          "event-step",
		Name:        "Event Step",
		Description: "Step for event testing",
		Status:      types.StepRunning,
		StartTime:   time.Now(),
	}

	event := types.StreamEvent{
		Type:      types.StreamStepStart,
		Step:      step,
		Timestamp: time.Now(),
	}

	if event.Type != types.StreamStepStart {
		t.Errorf("Expected type %s, got %s", types.StreamStepStart, event.Type)
	}

	if event.Step == nil {
		t.Error("Expected step to be set")
	} else if event.Step.ID != "event-step" {
		t.Errorf("Expected step ID 'event-step', got %s", event.Step.ID)
	}

	// Test other step event types
	eventTypes := []types.StreamEventType{
		types.StreamStepStart,
		types.StreamStepUpdate,
		types.StreamStepEnd,
	}

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			event := types.StreamEvent{
				Type: eventType,
				Step: step,
				Timestamp: time.Now(),
			}

			if event.Type != eventType {
				t.Errorf("Expected type %s, got %s", eventType, event.Type)
			}
		})
	}
}

// Benchmark tests for step tracking
func BenchmarkDeploymentModule(b *testing.B) {
	module := NewDeploymentModule()
	conn := NewMockStreamingConnection()
	conn.streamDelay = 0 // No delay for benchmarking
	ctx := context.Background()

	args := map[string]interface{}{
		"app_name": "benchmark-app",
		"version":  "v1.0.0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := module.Run(ctx, conn, args)
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
	}
}

func BenchmarkStepInfoCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = types.StepInfo{
			ID:          fmt.Sprintf("step-%d", i),
			Name:        fmt.Sprintf("Step %d", i),
			Description: fmt.Sprintf("Description for step %d", i),
			Status:      types.StepRunning,
			StartTime:   time.Now(),
			Metadata: map[string]interface{}{
				"step_number": i,
				"critical":    true,
			},
		}
	}
}

// Helper function that was missing from original test
func countFailedStepsTest(steps []types.StepInfo) int {
	count := 0
	for _, step := range steps {
		if step.Status == types.StepFailed {
			count++
		}
	}
	return count
}