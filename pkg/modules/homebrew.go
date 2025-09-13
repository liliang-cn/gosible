package modules

import (
	"context"
	"fmt"
	"strings"

	"github.com/liliang-cn/gosible/pkg/types"
)

// HomebrewModule manages packages using Homebrew on macOS
type HomebrewModule struct {
	*BaseModule
}

// NewHomebrewModule creates a new Homebrew module instance
func NewHomebrewModule() *HomebrewModule {
	doc := types.ModuleDoc{
		Name:        "homebrew",
		Description: "Manages packages using Homebrew on macOS",
		Parameters: map[string]types.ParamDoc{
			"name": {
				Description: "Name of the package to manage",
				Required:    false,
				Type:        "string",
			},
			"state": {
				Description: "State of the package (present, absent, latest, linked, unlinked)",
				Required:    false,
				Default:     "present",
				Type:        "string",
			},
			"cask": {
				Description: "Install as a cask application",
				Required:    false,
				Default:     false,
				Type:        "bool",
			},
			"update_homebrew": {
				Description: "Update Homebrew first",
				Required:    false,
				Default:     false,
				Type:        "bool",
			},
			"upgrade_all": {
				Description: "Upgrade all packages",
				Required:    false,
				Default:     false,
				Type:        "bool",
			},
		},
	}
	return &HomebrewModule{
		BaseModule: NewBaseModule("homebrew", doc),
	}
}

// Run executes the homebrew module
func (m *HomebrewModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Parse arguments
	name := m.GetStringArg(args, "name", "")
	namesSlice := m.GetSliceArg(args, "names")
	state := m.GetStringArg(args, "state", "present")
	updateHomebrew := m.GetBoolArg(args, "update_homebrew", false)
	upgradeAll := m.GetBoolArg(args, "upgrade_all", false)
	installOptions := m.GetStringArg(args, "install_options", "")
	cask := m.GetBoolArg(args, "cask", false)
	
	// Convert slice to string array
	var names []string
	for _, n := range namesSlice {
		if s, ok := n.(string); ok {
			names = append(names, s)
		}
	}

	// Validate state
	validStates := []string{"present", "absent", "latest", "linked", "unlinked"}
	if !contains(validStates, state) {
		return m.CreateErrorResult("", fmt.Sprintf("Invalid state: %s", state), nil), nil
	}

	// Collect all package names
	var packages []string
	if name != "" {
		packages = append(packages, name)
	}
	packages = append(packages, names...)

	if len(packages) == 0 && !upgradeAll && !updateHomebrew {
		return m.CreateErrorResult("", "No package specified", nil), nil
	}

	changed := false
	var outputs []string

	// Update homebrew if requested
	if updateHomebrew {
		result, err := conn.Execute(ctx, "brew update", types.ExecuteOptions{})
		if err != nil {
			return m.CreateErrorResult("", "Failed to update homebrew", err), nil
		}
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
			}
		}
	}

	// Upgrade all packages if requested
	if upgradeAll {
		result, err := conn.Execute(ctx, "brew upgrade", types.ExecuteOptions{})
		if err != nil {
			return m.CreateErrorResult("", "Failed to upgrade all packages", err), nil
		}
		changed = true
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
			}
		}
	}

	// Process each package
	for _, pkg := range packages {
		cmd := m.buildCommand(pkg, state, cask, installOptions)
		
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			// Check if package is already in desired state
			if result != nil && result.Data != nil {
				if stderr, ok := result.Data["stderr"].(string); ok {
					if state == "present" && strings.Contains(stderr, "already installed") {
						continue
					}
					if state == "absent" && strings.Contains(stderr, "not installed") {
						continue
					}
				}
			}
			return m.CreateErrorResult("", fmt.Sprintf("Failed to execute %s", cmd), err), nil
		}

		// Check if changes were made
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
				if strings.Contains(stdout, "Installing") || 
				   strings.Contains(stdout, "Upgrading") || 
				   strings.Contains(stdout, "Uninstalling") ||
				   strings.Contains(stdout, "Linking") ||
				   strings.Contains(stdout, "Unlinking") {
					changed = true
				}
			}
		}
	}

	// Build result message
	message := ""
	if len(packages) > 0 {
		message = fmt.Sprintf("Package(s) %s: %s", strings.Join(packages, ", "), state)
	} else if upgradeAll {
		message = "All packages upgraded"
	} else if updateHomebrew {
		message = "Homebrew updated"
	}

	return m.CreateSuccessResult("", changed, message, map[string]interface{}{
		"output": strings.Join(outputs, "\n"),
	}), nil
}

func (m *HomebrewModule) buildCommand(pkg, state string, cask bool, installOptions string) string {
	var parts []string
	
	switch state {
	case "present":
		parts = append(parts, "brew", "install")
		if cask {
			parts = append(parts, "--cask")
		}
		if installOptions != "" {
			parts = append(parts, installOptions)
		}
		parts = append(parts, pkg)
	case "absent":
		parts = append(parts, "brew", "uninstall")
		if cask {
			parts = append(parts, "--cask")
		}
		parts = append(parts, pkg)
	case "latest":
		parts = append(parts, "brew", "upgrade")
		if cask {
			parts = append(parts, "--cask")
		}
		parts = append(parts, pkg)
	case "linked":
		parts = append(parts, "brew", "link", pkg)
	case "unlinked":
		parts = append(parts, "brew", "unlink", pkg)
	}
	
	return strings.Join(parts, " ")
}

// contains checks if a string is in a slice
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// Validate checks if the module arguments are valid
func (m *HomebrewModule) Validate(args map[string]interface{}) error {
	state := m.GetStringArg(args, "state", "present")
	if state != "" {
		validStates := []string{"present", "absent", "latest", "linked", "unlinked"}
		if !contains(validStates, state) {
			return fmt.Errorf("invalid state: %s", state)
		}
	}

	// Check that at least one action is specified
	name := m.GetStringArg(args, "name", "")
	namesSlice := m.GetSliceArg(args, "names")
	updateHomebrew := m.GetBoolArg(args, "update_homebrew", false)
	upgradeAll := m.GetBoolArg(args, "upgrade_all", false)

	if name == "" && len(namesSlice) == 0 && !updateHomebrew && !upgradeAll {
		return fmt.Errorf("at least one of 'name', 'names', 'update_homebrew', or 'upgrade_all' must be specified")
	}

	return nil
}