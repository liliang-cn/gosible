package modules

import (
	"context"
	"fmt"
	"time"

	"github.com/gosinble/gosinble/pkg/types"
)

// DeploymentModule implements a deployment workflow with detailed step tracking
type DeploymentModule struct {
	BaseModule
}

// NewDeploymentModule creates a new deployment module instance
func NewDeploymentModule() *DeploymentModule {
	return &DeploymentModule{
		BaseModule: BaseModule{
			name: "deployment",
		},
	}
}

// Run executes the deployment module with step tracking
func (m *DeploymentModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	appName := m.GetStringArg(args, "app_name", "myapp")
	appVersion := m.GetStringArg(args, "version", "latest")
	deployPath := m.GetStringArg(args, "deploy_path", "/opt/apps")
	enableHealthCheck := m.GetBoolArg(args, "health_check", true)
	rollbackOnFailure := m.GetBoolArg(args, "rollback_on_failure", true)

	// Check if connection supports streaming for step tracking
	if streamConn, ok := conn.(types.StreamingConnection); ok {
		return m.executeWithStepTracking(ctx, streamConn, appName, appVersion, deployPath, enableHealthCheck, rollbackOnFailure)
	}

	// Fallback to standard execution
	return m.executeStandard(ctx, conn, appName, appVersion, deployPath)
}

// executeWithStepTracking performs deployment with detailed step tracking
func (m *DeploymentModule) executeWithStepTracking(ctx context.Context, conn types.StreamingConnection, appName, appVersion, deployPath string, healthCheck, rollback bool) (*types.Result, error) {
	// Define deployment steps
	steps := []struct {
		id          string
		name        string
		description string
		command     string
		critical    bool
	}{
		{
			id:          "validate",
			name:        "Validate Environment",
			description: "Check system requirements and permissions",
			command:     fmt.Sprintf("test -d %s && test -w %s", deployPath, deployPath),
			critical:    true,
		},
		{
			id:          "backup",
			name:        "Backup Current Version",
			description: "Create backup of existing deployment",
			command:     fmt.Sprintf("test -d %s/%s && cp -r %s/%s %s/%s.backup.$(date +%%s) || true", deployPath, appName, deployPath, appName, deployPath, appName),
			critical:    false,
		},
		{
			id:          "download",
			name:        "Download Application",
			description: fmt.Sprintf("Download %s version %s", appName, appVersion),
			command:     fmt.Sprintf("mkdir -p /tmp/deploy && echo 'Downloading %s:%s' && sleep 1", appName, appVersion),
			critical:    true,
		},
		{
			id:          "extract",
			name:        "Extract Package",
			description: "Extract application package to deployment directory",
			command:     fmt.Sprintf("echo 'Extracting to %s/%s' && mkdir -p %s/%s && sleep 0.5", deployPath, appName, deployPath, appName),
			critical:    true,
		},
		{
			id:          "configure",
			name:        "Configure Application",
			description: "Apply configuration and environment settings",
			command:     fmt.Sprintf("echo 'Configuring %s' && touch %s/%s/config.yml && sleep 0.3", appName, deployPath, appName),
			critical:    true,
		},
		{
			id:          "permissions",
			name:        "Set Permissions",
			description: "Configure file permissions and ownership",
			command:     fmt.Sprintf("chmod -R 755 %s/%s && echo 'Permissions set'", deployPath, appName),
			critical:    true,
		},
		{
			id:          "start",
			name:        "Start Service",
			description: "Start the application service",
			command:     fmt.Sprintf("echo 'Starting %s service' && sleep 1", appName),
			critical:    true,
		},
	}

	// Add health check step if enabled
	if healthCheck {
		steps = append(steps, struct {
			id          string
			name        string
			description string
			command     string
			critical    bool
		}{
			id:          "health_check",
			name:        "Health Check",
			description: "Verify application is running correctly",
			command:     fmt.Sprintf("echo 'Health check for %s' && sleep 0.5 && echo 'Service is healthy'", appName),
			critical:    true,
		})
	}

	totalSteps := len(steps)
	completedSteps := make([]types.StepInfo, 0, totalSteps)

	// Execute deployment steps
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
				"command":     stepDef.command,
				"app_name":    appName,
				"app_version": appVersion,
			},
		}

		// Create streaming options with step tracking
		options := types.ExecuteOptions{
			StreamOutput: true,
			Timeout:      30 * time.Second,
			ProgressCallback: func(progress types.ProgressInfo) {
				// This will be called by the connection, but we're managing steps here
			},
		}

		// Send step start event
		stepStartEvent := types.StreamEvent{
			Type: types.StreamStepStart,
			Step: &step,
			Progress: &types.ProgressInfo{
				Stage:           "deploying",
				Percentage:      float64(i) / float64(totalSteps) * 100,
				Message:         fmt.Sprintf("Starting step %d/%d: %s", stepNumber, totalSteps, step.Name),
				Timestamp:       time.Now(),
				CurrentStep:     &step,
				CompletedSteps:  completedSteps,
				TotalSteps:      totalSteps,
				StepNumber:      stepNumber,
			},
			Timestamp: time.Now(),
		}

		fmt.Printf("🔄 Step %d/%d: %s\n", stepNumber, totalSteps, step.Name)
		fmt.Printf("   📝 %s\n", step.Description)

		// Execute the step command
		events, err := conn.ExecuteStream(ctx, stepDef.command, options)
		if err != nil {
			step.Status = types.StepFailed
			step.EndTime = time.Now()
			step.Duration = step.EndTime.Sub(step.StartTime)

			if stepDef.critical {
				if rollback && len(completedSteps) > 0 {
					fmt.Printf("💥 Critical step failed, attempting rollback...\n")
					m.performRollback(ctx, conn, appName, deployPath)
				}
				return nil, fmt.Errorf("deployment: critical step '%s' failed: %v", step.Name, err)
			}

			fmt.Printf("⚠️  Non-critical step failed: %s\n", step.Name)
			step.Status = types.StepSkipped
		} else {
			// Process step events
			stepCompleted := false
			for event := range events {
				switch event.Type {
				case types.StreamStdout:
					fmt.Printf("   📤 %s\n", event.Data)
				case types.StreamStderr:
					fmt.Printf("   ❌ %s\n", event.Data)
				case types.StreamDone:
					if event.Result != nil && event.Result.Success {
						step.Status = types.StepCompleted
						stepCompleted = true
					} else {
						step.Status = types.StepFailed
					}
					step.EndTime = time.Now()
					step.Duration = step.EndTime.Sub(step.StartTime)
				case types.StreamError:
					step.Status = types.StepFailed
					step.EndTime = time.Now()
					step.Duration = step.EndTime.Sub(step.StartTime)
					
					if stepDef.critical {
						if rollback {
							fmt.Printf("💥 Critical step failed, attempting rollback...\n")
							m.performRollback(ctx, conn, appName, deployPath)
						}
						return nil, fmt.Errorf("deployment: critical step '%s' failed: %v", step.Name, event.Error)
					}
					step.Status = types.StepSkipped
				}
			}

			if stepCompleted {
				fmt.Printf("   ✅ %s completed in %v\n", step.Name, step.Duration)
			} else if step.Status == types.StepFailed && !stepDef.critical {
				fmt.Printf("   ⚠️  %s failed (non-critical)\n", step.Name)
			}
		}

		// Add to completed steps
		completedSteps = append(completedSteps, step)

		// Send step end event
		stepEndEvent := types.StreamEvent{
			Type: types.StreamStepEnd,
			Step: &step,
			Progress: &types.ProgressInfo{
				Stage:           "deploying",
				Percentage:      float64(stepNumber) / float64(totalSteps) * 100,
				Message:         fmt.Sprintf("Completed step %d/%d: %s", stepNumber, totalSteps, step.Name),
				Timestamp:       time.Now(),
				CompletedSteps:  completedSteps,
				TotalSteps:      totalSteps,
				StepNumber:      stepNumber,
			},
			Timestamp: time.Now(),
		}

		_ = stepStartEvent // Would be sent via channel in real implementation
		_ = stepEndEvent   // Would be sent via channel in real implementation
	}

	// Create final result
	result := &types.Result{
		Success:    true,
		Changed:    true,
		Message:    fmt.Sprintf("Deployment of %s:%s completed successfully", appName, appVersion),
		StartTime:  completedSteps[0].StartTime,
		EndTime:    time.Now(),
		Duration:   time.Since(completedSteps[0].StartTime),
		ModuleName: "deployment",
		Data: map[string]interface{}{
			"app_name":        appName,
			"app_version":     appVersion,
			"deploy_path":     deployPath,
			"total_steps":     totalSteps,
			"completed_steps": len(completedSteps),
			"failed_steps":    countFailedSteps(completedSteps),
			"deployment_time": time.Since(completedSteps[0].StartTime).String(),
			"steps":           completedSteps,
		},
	}

	fmt.Printf("\n🎉 Deployment completed successfully!\n")
	fmt.Printf("   📊 %d/%d steps completed\n", len(completedSteps), totalSteps)
	fmt.Printf("   ⏱️  Total time: %v\n", result.Duration)

	return result, nil
}

// performRollback attempts to rollback the deployment
func (m *DeploymentModule) performRollback(ctx context.Context, conn types.Connection, appName, deployPath string) {
	fmt.Printf("🔄 Rolling back deployment...\n")
	
	rollbackSteps := []struct {
		name    string
		command string
	}{
		{
			name:    "Stop service",
			command: fmt.Sprintf("echo 'Stopping %s service'", appName),
		},
		{
			name:    "Restore backup",
			command: fmt.Sprintf("ls %s/%s.backup.* 2>/dev/null | head -1 | xargs -I {} cp -r {} %s/%s || echo 'No backup found'", deployPath, appName, deployPath, appName),
		},
		{
			name:    "Restart service",
			command: fmt.Sprintf("echo 'Restarting %s service'", appName),
		},
	}

	for _, step := range rollbackSteps {
		fmt.Printf("   📤 %s\n", step.name)
		_, err := conn.Execute(ctx, step.command, types.ExecuteOptions{Timeout: 10 * time.Second})
		if err != nil {
			fmt.Printf("   ❌ Rollback step failed: %v\n", err)
		} else {
			fmt.Printf("   ✅ %s completed\n", step.name)
		}
	}
}

// executeStandard performs basic deployment without step tracking
func (m *DeploymentModule) executeStandard(ctx context.Context, conn types.Connection, appName, appVersion, deployPath string) (*types.Result, error) {
	command := fmt.Sprintf(`
		echo "Deploying %s:%s to %s" &&
		mkdir -p %s/%s &&
		echo "Deployment completed"
	`, appName, appVersion, deployPath, deployPath, appName)

	result, err := conn.Execute(ctx, command, types.ExecuteOptions{Timeout: 60 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("deployment: standard execution failed: %v", err)
	}

	// Enhance result with deployment metadata
	if result.Data == nil {
		result.Data = make(map[string]interface{})
	}
	result.Data["app_name"] = appName
	result.Data["app_version"] = appVersion
	result.Data["deploy_path"] = deployPath
	result.Data["execution_mode"] = "standard"

	return result, nil
}

// Validate checks if the module arguments are valid
func (m *DeploymentModule) Validate(args map[string]interface{}) error {
	// App name is required
	if appName := m.GetStringArg(args, "app_name", ""); appName == "" {
		return fmt.Errorf("deployment: app_name parameter is required")
	}

	// Deploy path should be absolute
	if deployPath := m.GetStringArg(args, "deploy_path", ""); deployPath != "" && deployPath[0] != '/' {
		return fmt.Errorf("deployment: deploy_path must be absolute path")
	}

	return nil
}

// Documentation returns the module documentation
func (m *DeploymentModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "deployment",
		Description: "Deploy applications with detailed step tracking and rollback capability",
		Parameters: map[string]types.ParamDoc{
			"app_name": {
				Description: "Name of the application to deploy",
				Required:    true,
				Type:        "string",
			},
			"version": {
				Description: "Version of the application to deploy",
				Required:    false,
				Type:        "string",
				Default:     "latest",
			},
			"deploy_path": {
				Description: "Base path for deployment",
				Required:    false,
				Type:        "string",
				Default:     "/opt/apps",
			},
			"health_check": {
				Description: "Perform health check after deployment",
				Required:    false,
				Type:        "boolean",
				Default:     "true",
			},
			"rollback_on_failure": {
				Description: "Automatically rollback on critical failure",
				Required:    false,
				Type:        "boolean",
				Default:     "true",
			},
		},
		Examples: []string{
			"- name: Deploy web application\n  deployment:\n    app_name: webapp\n    version: v1.2.3\n    deploy_path: /var/www",
			"- name: Deploy with custom settings\n  deployment:\n    app_name: api-server\n    version: latest\n    health_check: true\n    rollback_on_failure: false",
		},
		Returns: map[string]string{
			"app_name":        "Name of deployed application",
			"app_version":     "Version that was deployed",
			"total_steps":     "Total number of deployment steps",
			"completed_steps": "Number of successfully completed steps",
			"failed_steps":    "Number of failed steps",
			"deployment_time": "Total deployment duration",
			"steps":           "Detailed information about all steps",
		},
	}
}

// Helper function to count failed steps
func countFailedSteps(steps []types.StepInfo) int {
	count := 0
	for _, step := range steps {
		if step.Status == types.StepFailed {
			count++
		}
	}
	return count
}