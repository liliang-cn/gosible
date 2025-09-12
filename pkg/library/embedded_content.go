package library

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	
	"github.com/liliang-cn/gosible/pkg/types"
)

// EmbeddedContent manages files and directories embedded in the binary
type EmbeddedContent struct {
	fs          embed.FS
	basePath    string
	contentTasks *ContentTasks
}

// NewEmbeddedContent creates a new embedded content manager
func NewEmbeddedContent(embedFS embed.FS, basePath string) *EmbeddedContent {
	return &EmbeddedContent{
		fs:          embedFS,
		basePath:    basePath,
		contentTasks: NewContentTasks(),
	}
}

// LoadAll loads all embedded content into memory
func (ec *EmbeddedContent) LoadAll() error {
	return fs.WalkDir(ec.fs, ec.basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip the base directory itself
		if path == ec.basePath {
			return nil
		}
		
		// Get relative path from base
		relPath, err := filepath.Rel(ec.basePath, path)
		if err != nil {
			return err
		}
		
		if d.IsDir() {
			// Register directory
			ec.contentTasks.AddDirectory(relPath, "0755", "root", "root")
		} else {
			// Read file content
			content, err := ec.fs.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read embedded file %s: %w", path, err)
			}
			
			// Determine file mode based on extension
			mode := "0644"
			if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".py") {
				mode = "0755"
			}
			
			// Register file
			ec.contentTasks.AddFile(relPath, content, mode, "root", "root")
			
			// Also add to parent directory if exists
			dir := filepath.Dir(relPath)
			if dir != "." && dir != "" {
				ec.contentTasks.AddFileToDirectory(dir, filepath.Base(relPath), content, mode)
			}
		}
		
		return nil
	})
}

// DeployFile deploys a single embedded file
func (ec *EmbeddedContent) DeployFile(srcPath, destPath string, owner, group, mode string) []types.Task {
	content, err := ec.fs.ReadFile(filepath.Join(ec.basePath, srcPath))
	if err != nil {
		return []types.Task{{
			Name:   "Failed to read embedded file",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Cannot read embedded file %s: %v", srcPath, err),
			},
		}}
	}
	
	return []types.Task{
		{
			Name:   fmt.Sprintf("Deploy embedded file %s", srcPath),
			Module: "copy",
			Args: map[string]interface{}{
				"content": string(content),
				"dest":    destPath,
				"owner":   owner,
				"group":   group,
				"mode":    mode,
				"backup":  true,
			},
		},
	}
}

// DeployDirectory deploys an entire embedded directory
func (ec *EmbeddedContent) DeployDirectory(srcDir, destDir string, owner, group, dirMode, fileMode string) []types.Task {
	tasks := []types.Task{
		{
			Name:   fmt.Sprintf("Create directory %s", destDir),
			Module: "file",
			Args: map[string]interface{}{
				"path":  destDir,
				"state": "directory",
				"owner": owner,
				"group": group,
				"mode":  dirMode,
			},
		},
	}
	
	// Walk the embedded directory
	srcPath := filepath.Join(ec.basePath, srcDir)
	err := fs.WalkDir(ec.fs, srcPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip the source directory itself
		if path == srcPath {
			return nil
		}
		
		// Get relative path from source
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}
		
		destPath := filepath.Join(destDir, relPath)
		
		if d.IsDir() {
			// Create subdirectory
			tasks = append(tasks, types.Task{
				Name:   fmt.Sprintf("Create subdirectory %s", destPath),
				Module: "file",
				Args: map[string]interface{}{
					"path":  destPath,
					"state": "directory",
					"owner": owner,
					"group": group,
					"mode":  dirMode,
				},
			})
		} else {
			// Read and deploy file
			content, err := ec.fs.ReadFile(path)
			if err != nil {
				return err
			}
			
			// Use specific file mode or detect from extension
			mode := fileMode
			if mode == "" {
				mode = "0644"
				if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".py") {
					mode = "0755"
				}
			}
			
			tasks = append(tasks, types.Task{
				Name:   fmt.Sprintf("Deploy file %s", destPath),
				Module: "copy",
				Args: map[string]interface{}{
					"content": string(content),
					"dest":    destPath,
					"owner":   owner,
					"group":   group,
					"mode":    mode,
				},
			})
		}
		
		return nil
	})
	
	if err != nil {
		tasks = append(tasks, types.Task{
			Name:   "Failed to walk embedded directory",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Cannot walk embedded directory %s: %v", srcDir, err),
			},
		})
	}
	
	return tasks
}

// DeployTemplate deploys and renders an embedded template
func (ec *EmbeddedContent) DeployTemplate(templatePath, destPath string, vars map[string]interface{}, owner, group, mode string) []types.Task {
	content, err := ec.fs.ReadFile(filepath.Join(ec.basePath, templatePath))
	if err != nil {
		return []types.Task{{
			Name:   "Failed to read embedded template",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Cannot read embedded template %s: %v", templatePath, err),
			},
		}}
	}
	
	// Save template to temp location first
	tempPath := fmt.Sprintf("/tmp/gosible_embedded_template_%s", filepath.Base(templatePath))
	
	return []types.Task{
		{
			Name:   fmt.Sprintf("Save embedded template %s", templatePath),
			Module: "copy",
			Args: map[string]interface{}{
				"content": string(content),
				"dest":    tempPath,
				"mode":    "0644",
			},
		},
		{
			Name:   fmt.Sprintf("Render template to %s", destPath),
			Module: "template",
			Args: map[string]interface{}{
				"src":   tempPath,
				"dest":  destPath,
				"owner": owner,
				"group": group,
				"mode":  mode,
				"vars":  vars,
			},
		},
		{
			Name:   "Clean up temp template",
			Module: "file",
			Args: map[string]interface{}{
				"path":  tempPath,
				"state": "absent",
			},
		},
	}
}

// ListFiles lists all files in the embedded filesystem
func (ec *EmbeddedContent) ListFiles() ([]string, error) {
	var files []string
	
	err := fs.WalkDir(ec.fs, ec.basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if !d.IsDir() && path != ec.basePath {
			relPath, err := filepath.Rel(ec.basePath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		
		return nil
	})
	
	return files, err
}

// ListDirectories lists all directories in the embedded filesystem
func (ec *EmbeddedContent) ListDirectories() ([]string, error) {
	var dirs []string
	
	err := fs.WalkDir(ec.fs, ec.basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if d.IsDir() && path != ec.basePath {
			relPath, err := filepath.Rel(ec.basePath, path)
			if err != nil {
				return err
			}
			dirs = append(dirs, relPath)
		}
		
		return nil
	})
	
	return dirs, err
}

// FileExists checks if a file exists in the embedded filesystem
func (ec *EmbeddedContent) FileExists(path string) bool {
	fullPath := filepath.Join(ec.basePath, path)
	_, err := ec.fs.Open(fullPath)
	return err == nil
}

// ReadFile reads a file from the embedded filesystem
func (ec *EmbeddedContent) ReadFile(path string) ([]byte, error) {
	fullPath := filepath.Join(ec.basePath, path)
	return ec.fs.ReadFile(fullPath)
}

// SyncDirectory creates tasks to sync an embedded directory with a target
func (ec *EmbeddedContent) SyncDirectory(srcDir, destDir string, owner, group string) []types.Task {
	tasks := []types.Task{
		{
			Name:   fmt.Sprintf("Ensure target directory %s exists", destDir),
			Module: "file",
			Args: map[string]interface{}{
				"path":  destDir,
				"state": "directory",
				"owner": owner,
				"group": group,
				"mode":  "0755",
			},
		},
	}
	
	// Track files to keep
	var filesToKeep []string
	
	// Deploy all files from embedded directory
	srcPath := filepath.Join(ec.basePath, srcDir)
	err := fs.WalkDir(ec.fs, srcPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == srcPath || d.IsDir() {
			return err
		}
		
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}
		
		destPath := filepath.Join(destDir, relPath)
		filesToKeep = append(filesToKeep, destPath)
		
		content, err := ec.fs.ReadFile(path)
		if err != nil {
			return err
		}
		
		mode := "0644"
		if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".py") {
			mode = "0755"
		}
		
		tasks = append(tasks, types.Task{
			Name:   fmt.Sprintf("Sync file %s", relPath),
			Module: "copy",
			Args: map[string]interface{}{
				"content": string(content),
				"dest":    destPath,
				"owner":   owner,
				"group":   group,
				"mode":    mode,
			},
		})
		
		return nil
	})
	
	if err != nil {
		tasks = append(tasks, types.Task{
			Name:   "Failed to sync directory",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Cannot sync embedded directory %s: %v", srcDir, err),
			},
		})
		return tasks
	}
	
	// Remove files not in embedded content
	tasks = append(tasks, types.Task{
		Name:   "Find existing files in target directory",
		Module: "find",
		Args: map[string]interface{}{
			"paths":     destDir,
			"file_type": "file",
			"recurse":   true,
		},
		Register: "existing_files",
	})
	
	// Create a cleanup task for files not in the embedded content
	tasks = append(tasks, types.Task{
		Name:   "Remove files not in embedded content",
		Module: "file",
		Args: map[string]interface{}{
			"path":  "{{ item.path }}",
			"state": "absent",
		},
		Loop: "{{ existing_files.files }}",
		When: fmt.Sprintf("item.path not in %v", filesToKeep),
	})
	
	return tasks
}