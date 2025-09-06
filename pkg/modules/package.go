package modules

import (
	"context"
	"fmt"
	"strings"
	
	"github.com/gosinble/gosinble/pkg/types"
)

// PackageModule manages system packages
type PackageModule struct {
	BaseModule
}

// NewPackageModule creates a new package module instance
func NewPackageModule() *PackageModule {
	return &PackageModule{
		BaseModule: BaseModule{
			name: "package",
		},
	}
}

// Run executes the package module
func (m *PackageModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Get arguments
	name, _ := args["name"].(string)
	state, _ := args["state"].(string)
	updateCache, _ := args["update_cache"].(bool)
	
	// Default state is present
	if state == "" {
		state = "present"
	}
	
	result := &types.Result{
		Success: true,
		Changed: false,
		Data:    make(map[string]interface{}),
	}
	
	// Detect package manager
	pkgMgr := m.detectPackageManager(ctx, conn)
	result.Data["package_manager"] = pkgMgr
	
	if pkgMgr == "" {
		result.Success = false
		result.Error = fmt.Errorf("could not detect package manager")
		return result, nil
	}
	
	// Update cache if requested
	if updateCache {
		if err := m.updatePackageCache(ctx, conn, pkgMgr); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to update package cache: %v", err)
			return result, nil
		}
		result.Changed = true
	}
	
	// Handle multiple packages (space or comma separated)
	packages := m.parsePackageList(name)
	
	for _, pkg := range packages {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" {
			continue
		}
		
		// Check if package is installed
		installed := m.isPackageInstalled(ctx, conn, pkg, pkgMgr)
		
		switch state {
		case "present", "installed":
			if !installed {
				if err := m.installPackage(ctx, conn, pkg, pkgMgr); err != nil {
					result.Success = false
					result.Error = fmt.Errorf("failed to install %s: %v", pkg, err)
					return result, nil
				}
				result.Changed = true
			}
			
		case "absent", "removed":
			if installed {
				if err := m.removePackage(ctx, conn, pkg, pkgMgr); err != nil {
					result.Success = false
					result.Error = fmt.Errorf("failed to remove %s: %v", pkg, err)
					return result, nil
				}
				result.Changed = true
			}
			
		case "latest":
			if installed {
				updated, err := m.updatePackage(ctx, conn, pkg, pkgMgr)
				if err != nil {
					result.Success = false
					result.Error = fmt.Errorf("failed to update %s: %v", pkg, err)
					return result, nil
				}
				if updated {
					result.Changed = true
				}
			} else {
				// Install if not present
				if err := m.installPackage(ctx, conn, pkg, pkgMgr); err != nil {
					result.Success = false
					result.Error = fmt.Errorf("failed to install %s: %v", pkg, err)
					return result, nil
				}
				result.Changed = true
			}
			
		default:
			result.Success = false
			result.Error = fmt.Errorf("unsupported state: %s", state)
			return result, nil
		}
	}
	
	if result.Changed {
		result.Message = fmt.Sprintf("Package(s) %s state changed to %s", name, state)
	} else {
		result.Message = fmt.Sprintf("Package(s) %s already in state %s", name, state)
	}
	
	return result, nil
}

// detectPackageManager detects the system's package manager
func (m *PackageModule) detectPackageManager(ctx context.Context, conn types.Connection) string {
	managers := []struct {
		name string
		cmd  string
	}{
		{"apt", "which apt-get"},
		{"yum", "which yum"},
		{"dnf", "which dnf"},
		{"zypper", "which zypper"},
		{"pacman", "which pacman"},
		{"apk", "which apk"},
		{"pkg", "which pkg"},
	}
	
	for _, mgr := range managers {
		result, err := conn.Execute(ctx, mgr.cmd, types.ExecuteOptions{})
		if err == nil && result.Success && strings.TrimSpace(result.Message) != "" {
			return mgr.name
		}
	}
	
	return ""
}

// parsePackageList parses a package list string
func (m *PackageModule) parsePackageList(packages string) []string {
	// Support both comma and space separation
	packages = strings.ReplaceAll(packages, ",", " ")
	parts := strings.Fields(packages)
	return parts
}

// updatePackageCache updates the package manager cache
func (m *PackageModule) updatePackageCache(ctx context.Context, conn types.Connection, pkgMgr string) error {
	var cmd string
	switch pkgMgr {
	case "apt":
		cmd = "apt-get update"
	case "yum":
		cmd = "yum makecache"
	case "dnf":
		cmd = "dnf makecache"
	case "zypper":
		cmd = "zypper refresh"
	case "pacman":
		cmd = "pacman -Sy"
	case "apk":
		cmd = "apk update"
	case "pkg":
		cmd = "pkg update"
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgMgr)
	}
	
	_, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err
}

// isPackageInstalled checks if a package is installed
func (m *PackageModule) isPackageInstalled(ctx context.Context, conn types.Connection, pkg, pkgMgr string) bool {
	var cmd string
	switch pkgMgr {
	case "apt":
		cmd = fmt.Sprintf("dpkg -l %s 2>/dev/null | grep -q '^ii'", pkg)
	case "yum", "dnf":
		cmd = fmt.Sprintf("rpm -q %s >/dev/null 2>&1", pkg)
	case "zypper":
		cmd = fmt.Sprintf("rpm -q %s >/dev/null 2>&1", pkg)
	case "pacman":
		cmd = fmt.Sprintf("pacman -Q %s >/dev/null 2>&1", pkg)
	case "apk":
		cmd = fmt.Sprintf("apk info -e %s >/dev/null 2>&1", pkg)
	case "pkg":
		cmd = fmt.Sprintf("pkg info %s >/dev/null 2>&1", pkg)
	default:
		return false
	}
	
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err == nil && result.Success
}

// installPackage installs a package
func (m *PackageModule) installPackage(ctx context.Context, conn types.Connection, pkg, pkgMgr string) error {
	var cmd string
	switch pkgMgr {
	case "apt":
		cmd = fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", pkg)
	case "yum":
		cmd = fmt.Sprintf("yum install -y %s", pkg)
	case "dnf":
		cmd = fmt.Sprintf("dnf install -y %s", pkg)
	case "zypper":
		cmd = fmt.Sprintf("zypper install -y %s", pkg)
	case "pacman":
		cmd = fmt.Sprintf("pacman -S --noconfirm %s", pkg)
	case "apk":
		cmd = fmt.Sprintf("apk add %s", pkg)
	case "pkg":
		cmd = fmt.Sprintf("pkg install -y %s", pkg)
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgMgr)
	}
	
	_, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err
}

// removePackage removes a package
func (m *PackageModule) removePackage(ctx context.Context, conn types.Connection, pkg, pkgMgr string) error {
	var cmd string
	switch pkgMgr {
	case "apt":
		cmd = fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get remove -y %s", pkg)
	case "yum":
		cmd = fmt.Sprintf("yum remove -y %s", pkg)
	case "dnf":
		cmd = fmt.Sprintf("dnf remove -y %s", pkg)
	case "zypper":
		cmd = fmt.Sprintf("zypper remove -y %s", pkg)
	case "pacman":
		cmd = fmt.Sprintf("pacman -R --noconfirm %s", pkg)
	case "apk":
		cmd = fmt.Sprintf("apk del %s", pkg)
	case "pkg":
		cmd = fmt.Sprintf("pkg delete -y %s", pkg)
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgMgr)
	}
	
	_, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err
}

// updatePackage updates a package to latest version
func (m *PackageModule) updatePackage(ctx context.Context, conn types.Connection, pkg, pkgMgr string) (bool, error) {
	// First check if update is available
	var checkCmd string
	switch pkgMgr {
	case "apt":
		checkCmd = fmt.Sprintf("apt-get install --dry-run %s 2>/dev/null | grep -q 'upgraded'", pkg)
	case "yum":
		checkCmd = fmt.Sprintf("yum check-update %s >/dev/null 2>&1; [ $? -eq 100 ]", pkg)
	case "dnf":
		checkCmd = fmt.Sprintf("dnf check-update %s >/dev/null 2>&1; [ $? -eq 100 ]", pkg)
	default:
		// For other package managers, just try to update
		return true, m.installPackage(ctx, conn, pkg, pkgMgr)
	}
	
	result, _ := conn.Execute(ctx, checkCmd, types.ExecuteOptions{})
	if !result.Success {
		// No update available
		return false, nil
	}
	
	// Update the package
	return true, m.installPackage(ctx, conn, pkg, pkgMgr)
}

// Validate checks if the module arguments are valid
func (m *PackageModule) Validate(args map[string]interface{}) error {
	// Name is required
	name, ok := args["name"]
	if !ok || name == nil || name == "" {
		return types.NewValidationError("name", name, "required field is missing")
	}
	
	// Validate state if provided
	if state, ok := args["state"].(string); ok && state != "" {
		validStates := []string{"present", "absent", "latest", "installed", "removed"}
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
func (m *PackageModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "package",
		Description: "Manage packages",
		Parameters: map[string]types.ParamDoc{
			"name": {
				Description: "Name of the package(s) to manage (comma or space separated for multiple)",
				Required:    true,
				Type:        "string",
			},
			"state": {
				Description: "State of the package",
				Required:    false,
				Type:        "string",
				Default:     "present",
				Choices:     []string{"present", "absent", "latest", "installed", "removed"},
			},
			"update_cache": {
				Description: "Update the package cache before installing/removing",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
		},
		Examples: []string{
			"- name: Install nginx\n  package:\n    name: nginx\n    state: present",
			"- name: Install multiple packages\n  package:\n    name: git,vim,curl\n    state: present",
			"- name: Update package to latest\n  package:\n    name: docker\n    state: latest\n    update_cache: true",
			"- name: Remove package\n  package:\n    name: apache2\n    state: absent",
		},
		Returns: map[string]string{
			"package_manager": "Detected package manager (apt, yum, dnf, etc.)",
		},
	}
}