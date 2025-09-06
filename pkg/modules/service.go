package modules

import (
	"context"
	"fmt"
	"strings"
	
	"github.com/gosinble/gosinble/pkg/types"
)

// ServiceModule manages system services
type ServiceModule struct {
	BaseModule
}

// NewServiceModule creates a new service module instance
func NewServiceModule() *ServiceModule {
	return &ServiceModule{
		BaseModule: BaseModule{
			name: "service",
		},
	}
}

// Run executes the service module
func (m *ServiceModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Get arguments
	name, _ := args["name"].(string)
	state, _ := args["state"].(string)
	enabled, hasEnabled := args["enabled"]
	daemon_reload, _ := args["daemon_reload"].(bool)
	
	result := &types.Result{
		Success: true,
		Changed: false,
		Data:    make(map[string]interface{}),
	}
	
	// Detect init system
	initSystem := m.detectInitSystem(ctx, conn)
	result.Data["init_system"] = initSystem
	
	// Reload systemd daemon if requested
	if daemon_reload && initSystem == "systemd" {
		reloadCmd := "systemctl daemon-reload"
		if _, err := conn.Execute(ctx, reloadCmd, types.ExecuteOptions{}); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to reload systemd daemon: %v", err)
			return result, nil
		}
		result.Changed = true
	}
	
	// Handle service state
	if state != "" {
		changed, err := m.handleServiceState(ctx, conn, name, state, initSystem)
		if err != nil {
			result.Success = false
			result.Error = err
			return result, nil
		}
		if changed {
			result.Changed = true
		}
	}
	
	// Handle service enabled/disabled
	if hasEnabled {
		enabledBool, _ := enabled.(bool)
		changed, err := m.handleServiceEnabled(ctx, conn, name, enabledBool, initSystem)
		if err != nil {
			result.Success = false
			result.Error = err
			return result, nil
		}
		if changed {
			result.Changed = true
		}
	}
	
	// Get service status
	status := m.getServiceStatus(ctx, conn, name, initSystem)
	result.Data["status"] = status
	
	if result.Changed {
		result.Message = fmt.Sprintf("Service %s state changed", name)
	} else {
		result.Message = fmt.Sprintf("Service %s already in desired state", name)
	}
	
	return result, nil
}

// detectInitSystem detects the system's init system
func (m *ServiceModule) detectInitSystem(ctx context.Context, conn types.Connection) string {
	// Check for systemd
	checkCmd := "which systemctl 2>/dev/null && echo systemd"
	checkResult, err := conn.Execute(ctx, checkCmd, types.ExecuteOptions{})
	if err == nil && strings.Contains(checkResult.Message, "systemd") {
		return "systemd"
	}
	
	// Check for upstart
	checkCmd = "which initctl 2>/dev/null && echo upstart"
	checkResult, err = conn.Execute(ctx, checkCmd, types.ExecuteOptions{})
	if err == nil && strings.Contains(checkResult.Message, "upstart") {
		return "upstart"
	}
	
	// Check for sysvinit
	checkCmd = "which service 2>/dev/null && echo sysvinit"
	checkResult, err = conn.Execute(ctx, checkCmd, types.ExecuteOptions{})
	if err == nil && strings.Contains(checkResult.Message, "sysvinit") {
		return "sysvinit"
	}
	
	// Default to systemd for modern systems
	return "systemd"
}

// handleServiceState manages the service state (started, stopped, restarted, reloaded)
func (m *ServiceModule) handleServiceState(ctx context.Context, conn types.Connection, name, state, initSystem string) (bool, error) {
	currentStatus := m.getServiceStatus(ctx, conn, name, initSystem)
	
	switch state {
	case "started":
		if currentStatus == "running" || currentStatus == "active" {
			return false, nil
		}
		return m.startService(ctx, conn, name, initSystem)
		
	case "stopped":
		if currentStatus == "stopped" || currentStatus == "inactive" {
			return false, nil
		}
		return m.stopService(ctx, conn, name, initSystem)
		
	case "restarted":
		return m.restartService(ctx, conn, name, initSystem)
		
	case "reloaded":
		return m.reloadService(ctx, conn, name, initSystem)
		
	default:
		return false, fmt.Errorf("unsupported state: %s", state)
	}
}

// handleServiceEnabled manages service enabled/disabled state
func (m *ServiceModule) handleServiceEnabled(ctx context.Context, conn types.Connection, name string, enabled bool, initSystem string) (bool, error) {
	isEnabled := m.isServiceEnabled(ctx, conn, name, initSystem)
	
	if enabled == isEnabled {
		return false, nil
	}
	
	var cmd string
	switch initSystem {
	case "systemd":
		if enabled {
			cmd = fmt.Sprintf("systemctl enable %s", name)
		} else {
			cmd = fmt.Sprintf("systemctl disable %s", name)
		}
	case "sysvinit":
		if enabled {
			cmd = fmt.Sprintf("chkconfig %s on", name)
		} else {
			cmd = fmt.Sprintf("chkconfig %s off", name)
		}
	case "upstart":
		// Upstart doesn't have a standard enable/disable mechanism
		return false, nil
	default:
		return false, fmt.Errorf("cannot enable/disable service with init system: %s", initSystem)
	}
	
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return false, fmt.Errorf("failed to %s service: %v", map[bool]string{true: "enable", false: "disable"}[enabled], err)
	}
	
	return true, nil
}

// getServiceStatus gets the current status of a service
func (m *ServiceModule) getServiceStatus(ctx context.Context, conn types.Connection, name, initSystem string) string {
	var cmd string
	switch initSystem {
	case "systemd":
		cmd = fmt.Sprintf("systemctl is-active %s 2>/dev/null", name)
	case "sysvinit":
		cmd = fmt.Sprintf("service %s status 2>/dev/null | grep -q running && echo running || echo stopped", name)
	case "upstart":
		cmd = fmt.Sprintf("status %s 2>/dev/null | grep -q running && echo running || echo stopped", name)
	default:
		return "unknown"
	}
	
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return "stopped"
	}
	
	status := strings.TrimSpace(result.Message)
	if status == "" {
		return "stopped"
	}
	return status
}

// isServiceEnabled checks if a service is enabled
func (m *ServiceModule) isServiceEnabled(ctx context.Context, conn types.Connection, name, initSystem string) bool {
	var cmd string
	switch initSystem {
	case "systemd":
		cmd = fmt.Sprintf("systemctl is-enabled %s 2>/dev/null", name)
	case "sysvinit":
		cmd = fmt.Sprintf("chkconfig --list %s 2>/dev/null | grep -q ':on'", name)
	default:
		return false
	}
	
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return false
	}
	
	output := strings.TrimSpace(result.Message)
	return output == "enabled" || strings.Contains(output, ":on")
}

// startService starts a service
func (m *ServiceModule) startService(ctx context.Context, conn types.Connection, name, initSystem string) (bool, error) {
	var cmd string
	switch initSystem {
	case "systemd":
		cmd = fmt.Sprintf("systemctl start %s", name)
	case "sysvinit":
		cmd = fmt.Sprintf("service %s start", name)
	case "upstart":
		cmd = fmt.Sprintf("start %s", name)
	default:
		return false, fmt.Errorf("unsupported init system: %s", initSystem)
	}
	
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return false, fmt.Errorf("failed to start service: %v", err)
	}
	
	return true, nil
}

// stopService stops a service
func (m *ServiceModule) stopService(ctx context.Context, conn types.Connection, name, initSystem string) (bool, error) {
	var cmd string
	switch initSystem {
	case "systemd":
		cmd = fmt.Sprintf("systemctl stop %s", name)
	case "sysvinit":
		cmd = fmt.Sprintf("service %s stop", name)
	case "upstart":
		cmd = fmt.Sprintf("stop %s", name)
	default:
		return false, fmt.Errorf("unsupported init system: %s", initSystem)
	}
	
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return false, fmt.Errorf("failed to stop service: %v", err)
	}
	
	return true, nil
}

// restartService restarts a service
func (m *ServiceModule) restartService(ctx context.Context, conn types.Connection, name, initSystem string) (bool, error) {
	var cmd string
	switch initSystem {
	case "systemd":
		cmd = fmt.Sprintf("systemctl restart %s", name)
	case "sysvinit":
		cmd = fmt.Sprintf("service %s restart", name)
	case "upstart":
		cmd = fmt.Sprintf("restart %s", name)
	default:
		return false, fmt.Errorf("unsupported init system: %s", initSystem)
	}
	
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return false, fmt.Errorf("failed to restart service: %v", err)
	}
	
	return true, nil
}

// reloadService reloads a service
func (m *ServiceModule) reloadService(ctx context.Context, conn types.Connection, name, initSystem string) (bool, error) {
	var cmd string
	switch initSystem {
	case "systemd":
		cmd = fmt.Sprintf("systemctl reload %s", name)
	case "sysvinit":
		cmd = fmt.Sprintf("service %s reload", name)
	case "upstart":
		cmd = fmt.Sprintf("reload %s", name)
	default:
		return false, fmt.Errorf("unsupported init system: %s", initSystem)
	}
	
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		// Some services don't support reload, try restart
		return m.restartService(ctx, conn, name, initSystem)
	}
	
	return true, nil
}

// Validate checks if the module arguments are valid
func (m *ServiceModule) Validate(args map[string]interface{}) error {
	// Name is required
	name, ok := args["name"]
	if !ok || name == nil || name == "" {
		return types.NewValidationError("name", name, "required field is missing")
	}
	
	// Validate state if provided
	if state, ok := args["state"].(string); ok && state != "" {
		validStates := []string{"started", "stopped", "restarted", "reloaded"}
		valid := false
		for _, s := range validStates {
			if state == s {
				valid = true
				break
			}
		}
		if !valid {
			return types.NewValidationError("state", state, 
				fmt.Sprintf("must be one of: %s", strings.Join(validStates, ", ")))
		}
	}
	
	return nil
}

// Documentation returns the module documentation
func (m *ServiceModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "service",
		Description: "Manage services",
		Parameters: map[string]types.ParamDoc{
			"name": {
				Description: "Name of the service",
				Required:    true,
				Type:        "string",
			},
			"state": {
				Description: "State of the service",
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
				Description: "Run daemon-reload before doing any other operations (systemd only)",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
		},
		Examples: []string{
			"- name: Start nginx service\n  service:\n    name: nginx\n    state: started",
			"- name: Enable and start service\n  service:\n    name: httpd\n    state: started\n    enabled: true",
			"- name: Reload systemd and restart service\n  service:\n    name: myapp\n    state: restarted\n    daemon_reload: true",
		},
		Returns: map[string]string{
			"status":      "Current status of the service",
			"init_system": "Detected init system (systemd, sysvinit, upstart)",
		},
	}
}