package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// NpmModule manages Node.js packages via npm
type NpmModule struct {
	BaseModule
}

// NewNpmModule creates a new npm module instance
func NewNpmModule() *NpmModule {
	return &NpmModule{
		BaseModule: BaseModule{},
	}
}

// Name returns the module name
func (m *NpmModule) Name() string {
	return "npm"
}

// Capabilities returns the module capabilities
func (m *NpmModule) Capabilities() *types.ModuleCapability {
	return &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     false,
		Platform:     "posix",
		RequiresRoot: false,
	}
}

// Validate validates the module arguments
func (m *NpmModule) Validate(args map[string]interface{}) error {
	// Name is optional (can install from package.json)
	// name := m.GetStringArg(args, "name", "")
	
	// State validation
	state := m.GetStringArg(args, "state", "present")
	validStates := []string{"present", "absent", "latest"}
	if !m.isValidChoice(state, validStates) {
		return types.NewValidationError("state", state, fmt.Sprintf("state must be one of: %v", validStates))
	}

	// Global validation
	global, ok := args["global"]
	if ok {
		if _, ok := global.(bool); !ok {
			return types.NewValidationError("global", global, "global must be a boolean")
		}
	}

	// Production validation
	production, ok := args["production"]
	if ok {
		if _, ok := production.(bool); !ok {
			return types.NewValidationError("production", production, "production must be a boolean")
		}
	}

	return nil
}

// Run executes the npm module
func (m *NpmModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)

	// Parse arguments
	name := m.GetStringArg(args, "name", "")
	version := m.GetStringArg(args, "version", "")
	path := m.GetStringArg(args, "path", "")
	executable := m.GetStringArg(args, "executable", "npm")
	registry := m.GetStringArg(args, "registry", "")
	state := m.GetStringArg(args, "state", "present")
	global := m.GetBoolArg(args, "global", false)
	production := m.GetBoolArg(args, "production", false)
	ignoreScripts := m.GetBoolArg(args, "ignore_scripts", false)
	unsafePerm := m.GetBoolArg(args, "unsafe_perm", false)
	ciMode := m.GetBoolArg(args, "ci", false)

	// Build npm command options
	npmOpts := []string{}
	if global {
		npmOpts = append(npmOpts, "-g")
	}
	if production {
		npmOpts = append(npmOpts, "--production")
	}
	if ignoreScripts {
		npmOpts = append(npmOpts, "--ignore-scripts")
	}
	if unsafePerm {
		npmOpts = append(npmOpts, "--unsafe-perm")
	}
	if registry != "" {
		npmOpts = append(npmOpts, fmt.Sprintf("--registry=%s", registry))
	}

	// Set execution options
	execOpts := types.ExecuteOptions{}
	if path != "" && !global {
		execOpts.WorkingDir = path
	}

	// Initialize result
	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"name":    name,
		"version": version,
		"state":   state,
		"global":  global,
		"path":    path,
	})

	changed := false
	var message string

	// Handle package.json installation (no name specified)
	if name == "" {
		if state != "present" {
			return nil, fmt.Errorf("name is required when state is %s", state)
		}

		if checkMode {
			result.Changed = true
			result.Message = "Would install dependencies from package.json"
			return result, nil
		}

		var cmd string
		if ciMode {
			cmd = fmt.Sprintf("%s ci", executable)
		} else {
			cmd = fmt.Sprintf("%s install", executable)
		}
		if len(npmOpts) > 0 {
			cmd = fmt.Sprintf("%s %s", cmd, strings.Join(npmOpts, " "))
		}

		output, err := conn.Execute(ctx, cmd, execOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to install dependencies: %w", err)
		}

		// Check if packages were installed
		if stdout, ok := output.Data["stdout"].(string); ok {
			if strings.Contains(stdout, "added") || strings.Contains(stdout, "updated") {
				changed = true
				message = "Installed dependencies from package.json"
			}
		} else {
			message = "Dependencies already up to date"
		}
	} else {
		// Handle individual packages
		packages := m.parsePackageList(name)
		var messages []string

		for _, pkg := range packages {
			pkgChanged, msg, err := m.handlePackage(ctx, conn, pkg, version, state, 
				executable, npmOpts, global, path, checkMode)
			if err != nil {
				return nil, err
			}
			if pkgChanged {
				changed = true
			}
			if msg != "" {
				messages = append(messages, msg)
			}
		}

		if len(messages) > 0 {
			message = strings.Join(messages, ", ")
		} else {
			message = "No changes needed"
		}
	}

	result.Changed = changed
	result.Message = message

	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

func (m *NpmModule) handlePackage(ctx context.Context, conn types.Connection, pkg, version, state, 
	executable string, npmOpts []string, global bool, path string, checkMode bool) (bool, string, error) {
	
	// Check if package is installed
	installed, installedVersion, err := m.getPackageInfo(ctx, conn, pkg, executable, global, path)
	if err != nil {
		return false, "", err
	}

	execOpts := types.ExecuteOptions{}
	if path != "" && !global {
		execOpts.WorkingDir = path
	}

	switch state {
	case "present":
		if !installed || (version != "" && installedVersion != version) {
			if checkMode {
				if version != "" {
					return true, fmt.Sprintf("Would install %s@%s", pkg, version), nil
				}
				return true, fmt.Sprintf("Would install %s", pkg), nil
			}

			cmd := fmt.Sprintf("%s install", executable)
			if len(npmOpts) > 0 {
				cmd = fmt.Sprintf("%s %s", cmd, strings.Join(npmOpts, " "))
			}
			if version != "" {
				cmd = fmt.Sprintf("%s %s@%s", cmd, pkg, version)
			} else {
				cmd = fmt.Sprintf("%s %s", cmd, pkg)
			}

			if _, err := conn.Execute(ctx, cmd, execOpts); err != nil {
				return false, "", fmt.Errorf("failed to install %s: %w", pkg, err)
			}
			
			if version != "" {
				return true, fmt.Sprintf("Installed %s@%s", pkg, version), nil
			}
			return true, fmt.Sprintf("Installed %s", pkg), nil
		}
		return false, "", nil

	case "absent":
		if installed {
			if checkMode {
				return true, fmt.Sprintf("Would uninstall %s", pkg), nil
			}

			cmd := fmt.Sprintf("%s uninstall", executable)
			if len(npmOpts) > 0 {
				cmd = fmt.Sprintf("%s %s", cmd, strings.Join(npmOpts, " "))
			}
			cmd = fmt.Sprintf("%s %s", cmd, pkg)

			if _, err := conn.Execute(ctx, cmd, execOpts); err != nil {
				return false, "", fmt.Errorf("failed to uninstall %s: %w", pkg, err)
			}
			return true, fmt.Sprintf("Uninstalled %s", pkg), nil
		}
		return false, "", nil

	case "latest":
		if checkMode {
			return true, fmt.Sprintf("Would update %s to latest", pkg), nil
		}

		cmd := fmt.Sprintf("%s update", executable)
		if len(npmOpts) > 0 {
			cmd = fmt.Sprintf("%s %s", cmd, strings.Join(npmOpts, " "))
		}
		cmd = fmt.Sprintf("%s %s", cmd, pkg)

		output, err := conn.Execute(ctx, cmd, execOpts)
		if err != nil {
			return false, "", fmt.Errorf("failed to update %s: %w", pkg, err)
		}
		
		if stdout, ok := output.Data["stdout"].(string); ok && strings.Contains(stdout, "updated") {
			return true, fmt.Sprintf("Updated %s to latest", pkg), nil
		}
		return false, "", nil
	}

	return false, "", nil
}

func (m *NpmModule) getPackageInfo(ctx context.Context, conn types.Connection, pkg, executable string, 
	global bool, path string) (bool, string, error) {
	
	cmd := fmt.Sprintf("%s list --depth=0 --json", executable)
	if global {
		cmd = fmt.Sprintf("%s -g", cmd)
	}
	cmd = fmt.Sprintf("%s %s 2>/dev/null", cmd, pkg)
	
	execOpts := types.ExecuteOptions{}
	if path != "" && !global {
		execOpts.WorkingDir = path
	}

	result, err := conn.Execute(ctx, cmd, execOpts)
	if err != nil || !result.Success {
		return false, "", nil
	}

	// Parse JSON output to check if package exists
	// For simplicity, just check if package name appears in output
	if stdout, ok := result.Data["stdout"].(string); ok && strings.Contains(stdout, fmt.Sprintf("\"%s\"", pkg)) {
		// Try to extract version
		lines := strings.Split(stdout, "\n")
		for _, line := range lines {
			if strings.Contains(line, "\"version\"") {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					version := strings.Trim(parts[1], " \",")
					return true, version, nil
				}
			}
		}
		return true, "", nil
	}

	return false, "", nil
}

func (m *NpmModule) parsePackageList(packages string) []string {
	if packages == "" {
		return []string{}
	}
	
	// Split by comma or space
	var result []string
	for _, pkg := range strings.Split(packages, ",") {
		pkg = strings.TrimSpace(pkg)
		if pkg != "" {
			result = append(result, pkg)
		}
	}
	return result
}

func (m *NpmModule) isValidChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}