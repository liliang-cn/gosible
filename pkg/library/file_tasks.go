package library

import (
	"github.com/liliang-cn/gosinble/pkg/types"
)

// FileTasks provides common file and directory operations
type FileTasks struct{}

// NewFileTasks creates a new FileTasks instance
func NewFileTasks() *FileTasks {
	return &FileTasks{}
}

// EnsureFile creates tasks to ensure a file exists with specific content and permissions
func (ft *FileTasks) EnsureFile(path, content, owner, group, mode string) []types.Task {
	return []types.Task{
		{
			Name:   "Check if file exists",
			Module: "stat",
			Args: map[string]interface{}{
				"path": path,
			},
			Register: "file_stat",
		},
		{
			Name:   "Create file if not exists",
			Module: "copy",
			Args: map[string]interface{}{
				"content": content,
				"dest":    path,
				"owner":   owner,
				"group":   group,
				"mode":    mode,
			},
			When: "not file_stat.stat.exists",
		},
		{
			Name:   "Verify file content",
			Module: "lineinfile",
			Args: map[string]interface{}{
				"path":   path,
				"line":   content,
				"create": true,
			},
		},
		{
			Name:   "Set file permissions",
			Module: "file",
			Args: map[string]interface{}{
				"path":  path,
				"owner": owner,
				"group": group,
				"mode":  mode,
			},
		},
	}
}

// EnsureDirectory creates tasks to ensure a directory structure exists
func (ft *FileTasks) EnsureDirectory(path, owner, group, mode string, recursive bool) []types.Task {
	return []types.Task{
		{
			Name:   "Create directory structure",
			Module: "file",
			Args: map[string]interface{}{
				"path":  path,
				"state": "directory",
				"owner": owner,
				"group": group,
				"mode":  mode,
				"recurse": recursive,
			},
		},
	}
}

// BackupFile creates tasks to backup a file before modification
func (ft *FileTasks) BackupFile(path string) []types.Task {
	return []types.Task{
		{
			Name:   "Check if file exists",
			Module: "stat",
			Args: map[string]interface{}{
				"path": path,
			},
			Register: "file_to_backup",
		},
		{
			Name:   "Create backup directory",
			Module: "file",
			Args: map[string]interface{}{
				"path":  "/var/backups/gosinble",
				"state": "directory",
				"mode":  "0755",
			},
			When: "file_to_backup.stat.exists",
		},
		{
			Name:   "Backup file",
			Module: "copy",
			Args: map[string]interface{}{
				"src":    path,
				"dest":   "/var/backups/gosinble/{{ inventory_hostname }}-{{ path | basename }}-{{ ansible_date_time.epoch }}",
				"remote_src": true,
			},
			When: "file_to_backup.stat.exists",
		},
	}
}

// TemplateConfig creates tasks to deploy a configuration from a template
func (ft *FileTasks) TemplateConfig(template, dest, owner, group, mode string, validateCmd string, restartService string) []types.Task {
	tasks := []types.Task{
		// Backup existing config
		{
			Name:   "Backup existing configuration",
			Module: "copy",
			Args: map[string]interface{}{
				"src":    dest,
				"dest":   dest + ".bak-{{ ansible_date_time.epoch }}",
				"remote_src": true,
			},
			IgnoreErrors: true,
		},
		// Deploy new config
		{
			Name:   "Deploy configuration from template",
			Module: "template",
			Args: map[string]interface{}{
				"src":   template,
				"dest":  dest,
				"owner": owner,
				"group": group,
				"mode":  mode,
				"backup": true,
			},
			Register: "config_deployed",
		},
	}
	
	// Add validation if specified
	if validateCmd != "" {
		tasks = append(tasks, types.Task{
			Name:   "Validate configuration",
			Module: "command",
			Args: map[string]interface{}{
				"cmd": validateCmd + " " + dest,
			},
			When: "config_deployed.changed",
		})
	}
	
	// Add service restart if specified
	if restartService != "" {
		tasks[1].Notify = []string{"restart " + restartService}
	}
	
	return tasks
}

// SyncDirectory creates tasks to synchronize directories
func (ft *FileTasks) SyncDirectory(src, dest string, delete bool) []types.Task {
	return []types.Task{
		{
			Name:   "Synchronize directory",
			Module: "synchronize",
			Args: map[string]interface{}{
				"src":    src,
				"dest":   dest,
				"delete": delete,
				"recursive": true,
			},
		},
	}
}

// ManageSymlink creates tasks to manage symbolic links
func (ft *FileTasks) ManageSymlink(src, dest string) []types.Task {
	return []types.Task{
		{
			Name:   "Create symbolic link",
			Module: "file",
			Args: map[string]interface{}{
				"src":   src,
				"dest":  dest,
				"state": "link",
			},
		},
	}
}

// CleanupOldFiles creates tasks to remove old files
func (ft *FileTasks) CleanupOldFiles(path string, age string) []types.Task {
	return []types.Task{
		{
			Name:   "Find old files",
			Module: "find",
			Args: map[string]interface{}{
				"path": path,
				"age":  age,
			},
			Register: "old_files",
		},
		{
			Name:   "Remove old files",
			Module: "file",
			Args: map[string]interface{}{
				"path":  "{{ item.path }}",
				"state": "absent",
			},
			Loop: "{{ old_files.files }}",
			When: "old_files.matched > 0",
		},
	}
}

// SetPermissions creates tasks to set file/directory permissions recursively
func (ft *FileTasks) SetPermissions(path, owner, group, fileMode, dirMode string) []types.Task {
	return []types.Task{
		{
			Name:   "Set directory permissions",
			Module: "shell",
			Args: map[string]interface{}{
				"cmd": "find " + path + " -type d -exec chmod " + dirMode + " {} \\;",
			},
		},
		{
			Name:   "Set file permissions",
			Module: "shell",
			Args: map[string]interface{}{
				"cmd": "find " + path + " -type f -exec chmod " + fileMode + " {} \\;",
			},
		},
		{
			Name:   "Set ownership",
			Module: "file",
			Args: map[string]interface{}{
				"path":    path,
				"owner":   owner,
				"group":   group,
				"recurse": true,
			},
		},
	}
}