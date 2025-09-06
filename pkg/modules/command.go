package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gosinble/gosinble/pkg/types"
)

// CommandModule implements the command module for executing arbitrary commands
type CommandModule struct {
	*BaseModule
}

// NewCommandModule creates a new command module
func NewCommandModule() *CommandModule {
	doc := types.ModuleDoc{
		Name:        "command",
		Description: "Execute commands on targets",
		Parameters: map[string]types.ParamDoc{
			"cmd": {
				Description: "The command to execute",
				Required:    true,
				Type:        "string",
			},
			"chdir": {
				Description: "Change to this directory before running the command",
				Required:    false,
				Type:        "string",
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
			"timeout": {
				Description: "Timeout for the command in seconds",
				Required:    false,
				Type:        "int",
				Default:     30,
			},
			"warn": {
				Description: "Enable or disable warnings",
				Required:    false,
				Type:        "bool",
				Default:     true,
			},
			"stdin": {
				Description: "Set the stdin of the command directly to the specified value",
				Required:    false,
				Type:        "string",
			},
			"env": {
				Description: "Environment variables to set for the command",
				Required:    false,
				Type:        "map",
			},
			"become": {
				Description: "Run command with elevated privileges",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"become_user": {
				Description: "Run command as this user",
				Required:    false,
				Type:        "string",
			},
		},
		Examples: []string{
			`- name: Return motd to registered var
  command: cat /etc/motd
  register: mymotd`,
			`- name: Change the working directory
  command: /usr/bin/make_database.sh arg1 arg2
  args:
    chdir: /tmp`,
			`- name: Run command with timeout
  command: /bin/long_running_command
  args:
    timeout: 300`,
		},
		Returns: map[string]string{
			"stdout":    "Standard output of the command",
			"stderr":    "Standard error of the command",
			"exit_code": "Exit code of the command",
			"cmd":       "The executed command",
		},
	}

	return &CommandModule{
		BaseModule: NewBaseModule("command", doc),
	}
}

// Validate validates the module arguments
func (m *CommandModule) Validate(args map[string]interface{}) error {
	// Validate required fields
	if err := m.ValidateRequired(args, []string{"cmd"}); err != nil {
		return err
	}

	// Validate field types
	fieldTypes := map[string]string{
		"cmd":         "string",
		"chdir":       "string",
		"creates":     "string",
		"removes":     "string",
		"timeout":     "int",
		"warn":        "bool",
		"stdin":       "string",
		"env":         "map",
		"become":      "bool",
		"become_user": "string",
	}
	if err := m.ValidateTypes(args, fieldTypes); err != nil {
		return err
	}

	// Validate timeout is positive
	if timeout, err := m.GetIntArg(args, "timeout", 30); err != nil {
		return err
	} else if timeout < 0 {
		return types.NewValidationError("timeout", timeout, "timeout must be positive")
	}

	return nil
}

// Run executes the command module
func (m *CommandModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	return m.ExecuteWithTiming(ctx, conn, args, func() (*types.Result, error) {
		host := m.GetHostFromConnection(conn)

		// Get parameters
		cmd := m.GetStringArg(args, "cmd", "")
		chdir := m.GetStringArg(args, "chdir", "")
		creates := m.GetStringArg(args, "creates", "")
		removes := m.GetStringArg(args, "removes", "")
		timeoutSecs, _ := m.GetIntArg(args, "timeout", 30)
		warn := m.GetBoolArg(args, "warn", true)
		stdin := m.GetStringArg(args, "stdin", "")
		envMap := m.GetMapArg(args, "env")
		become := m.GetBoolArg(args, "become", false)
		becomeUser := m.GetStringArg(args, "become_user", "")
		
		// Check mode handling
		if m.CheckMode(args) {
			return m.CreateCheckModeResult(host, true, fmt.Sprintf("Would execute: %s", cmd), map[string]interface{}{
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

		// Prepare execution options
		options := types.ExecuteOptions{
			WorkingDir: chdir,
			Timeout:    time.Duration(timeoutSecs) * time.Second,
			Sudo:       become,
			User:       becomeUser,
		}

		// Set environment variables
		if envMap != nil {
			options.Env = make(map[string]string)
			for k, v := range envMap {
				options.Env[k] = types.ConvertToString(v)
			}
		}

		// Execute command with timeout handling
		result, err := m.HandleTimeout(ctx, options.Timeout, func(timeoutCtx context.Context) (*types.Result, error) {
			// Handle stdin if provided
			finalCmd := cmd
			if stdin != "" {
				finalCmd = fmt.Sprintf("echo %s | %s", m.escapeShell(stdin), cmd)
			}

			return conn.Execute(timeoutCtx, finalCmd, options)
		})

		if err != nil {
			return m.CreateErrorResult(host, fmt.Sprintf("Failed to execute command: %s", cmd), err), nil
		}

		// The connection already provides a result, but we need to ensure it has the correct format
		if result != nil {
			result.ModuleName = m.name
			result.Host = host

			// Add module-specific data
			if result.Data == nil {
				result.Data = make(map[string]interface{})
			}
			result.Data["cmd"] = cmd

			// Warn about non-zero exit codes if not expected
			if exitCode, ok := result.Data["exit_code"].(int); ok && exitCode != 0 && result.Success {
				result.Message = fmt.Sprintf("Command executed with non-zero exit code: %d", exitCode)
			}
		}

		return result, nil
	})
}

// checkFileExists checks if a file exists using the connection
func (m *CommandModule) checkFileExists(conn types.Connection, path string) (bool, error) {
	// Use a simple test command to check if file exists
	result, err := conn.Execute(context.Background(), fmt.Sprintf("test -e %s", m.escapeShell(path)), types.ExecuteOptions{})
	if err != nil {
		return false, err
	}

	return result.Success, nil
}

// escapeShell escapes shell special characters
func (m *CommandModule) escapeShell(input string) string {
	// Simple shell escaping - in production, use a more robust solution
	input = strings.ReplaceAll(input, "'", "'\"'\"'")
	return fmt.Sprintf("'%s'", input)
}

// checkAndWarnDangerousCommand warns about potentially dangerous commands
func (m *CommandModule) checkAndWarnDangerousCommand(cmd string) {
	dangerous := []string{"rm -rf", "mkfs", "dd ", "shutdown", "reboot", "halt", "init 0", "init 6"}
	
	cmdLower := strings.ToLower(cmd)
	for _, danger := range dangerous {
		if strings.Contains(cmdLower, danger) {
			m.LogWarn("Potentially dangerous command detected: %s", cmd)
			break
		}
	}
}