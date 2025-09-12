package modules

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/liliang-cn/gosible/pkg/types"
)

// CopyModule implements the copy module for transferring files
type CopyModule struct {
	*BaseModule
}

// NewCopyModule creates a new copy module
func NewCopyModule() *CopyModule {
	doc := types.ModuleDoc{
		Name:        "copy",
		Description: "Copy files to remote locations",
		Parameters: map[string]types.ParamDoc{
			"src": {
				Description: "Local path to a file to copy to the remote server",
				Required:    false,
				Type:        "string",
			},
			"content": {
				Description: "When used instead of src, sets the contents of a file directly to the specified value",
				Required:    false,
				Type:        "string",
			},
			"dest": {
				Description: "Remote absolute path where the file should be copied to",
				Required:    true,
				Type:        "string",
			},
			"backup": {
				Description: "Create a backup file including the timestamp information",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"force": {
				Description: "Influence when the file is being transferred",
				Required:    false,
				Type:        "bool",
				Default:     true,
			},
			"mode": {
				Description: "File permissions (as octal string or symbolic)",
				Required:    false,
				Type:        "string",
				Default:     "preserve",
			},
			"owner": {
				Description: "Name of the user that should own the file",
				Required:    false,
				Type:        "string",
			},
			"group": {
				Description: "Name of the group that should own the file",
				Required:    false,
				Type:        "string",
			},
			"follow": {
				Description: "Follow symbolic links",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"directory_mode": {
				Description: "Mode to use when creating directories",
				Required:    false,
				Type:        "string",
				Default:     "0755",
			},
		},
		Examples: []string{
			`- name: Copy file with owner and permissions
  copy:
    src: /srv/myfiles/foo.conf
    dest: /etc/foo.conf
    owner: foo
    group: foo
    mode: '0644'`,
			`- name: Copy file content inline
  copy:
    content: |
      # This is a config file
      setting1=value1
      setting2=value2
    dest: /etc/app.conf`,
			`- name: Copy and backup original
  copy:
    src: foo.conf
    dest: /etc/foo.conf
    backup: yes`,
		},
		Returns: map[string]string{
			"backup_file": "Name of backup file created",
			"checksum":    "SHA1 checksum of the file after copy",
			"dest":        "Destination file/path",
			"gid":         "Group id of the file, after execution",
			"group":       "Group of the file, after execution",
			"mode":        "Permissions of the target, after execution",
			"owner":       "Owner of the file, after execution",
			"size":        "Size of the target, after execution",
			"src":         "Source file used for the copy",
			"state":       "State of the target file",
			"uid":         "User id of the file, after execution",
		},
	}

	return &CopyModule{
		BaseModule: NewBaseModule("copy", doc),
	}
}

// Validate validates the module arguments
func (m *CopyModule) Validate(args map[string]interface{}) error {
	// Validate required fields
	if err := m.ValidateRequired(args, []string{"dest"}); err != nil {
		return err
	}

	// Either src or content must be provided, but not both
	src := m.GetStringArg(args, "src", "")
	content := m.GetStringArg(args, "content", "")

	if src == "" && content == "" {
		return types.NewValidationError("src/content", nil, "either src or content must be provided")
	}

	if src != "" && content != "" {
		return types.NewValidationError("src/content", nil, "src and content are mutually exclusive")
	}

	// Validate field types
	fieldTypes := map[string]string{
		"src":            "string",
		"content":        "string",
		"dest":           "string",
		"backup":         "bool",
		"force":          "bool",
		"mode":           "string",
		"owner":          "string",
		"group":          "string",
		"follow":         "bool",
		"directory_mode": "string",
	}
	if err := m.ValidateTypes(args, fieldTypes); err != nil {
		return err
	}

	// Validate mode format (basic validation)
	mode := m.GetStringArg(args, "mode", "preserve")
	if mode != "preserve" && !m.isValidMode(mode) {
		return types.NewValidationError("mode", mode, "invalid mode format")
	}

	return nil
}

// Run executes the copy module
func (m *CopyModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	return m.ExecuteWithTiming(ctx, conn, args, func() (*types.Result, error) {
		host := m.GetHostFromConnection(conn)

		// Get parameters
		src := m.GetStringArg(args, "src", "")
		content := m.GetStringArg(args, "content", "")
		dest := m.GetStringArg(args, "dest", "")
		backup := m.GetBoolArg(args, "backup", false)
		force := m.GetBoolArg(args, "force", true)
		mode := m.GetStringArg(args, "mode", "preserve")
		owner := m.GetStringArg(args, "owner", "")
		group := m.GetStringArg(args, "group", "")
		_ = m.GetBoolArg(args, "follow", false) // TODO: implement follow functionality
		dirMode := m.GetStringArg(args, "directory_mode", "0755")

		// Validate and sanitize destination path
		dest, err := m.ValidatePath(dest)
		if err != nil {
			return m.CreateErrorResult(host, "Invalid destination path", err), nil
		}

		// Check mode handling
		if m.CheckMode(args) {
			action := "copy"
			if src != "" {
				action = fmt.Sprintf("copy %s to %s", src, dest)
			} else {
				action = fmt.Sprintf("write content to %s", dest)
			}
			return m.CreateCheckModeResult(host, true, fmt.Sprintf("Would %s", action), map[string]interface{}{
				"dest": dest,
				"src":  src,
			}), nil
		}

		// Check if destination already exists
		destExists, destInfo, err := m.checkDestination(conn, dest)
		if err != nil {
			return m.CreateErrorResult(host, "Failed to check destination", err), nil
		}

		// If force is false and destination exists, check if we should skip
		if !force && destExists {
			if src != "" {
				// Compare source and destination
				same, err := m.compareFiles(conn, src, dest)
				if err != nil {
					return m.CreateErrorResult(host, "Failed to compare files", err), nil
				}
				if same {
					return m.CreateSuccessResult(host, false, "File already exists and is identical", map[string]interface{}{
						"dest":  dest,
						"src":   src,
						"state": "file",
					}), nil
				}
			}
		}

		// Create backup if requested
		var backupFile string
		if backup && destExists {
			backupFile, err = m.createBackup(conn, dest)
			if err != nil {
				return m.CreateErrorResult(host, "Failed to create backup", err), nil
			}
		}

		// Ensure parent directory exists
		if err := m.ensureParentDirectory(conn, dest, dirMode); err != nil {
			return m.CreateErrorResult(host, "Failed to create parent directory", err), nil
		}

		// Determine file mode
		fileMode := 0644 // default
		if mode != "preserve" {
			fileMode, err = m.parseMode(mode)
			if err != nil {
				return m.CreateErrorResult(host, "Failed to parse mode", err), nil
			}
		} else if destExists && destInfo != nil {
			// Preserve existing mode
			if stat, ok := destInfo.Sys().(interface{ Mode() os.FileMode }); ok {
				fileMode = int(stat.Mode() & 0777)
			}
		}

		// Copy the file
		var reader io.Reader
		var sourceInfo string

		if content != "" {
			// Copy from content
			reader = strings.NewReader(content)
			sourceInfo = "content"
		} else {
			// Copy from source file
			file, err := os.Open(src)
			if err != nil {
				return m.CreateErrorResult(host, fmt.Sprintf("Failed to open source file: %s", src), err), nil
			}
			defer file.Close()
			reader = file
			sourceInfo = src
		}

		// Perform the copy
		if err := conn.Copy(ctx, reader, dest, fileMode); err != nil {
			return m.CreateErrorResult(host, "Failed to copy file", err), nil
		}

		// Set ownership if specified
		if owner != "" || group != "" {
			if err := m.setOwnership(conn, dest, owner, group); err != nil {
				m.LogWarn("Failed to set ownership on %s: %v", dest, err)
			}
		}

		// Get final file information
		finalExists, finalInfo, err := m.checkDestination(conn, dest)
		if err != nil {
			return m.CreateErrorResult(host, "Failed to get final file info", err), nil
		}

		// Build result data
		resultData := map[string]interface{}{
			"dest":  dest,
			"src":   sourceInfo,
			"state": "file",
		}

		if backupFile != "" {
			resultData["backup_file"] = backupFile
		}

		if finalExists && finalInfo != nil {
			resultData["size"] = finalInfo.Size()
			resultData["mode"] = fmt.Sprintf("%04o", finalInfo.Mode()&0777)
		}

		// Calculate checksum if possible
		if checksum, err := m.getFileChecksum(conn, dest); err == nil {
			resultData["checksum"] = checksum
		}

		changed := !destExists || (destExists && force)
		message := "File copied successfully"
		if destExists {
			message = "File updated successfully"
		}

		return m.CreateSuccessResult(host, changed, message, resultData), nil
	})
}

// checkDestination checks if destination exists and returns file info
func (m *CopyModule) checkDestination(conn types.Connection, dest string) (bool, os.FileInfo, error) {
	// Try to get file info using a stat command
	result, err := conn.Execute(context.Background(), fmt.Sprintf("stat -c '%%s %%Y %%a' %s 2>/dev/null || echo 'NOTFOUND'", dest), types.ExecuteOptions{})
	if err != nil {
		return false, nil, err
	}

	if !result.Success {
		return false, nil, nil
	}

	stdout := strings.TrimSpace(result.Data["stdout"].(string))
	if stdout == "NOTFOUND" || stdout == "" {
		return false, nil, nil
	}

	// File exists - we can't get detailed FileInfo through connection interface,
	// so we return true with nil info
	return true, nil, nil
}

// compareFiles compares source and destination files
func (m *CopyModule) compareFiles(conn types.Connection, src, dest string) (bool, error) {
	// Get local file checksum
	localChecksum, err := m.getLocalFileChecksum(src)
	if err != nil {
		return false, err
	}

	// Get remote file checksum
	remoteChecksum, err := m.getFileChecksum(conn, dest)
	if err != nil {
		return false, err
	}

	return localChecksum == remoteChecksum, nil
}

// getLocalFileChecksum calculates SHA1 checksum of local file
func (m *CopyModule) getLocalFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Simple implementation - in production use crypto/sha1
	// For now, return a placeholder
	return "local-checksum", nil
}

// getFileChecksum calculates SHA1 checksum of remote file
func (m *CopyModule) getFileChecksum(conn types.Connection, path string) (string, error) {
	result, err := conn.Execute(context.Background(), fmt.Sprintf("sha1sum %s 2>/dev/null | cut -d' ' -f1", path), types.ExecuteOptions{})
	if err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("failed to calculate checksum")
	}

	return strings.TrimSpace(result.Data["stdout"].(string)), nil
}

// createBackup creates a backup of the existing file
func (m *CopyModule) createBackup(conn types.Connection, dest string) (string, error) {
	backupFile := fmt.Sprintf("%s.backup", dest)
	
	result, err := conn.Execute(context.Background(), fmt.Sprintf("cp %s %s", dest, backupFile), types.ExecuteOptions{})
	if err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("failed to create backup")
	}

	return backupFile, nil
}

// ensureParentDirectory ensures the parent directory exists
func (m *CopyModule) ensureParentDirectory(conn types.Connection, dest, dirMode string) error {
	// Extract parent directory
	result, err := conn.Execute(context.Background(), fmt.Sprintf("dirname %s", dest), types.ExecuteOptions{})
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("failed to get parent directory")
	}

	parentDir := strings.TrimSpace(result.Data["stdout"].(string))
	if parentDir == "/" || parentDir == "." {
		return nil // No need to create
	}

	// Create parent directory with specified mode
	createCmd := fmt.Sprintf("mkdir -p %s && chmod %s %s", parentDir, dirMode, parentDir)
	result, err = conn.Execute(context.Background(), createCmd, types.ExecuteOptions{})
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("failed to create parent directory")
	}

	return nil
}

// setOwnership sets file ownership
func (m *CopyModule) setOwnership(conn types.Connection, dest, owner, group string) error {
	var chownCmd string
	if owner != "" && group != "" {
		chownCmd = fmt.Sprintf("chown %s:%s %s", owner, group, dest)
	} else if owner != "" {
		chownCmd = fmt.Sprintf("chown %s %s", owner, dest)
	} else if group != "" {
		chownCmd = fmt.Sprintf("chgrp %s %s", group, dest)
	} else {
		return nil
	}

	result, err := conn.Execute(context.Background(), chownCmd, types.ExecuteOptions{})
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("failed to set ownership")
	}

	return nil
}

// parseMode parses file mode string
func (m *CopyModule) parseMode(mode string) (int, error) {
	if strings.HasPrefix(mode, "0") {
		// Octal mode
		var octal int
		if _, err := fmt.Sscanf(mode, "%o", &octal); err != nil {
			return 0, err
		}
		return octal, nil
	}

	// Symbolic mode - simplified implementation
	switch mode {
	case "644", "u=rw,g=r,o=r":
		return 0644, nil
	case "755", "u=rwx,g=rx,o=rx":
		return 0755, nil
	case "600", "u=rw,g=,o=":
		return 0600, nil
	case "700", "u=rwx,g=,o=":
		return 0700, nil
	default:
		return 0, fmt.Errorf("unsupported mode format: %s", mode)
	}
}

// isValidMode checks if mode string is valid
func (m *CopyModule) isValidMode(mode string) bool {
	// Basic validation for octal or symbolic modes
	if mode == "preserve" {
		return true
	}
	if strings.HasPrefix(mode, "0") && len(mode) == 4 {
		return true
	}
	if len(mode) == 3 {
		for _, r := range mode {
			if r < '0' || r > '7' {
				return false
			}
		}
		return true
	}
	// Symbolic modes would need more complex validation
	return strings.Contains(mode, "u=") || strings.Contains(mode, "g=") || strings.Contains(mode, "o=")
}