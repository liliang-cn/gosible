package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	
	"github.com/gosinble/gosinble/pkg/types"
)

// FileModule manages files and directories
type FileModule struct {
	BaseModule
}

// NewFileModule creates a new file module instance
func NewFileModule() *FileModule {
	return &FileModule{
		BaseModule: BaseModule{
			name: "file",
		},
	}
}

// Run executes the file module
func (m *FileModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Get arguments
	path, _ := args["path"].(string)
	state, _ := args["state"].(string)
	mode, _ := args["mode"].(string)
	owner, _ := args["owner"].(string)
	group, _ := args["group"].(string)
	src, _ := args["src"].(string)
	recurse, _ := args["recurse"].(bool)
	force, _ := args["force"].(bool)
	
	// Default state is file
	if state == "" {
		state = "file"
	}
	
	result := &types.Result{
		Success: true,
		Changed: false,
		Data:    make(map[string]interface{}),
	}
	
	// Check if file/directory exists
	checkCmd := fmt.Sprintf("test -e %s && echo EXISTS || echo NOTEXISTS", path)
	checkResult, err := conn.Execute(ctx, checkCmd, types.ExecuteOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to check path: %v", err)
	}
	
	exists := strings.TrimSpace(checkResult.Message) == "EXISTS"
	
	switch state {
	case "directory":
		return m.handleDirectory(ctx, conn, path, mode, owner, group, exists, recurse)
		
	case "file":
		return m.handleFile(ctx, conn, path, mode, owner, group, exists)
		
	case "link":
		return m.handleLink(ctx, conn, path, src, force, exists)
		
	case "absent":
		return m.handleAbsent(ctx, conn, path, exists)
		
	case "touch":
		return m.handleTouch(ctx, conn, path, mode, owner, group)
		
	default:
		result.Success = false
		result.Error = fmt.Errorf("unsupported state: %s", state)
		return result, nil
	}
}

// handleDirectory creates or updates a directory
func (m *FileModule) handleDirectory(ctx context.Context, conn types.Connection, path, mode, owner, group string, exists bool, recurse bool) (*types.Result, error) {
	result := &types.Result{
		Success: true,
		Changed: false,
		Data:    make(map[string]interface{}),
	}
	
	// Create directory if it doesn't exist
	if !exists {
		mkdirCmd := "mkdir -p " + path
		if _, err := conn.Execute(ctx, mkdirCmd, types.ExecuteOptions{}); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to create directory: %v", err)
			return result, nil
		}
		result.Changed = true
		result.Message = "Directory created"
	} else {
		// Check if it's actually a directory
		checkCmd := fmt.Sprintf("test -d %s && echo DIR || echo NOTDIR", path)
		checkResult, _ := conn.Execute(ctx, checkCmd, types.ExecuteOptions{})
		if strings.TrimSpace(checkResult.Message) != "DIR" {
			result.Success = false
			result.Error = fmt.Errorf("path exists but is not a directory")
			return result, nil
		}
	}
	
	// Set permissions if specified
	if mode != "" {
		if err := m.setMode(ctx, conn, path, mode, recurse); err != nil {
			result.Success = false
			result.Error = err
			return result, nil
		}
		result.Changed = true
	}
	
	// Set ownership if specified
	if owner != "" || group != "" {
		if err := m.setOwnership(ctx, conn, path, owner, group, recurse); err != nil {
			result.Success = false
			result.Error = err
			return result, nil
		}
		result.Changed = true
	}
	
	if !result.Changed {
		result.Message = "Directory already exists"
	}
	
	return result, nil
}

// handleFile creates or updates a file
func (m *FileModule) handleFile(ctx context.Context, conn types.Connection, path, mode, owner, group string, exists bool) (*types.Result, error) {
	result := &types.Result{
		Success: true,
		Changed: false,
		Data:    make(map[string]interface{}),
	}
	
	// Create file if it doesn't exist
	if !exists {
		touchCmd := fmt.Sprintf("touch %s", path)
		if _, err := conn.Execute(ctx, touchCmd, types.ExecuteOptions{}); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to create file: %v", err)
			return result, nil
		}
		result.Changed = true
		result.Message = "File created"
	} else {
		// Check if it's actually a file
		checkCmd := fmt.Sprintf("test -f %s && echo FILE || echo NOTFILE", path)
		checkResult, _ := conn.Execute(ctx, checkCmd, types.ExecuteOptions{})
		if strings.TrimSpace(checkResult.Message) != "FILE" {
			result.Success = false
			result.Error = fmt.Errorf("path exists but is not a file")
			return result, nil
		}
		result.Message = "File already exists"
	}
	
	// Set permissions if specified
	if mode != "" {
		if err := m.setMode(ctx, conn, path, mode, false); err != nil {
			result.Success = false
			result.Error = err
			return result, nil
		}
		result.Changed = true
	}
	
	// Set ownership if specified
	if owner != "" || group != "" {
		if err := m.setOwnership(ctx, conn, path, owner, group, false); err != nil {
			result.Success = false
			result.Error = err
			return result, nil
		}
		result.Changed = true
	}
	
	return result, nil
}

// handleLink creates a symbolic link
func (m *FileModule) handleLink(ctx context.Context, conn types.Connection, path, src string, force bool, exists bool) (*types.Result, error) {
	result := &types.Result{
		Success: true,
		Changed: false,
		Data:    make(map[string]interface{}),
	}
	
	if src == "" {
		result.Success = false
		result.Error = fmt.Errorf("src is required for link state")
		return result, nil
	}
	
	// Check if link already exists and points to correct target
	if exists {
		checkCmd := fmt.Sprintf("readlink %s", path)
		checkResult, err := conn.Execute(ctx, checkCmd, types.ExecuteOptions{})
		if err == nil && strings.TrimSpace(checkResult.Message) == src {
			result.Message = "Link already exists with correct target"
			return result, nil
		}
		
		// Remove existing file/link if force is true
		if force {
			rmCmd := fmt.Sprintf("rm -f %s", path)
			if _, err := conn.Execute(ctx, rmCmd, types.ExecuteOptions{}); err != nil {
				result.Success = false
				result.Error = fmt.Errorf("failed to remove existing path: %v", err)
				return result, nil
			}
		} else {
			result.Success = false
			result.Error = fmt.Errorf("path already exists, use force=true to replace")
			return result, nil
		}
	}
	
	// Create symbolic link
	lnCmd := fmt.Sprintf("ln -s %s %s", src, path)
	if _, err := conn.Execute(ctx, lnCmd, types.ExecuteOptions{}); err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to create link: %v", err)
		return result, nil
	}
	
	result.Changed = true
	result.Message = "Link created"
	return result, nil
}

// handleAbsent removes a file or directory
func (m *FileModule) handleAbsent(ctx context.Context, conn types.Connection, path string, exists bool) (*types.Result, error) {
	result := &types.Result{
		Success: true,
		Changed: false,
		Data:    make(map[string]interface{}),
	}
	
	if !exists {
		result.Message = "Path already absent"
		return result, nil
	}
	
	// Remove the path
	rmCmd := fmt.Sprintf("rm -rf %s", path)
	if _, err := conn.Execute(ctx, rmCmd, types.ExecuteOptions{}); err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to remove path: %v", err)
		return result, nil
	}
	
	result.Changed = true
	result.Message = "Path removed"
	return result, nil
}

// handleTouch updates file timestamps
func (m *FileModule) handleTouch(ctx context.Context, conn types.Connection, path, mode, owner, group string) (*types.Result, error) {
	result := &types.Result{
		Success: true,
		Changed: true,
		Data:    make(map[string]interface{}),
	}
	
	// Touch the file
	touchCmd := fmt.Sprintf("touch %s", path)
	if _, err := conn.Execute(ctx, touchCmd, types.ExecuteOptions{}); err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to touch file: %v", err)
		return result, nil
	}
	
	// Set permissions if specified
	if mode != "" {
		if err := m.setMode(ctx, conn, path, mode, false); err != nil {
			result.Success = false
			result.Error = err
			return result, nil
		}
	}
	
	// Set ownership if specified
	if owner != "" || group != "" {
		if err := m.setOwnership(ctx, conn, path, owner, group, false); err != nil {
			result.Success = false
			result.Error = err
			return result, nil
		}
	}
	
	result.Message = "File touched"
	return result, nil
}

// setMode sets file permissions
func (m *FileModule) setMode(ctx context.Context, conn types.Connection, path, mode string, recurse bool) error {
	// Validate mode
	if _, err := strconv.ParseInt(mode, 8, 32); err != nil {
		return fmt.Errorf("invalid mode: %s", mode)
	}
	
	chmodCmd := fmt.Sprintf("chmod %s %s", mode, path)
	if recurse {
		chmodCmd = fmt.Sprintf("chmod -R %s %s", mode, path)
	}
	
	if _, err := conn.Execute(ctx, chmodCmd, types.ExecuteOptions{}); err != nil {
		return fmt.Errorf("failed to set mode: %v", err)
	}
	
	return nil
}

// setOwnership sets file ownership
func (m *FileModule) setOwnership(ctx context.Context, conn types.Connection, path, owner, group string, recurse bool) error {
	if owner == "" && group == "" {
		return nil
	}
	
	ownership := ""
	if owner != "" && group != "" {
		ownership = fmt.Sprintf("%s:%s", owner, group)
	} else if owner != "" {
		ownership = owner
	} else {
		ownership = ":" + group
	}
	
	chownCmd := fmt.Sprintf("chown %s %s", ownership, path)
	if recurse {
		chownCmd = fmt.Sprintf("chown -R %s %s", ownership, path)
	}
	
	if _, err := conn.Execute(ctx, chownCmd, types.ExecuteOptions{}); err != nil {
		return fmt.Errorf("failed to set ownership: %v", err)
	}
	
	return nil
}

// Validate checks if the module arguments are valid
func (m *FileModule) Validate(args map[string]interface{}) error {
	// Path is required
	path, ok := args["path"]
	if !ok || path == nil || path == "" {
		return types.NewValidationError("path", path, "required field is missing")
	}
	
	// Validate state if provided
	if state, ok := args["state"].(string); ok {
		validStates := []string{"file", "directory", "link", "absent", "touch", "hard"}
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
		
		// src is required for link state
		if state == "link" {
			if src, ok := args["src"]; !ok || src == nil || src == "" {
				return types.NewValidationError("src", src, "required when state=link")
			}
		}
	}
	
	// Validate mode if provided
	if mode, ok := args["mode"].(string); ok && mode != "" {
		if _, err := strconv.ParseInt(mode, 8, 32); err != nil {
			return types.NewValidationError("mode", mode, "must be an octal number")
		}
	}
	
	return nil
}

// Documentation returns the module documentation
func (m *FileModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "file",
		Description: "Manage files and file properties",
		Parameters: map[string]types.ParamDoc{
			"path": {
				Description: "Path to the file or directory",
				Required:    true,
				Type:        "string",
			},
			"state": {
				Description: "State of the file (file, directory, link, absent, touch)",
				Required:    false,
				Type:        "string",
				Default:     "file",
				Choices:     []string{"file", "directory", "link", "absent", "touch"},
			},
			"mode": {
				Description: "Permissions of the file or directory (octal)",
				Required:    false,
				Type:        "string",
			},
			"owner": {
				Description: "Owner of the file or directory",
				Required:    false,
				Type:        "string",
			},
			"group": {
				Description: "Group of the file or directory",
				Required:    false,
				Type:        "string",
			},
			"src": {
				Description: "Source path for symlinks (required when state=link)",
				Required:    false,
				Type:        "string",
			},
			"recurse": {
				Description: "Recursively apply attributes to directory contents",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"force": {
				Description: "Force creation of symlinks",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
		},
		Examples: []string{
			"- name: Create directory\n  file:\n    path: /tmp/test\n    state: directory\n    mode: '0755'",
			"- name: Create symlink\n  file:\n    src: /opt/app/bin\n    path: /usr/local/bin/app\n    state: link",
			"- name: Remove file\n  file:\n    path: /tmp/unwanted\n    state: absent",
		},
		Returns: map[string]string{
			"path":    "Path to the file or directory",
			"state":   "State of the file after module execution",
			"changed": "Whether the file was modified",
		},
	}
}