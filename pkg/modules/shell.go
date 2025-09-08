package modules

import (
	"context"
	"fmt"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// ShellModule implements the shell module for executing commands through shell
type ShellModule struct {
	*BaseModule
}

// NewShellModule creates a new shell module
func NewShellModule() *ShellModule {
	doc := types.ModuleDoc{
		Name:        "shell",
		Description: "Execute shell commands on targets",
		Parameters: map[string]types.ParamDoc{
			"cmd": {
				Description: "The shell command to execute",
				Required:    true,
				Type:        "string",
			},
			"chdir": {
				Description: "Change to this directory before running the command",
				Required:    false,
				Type:        "string",
			},
			"executable": {
				Description: "Change the shell used to execute the command",
				Required:    false,
				Type:        "string",
				Default:     "/bin/sh",
			},
			"creates": {
				Description: "A filename or glob pattern. If it already exists, this step will not be run",
				Required:    false,
				Type:        "string",
			},
			"removes": {
				Description: "A filename or glob pattern. If it does not exist, this step will not be run",
				Required:    false,
				Type:        "string",
			},
			"warn": {
				Description: "Enable or disable warnings",
				Required:    false,
				Type:        "bool",
				Default:     true,
			},
		},
		Examples: []string{
			`- name: Execute complex shell command
  shell: echo "Hello" | grep -o H`,
			`- name: Run command in specific directory
  shell: ls -la
  args:
    chdir: /tmp`,
			`- name: Use different shell
  shell: echo $0
  args:
    executable: /bin/bash`,
		},
		Returns: map[string]string{
			"stdout":    "Standard output of the command",
			"stderr":    "Standard error of the command",
			"exit_code": "Exit code of the command",
			"cmd":       "The executed command",
		},
	}

	return &ShellModule{
		BaseModule: NewBaseModule("shell", doc),
	}
}

// Validate validates the module arguments
func (m *ShellModule) Validate(args map[string]interface{}) error {
	// Validate required fields
	if err := m.ValidateRequired(args, []string{"cmd"}); err != nil {
		return err
	}

	// Validate field types
	fieldTypes := map[string]string{
		"cmd":        "string",
		"chdir":      "string",
		"executable": "string",
		"creates":    "string",
		"removes":    "string",
		"warn":       "bool",
	}
	return m.ValidateTypes(args, fieldTypes)
}

// Run executes the shell module
func (m *ShellModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	return m.ExecuteWithTiming(ctx, conn, args, func() (*types.Result, error) {
		host := m.GetHostFromConnection(conn)

		// Get parameters
		cmd := m.GetStringArg(args, "cmd", "")
		chdir := m.GetStringArg(args, "chdir", "")
		executable := m.GetStringArg(args, "executable", "/bin/sh")
		creates := m.GetStringArg(args, "creates", "")
		removes := m.GetStringArg(args, "removes", "")
		warn := m.GetBoolArg(args, "warn", true)

		// Check mode handling
		if m.CheckMode(args) {
			return m.CreateCheckModeResult(host, true, fmt.Sprintf("Would execute shell command: %s", cmd), map[string]interface{}{
				"cmd": cmd,
			}), nil
		}

		// Check creates condition
		if creates != "" {
			if exists, err := m.checkFileExists(conn, creates); err != nil {
				return m.CreateErrorResult(host, "Failed to check creates condition", err), nil
			} else if exists {
				return m.CreateSuccessResult(host, false, fmt.Sprintf("Skipped, since %s exists", creates), map[string]interface{}{
					"cmd":     cmd,
					"skipped": true,
				}), nil
			}
		}

		// Check removes condition
		if removes != "" {
			if exists, err := m.checkFileExists(conn, removes); err != nil {
				return m.CreateErrorResult(host, "Failed to check removes condition", err), nil
			} else if !exists {
				return m.CreateSuccessResult(host, false, fmt.Sprintf("Skipped, since %s does not exist", removes), map[string]interface{}{
					"cmd":     cmd,
					"skipped": true,
				}), nil
			}
		}

		// Show warnings for potentially dangerous commands
		if warn {
			m.checkAndWarnDangerousCommand(cmd)
		}

		// Prepare the shell command
		shellCmd := fmt.Sprintf("%s -c %s", executable, m.escapeShell(cmd))

		// Prepare execution options
		options := types.ExecuteOptions{
			WorkingDir: chdir,
		}

		// Execute the shell command
		result, err := conn.Execute(ctx, shellCmd, options)
		if err != nil {
			return m.CreateErrorResult(host, fmt.Sprintf("Failed to execute shell command: %s", cmd), err), nil
		}

		// Update result with module-specific information
		if result != nil {
			result.ModuleName = m.name
			result.Host = host
			
			if result.Data == nil {
				result.Data = make(map[string]interface{})
			}
			result.Data["cmd"] = cmd
			result.Data["shell"] = executable
		}

		return result, nil
	})
}

// checkFileExists checks if a file exists using the connection
func (m *ShellModule) checkFileExists(conn types.Connection, path string) (bool, error) {
	result, err := conn.Execute(context.Background(), fmt.Sprintf("test -e %s", m.escapeShell(path)), types.ExecuteOptions{})
	if err != nil {
		return false, err
	}
	return result.Success, nil
}

// escapeShell escapes shell special characters
func (m *ShellModule) escapeShell(input string) string {
	// Simple shell escaping
	input = fmt.Sprintf("'%s'", input)
	return input
}

// checkAndWarnDangerousCommand warns about potentially dangerous shell commands
func (m *ShellModule) checkAndWarnDangerousCommand(cmd string) {
	dangerous := []string{"rm -rf", "mkfs", "dd if=", "shutdown", "reboot", "halt", ":(){ :|:& };:"}
	
	for _, danger := range dangerous {
		if containsPattern(cmd, danger) {
			m.LogWarn("Potentially dangerous shell command detected: %s", cmd)
			break
		}
	}
}

// containsPattern checks if command contains dangerous pattern
func containsPattern(cmd, pattern string) bool {
	return len(cmd) >= len(pattern) && 
		   (cmd == pattern || 
			fmt.Sprintf(" %s", cmd)[1:len(pattern)+1] == pattern ||
			cmd[len(cmd)-len(pattern):] == pattern ||
			fmt.Sprintf(" %s ", cmd)[1:len(cmd)+1] != cmd)
}