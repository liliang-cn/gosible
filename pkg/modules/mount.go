package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// MountModule manages filesystem mounts
type MountModule struct {
	BaseModule
}

// MountEntry represents a mount point entry
type MountEntry struct {
	Device     string `json:"device"`
	MountPoint string `json:"mount_point"`
	FSType     string `json:"fstype"`
	Options    string `json:"options"`
	Dump       int    `json:"dump"`
	Pass       int    `json:"pass"`
}

// NewMountModule creates a new mount module instance
func NewMountModule() *MountModule {
	return &MountModule{
		BaseModule: BaseModule{},
	}
}

// Name returns the module name
func (m *MountModule) Name() string {
	return "mount"
}

// Capabilities returns the module capabilities
func (m *MountModule) Capabilities() *types.ModuleCapability {
	return &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     true,
		Platform:     "linux",
		RequiresRoot: true,
	}
}

// Validate validates the module arguments
func (m *MountModule) Validate(args map[string]interface{}) error {
	// Required parameters
	path := m.GetStringArg(args, "path", "")
	if path == "" {
		return types.NewValidationError("path", nil, "path is required")
	}

	// State validation
	state := m.GetStringArg(args, "state", "mounted")
	validStates := []string{"mounted", "present", "unmounted", "absent", "remounted"}
	if !m.isValidChoice(state, validStates) {
		return types.NewValidationError("state", state, fmt.Sprintf("state must be one of: %v", validStates))
	}

	// For mounted/present states, src is required
	if state == "mounted" || state == "present" {
		src := m.GetStringArg(args, "src", "")
		if src == "" {
			return types.NewValidationError("src", nil, "src is required when state is mounted or present")
		}
		
		fstype := m.GetStringArg(args, "fstype", "")
		if fstype == "" && state == "mounted" {
			return types.NewValidationError("fstype", nil, "fstype is required when state is mounted")
		}
	}

	// Validate dump and pass values
	dump, _ := m.GetIntArg(args, "dump", 0)
	if dump < 0 || dump > 1 {
		return types.NewValidationError("dump", dump, "dump must be 0 or 1")
	}

	pass, _ := m.GetIntArg(args, "pass", 0)
	if pass < 0 || pass > 2 {
		return types.NewValidationError("pass", pass, "pass must be 0, 1, or 2")
	}

	return nil
}

// Run executes the mount module
func (m *MountModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)
	diffMode := m.DiffMode(args)

	// Parse arguments
	path := m.GetStringArg(args, "path", "")
	src := m.GetStringArg(args, "src", "")
	fstype := m.GetStringArg(args, "fstype", "")
	opts := m.GetStringArg(args, "opts", "defaults")
	state := m.GetStringArg(args, "state", "mounted")
	dump, _ := m.GetIntArg(args, "dump", 0)
	pass, _ := m.GetIntArg(args, "pass", 0)
	backup := m.GetBoolArg(args, "backup", false)
	fstab := m.GetStringArg(args, "fstab", "/etc/fstab")

	// Read current fstab
	currentFstab, err := m.readFstab(ctx, conn, fstab)
	if err != nil {
		return nil, fmt.Errorf("failed to read fstab: %w", err)
	}

	// Store original for diff mode
	originalFstab := currentFstab

	// Find existing entry
	existingEntry := m.findMountEntry(currentFstab, path)
	
	// Initialize result
	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"path":  path,
		"state": state,
	})

	changed := false
	var changes []string

	switch state {
	case "mounted":
		// Ensure entry is in fstab and mounted
		if existingEntry == nil {
			// Add to fstab
			changed = true
			changes = append(changes, fmt.Sprintf("Added mount entry for %s", path))
			
			if !checkMode {
				newEntry := fmt.Sprintf("%s %s %s %s %d %d", src, path, fstype, opts, dump, pass)
				currentFstab = append(currentFstab, newEntry)
				
				if backup {
					if err := m.backupFile(ctx, conn, fstab); err != nil {
						return nil, fmt.Errorf("failed to backup fstab: %w", err)
					}
				}
				
				if err := m.writeFstab(ctx, conn, fstab, currentFstab); err != nil {
					return nil, fmt.Errorf("failed to write fstab: %w", err)
				}
			}
		} else {
			// Check if entry needs updating
			expectedEntry := fmt.Sprintf("%s %s %s %s %d %d", src, path, fstype, opts, dump, pass)
			if *existingEntry != expectedEntry {
				changed = true
				changes = append(changes, fmt.Sprintf("Updated mount entry for %s", path))
				
				if !checkMode {
					// Update the entry
					for i, line := range currentFstab {
						if strings.Contains(line, path) {
							currentFstab[i] = expectedEntry
							break
						}
					}
					
					if backup {
						if err := m.backupFile(ctx, conn, fstab); err != nil {
							return nil, fmt.Errorf("failed to backup fstab: %w", err)
						}
					}
					
					if err := m.writeFstab(ctx, conn, fstab, currentFstab); err != nil {
						return nil, fmt.Errorf("failed to write fstab: %w", err)
					}
				}
			}
		}

		// Check if currently mounted
		isMounted, err := m.isMounted(ctx, conn, path)
		if err != nil {
			return nil, fmt.Errorf("failed to check mount status: %w", err)
		}

		if !isMounted {
			changed = true
			changes = append(changes, fmt.Sprintf("Mounted %s", path))
			
			if !checkMode {
				if err := m.mount(ctx, conn, src, path, fstype, opts); err != nil {
					return nil, fmt.Errorf("failed to mount: %w", err)
				}
			}
		}

	case "present":
		// Ensure entry is in fstab but don't mount
		if existingEntry == nil {
			changed = true
			changes = append(changes, fmt.Sprintf("Added mount entry for %s", path))
			
			if !checkMode {
				newEntry := fmt.Sprintf("%s %s %s %s %d %d", src, path, fstype, opts, dump, pass)
				currentFstab = append(currentFstab, newEntry)
				
				if backup {
					if err := m.backupFile(ctx, conn, fstab); err != nil {
						return nil, fmt.Errorf("failed to backup fstab: %w", err)
					}
				}
				
				if err := m.writeFstab(ctx, conn, fstab, currentFstab); err != nil {
					return nil, fmt.Errorf("failed to write fstab: %w", err)
				}
			}
		}

	case "unmounted":
		// Keep in fstab but unmount
		isMounted, err := m.isMounted(ctx, conn, path)
		if err != nil {
			return nil, fmt.Errorf("failed to check mount status: %w", err)
		}

		if isMounted {
			changed = true
			changes = append(changes, fmt.Sprintf("Unmounted %s", path))
			
			if !checkMode {
				if err := m.unmount(ctx, conn, path); err != nil {
					return nil, fmt.Errorf("failed to unmount: %w", err)
				}
			}
		}

	case "absent":
		// Remove from fstab and unmount
		if existingEntry != nil {
			changed = true
			changes = append(changes, fmt.Sprintf("Removed mount entry for %s", path))
			
			if !checkMode {
				// Remove the entry
				newFstab := []string{}
				for _, line := range currentFstab {
					if !strings.Contains(line, path) {
						newFstab = append(newFstab, line)
					}
				}
				currentFstab = newFstab
				
				if backup {
					if err := m.backupFile(ctx, conn, fstab); err != nil {
						return nil, fmt.Errorf("failed to backup fstab: %w", err)
					}
				}
				
				if err := m.writeFstab(ctx, conn, fstab, currentFstab); err != nil {
					return nil, fmt.Errorf("failed to write fstab: %w", err)
				}
			}
		}

		// Unmount if mounted
		isMounted, err := m.isMounted(ctx, conn, path)
		if err != nil {
			return nil, fmt.Errorf("failed to check mount status: %w", err)
		}

		if isMounted {
			changed = true
			changes = append(changes, fmt.Sprintf("Unmounted %s", path))
			
			if !checkMode {
				if err := m.unmount(ctx, conn, path); err != nil {
					return nil, fmt.Errorf("failed to unmount: %w", err)
				}
			}
		}

	case "remounted":
		// Remount the filesystem
		isMounted, err := m.isMounted(ctx, conn, path)
		if err != nil {
			return nil, fmt.Errorf("failed to check mount status: %w", err)
		}

		if isMounted {
			changed = true
			changes = append(changes, fmt.Sprintf("Remounted %s", path))
			
			if !checkMode {
				if err := m.remount(ctx, conn, path, opts); err != nil {
					return nil, fmt.Errorf("failed to remount: %w", err)
				}
			}
		}
	}

	result.Changed = changed
	
	if changed {
		result.Message = strings.Join(changes, ", ")
		
		if diffMode && (state == "mounted" || state == "present" || state == "absent") {
			result.Diff = m.GenerateDiff(
				strings.Join(originalFstab, "\n"),
				strings.Join(currentFstab, "\n"),
			)
		}
	} else {
		result.Message = "Mount point is already in desired state"
	}

	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Helper methods

func (m *MountModule) readFstab(ctx context.Context, conn types.Connection, fstab string) ([]string, error) {
	result, err := conn.Execute(ctx, fmt.Sprintf("cat %s", fstab), types.ExecuteOptions{})
	if err != nil {
		if result != nil && result.Error != nil && strings.Contains(result.Error.Error(), "No such file") {
			// fstab doesn't exist, return empty
			return []string{}, nil
		}
		return nil, err
	}
	
	var lines []string
	if stdout, ok := result.Data["stdout"].(string); ok {
		lines = strings.Split(strings.TrimSpace(stdout), "\n")
	}
	return lines, nil
}

func (m *MountModule) writeFstab(ctx context.Context, conn types.Connection, fstab string, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	tmpFile := fmt.Sprintf("%s.tmp.%d", fstab, time.Now().Unix())
	
	// Write to temp file
	cmd := fmt.Sprintf("echo '%s' > %s", strings.ReplaceAll(content, "'", "'\\''"), tmpFile)
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return err
	}
	
	// Move temp file to final location
	if _, err := conn.Execute(ctx, fmt.Sprintf("mv %s %s", tmpFile, fstab), types.ExecuteOptions{}); err != nil {
		return err
	}
	
	return nil
}

func (m *MountModule) findMountEntry(lines []string, path string) *string {
	for _, line := range lines {
		// Skip comments and empty lines
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == path {
			return &line
		}
	}
	return nil
}

func (m *MountModule) isMounted(ctx context.Context, conn types.Connection, path string) (bool, error) {
	result, err := conn.Execute(ctx, fmt.Sprintf("mount | grep ' %s '", path), types.ExecuteOptions{})
	if err != nil {
		// grep returns exit code 1 when no match found
		if result != nil && !result.Success {
			return false, nil
		}
		return false, err
	}
	if stdout, ok := result.Data["stdout"].(string); ok {
		return stdout != "", nil
	}
	return false, nil
}

func (m *MountModule) mount(ctx context.Context, conn types.Connection, src, path, fstype, opts string) error {
	cmd := fmt.Sprintf("mount -t %s -o %s %s %s", fstype, opts, src, path)
	_, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err
}

func (m *MountModule) unmount(ctx context.Context, conn types.Connection, path string) error {
	_, err := conn.Execute(ctx, fmt.Sprintf("umount %s", path), types.ExecuteOptions{})
	return err
}

func (m *MountModule) remount(ctx context.Context, conn types.Connection, path, opts string) error {
	cmd := fmt.Sprintf("mount -o remount,%s %s", opts, path)
	_, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err
}

func (m *MountModule) backupFile(ctx context.Context, conn types.Connection, file string) error {
	backupFile := fmt.Sprintf("%s.backup.%d", file, time.Now().Unix())
	_, err := conn.Execute(ctx, fmt.Sprintf("cp %s %s", file, backupFile), types.ExecuteOptions{})
	return err
}

func (m *MountModule) isValidChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}