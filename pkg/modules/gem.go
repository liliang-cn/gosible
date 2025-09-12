package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// GemModule manages Ruby gems
type GemModule struct {
	BaseModule
}

// NewGemModule creates a new gem module instance
func NewGemModule() *GemModule {
	return &GemModule{
		BaseModule: BaseModule{},
	}
}

// Name returns the module name
func (m *GemModule) Name() string {
	return "gem"
}

// Capabilities returns the module capabilities
func (m *GemModule) Capabilities() *types.ModuleCapability {
	return &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     false,
		Platform:     "posix",
		RequiresRoot: false,
	}
}

// Validate validates the module arguments
func (m *GemModule) Validate(args map[string]interface{}) error {
	// Name is required
	name := m.GetStringArg(args, "name", "")
	if name == "" {
		return types.NewValidationError("name", nil, "name is required")
	}

	// State validation
	state := m.GetStringArg(args, "state", "present")
	validStates := []string{"present", "absent", "latest"}
	if !m.isValidChoice(state, validStates) {
		return types.NewValidationError("state", state, fmt.Sprintf("state must be one of: %v", validStates))
	}

	// User install validation
	userInstall, ok := args["user_install"]
	if ok {
		if _, ok := userInstall.(bool); !ok {
			return types.NewValidationError("user_install", userInstall, "user_install must be a boolean")
		}
	}

	// Pre-release validation
	preRelease, ok := args["pre_release"]
	if ok {
		if _, ok := preRelease.(bool); !ok {
			return types.NewValidationError("pre_release", preRelease, "pre_release must be a boolean")
		}
	}

	return nil
}

// Run executes the gem module
func (m *GemModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)

	// Parse arguments
	name := m.GetStringArg(args, "name", "")
	version := m.GetStringArg(args, "version", "")
	state := m.GetStringArg(args, "state", "present")
	gemSource := m.GetStringArg(args, "source", "")
	includeDependencies := m.GetBoolArg(args, "include_dependencies", true)
	userInstall := m.GetBoolArg(args, "user_install", true)
	executable := m.GetStringArg(args, "executable", "gem")
	installDir := m.GetStringArg(args, "install_dir", "")
	binDir := m.GetStringArg(args, "bin_dir", "")
	preRelease := m.GetBoolArg(args, "pre_release", false)
	env := m.GetMapArg(args, "env_vars")
	buildFlags := m.GetStringArg(args, "build_flags", "")

	// Build gem command options
	gemOpts := []string{}
	if userInstall {
		gemOpts = append(gemOpts, "--user-install")
	}
	if !includeDependencies {
		gemOpts = append(gemOpts, "--ignore-dependencies")
	}
	if gemSource != "" {
		gemOpts = append(gemOpts, fmt.Sprintf("--source %s", gemSource))
	}
	if installDir != "" {
		gemOpts = append(gemOpts, fmt.Sprintf("--install-dir %s", installDir))
	}
	if binDir != "" {
		gemOpts = append(gemOpts, fmt.Sprintf("--bindir %s", binDir))
	}
	if preRelease {
		gemOpts = append(gemOpts, "--pre")
	}
	if buildFlags != "" {
		gemOpts = append(gemOpts, fmt.Sprintf("-- %s", buildFlags))
	}

	// Set execution options
	execOpts := types.ExecuteOptions{}
	if env != nil {
		execOpts.Env = make(map[string]string)
		for k, v := range env {
			execOpts.Env[k] = fmt.Sprintf("%v", v)
		}
	}

	// Initialize result
	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"name":    name,
		"version": version,
		"state":   state,
	})

	// Parse gem list
	gems := m.parseGemList(name)
	var messages []string
	changed := false

	for _, gem := range gems {
		gemChanged, msg, err := m.handleGem(ctx, conn, gem, version, state, 
			executable, gemOpts, checkMode, execOpts)
		if err != nil {
			return nil, err
		}
		if gemChanged {
			changed = true
		}
		if msg != "" {
			messages = append(messages, msg)
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

func (m *GemModule) handleGem(ctx context.Context, conn types.Connection, gem, version, state, 
	executable string, gemOpts []string, checkMode bool, execOpts types.ExecuteOptions) (bool, string, error) {
	
	// Check if gem is installed
	installed, installedVersion, err := m.getGemInfo(ctx, conn, gem, executable, execOpts)
	if err != nil {
		return false, "", err
	}

	switch state {
	case "present":
		if !installed || (version != "" && installedVersion != version) {
			if checkMode {
				if version != "" {
					return true, fmt.Sprintf("Would install %s version %s", gem, version), nil
				}
				return true, fmt.Sprintf("Would install %s", gem), nil
			}

			cmd := fmt.Sprintf("%s install", executable)
			if len(gemOpts) > 0 {
				cmd = fmt.Sprintf("%s %s", cmd, strings.Join(gemOpts, " "))
			}
			if version != "" {
				cmd = fmt.Sprintf("%s %s --version %s", cmd, gem, version)
			} else {
				cmd = fmt.Sprintf("%s %s", cmd, gem)
			}

			if _, err := conn.Execute(ctx, cmd, execOpts); err != nil {
				return false, "", fmt.Errorf("failed to install %s: %w", gem, err)
			}
			
			if version != "" {
				return true, fmt.Sprintf("Installed %s version %s", gem, version), nil
			}
			return true, fmt.Sprintf("Installed %s", gem), nil
		}
		return false, "", nil

	case "absent":
		if installed {
			if checkMode {
				return true, fmt.Sprintf("Would uninstall %s", gem), nil
			}

			cmd := fmt.Sprintf("%s uninstall -x %s", executable, gem)
			if version != "" {
				cmd = fmt.Sprintf("%s --version %s", cmd, version)
			} else {
				cmd = fmt.Sprintf("%s --all", cmd)
			}

			if _, err := conn.Execute(ctx, cmd, execOpts); err != nil {
				return false, "", fmt.Errorf("failed to uninstall %s: %w", gem, err)
			}
			return true, fmt.Sprintf("Uninstalled %s", gem), nil
		}
		return false, "", nil

	case "latest":
		if checkMode {
			return true, fmt.Sprintf("Would update %s to latest", gem), nil
		}

		// Check latest version available
		latestVersion, err := m.getLatestVersion(ctx, conn, gem, executable, execOpts)
		if err != nil {
			return false, "", err
		}

		if !installed || installedVersion != latestVersion {
			cmd := fmt.Sprintf("%s install", executable)
			if len(gemOpts) > 0 {
				cmd = fmt.Sprintf("%s %s", cmd, strings.Join(gemOpts, " "))
			}
			cmd = fmt.Sprintf("%s %s", cmd, gem)

			if _, err := conn.Execute(ctx, cmd, execOpts); err != nil {
				return false, "", fmt.Errorf("failed to update %s: %w", gem, err)
			}
			return true, fmt.Sprintf("Updated %s to latest version", gem), nil
		}
		return false, "", nil
	}

	return false, "", nil
}

func (m *GemModule) getGemInfo(ctx context.Context, conn types.Connection, gem, executable string, 
	execOpts types.ExecuteOptions) (bool, string, error) {
	
	cmd := fmt.Sprintf("%s list --local %s 2>/dev/null", executable, gem)
	result, err := conn.Execute(ctx, cmd, execOpts)
	if err != nil {
		return false, "", err
	}

	// Parse output for gem and version
	var lines []string
	if stdout, ok := result.Data["stdout"].(string); ok {
		lines = strings.Split(stdout, "\n")
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, gem) {
			// Extract version from parentheses
			if idx := strings.Index(line, "("); idx > 0 {
				if endIdx := strings.Index(line[idx:], ")"); endIdx > 0 {
					version := strings.TrimSpace(line[idx+1 : idx+endIdx])
					// Remove any default markers
					version = strings.TrimSuffix(version, " default")
					return true, version, nil
				}
			}
			return true, "", nil
		}
	}

	return false, "", nil
}

func (m *GemModule) getLatestVersion(ctx context.Context, conn types.Connection, gem, executable string,
	execOpts types.ExecuteOptions) (string, error) {
	
	cmd := fmt.Sprintf("%s search ^%s$ --remote 2>/dev/null | head -1", executable, gem)
	result, err := conn.Execute(ctx, cmd, execOpts)
	if err != nil {
		return "", err
	}

	// Parse output for latest version
	line := ""
	if stdout, ok := result.Data["stdout"].(string); ok {
		line = strings.TrimSpace(stdout)
	}
	if idx := strings.Index(line, "("); idx > 0 {
		if endIdx := strings.Index(line[idx:], ")"); endIdx > 0 {
			version := strings.TrimSpace(line[idx+1 : idx+endIdx])
			return version, nil
		}
	}

	return "", fmt.Errorf("could not determine latest version for %s", gem)
}

func (m *GemModule) parseGemList(gems string) []string {
	if gems == "" {
		return []string{}
	}
	
	// Split by comma or space
	var result []string
	for _, gem := range strings.Split(gems, ",") {
		gem = strings.TrimSpace(gem)
		if gem != "" {
			result = append(result, gem)
		}
	}
	return result
}

func (m *GemModule) isValidChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}