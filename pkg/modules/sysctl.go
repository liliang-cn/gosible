package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// SysctlModule manages kernel parameters via sysctl
type SysctlModule struct {
	BaseModule
}

// NewSysctlModule creates a new sysctl module instance
func NewSysctlModule() *SysctlModule {
	return &SysctlModule{
		BaseModule: BaseModule{},
	}
}

// Name returns the module name
func (m *SysctlModule) Name() string {
	return "sysctl"
}

// Capabilities returns the module capabilities  
func (m *SysctlModule) Capabilities() *types.ModuleCapability {
	return &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     true,
		Platform:     "linux",
		RequiresRoot: true,
	}
}

// Validate validates the module arguments
func (m *SysctlModule) Validate(args map[string]interface{}) error {
	// Required parameters
	name := m.GetStringArg(args, "name", "")
	if name == "" {
		return types.NewValidationError("name", nil, "name is required")
	}

	// State validation
	state := m.GetStringArg(args, "state", "present")
	validStates := []string{"present", "absent"}
	if !m.isValidChoice(state, validStates) {
		return types.NewValidationError("state", state, fmt.Sprintf("state must be one of: %v", validStates))
	}

	// Value is required for present state
	if state == "present" {
		value := m.GetStringArg(args, "value", "")
		if value == "" {
			return types.NewValidationError("value", nil, "value is required when state is present")
		}
	}

	// Validate sysctl_set
	sysctlSet, ok := args["sysctl_set"]
	if ok {
		if _, ok := sysctlSet.(bool); !ok {
			return types.NewValidationError("sysctl_set", sysctlSet, "sysctl_set must be a boolean")
		}
	}

	// Validate reload
	reload, ok := args["reload"]
	if ok {
		if _, ok := reload.(bool); !ok {
			return types.NewValidationError("reload", reload, "reload must be a boolean")
		}
	}

	return nil
}

// Run executes the sysctl module
func (m *SysctlModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)
	diffMode := m.DiffMode(args)

	// Parse arguments
	name := m.GetStringArg(args, "name", "")
	value := m.GetStringArg(args, "value", "")
	state := m.GetStringArg(args, "state", "present")
	sysctlFile := m.GetStringArg(args, "sysctl_file", "/etc/sysctl.conf")
	sysctlSet := m.GetBoolArg(args, "sysctl_set", true)
	reload := m.GetBoolArg(args, "reload", true)
	ignoreFail := m.GetBoolArg(args, "ignoreerrors", false)

	// Convert dots to slashes for /proc/sys path
	procPath := "/proc/sys/" + strings.ReplaceAll(name, ".", "/")

	// Get current value
	currentValue, err := m.getCurrentValue(ctx, conn, name, procPath)
	if err != nil && !ignoreFail {
		return nil, fmt.Errorf("failed to get current value for %s: %w", name, err)
	}

	// Read current sysctl config file
	currentConfig, err := m.readSysctlFile(ctx, conn, sysctlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read sysctl file: %w", err)
	}

	// Store original for diff mode
	originalConfig := make(map[string]string)
	for k, v := range currentConfig {
		originalConfig[k] = v
	}

	// Initialize result
	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"name":          name,
		"value":         value,
		"current_value": currentValue,
		"state":         state,
	})

	changed := false
	var changes []string

	if state == "present" {
		// Check if we need to update the running value
		if currentValue != value && sysctlSet {
			changed = true
			changes = append(changes, fmt.Sprintf("Set %s = %s (was %s)", name, value, currentValue))
			
			if !checkMode {
				if err := m.setSysctlValue(ctx, conn, name, value, procPath); err != nil {
					if !ignoreFail {
						return nil, fmt.Errorf("failed to set sysctl value: %w", err)
					}
					if result.Data == nil {
						result.Data = make(map[string]interface{})
					}
					result.Data["warning"] = fmt.Sprintf("Failed to set runtime value: %v", err)
				}
			}
		}

		// Check if we need to update the config file
		configValue, exists := currentConfig[name]
		if !exists || configValue != value {
			changed = true
			if exists {
				changes = append(changes, fmt.Sprintf("Updated %s in %s", name, sysctlFile))
			} else {
				changes = append(changes, fmt.Sprintf("Added %s to %s", name, sysctlFile))
			}
			
			currentConfig[name] = value
			
			if !checkMode {
				if err := m.writeSysctlFile(ctx, conn, sysctlFile, currentConfig); err != nil {
					return nil, fmt.Errorf("failed to write sysctl file: %w", err)
				}
			}
		}

	} else { // state == "absent"
		// Check if parameter exists in config
		if _, exists := currentConfig[name]; exists {
			changed = true
			changes = append(changes, fmt.Sprintf("Removed %s from %s", name, sysctlFile))
			
			delete(currentConfig, name)
			
			if !checkMode {
				if err := m.writeSysctlFile(ctx, conn, sysctlFile, currentConfig); err != nil {
					return nil, fmt.Errorf("failed to write sysctl file: %w", err)
				}
			}
		}
	}

	// Reload sysctl if needed and requested
	if changed && reload && !checkMode {
		if err := m.reloadSysctl(ctx, conn); err != nil {
			if !ignoreFail {
				return nil, fmt.Errorf("failed to reload sysctl: %w", err)
			}
			if result.Data == nil {
				result.Data = make(map[string]interface{})
			}
			result.Data["warning"] = fmt.Sprintf("Failed to reload sysctl: %v", err)
		}
	}

	result.Changed = changed
	
	if changed {
		result.Message = strings.Join(changes, ", ")
		
		if diffMode {
			// Generate diff for config file changes
			originalContent := m.formatSysctlConfig(originalConfig)
			newContent := m.formatSysctlConfig(currentConfig)
			result.Diff = m.GenerateDiff(originalContent, newContent)
		}
	} else {
		result.Message = "Sysctl parameter is already in desired state"
	}

	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Helper methods

func (m *SysctlModule) getCurrentValue(ctx context.Context, conn types.Connection, name, procPath string) (string, error) {
	// Try using sysctl command first
	result, err := conn.Execute(ctx, fmt.Sprintf("sysctl -n %s", name), types.ExecuteOptions{})
	if err == nil && result.Success {
		if stdout, ok := result.Data["stdout"].(string); ok {
			return strings.TrimSpace(stdout), nil
		}
		return "", fmt.Errorf("no stdout in result")
	}

	// Fallback to reading from /proc/sys
	result, err = conn.Execute(ctx, fmt.Sprintf("cat %s", procPath), types.ExecuteOptions{})
	if err != nil {
		return "", err
	}
	
	if stdout, ok := result.Data["stdout"].(string); ok {
		return strings.TrimSpace(stdout), nil
	}
	return "", fmt.Errorf("no stdout in result")
}

func (m *SysctlModule) setSysctlValue(ctx context.Context, conn types.Connection, name, value, procPath string) error {
	// Try using sysctl command first
	result, err := conn.Execute(ctx, fmt.Sprintf("sysctl -w %s=%s", name, value), types.ExecuteOptions{})
	if err == nil && result.Success {
		return nil
	}

	// Fallback to writing to /proc/sys
	_, err = conn.Execute(ctx, fmt.Sprintf("echo '%s' > %s", value, procPath), types.ExecuteOptions{})
	return err
}

func (m *SysctlModule) readSysctlFile(ctx context.Context, conn types.Connection, file string) (map[string]string, error) {
	config := make(map[string]string)
	
	result, err := conn.Execute(ctx, fmt.Sprintf("cat %s 2>/dev/null || true", file), types.ExecuteOptions{})
	if err != nil {
		return config, err
	}
	
	var lines []string
	if stdout, ok := result.Data["stdout"].(string); ok {
		lines = strings.Split(stdout, "\n")
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Parse key = value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			config[key] = value
		}
	}
	
	return config, nil
}

func (m *SysctlModule) writeSysctlFile(ctx context.Context, conn types.Connection, file string, config map[string]string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(file)
	if _, err := conn.Execute(ctx, fmt.Sprintf("mkdir -p %s", dir), types.ExecuteOptions{}); err != nil {
		return err
	}
	
	// Format config content
	content := m.formatSysctlConfig(config)
	
	// Write to temp file first
	tmpFile := fmt.Sprintf("%s.tmp.%d", file, time.Now().Unix())
	cmd := fmt.Sprintf("cat > %s << 'EOF'\n%s\nEOF", tmpFile, content)
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return err
	}
	
	// Move temp file to final location
	if _, err := conn.Execute(ctx, fmt.Sprintf("mv %s %s", tmpFile, file), types.ExecuteOptions{}); err != nil {
		// Clean up temp file on error
		conn.Execute(ctx, fmt.Sprintf("rm -f %s", tmpFile), types.ExecuteOptions{})
		return err
	}
	
	return nil
}

func (m *SysctlModule) formatSysctlConfig(config map[string]string) string {
	var lines []string
	
	// Add header comment
	lines = append(lines, "# Sysctl configuration managed by Gosinble")
	lines = append(lines, fmt.Sprintf("# Generated at %s", time.Now().Format(time.RFC3339)))
	lines = append(lines, "")
	
	// Sort keys for consistent output
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	
	// Format each parameter
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s = %s", key, config[key]))
	}
	
	return strings.Join(lines, "\n")
}

func (m *SysctlModule) reloadSysctl(ctx context.Context, conn types.Connection) error {
	// Try sysctl --system first (newer systems)
	result, err := conn.Execute(ctx, "sysctl --system", types.ExecuteOptions{})
	if err == nil && result.Success {
		return nil
	}
	
	// Fallback to sysctl -p
	_, err = conn.Execute(ctx, "sysctl -p", types.ExecuteOptions{})
	return err
}

func (m *SysctlModule) isValidChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}