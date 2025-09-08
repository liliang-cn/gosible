package library

import (
	"github.com/liliang-cn/gosinble/pkg/types"
)

// ArchiveTasks provides common archive and compression operations
type ArchiveTasks struct{}

// NewArchiveTasks creates a new ArchiveTasks instance
func NewArchiveTasks() *ArchiveTasks {
	return &ArchiveTasks{}
}

// CreateArchive creates tasks to create an archive from files/directories
func (at *ArchiveTasks) CreateArchive(paths []string, dest, format string, exclude []string) []types.Task {
	args := map[string]interface{}{
		"dest":   dest,
		"format": format,
	}
	
	// Handle single or multiple paths
	if len(paths) == 1 {
		args["path"] = paths[0]
	} else if len(paths) > 1 {
		pathInterfaces := make([]interface{}, len(paths))
		for i, p := range paths {
			pathInterfaces[i] = p
		}
		args["path"] = pathInterfaces
	}
	
	// Add exclude patterns if provided
	if len(exclude) > 0 {
		excludeInterfaces := make([]interface{}, len(exclude))
		for i, e := range exclude {
			excludeInterfaces[i] = e
		}
		args["exclude"] = excludeInterfaces
	}
	
	return []types.Task{
		{
			Name:   "Create archive",
			Module: "archive",
			Args:   args,
		},
	}
}

// CreateTarGzArchive creates tasks to create a tar.gz archive
func (at *ArchiveTasks) CreateTarGzArchive(path, dest string) []types.Task {
	return []types.Task{
		{
			Name:   "Create tar.gz archive",
			Module: "archive",
			Args: map[string]interface{}{
				"path":   path,
				"dest":   dest,
				"format": "gz",
			},
		},
	}
}

// CreateZipArchive creates tasks to create a zip archive
func (at *ArchiveTasks) CreateZipArchive(path, dest string) []types.Task {
	return []types.Task{
		{
			Name:   "Create zip archive",
			Module: "archive",
			Args: map[string]interface{}{
				"path":   path,
				"dest":   dest,
				"format": "zip",
			},
		},
	}
}

// ExtractArchive creates tasks to extract an archive
func (at *ArchiveTasks) ExtractArchive(src, dest string, remote bool) []types.Task {
	return []types.Task{
		{
			Name:   "Extract archive",
			Module: "unarchive",
			Args: map[string]interface{}{
				"src":    src,
				"dest":   dest,
				"remote": remote,
			},
		},
	}
}

// ExtractRemoteArchive creates tasks to download and extract a remote archive
func (at *ArchiveTasks) ExtractRemoteArchive(url, dest string) []types.Task {
	return []types.Task{
		{
			Name:   "Download and extract remote archive",
			Module: "unarchive",
			Args: map[string]interface{}{
				"src":    url,
				"dest":   dest,
				"remote": true,
			},
		},
	}
}

// BackupDirectory creates tasks to backup a directory as an archive
func (at *ArchiveTasks) BackupDirectory(path, backupDir string) []types.Task {
	return []types.Task{
		{
			Name:   "Create backup directory",
			Module: "file",
			Args: map[string]interface{}{
				"path":  backupDir,
				"state": "directory",
				"mode":  "0755",
			},
		},
		{
			Name:   "Create backup archive",
			Module: "archive",
			Args: map[string]interface{}{
				"path":   path,
				"dest":   "{{ backup_dir }}/{{ inventory_hostname }}-{{ ansible_date_time.epoch }}.tar.gz",
				"format": "gz",
			},
			Vars: map[string]interface{}{
				"backup_dir": backupDir,
			},
		},
	}
}

// CompressFiles creates tasks to compress multiple files into an archive
func (at *ArchiveTasks) CompressFiles(files []string, dest, format string) []types.Task {
	fileInterfaces := make([]interface{}, len(files))
	for i, f := range files {
		fileInterfaces[i] = f
	}
	
	return []types.Task{
		{
			Name:   "Compress files",
			Module: "archive",
			Args: map[string]interface{}{
				"path":   fileInterfaces,
				"dest":   dest,
				"format": format,
			},
		},
	}
}