package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// PipModule manages Python packages via pip
type PipModule struct {
	BaseModule
}

// NewPipModule creates a new pip module instance
func NewPipModule() *PipModule {
	return &PipModule{
		BaseModule: BaseModule{},
	}
}

// Name returns the module name
func (m *PipModule) Name() string {
	return "pip"
}

// Capabilities returns the module capabilities
func (m *PipModule) Capabilities() *types.ModuleCapability {
	return &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     false,
		Platform:     "posix",
		RequiresRoot: false,
	}
}

// Validate validates the module arguments
func (m *PipModule) Validate(args map[string]interface{}) error {
	// Either name or requirements is required
	name := m.GetStringArg(args, "name", "")
	requirements := m.GetStringArg(args, "requirements", "")
	
	if name == "" && requirements == "" {
		return types.NewValidationError("name/requirements", nil, "either name or requirements is required")
	}

	// State validation
	state := m.GetStringArg(args, "state", "present")
	validStates := []string{"present", "absent", "latest", "forcereinstall"}
	if !m.isValidChoice(state, validStates) {
		return types.NewValidationError("state", state, fmt.Sprintf("state must be one of: %v", validStates))
	}

	return nil
}

// Run executes the pip module
func (m *PipModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)

	// Parse arguments
	name := m.GetStringArg(args, "name", "")
	requirements := m.GetStringArg(args, "requirements", "")
	version := m.GetStringArg(args, "version", "")
	state := m.GetStringArg(args, "state", "present")
	virtualenv := m.GetStringArg(args, "virtualenv", "")
	virtualenvCommand := m.GetStringArg(args, "virtualenv_command", "virtualenv")
	virtualenvPython := m.GetStringArg(args, "virtualenv_python", "")
	extraArgs := m.GetStringArg(args, "extra_args", "")
	editable := m.GetBoolArg(args, "editable", false)
	chdir := m.GetStringArg(args, "chdir", "")
	executable := m.GetStringArg(args, "executable", "")
	umask := m.GetStringArg(args, "umask", "")

	// Determine pip command
	pipCmd := "pip"
	if executable != "" {
		pipCmd = executable
	} else if virtualenv != "" {
		pipCmd = fmt.Sprintf("%s/bin/pip", virtualenv)
	}

	// Create virtualenv if needed
	if virtualenv != "" {
		exists, err := m.virtualenvExists(ctx, conn, virtualenv)
		if err != nil {
			return nil, fmt.Errorf("failed to check virtualenv: %w", err)
		}
		
		if !exists {
			if checkMode {
				result := m.CreateSuccessResult(hostname, true, fmt.Sprintf("Would create virtualenv: %s", virtualenv), nil)
				return result, nil
			}
			
			if err := m.createVirtualenv(ctx, conn, virtualenv, virtualenvCommand, virtualenvPython); err != nil {
				return nil, fmt.Errorf("failed to create virtualenv: %w", err)
			}
		}
	}

	// Initialize result
	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"name":       name,
		"version":    version,
		"state":      state,
		"virtualenv": virtualenv,
	})

	changed := false
	var messages []string

	// Handle requirements file
	if requirements != "" {
		if state == "present" {
			if checkMode {
				result.Changed = true
				result.Message = fmt.Sprintf("Would install requirements from %s", requirements)
				return result, nil
			}

			cmd := fmt.Sprintf("%s install -r %s", pipCmd, requirements)
			if extraArgs != "" {
				cmd = fmt.Sprintf("%s %s", cmd, extraArgs)
			}
			
			opts := types.ExecuteOptions{}
			if chdir != "" {
				opts.WorkingDir = chdir
			}
			if umask != "" {
				opts.Env = map[string]string{"UMASK": umask}
			}

			output, err := conn.Execute(ctx, cmd, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to install requirements: %w", err)
			}
			
			// Check if anything was installed
			if stdout, ok := output.Data["stdout"].(string); ok && strings.Contains(stdout, "Successfully installed") {
				changed = true
				messages = append(messages, fmt.Sprintf("Installed requirements from %s", requirements))
			}
		}
	} else {
		// Handle individual packages
		packages := m.parsePackageList(name)
		
		for _, pkg := range packages {
			pkgChanged, msg, err := m.handlePackage(ctx, conn, pkg, version, state, pipCmd, 
				extraArgs, editable, chdir, umask, checkMode)
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
	}

	result.Changed = changed
	if len(messages) > 0 {
		result.Message = strings.Join(messages, ", ")
	} else {
		result.Message = "No changes needed"
	}

	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

func (m *PipModule) handlePackage(ctx context.Context, conn types.Connection, pkg, version, state, pipCmd, 
	extraArgs string, editable bool, chdir, umask string, checkMode bool) (bool, string, error) {
	
	// Check if package is installed
	installed, installedVersion, err := m.getPackageInfo(ctx, conn, pkg, pipCmd)
	if err != nil {
		return false, "", err
	}

	opts := types.ExecuteOptions{}
	if chdir != "" {
		opts.WorkingDir = chdir
	}
	if umask != "" {
		opts.Env = map[string]string{"UMASK": umask}
	}

	switch state {
	case "present":
		if !installed || (version != "" && installedVersion != version) {
			if checkMode {
				if version != "" {
					return true, fmt.Sprintf("Would install %s==%s", pkg, version), nil
				}
				return true, fmt.Sprintf("Would install %s", pkg), nil
			}

			cmd := fmt.Sprintf("%s install", pipCmd)
			if editable {
				cmd = fmt.Sprintf("%s -e", cmd)
			}
			if version != "" {
				cmd = fmt.Sprintf("%s %s==%s", cmd, pkg, version)
			} else {
				cmd = fmt.Sprintf("%s %s", cmd, pkg)
			}
			if extraArgs != "" {
				cmd = fmt.Sprintf("%s %s", cmd, extraArgs)
			}

			if _, err := conn.Execute(ctx, cmd, opts); err != nil {
				return false, "", fmt.Errorf("failed to install %s: %w", pkg, err)
			}
			
			if version != "" {
				return true, fmt.Sprintf("Installed %s==%s", pkg, version), nil
			}
			return true, fmt.Sprintf("Installed %s", pkg), nil
		}
		return false, "", nil

	case "absent":
		if installed {
			if checkMode {
				return true, fmt.Sprintf("Would uninstall %s", pkg), nil
			}

			cmd := fmt.Sprintf("%s uninstall -y %s", pipCmd, pkg)
			if extraArgs != "" {
				cmd = fmt.Sprintf("%s %s", cmd, extraArgs)
			}

			if _, err := conn.Execute(ctx, cmd, opts); err != nil {
				return false, "", fmt.Errorf("failed to uninstall %s: %w", pkg, err)
			}
			return true, fmt.Sprintf("Uninstalled %s", pkg), nil
		}
		return false, "", nil

	case "latest":
		if checkMode {
			return true, fmt.Sprintf("Would upgrade %s to latest", pkg), nil
		}

		cmd := fmt.Sprintf("%s install --upgrade %s", pipCmd, pkg)
		if extraArgs != "" {
			cmd = fmt.Sprintf("%s %s", cmd, extraArgs)
		}

		output, err := conn.Execute(ctx, cmd, opts)
		if err != nil {
			return false, "", fmt.Errorf("failed to upgrade %s: %w", pkg, err)
		}
		
		if stdout, ok := output.Data["stdout"].(string); ok {
			if strings.Contains(stdout, "Successfully installed") || 
			   strings.Contains(stdout, "Requirement already up-to-date") {
				if strings.Contains(stdout, "Successfully installed") {
					return true, fmt.Sprintf("Upgraded %s to latest", pkg), nil
				}
			}
		}
		return false, "", nil

	case "forcereinstall":
		if checkMode {
			return true, fmt.Sprintf("Would force reinstall %s", pkg), nil
		}

		cmd := fmt.Sprintf("%s install --force-reinstall", pipCmd)
		if version != "" {
			cmd = fmt.Sprintf("%s %s==%s", cmd, pkg, version)
		} else {
			cmd = fmt.Sprintf("%s %s", cmd, pkg)
		}
		if extraArgs != "" {
			cmd = fmt.Sprintf("%s %s", cmd, extraArgs)
		}

		if _, err := conn.Execute(ctx, cmd, opts); err != nil {
			return false, "", fmt.Errorf("failed to force reinstall %s: %w", pkg, err)
		}
		return true, fmt.Sprintf("Force reinstalled %s", pkg), nil
	}

	return false, "", nil
}

func (m *PipModule) getPackageInfo(ctx context.Context, conn types.Connection, pkg, pipCmd string) (bool, string, error) {
	cmd := fmt.Sprintf("%s show %s 2>/dev/null", pipCmd, pkg)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return false, "", nil
	}

	// Parse output for version
	var lines []string
	if stdout, ok := result.Data["stdout"].(string); ok {
		lines = strings.Split(stdout, "\n")
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			return true, version, nil
		}
	}

	return true, "", nil
}

func (m *PipModule) virtualenvExists(ctx context.Context, conn types.Connection, path string) (bool, error) {
	cmd := fmt.Sprintf("test -d %s/bin && test -f %s/bin/pip", path, path)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		if result != nil && !result.Success {
			return false, nil
		}
		return false, err
	}
	return result.Success, nil
}

func (m *PipModule) createVirtualenv(ctx context.Context, conn types.Connection, path, command, python string) error {
	cmd := command
	if python != "" {
		cmd = fmt.Sprintf("%s -p %s", cmd, python)
	}
	cmd = fmt.Sprintf("%s %s", cmd, path)
	
	_, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err
}

func (m *PipModule) parsePackageList(packages string) []string {
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

func (m *PipModule) isValidChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}