package utils

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// CommandChecker checks for the availability of system commands
type CommandChecker struct {
	mu              sync.RWMutex
	cache           map[string]bool
	installCommands map[string]map[string]string // command -> OS -> install command
}

// NewCommandChecker creates a new command checker with common utilities
func NewCommandChecker() *CommandChecker {
	return &CommandChecker{
		cache: make(map[string]bool),
		installCommands: map[string]map[string]string{
			"curl": {
				"darwin":  "brew install curl",
				"linux":   "apt-get install curl || yum install curl || dnf install curl",
				"windows": "choco install curl",
			},
			"wget": {
				"darwin":  "brew install wget",
				"linux":   "apt-get install wget || yum install wget || dnf install wget",
				"windows": "choco install wget",
			},
			"tar": {
				"darwin":  "# tar is pre-installed on macOS",
				"linux":   "apt-get install tar || yum install tar || dnf install tar",
				"windows": "# tar is available in Windows 10+",
			},
			"zip": {
				"darwin":  "# zip is pre-installed on macOS",
				"linux":   "apt-get install zip || yum install zip || dnf install zip",
				"windows": "choco install zip",
			},
			"unzip": {
				"darwin":  "# unzip is pre-installed on macOS",
				"linux":   "apt-get install unzip || yum install unzip || dnf install unzip",
				"windows": "choco install unzip",
			},
			"git": {
				"darwin":  "brew install git",
				"linux":   "apt-get install git || yum install git || dnf install git",
				"windows": "choco install git",
			},
			"make": {
				"darwin":  "xcode-select --install",
				"linux":   "apt-get install make || yum install make || dnf install make",
				"windows": "choco install make",
			},
			"rsync": {
				"darwin":  "brew install rsync",
				"linux":   "apt-get install rsync || yum install rsync || dnf install rsync",
				"windows": "choco install rsync",
			},
		},
	}
}

// CommandAvailable checks if a command is available in the system PATH
func (c *CommandChecker) CommandAvailable(command string) bool {
	c.mu.RLock()
	if available, ok := c.cache[command]; ok {
		c.mu.RUnlock()
		return available
	}
	c.mu.RUnlock()

	_, err := exec.LookPath(command)
	available := err == nil

	c.mu.Lock()
	c.cache[command] = available
	c.mu.Unlock()

	return available
}

// CommandAvailableWithContext checks if a command is available with context
func (c *CommandChecker) CommandAvailableWithContext(ctx context.Context, command string) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
		return c.CommandAvailable(command), nil
	}
}

// CheckRequired checks a list of required commands and returns missing ones
func (c *CommandChecker) CheckRequired(commands []string) (missing []string, err error) {
	for _, cmd := range commands {
		if !c.CommandAvailable(cmd) {
			missing = append(missing, cmd)
		}
	}
	return missing, nil
}

// CheckRequiredWithInstallInfo checks required commands and provides install instructions
func (c *CommandChecker) CheckRequiredWithInstallInfo(commands []string) (map[string]string, error) {
	missing := make(map[string]string)
	osName := runtime.GOOS

	for _, cmd := range commands {
		if !c.CommandAvailable(cmd) {
			if installCmds, ok := c.installCommands[cmd]; ok {
				if installCmd, ok := installCmds[osName]; ok {
					missing[cmd] = installCmd
				} else {
					missing[cmd] = fmt.Sprintf("Please install %s manually", cmd)
				}
			} else {
				missing[cmd] = fmt.Sprintf("Please install %s manually", cmd)
			}
		}
	}

	return missing, nil
}

// EnsureCommand ensures a command is available or returns installation instructions
func (c *CommandChecker) EnsureCommand(command string) error {
	if c.CommandAvailable(command) {
		return nil
	}

	osName := runtime.GOOS
	installInfo := ""

	if installCmds, ok := c.installCommands[command]; ok {
		if installCmd, ok := installCmds[osName]; ok {
			installInfo = installCmd
		}
	}

	if installInfo == "" {
		return fmt.Errorf("command '%s' not found, please install it manually", command)
	}

	return fmt.Errorf("command '%s' not found, install with: %s", command, installInfo)
}

// GetVersion gets the version of a command
func (c *CommandChecker) GetVersion(command string) (string, error) {
	if !c.CommandAvailable(command) {
		return "", fmt.Errorf("command '%s' not found", command)
	}

	// Common version flags
	versionFlags := []string{"--version", "-version", "version", "-v"}
	
	for _, flag := range versionFlags {
		cmd := exec.Command(command, flag)
		output, err := cmd.CombinedOutput()
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	}

	return "", fmt.Errorf("unable to get version for '%s'", command)
}

// InstallCommand attempts to install a command using the appropriate package manager
func (c *CommandChecker) InstallCommand(ctx context.Context, command string, sudo bool) error {
	if c.CommandAvailable(command) {
		return nil // Already installed
	}

	osName := runtime.GOOS
	installCmds, ok := c.installCommands[command]
	if !ok {
		return fmt.Errorf("no installation method known for '%s'", command)
	}

	installCmd, ok := installCmds[osName]
	if !ok {
		return fmt.Errorf("no installation method for '%s' on %s", command, osName)
	}

	// Skip comments
	if strings.HasPrefix(installCmd, "#") {
		return fmt.Errorf("%s", strings.TrimPrefix(installCmd, "# "))
	}

	// Handle multiple package managers (Linux)
	if strings.Contains(installCmd, "||") {
		commands := strings.Split(installCmd, "||")
		for _, cmd := range commands {
			cmd = strings.TrimSpace(cmd)
			if err := c.tryInstallWithCommand(ctx, cmd, sudo); err == nil {
				// Clear cache after successful installation
				c.mu.Lock()
				delete(c.cache, command)
				c.mu.Unlock()
				return nil
			}
		}
		return fmt.Errorf("failed to install '%s' with any available package manager", command)
	}

	// Single installation command
	if err := c.tryInstallWithCommand(ctx, installCmd, sudo); err != nil {
		return err
	}

	// Clear cache after successful installation
	c.mu.Lock()
	delete(c.cache, command)
	c.mu.Unlock()

	return nil
}

// tryInstallWithCommand attempts to run an installation command
func (c *CommandChecker) tryInstallWithCommand(ctx context.Context, installCmd string, sudo bool) error {
	parts := strings.Fields(installCmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty installation command")
	}

	// Check if the package manager exists
	pkgManager := parts[0]
	if !c.CommandAvailable(pkgManager) {
		return fmt.Errorf("package manager '%s' not found", pkgManager)
	}

	// Add sudo if needed and not already present
	if sudo && runtime.GOOS != "windows" && parts[0] != "sudo" {
		parts = append([]string{"sudo"}, parts...)
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("installation failed: %s, output: %s", err, string(output))
	}

	return nil
}

// CommonDependencies represents common system dependencies
type CommonDependencies struct {
	Required []string // Commands that must be present
	Optional []string // Commands that are nice to have
}

// GetCommonDependencies returns common dependencies for different use cases
func GetCommonDependencies(useCase string) *CommonDependencies {
	switch useCase {
	case "web":
		return &CommonDependencies{
			Required: []string{"curl", "tar"},
			Optional: []string{"wget", "unzip"},
		}
	case "archive":
		return &CommonDependencies{
			Required: []string{"tar"},
			Optional: []string{"zip", "unzip", "gzip", "bzip2"},
		}
	case "build":
		return &CommonDependencies{
			Required: []string{"make", "git"},
			Optional: []string{"gcc", "g++", "cmake"},
		}
	case "ansible-like":
		return &CommonDependencies{
			Required: []string{"ssh", "scp"},
			Optional: []string{"rsync", "curl", "tar"},
		}
	default:
		return &CommonDependencies{
			Required: []string{},
			Optional: []string{"curl", "wget", "tar", "zip", "unzip"},
		}
	}
}

// CheckDependencies checks all dependencies and returns a report
func (c *CommandChecker) CheckDependencies(deps *CommonDependencies) (*DependencyReport, error) {
	report := &DependencyReport{
		Required: make(map[string]bool),
		Optional: make(map[string]bool),
		Missing:  make(map[string]string),
	}

	// Check required dependencies
	for _, cmd := range deps.Required {
		available := c.CommandAvailable(cmd)
		report.Required[cmd] = available
		if !available {
			if missing, err := c.CheckRequiredWithInstallInfo([]string{cmd}); err == nil {
				for k, v := range missing {
					report.Missing[k] = v
				}
			}
		}
	}

	// Check optional dependencies
	for _, cmd := range deps.Optional {
		report.Optional[cmd] = c.CommandAvailable(cmd)
	}

	report.AllRequiredPresent = len(report.Missing) == 0

	return report, nil
}

// DependencyReport contains the results of a dependency check
type DependencyReport struct {
	Required           map[string]bool   // Required commands and their availability
	Optional           map[string]bool   // Optional commands and their availability
	Missing            map[string]string // Missing commands and installation instructions
	AllRequiredPresent bool              // Whether all required dependencies are present
}

// String returns a human-readable report
func (r *DependencyReport) String() string {
	var sb strings.Builder

	sb.WriteString("Dependency Check Report:\n")
	sb.WriteString("========================\n\n")

	sb.WriteString("Required Commands:\n")
	for cmd, available := range r.Required {
		status := "✓"
		if !available {
			status = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s %s\n", status, cmd))
	}

	if len(r.Optional) > 0 {
		sb.WriteString("\nOptional Commands:\n")
		for cmd, available := range r.Optional {
			status := "✓"
			if !available {
				status = "○"
			}
			sb.WriteString(fmt.Sprintf("  %s %s\n", status, cmd))
		}
	}

	if len(r.Missing) > 0 {
		sb.WriteString("\nMissing Commands - Installation Instructions:\n")
		for cmd, instruction := range r.Missing {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", cmd, instruction))
		}
	}

	return sb.String()
}