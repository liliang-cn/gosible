package modules

import (
	"context"
	"fmt"
	"strings"

	"github.com/liliang-cn/gosible/pkg/types"
)

// AptModule manages packages using APT on Debian/Ubuntu systems
type AptModule struct {
	*BaseModule
}

// NewAptModule creates a new APT module instance
func NewAptModule() *AptModule {
	doc := types.ModuleDoc{
		Name:        "apt",
		Description: "Manages packages using APT on Debian/Ubuntu systems",
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
				Description: "State of the package (present, absent, latest, build-dep)",
				Required:    false,
				Default:     "present",
				Type:        "string",
			},
			"update_cache": {
				Description: "Update the APT cache",
				Required:    false,
				Default:     false,
				Type:        "bool",
			},
			"upgrade": {
				Description: "Upgrade packages (dist, full, safe, yes)",
				Required:    false,
				Type:        "string",
			},
			"autoremove": {
				Description: "Remove packages that are no longer needed",
				Required:    false,
				Default:     false,
				Type:        "bool",
			},
			"autoclean": {
				Description: "Remove obsolete packages from cache",
				Required:    false,
				Default:     false,
				Type:        "bool",
			},
			"force_apt_get": {
				Description: "Force usage of apt-get instead of apt",
				Required:    false,
				Default:     false,
				Type:        "bool",
			},
		},
	}
	return &AptModule{
		BaseModule: NewBaseModule("apt", doc),
	}
}

// Run executes the apt module
func (m *AptModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Parse arguments
	name := m.GetStringArg(args, "name", "")
	namesSlice := m.GetSliceArg(args, "names")
	state := m.GetStringArg(args, "state", "present")
	updateCache := m.GetBoolArg(args, "update_cache", false)
	upgradePackages := m.GetStringArg(args, "upgrade", "")
	autoremove := m.GetBoolArg(args, "autoremove", false)
	autoclean := m.GetBoolArg(args, "autoclean", false)
	forceAptGet := m.GetBoolArg(args, "force_apt_get", false)

	// Convert slice to string array
	var names []string
	for _, n := range namesSlice {
		if s, ok := n.(string); ok {
			names = append(names, s)
		}
	}

	// Validate state
	validStates := []string{"present", "absent", "latest", "build-dep"}
	if !contains(validStates, state) {
		return m.CreateErrorResult("", fmt.Sprintf("Invalid state: %s", state), nil), nil
	}

	// Validate upgrade option
	if upgradePackages != "" {
		validUpgrades := []string{"dist", "full", "safe", "yes"}
		if !contains(validUpgrades, upgradePackages) {
			return m.CreateErrorResult("", fmt.Sprintf("Invalid upgrade option: %s", upgradePackages), nil), nil
		}
	}

	// Collect all package names
	var packages []string
	if name != "" {
		packages = append(packages, name)
	}
	packages = append(packages, names...)

	// Choose apt or apt-get
	aptCmd := "apt"
	if forceAptGet {
		aptCmd = "apt-get"
	}

	changed := false
	var outputs []string

	// Update cache if requested
	if updateCache {
		cmd := fmt.Sprintf("DEBIAN_FRONTEND=noninteractive %s update", aptCmd)
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			return m.CreateErrorResult("", "Failed to update APT cache", err), nil
		}
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
			}
		}
	}

	// Handle system upgrade
	if upgradePackages != "" {
		var cmd string
		switch upgradePackages {
		case "dist":
			cmd = fmt.Sprintf("DEBIAN_FRONTEND=noninteractive %s dist-upgrade -y", aptCmd)
		case "full":
			cmd = fmt.Sprintf("DEBIAN_FRONTEND=noninteractive %s full-upgrade -y", aptCmd)
		case "safe", "yes":
			cmd = fmt.Sprintf("DEBIAN_FRONTEND=noninteractive %s upgrade -y", aptCmd)
		}
		
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			return m.CreateErrorResult("", fmt.Sprintf("Failed to upgrade packages: %s", upgradePackages), err), nil
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
		cmd := m.buildAptCommand(pkg, state, aptCmd)
		
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			// Check if package is already in desired state
			if result != nil && result.Data != nil {
				if stderr, ok := result.Data["stderr"].(string); ok {
					stdout, _ := result.Data["stdout"].(string)
					combined := stderr + stdout
					
					if state == "present" && strings.Contains(combined, "is already the newest version") {
						continue
					}
					if state == "absent" && (strings.Contains(combined, "is not installed") || 
					                        strings.Contains(combined, "Unable to locate package")) {
						continue
					}
				}
			}
			return m.CreateErrorResult("", fmt.Sprintf("Failed to %s package %s", state, pkg), err), nil
		}

		// Check if changes were made
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
				if strings.Contains(stdout, "newly installed") ||
				   strings.Contains(stdout, "upgraded") ||
				   strings.Contains(stdout, "to remove") ||
				   strings.Contains(stdout, "downgraded") {
					changed = true
				}
			}
		}
	}

	// Autoremove if requested
	if autoremove {
		cmd := fmt.Sprintf("DEBIAN_FRONTEND=noninteractive %s autoremove -y", aptCmd)
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			return m.CreateErrorResult("", "Failed to autoremove packages", err), nil
		}
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
				if strings.Contains(stdout, "to remove") {
					changed = true
				}
			}
		}
	}

	// Autoclean if requested
	if autoclean {
		cmd := fmt.Sprintf("DEBIAN_FRONTEND=noninteractive %s autoclean -y", aptCmd)
		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			return m.CreateErrorResult("", "Failed to autoclean", err), nil
		}
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				outputs = append(outputs, stdout)
			}
		}
	}

	// Build result message
	message := ""
	if len(packages) > 0 {
		message = fmt.Sprintf("Package(s) %s: %s", strings.Join(packages, ", "), state)
	} else if upgradePackages != "" {
		message = fmt.Sprintf("System upgrade: %s", upgradePackages)
	} else if updateCache {
		message = "APT cache updated"
	} else if autoremove {
		message = "Autoremove completed"
	} else if autoclean {
		message = "Autoclean completed"
	}

	return m.CreateSuccessResult("", changed, message, map[string]interface{}{
		"output": strings.Join(outputs, "\n"),
	}), nil
}

func (m *AptModule) buildAptCommand(pkg, state, aptCmd string) string {
	base := fmt.Sprintf("DEBIAN_FRONTEND=noninteractive %s", aptCmd)
	
	switch state {
	case "present":
		return fmt.Sprintf("%s install -y %s", base, pkg)
	case "absent":
		return fmt.Sprintf("%s remove -y %s", base, pkg)
	case "latest":
		return fmt.Sprintf("%s install -y %s", base, pkg)
	case "build-dep":
		return fmt.Sprintf("%s build-dep -y %s", base, pkg)
	default:
		return fmt.Sprintf("%s install -y %s", base, pkg)
	}
}


// Validate checks if the module arguments are valid
func (m *AptModule) Validate(args map[string]interface{}) error {
	state := m.GetStringArg(args, "state", "present")
	if state != "" {
		validStates := []string{"present", "absent", "latest", "build-dep"}
		if !contains(validStates, state) {
			return fmt.Errorf("invalid state: %s", state)
		}
	}

	upgrade := m.GetStringArg(args, "upgrade", "")
	if upgrade != "" {
		validUpgrades := []string{"dist", "full", "safe", "yes"}
		if !contains(validUpgrades, upgrade) {
			return fmt.Errorf("invalid upgrade option: %s", upgrade)
		}
	}

	// Check that at least one action is specified
	name := m.GetStringArg(args, "name", "")
	namesSlice := m.GetSliceArg(args, "names")
	updateCache := m.GetBoolArg(args, "update_cache", false)
	autoremove := m.GetBoolArg(args, "autoremove", false)
	autoclean := m.GetBoolArg(args, "autoclean", false)

	if name == "" && len(namesSlice) == 0 && !updateCache && upgrade == "" && !autoremove && !autoclean {
		return fmt.Errorf("at least one action must be specified")
	}

	return nil
}