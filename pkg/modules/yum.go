package modules

import (
	"context"
	"fmt"
	"strings"

	"github.com/liliang-cn/gosible/pkg/types"
)

// YumModule manages packages using YUM on RHEL/CentOS systems
type YumModule struct {
	*BaseModule
}

// NewYumModule creates a new YUM module instance
func NewYumModule() *YumModule {
	doc := types.ModuleDoc{
		Name:        "yum",
		Description: "Manages packages using YUM on RHEL/CentOS systems",
		Parameters: map[string]types.ParamDoc{
			"name": {
				Description: "Name of the package to manage",
				Required:    false,
				Type:        "string",
			},
			"names": {
				Description: "List of packages to manage",
				Required:    false,
				Type:        "list",
			},
			"state": {
				Description: "State of the package (present, absent, latest, installed, removed)",
				Required:    false,
				Default:     "present",
				Type:        "string",
			},
			"enablerepo": {
				Description: "Enable specific repositories",
				Required:    false,
				Type:        "string",
			},
			"disablerepo": {
				Description: "Disable specific repositories",
				Required:    false,
				Type:        "string",
			},
			"update_cache": {
				Description: "Update the YUM cache",
				Required:    false,
				Default:     false,
				Type:        "bool",
			},
			"security": {
				Description: "Apply security updates only",
				Required:    false,
				Default:     false,
				Type:        "bool",
			},
			"autoremove": {
				Description: "Remove packages that are no longer needed",
				Required:    false,
				Default:     false,
				Type:        "bool",
			},
		},
	}
	return &YumModule{
		BaseModule: NewBaseModule("yum", doc),
	}
}

// Run executes the yum module
func (m *YumModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Parse arguments
	name := m.GetStringArg(args, "name", "")
	namesSlice := m.GetSliceArg(args, "names")
	state := m.GetStringArg(args, "state", "present")
	enablerepo := m.GetStringArg(args, "enablerepo", "")
	disablerepo := m.GetStringArg(args, "disablerepo", "")
	updateCache := m.GetBoolArg(args, "update_cache", false)
	securityUpdates := m.GetBoolArg(args, "security", false)
	autoremove := m.GetBoolArg(args, "autoremove", false)

	// Convert slice to string array
	var names []string
	for _, n := range namesSlice {
		if s, ok := n.(string); ok {
			names = append(names, s)
		}
	}

	// Normalize state
	if state == "installed" {
		state = "present"
	}
	if state == "removed" {
		state = "absent"
	}

	// Validate state
	validStates := []string{"present", "absent", "latest"}
	if !containsStr(validStates, state) {
		return m.CreateErrorResult("", fmt.Sprintf("Invalid state: %s", state), nil), nil
	}

	// Collect all package names
	var packages []string
	if name != "" {
		// Handle comma-separated names
		if strings.Contains(name, ",") {
			packages = append(packages, strings.Split(name, ",")...)
		} else {
			packages = append(packages, name)
		}
	}
	packages = append(packages, names...)

	// Build yum options
	var yumOptions []string
	if enablerepo != "" {
		yumOptions = append(yumOptions, fmt.Sprintf("--enablerepo=%s", enablerepo))
	}
	if disablerepo != "" {
		yumOptions = append(yumOptions, fmt.Sprintf("--disablerepo=%s", disablerepo))
	}
	if securityUpdates {
		yumOptions = append(yumOptions, "--security")
	}
	optionsStr := strings.Join(yumOptions, " ")

	changed := false
	var outputs []string

	// Update cache if requested
	if updateCache {
		cmd := "yum makecache"
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			return m.CreateErrorResult("", "Failed to update YUM cache", err), nil
		}
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
			}
		}
	}

	// Handle security updates without specific packages
	if securityUpdates && len(packages) == 0 {
		cmd := fmt.Sprintf("yum update -y --security %s", optionsStr)
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			return m.CreateErrorResult("", "Failed to apply security updates", err), nil
		}
		changed = true
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
				if strings.Contains(stdout, "No packages marked for update") {
					changed = false
				}
			}
		}
	}

	// Process each package
	for _, pkg := range packages {
		cmd := m.buildYumCommand(pkg, state, optionsStr)
		
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			// Check if package is already in desired state
			if result != nil && result.Data != nil {
				stderr, _ := result.Data["stderr"].(string)
				stdout, _ := result.Data["stdout"].(string)
				combined := stderr + stdout
				
				if state == "present" && strings.Contains(combined, "already installed and latest version") {
					continue
				}
				if state == "absent" && (strings.Contains(combined, "No Match for argument") ||
				                         strings.Contains(combined, "No Packages marked for removal")) {
					continue
				}
			}
			return m.CreateErrorResult("", fmt.Sprintf("Failed to %s package %s", state, pkg), err), nil
		}

		// Check if changes were made
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
				if strings.Contains(stdout, "Installing") ||
				   strings.Contains(stdout, "Updating") ||
				   strings.Contains(stdout, "Removing") ||
				   strings.Contains(stdout, "Erasing") {
					if !strings.Contains(stdout, "Nothing to do") {
						changed = true
					}
				}
			}
		}
	}

	// Autoremove if requested
	if autoremove {
		cmd := "yum autoremove -y"
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			return m.CreateErrorResult("", "Failed to autoremove packages", err), nil
		}
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
				if strings.Contains(stdout, "Removing") {
					changed = true
				}
			}
		}
	}

	// Build result message
	message := ""
	if len(packages) > 0 {
		message = fmt.Sprintf("Package(s) %s: %s", strings.Join(packages, ", "), state)
	} else if securityUpdates {
		message = "Security updates applied"
	} else if updateCache {
		message = "YUM cache updated"
	} else if autoremove {
		message = "Autoremove completed"
	}

	return m.CreateSuccessResult("", changed, message, map[string]interface{}{
		"output": strings.Join(outputs, "\n"),
	}), nil
}

func (m *YumModule) buildYumCommand(pkg, state, options string) string {
	switch state {
	case "present":
		return fmt.Sprintf("yum install -y %s %s", options, pkg)
	case "absent":
		return fmt.Sprintf("yum remove -y %s %s", options, pkg)
	case "latest":
		return fmt.Sprintf("yum update -y %s %s || yum install -y %s %s", options, pkg, options, pkg)
	default:
		return fmt.Sprintf("yum install -y %s %s", options, pkg)
	}
}

// containsStr checks if a string is in a slice
func containsStr(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// Validate checks if the module arguments are valid
func (m *YumModule) Validate(args map[string]interface{}) error {
	state := m.GetStringArg(args, "state", "present")
	if state != "" {
		// Normalize state for validation
		if state == "installed" {
			state = "present"
		}
		if state == "removed" {
			state = "absent"
		}
		
		validStates := []string{"present", "absent", "latest"}
		if !containsStr(validStates, state) {
			return fmt.Errorf("invalid state: %s", state)
		}
	}

	// Check that at least one action is specified
	name := m.GetStringArg(args, "name", "")
	namesSlice := m.GetSliceArg(args, "names")
	updateCache := m.GetBoolArg(args, "update_cache", false)
	security := m.GetBoolArg(args, "security", false)
	autoremove := m.GetBoolArg(args, "autoremove", false)

	if name == "" && len(namesSlice) == 0 && !updateCache && !security && !autoremove {
		return fmt.Errorf("at least one action must be specified")
	}

	return nil
}