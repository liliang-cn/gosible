package modules

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// SystemdModule manages systemd services with full check/diff mode support
type SystemdModule struct {
	*BaseModule
}

// NewSystemdModule creates a new systemd module instance
func NewSystemdModule() *SystemdModule {
	doc := types.ModuleDoc{
		Name:        "systemd",
		Description: "Manage systemd services and units",
		Parameters: map[string]types.ParamDoc{
			"name": {
				Description: "Name of the service unit (without .service suffix)",
				Required:    true,
				Type:        "string",
			},
			"state": {
				Description: "Desired state of the service",
				Required:    false,
				Type:        "string",
				Choices:     []string{"started", "stopped", "restarted", "reloaded"},
			},
			"enabled": {
				Description: "Whether the service should start on boot",
				Required:    false,
				Type:        "bool",
			},
			"daemon_reload": {
				Description: "Run daemon-reload before doing any other operations",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"masked": {
				Description: "Whether the service should be masked",
				Required:    false,
				Type:        "bool",
			},
			"force": {
				Description: "Force start/stop/restart operations",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"no_block": {
				Description: "Do not synchronously wait for operation to complete",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
		},
		Examples: []string{
			"- name: Start and enable nginx\n  systemd:\n    name: nginx\n    state: started\n    enabled: true",
			"- name: Stop service and reload daemon\n  systemd:\n    name: myapp\n    state: stopped\n    daemon_reload: true",
			"- name: Mask a service\n  systemd:\n    name: unwanted-service\n    masked: true",
			"- name: Restart service with force\n  systemd:\n    name: stubborn-service\n    state: restarted\n    force: true",
		},
		Returns: map[string]string{
			"status":       "Current status information about the service",
			"changed":      "Whether any changes were made",
			"before_state": "Service state before operation",
			"after_state":  "Service state after operation",
		},
	}

	base := NewBaseModule("systemd", doc)
	
	// Set capabilities - systemd module supports both check and diff modes
	capabilities := &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     true,
		AsyncMode:    false,
		Platform:     "linux",
		RequiresRoot: true,
	}
	base.SetCapabilities(capabilities)

	return &SystemdModule{
		BaseModule: base,
	}
}

// SystemdServiceState represents the complete state of a systemd service
type SystemdServiceState struct {
	Name         string            `json:"name"`
	LoadState    string            `json:"load_state"`    // loaded, not-found, bad-setting, error, masked
	ActiveState  string            `json:"active_state"`  // active, reloading, inactive, failed, activating, deactivating
	SubState     string            `json:"sub_state"`     // running, dead, exited, failed, start, stop, auto-restart
	EnabledState string            `json:"enabled_state"` // enabled, disabled, static, masked, indirect, generated
	UnitPath     string            `json:"unit_path"`     // Path to unit file
	Properties   map[string]string `json:"properties"`    // Additional properties
}

// Run executes the systemd module
func (m *SystemdModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)
	diffMode := m.DiffMode(args)
	
	// Parse and validate arguments
	serviceName := m.GetStringArg(args, "name", "")
	if serviceName == "" {
		return nil, fmt.Errorf("name parameter is required")
	}
	
	desiredState := m.GetStringArg(args, "state", "")
	desiredEnabled := args["enabled"]
	daemonReload := m.GetBoolArg(args, "daemon_reload", false)
	desiredMasked := args["masked"]
	force := m.GetBoolArg(args, "force", false)
	noBlock := m.GetBoolArg(args, "no_block", false)

	// Get current service state
	currentState, err := m.getServiceState(ctx, conn, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service state: %w", err)
	}

	// Store original state for diff mode
	beforeState := *currentState
	
	// Track planned changes
	changes := make([]string, 0)
	actuallyChanged := false

	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"name":         serviceName,
		"before_state": beforeState,
	})

	// Handle daemon reload first if requested
	if daemonReload {
		if checkMode {
			changes = append(changes, "would reload systemd daemon")
		} else {
			if err := m.reloadSystemdDaemon(ctx, conn); err != nil {
				return nil, fmt.Errorf("failed to reload systemd daemon: %w", err)
			}
			changes = append(changes, "reloaded systemd daemon")
			actuallyChanged = true
		}
	}

	// Handle masking/unmasking
	if desiredMasked != nil {
		shouldBeMasked := m.IsTruthy(desiredMasked)
		isMasked := currentState.LoadState == "masked" || currentState.EnabledState == "masked"
		
		if shouldBeMasked && !isMasked {
			if checkMode {
				changes = append(changes, fmt.Sprintf("would mask service %s", serviceName))
				currentState.LoadState = "masked"
				currentState.EnabledState = "masked"
			} else {
				if err := m.maskService(ctx, conn, serviceName); err != nil {
					return nil, fmt.Errorf("failed to mask service: %w", err)
				}
				changes = append(changes, fmt.Sprintf("masked service %s", serviceName))
				actuallyChanged = true
				// Update current state
				currentState.LoadState = "masked"
				currentState.EnabledState = "masked"
			}
		} else if !shouldBeMasked && isMasked {
			if checkMode {
				changes = append(changes, fmt.Sprintf("would unmask service %s", serviceName))
				currentState.LoadState = "loaded"
				currentState.EnabledState = "disabled" // Default after unmask
			} else {
				if err := m.unmaskService(ctx, conn, serviceName); err != nil {
					return nil, fmt.Errorf("failed to unmask service: %w", err)
				}
				changes = append(changes, fmt.Sprintf("unmasked service %s", serviceName))
				actuallyChanged = true
				// Refresh state after unmask
				if refreshedState, err := m.getServiceState(ctx, conn, serviceName); err == nil {
					currentState = refreshedState
				}
			}
		}
	}

	// Handle service state changes (only if not masked)
	if desiredState != "" && currentState.LoadState != "masked" {
		stateChanged, stateChangeMsg, err := m.handleServiceStateChange(ctx, conn, serviceName, desiredState, currentState, checkMode, force, noBlock)
		if err != nil {
			return nil, fmt.Errorf("failed to change service state: %w", err)
		}
		if stateChanged {
			changes = append(changes, stateChangeMsg)
			if !checkMode {
				actuallyChanged = true
			}
		}
	}

	// Handle enabled/disabled state (only if not masked)
	if desiredEnabled != nil && currentState.LoadState != "masked" {
		shouldBeEnabled := m.IsTruthy(desiredEnabled)
		enabledChanged, enabledChangeMsg, err := m.handleServiceEnabledChange(ctx, conn, serviceName, shouldBeEnabled, currentState, checkMode)
		if err != nil {
			return nil, fmt.Errorf("failed to change enabled state: %w", err)
		}
		if enabledChanged {
			changes = append(changes, enabledChangeMsg)
			if !checkMode {
				actuallyChanged = true
			}
		}
	}

	// Get final state (or simulated final state for check mode)
	var finalState *SystemdServiceState
	if checkMode {
		finalState = currentState // Use the simulated state
	} else {
		// Get actual final state
		if actuallyChanged {
			if refreshedState, err := m.getServiceState(ctx, conn, serviceName); err == nil {
				finalState = refreshedState
			} else {
				finalState = currentState // Fallback to previous state
			}
		} else {
			finalState = currentState
		}
	}

	// Set result properties
	result.Changed = actuallyChanged || (checkMode && len(changes) > 0)
	result.Data["after_state"] = *finalState
	result.Data["status"] = m.formatServiceStatus(finalState)
	result.Data["changes"] = changes
	
	if checkMode {
		result.Simulated = true
		result.Data["check_mode"] = true
	}

	// Generate diff if requested
	if diffMode && (actuallyChanged || (checkMode && len(changes) > 0)) {
		diff := m.generateServiceDiff(&beforeState, finalState, changes)
		result.Diff = diff
	}

	// Set appropriate message
	if len(changes) > 0 {
		if checkMode {
			result.Message = fmt.Sprintf("Would make changes to service %s: %s", serviceName, strings.Join(changes, ", "))
		} else {
			result.Message = fmt.Sprintf("Changed service %s: %s", serviceName, strings.Join(changes, ", "))
		}
	} else {
		result.Message = fmt.Sprintf("Service %s is already in desired state", serviceName)
	}

	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// getServiceState retrieves the current state of a systemd service
func (m *SystemdModule) getServiceState(ctx context.Context, conn types.Connection, serviceName string) (*SystemdServiceState, error) {
	state := &SystemdServiceState{
		Name:       serviceName,
		Properties: make(map[string]string),
	}

	// Get detailed service information using systemctl show
	showCmd := fmt.Sprintf("systemctl show %s --no-page", serviceName)
	showResult, err := conn.Execute(ctx, showCmd, types.ExecuteOptions{})
	if err != nil {
		// If show fails, try basic status check
		return m.getBasicServiceState(ctx, conn, serviceName)
	}
	
	// Check if service was not found (exit code 4)
	if !showResult.Success {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	// Parse systemctl show output
	lines := strings.Split(showResult.Message, "\n")
	for _, line := range lines {
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]
				state.Properties[key] = value

				switch key {
				case "LoadState":
					state.LoadState = value
				case "ActiveState":
					state.ActiveState = value
				case "SubState":
					state.SubState = value
				case "UnitFileState":
					state.EnabledState = value
				case "FragmentPath":
					state.UnitPath = value
				}
			}
		}
	}

	// Fallback to basic checks if properties are missing
	if state.LoadState == "" || state.ActiveState == "" {
		basicState, err := m.getBasicServiceState(ctx, conn, serviceName)
		if err == nil {
			if state.LoadState == "" {
				state.LoadState = basicState.LoadState
			}
			if state.ActiveState == "" {
				state.ActiveState = basicState.ActiveState
			}
			if state.EnabledState == "" {
				state.EnabledState = basicState.EnabledState
			}
		}
	}

	return state, nil
}

// getBasicServiceState gets service state using basic systemctl commands (fallback)
func (m *SystemdModule) getBasicServiceState(ctx context.Context, conn types.Connection, serviceName string) (*SystemdServiceState, error) {
	state := &SystemdServiceState{
		Name:       serviceName,
		Properties: make(map[string]string),
	}

	// Check if service is active
	activeCmd := fmt.Sprintf("systemctl is-active %s", serviceName)
	activeResult, _ := conn.Execute(ctx, activeCmd, types.ExecuteOptions{})
	if activeResult != nil {
		state.ActiveState = strings.TrimSpace(activeResult.Message)
		if state.ActiveState == "" {
			state.ActiveState = "inactive"
		}
	} else {
		state.ActiveState = "inactive"
	}

	// Check if service is enabled
	enabledCmd := fmt.Sprintf("systemctl is-enabled %s 2>/dev/null", serviceName)
	enabledResult, _ := conn.Execute(ctx, enabledCmd, types.ExecuteOptions{})
	if enabledResult != nil && enabledResult.Success {
		state.EnabledState = strings.TrimSpace(enabledResult.Message)
	} else {
		state.EnabledState = "disabled"
	}

	// Try to determine load state
	statusCmd := fmt.Sprintf("systemctl status %s", serviceName)
	statusResult, _ := conn.Execute(ctx, statusCmd, types.ExecuteOptions{})
	if statusResult != nil {
		if strings.Contains(statusResult.Message, "could not be found") {
			return nil, fmt.Errorf("service %s not found", serviceName)
		} else if strings.Contains(statusResult.Message, "masked") {
			state.LoadState = "masked"
		} else {
			state.LoadState = "loaded"
		}
	} else {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	return state, nil
}

// handleServiceStateChange manages service state transitions
func (m *SystemdModule) handleServiceStateChange(ctx context.Context, conn types.Connection, serviceName, desiredState string, currentState *SystemdServiceState, checkMode, force, noBlock bool) (bool, string, error) {
	switch desiredState {
	case "started":
		return m.handleStartService(ctx, conn, serviceName, currentState, checkMode, force, noBlock)
	case "stopped":
		return m.handleStopService(ctx, conn, serviceName, currentState, checkMode, force, noBlock)
	case "restarted":
		return m.handleRestartService(ctx, conn, serviceName, currentState, checkMode, force, noBlock)
	case "reloaded":
		return m.handleReloadService(ctx, conn, serviceName, currentState, checkMode, force, noBlock)
	default:
		return false, "", fmt.Errorf("invalid state: %s", desiredState)
	}
}

// handleStartService starts a service if not running
func (m *SystemdModule) handleStartService(ctx context.Context, conn types.Connection, serviceName string, currentState *SystemdServiceState, checkMode, force, noBlock bool) (bool, string, error) {
	if currentState.ActiveState == "active" && currentState.SubState == "running" {
		return false, "", nil // Already running
	}

	if checkMode {
		currentState.ActiveState = "active"
		currentState.SubState = "running"
		return true, fmt.Sprintf("would start service %s", serviceName), nil
	}

	// Build command
	cmd := fmt.Sprintf("systemctl start %s", serviceName)
	if noBlock {
		cmd += " --no-block"
	}

	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to start service: %w", err)
	}
	if !result.Success {
		return false, "", fmt.Errorf("failed to start service: %s", result.Message)
	}

	return true, fmt.Sprintf("started service %s", serviceName), nil
}

// handleStopService stops a service if running
func (m *SystemdModule) handleStopService(ctx context.Context, conn types.Connection, serviceName string, currentState *SystemdServiceState, checkMode, force, noBlock bool) (bool, string, error) {
	if currentState.ActiveState == "inactive" || currentState.ActiveState == "failed" {
		return false, "", nil // Already stopped
	}

	if checkMode {
		currentState.ActiveState = "inactive"
		currentState.SubState = "dead"
		return true, fmt.Sprintf("would stop service %s", serviceName), nil
	}

	// Build command
	cmd := fmt.Sprintf("systemctl stop %s", serviceName)
	if force {
		cmd += " --force"
	}
	if noBlock {
		cmd += " --no-block"
	}

	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to stop service: %w", err)
	}
	if !result.Success {
		return false, "", fmt.Errorf("failed to stop service: %s", result.Message)
	}

	return true, fmt.Sprintf("stopped service %s", serviceName), nil
}

// handleRestartService restarts a service
func (m *SystemdModule) handleRestartService(ctx context.Context, conn types.Connection, serviceName string, currentState *SystemdServiceState, checkMode, force, noBlock bool) (bool, string, error) {
	if checkMode {
		currentState.ActiveState = "active"
		currentState.SubState = "running"
		return true, fmt.Sprintf("would restart service %s", serviceName), nil
	}

	// Build command
	cmd := fmt.Sprintf("systemctl restart %s", serviceName)
	if noBlock {
		cmd += " --no-block"
	}

	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to restart service: %w", err)
	}
	if !result.Success {
		return false, "", fmt.Errorf("failed to restart service: %s", result.Message)
	}

	return true, fmt.Sprintf("restarted service %s", serviceName), nil
}

// handleReloadService reloads a service configuration
func (m *SystemdModule) handleReloadService(ctx context.Context, conn types.Connection, serviceName string, currentState *SystemdServiceState, checkMode, force, noBlock bool) (bool, string, error) {
	if checkMode {
		return true, fmt.Sprintf("would reload service %s", serviceName), nil
	}

	// Try reload first
	cmd := fmt.Sprintf("systemctl reload %s", serviceName)
	if noBlock {
		cmd += " --no-block"
	}

	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		// If reload fails, try restart as fallback
		return m.handleRestartService(ctx, conn, serviceName, currentState, checkMode, force, noBlock)
	}

	return true, fmt.Sprintf("reloaded service %s", serviceName), nil
}

// handleServiceEnabledChange manages service enabled/disabled state
func (m *SystemdModule) handleServiceEnabledChange(ctx context.Context, conn types.Connection, serviceName string, shouldBeEnabled bool, currentState *SystemdServiceState, checkMode bool) (bool, string, error) {
	currentlyEnabled := currentState.EnabledState == "enabled"

	if shouldBeEnabled == currentlyEnabled {
		return false, "", nil // Already in desired state
	}

	var cmd, action string
	if shouldBeEnabled {
		cmd = fmt.Sprintf("systemctl enable %s", serviceName)
		action = "enabled"
	} else {
		cmd = fmt.Sprintf("systemctl disable %s", serviceName)
		action = "disabled"
	}

	if checkMode {
		if shouldBeEnabled {
			currentState.EnabledState = "enabled"
		} else {
			currentState.EnabledState = "disabled"
		}
		return true, fmt.Sprintf("would %s service %s", action, serviceName), nil
	}

	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to %s service: %w", action, err)
	}
	if !result.Success {
		return false, "", fmt.Errorf("failed to %s service: %s", action, result.Message)
	}

	return true, fmt.Sprintf("%s service %s", action, serviceName), nil
}

// maskService masks a systemd service
func (m *SystemdModule) maskService(ctx context.Context, conn types.Connection, serviceName string) error {
	cmd := fmt.Sprintf("systemctl mask %s", serviceName)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("command failed: %s", result.Message)
	}
	return nil
}

// unmaskService unmasks a systemd service
func (m *SystemdModule) unmaskService(ctx context.Context, conn types.Connection, serviceName string) error {
	cmd := fmt.Sprintf("systemctl unmask %s", serviceName)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("command failed: %s", result.Message)
	}
	return nil
}

// reloadSystemdDaemon reloads the systemd daemon
func (m *SystemdModule) reloadSystemdDaemon(ctx context.Context, conn types.Connection) error {
	cmd := "systemctl daemon-reload"
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("command failed: %s", result.Message)
	}
	return nil
}

// formatServiceStatus creates a human-readable status string
func (m *SystemdModule) formatServiceStatus(state *SystemdServiceState) string {
	return fmt.Sprintf("LoadState=%s, ActiveState=%s, SubState=%s, EnabledState=%s", 
		state.LoadState, state.ActiveState, state.SubState, state.EnabledState)
}

// generateServiceDiff creates a diff showing service state changes
func (m *SystemdModule) generateServiceDiff(before, after *SystemdServiceState, changes []string) *types.DiffResult {
	beforeLines := []string{
		fmt.Sprintf("name: %s", before.Name),
		fmt.Sprintf("load_state: %s", before.LoadState),
		fmt.Sprintf("active_state: %s", before.ActiveState),
		fmt.Sprintf("sub_state: %s", before.SubState),
		fmt.Sprintf("enabled_state: %s", before.EnabledState),
	}

	afterLines := []string{
		fmt.Sprintf("name: %s", after.Name),
		fmt.Sprintf("load_state: %s", after.LoadState),
		fmt.Sprintf("active_state: %s", after.ActiveState),
		fmt.Sprintf("sub_state: %s", after.SubState),
		fmt.Sprintf("enabled_state: %s", after.EnabledState),
	}

	// Generate unified diff format
	var diffLines []string
	diffLines = append(diffLines, "--- before")
	diffLines = append(diffLines, "+++ after")
	
	for i, line := range beforeLines {
		if i < len(afterLines) && line != afterLines[i] {
			diffLines = append(diffLines, fmt.Sprintf("-%s", line))
			diffLines = append(diffLines, fmt.Sprintf("+%s", afterLines[i]))
		}
	}

	return &types.DiffResult{
		Before:      strings.Join(beforeLines, "\n"),
		After:       strings.Join(afterLines, "\n"),
		BeforeLines: beforeLines,
		AfterLines:  afterLines,
		Prepared:    true,
		Diff:        strings.Join(diffLines, "\n"),
	}
}

// Validate checks if the module arguments are valid
func (m *SystemdModule) Validate(args map[string]interface{}) error {
	// Name is required
	name := m.GetStringArg(args, "name", "")
	if name == "" {
		return types.NewValidationError("name", name, "required parameter")
	}

	// Validate service name format (basic validation)
	validName := regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	if !validName.MatchString(name) {
		return types.NewValidationError("name", name, "invalid service name format")
	}

	// Validate state choices
	if err := m.ValidateChoices(args, "state", []string{"started", "stopped", "restarted", "reloaded"}); err != nil {
		return err
	}

	// Validate boolean parameters
	boolParams := []string{"enabled", "daemon_reload", "masked", "force", "no_block"}
	for _, param := range boolParams {
		if value, exists := args[param]; exists && value != nil {
			switch value.(type) {
			case bool:
				// Valid
			case string:
				strVal := strings.ToLower(strings.TrimSpace(value.(string)))
				if strVal != "true" && strVal != "false" && strVal != "yes" && strVal != "no" && 
				   strVal != "on" && strVal != "off" && strVal != "1" && strVal != "0" {
					return types.NewValidationError(param, value, "must be a boolean value")
				}
			default:
				return types.NewValidationError(param, value, "must be a boolean value")
			}
		}
	}

	return nil
}

// Documentation returns module documentation
func (m *SystemdModule) Documentation() types.ModuleDoc {
	return m.BaseModule.Documentation()
}