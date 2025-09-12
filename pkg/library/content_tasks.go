package library

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"github.com/liliang-cn/gosible/pkg/types"
)

// ContentTasks provides operations for distributing built-in files and folders
type ContentTasks struct {
	// Store embedded content
	files map[string]FileContent
	directories map[string]DirectoryContent
}

// FileContent represents an embedded file with its metadata
type FileContent struct {
	Path    string
	Content []byte
	Mode    string
	Owner   string
	Group   string
	Base64  bool // Whether content is base64 encoded
}

// DirectoryContent represents an embedded directory structure
type DirectoryContent struct {
	Path  string
	Mode  string
	Owner string
	Group string
	Files []FileContent
}

// NewContentTasks creates a new ContentTasks instance
func NewContentTasks() *ContentTasks {
	return &ContentTasks{
		files:       make(map[string]FileContent),
		directories: make(map[string]DirectoryContent),
	}
}

// AddFile registers a file to be distributed
func (ct *ContentTasks) AddFile(name string, content []byte, mode, owner, group string) {
	ct.files[name] = FileContent{
		Path:    name,
		Content: content,
		Mode:    mode,
		Owner:   owner,
		Group:   group,
		Base64:  false,
	}
}

// AddFileBase64 registers a base64-encoded file to be distributed
func (ct *ContentTasks) AddFileBase64(name string, content string, mode, owner, group string) {
	ct.files[name] = FileContent{
		Path:    name,
		Content: []byte(content),
		Mode:    mode,
		Owner:   owner,
		Group:   group,
		Base64:  true,
	}
}

// AddDirectory registers a directory structure to be distributed
func (ct *ContentTasks) AddDirectory(name string, mode, owner, group string) {
	ct.directories[name] = DirectoryContent{
		Path:  name,
		Mode:  mode,
		Owner: owner,
		Group: group,
		Files: []FileContent{},
	}
}

// AddFileToDirectory adds a file to a registered directory
func (ct *ContentTasks) AddFileToDirectory(dirName, fileName string, content []byte, mode string) error {
	dir, exists := ct.directories[dirName]
	if !exists {
		return fmt.Errorf("directory %s not registered", dirName)
	}
	
	dir.Files = append(dir.Files, FileContent{
		Path:    filepath.Join(dir.Path, fileName),
		Content: content,
		Mode:    mode,
		Owner:   dir.Owner,
		Group:   dir.Group,
		Base64:  false,
	})
	
	ct.directories[dirName] = dir
	return nil
}

// DeployFile creates tasks to deploy a single embedded file
func (ct *ContentTasks) DeployFile(name, destPath string) []types.Task {
	file, exists := ct.files[name]
	if !exists {
		return []types.Task{{
			Name:   "File not found",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Built-in file '%s' not found", name),
			},
		}}
	}
	
	content := file.Content
	if file.Base64 {
		decoded, err := base64.StdEncoding.DecodeString(string(content))
		if err == nil {
			content = decoded
		}
	}
	
	return []types.Task{
		{
			Name:   fmt.Sprintf("Deploy built-in file %s", name),
			Module: "copy",
			Args: map[string]interface{}{
				"content": string(content),
				"dest":    destPath,
				"owner":   file.Owner,
				"group":   file.Group,
				"mode":    file.Mode,
				"backup":  true,
			},
		},
	}
}

// DeployDirectory creates tasks to deploy an entire directory structure
func (ct *ContentTasks) DeployDirectory(name, destPath string) []types.Task {
	dir, exists := ct.directories[name]
	if !exists {
		return []types.Task{{
			Name:   "Directory not found",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Built-in directory '%s' not found", name),
			},
		}}
	}
	
	tasks := []types.Task{
		{
			Name:   fmt.Sprintf("Create directory %s", destPath),
			Module: "file",
			Args: map[string]interface{}{
				"path":  destPath,
				"state": "directory",
				"owner": dir.Owner,
				"group": dir.Group,
				"mode":  dir.Mode,
			},
		},
	}
	
	// Deploy all files in the directory
	for _, file := range dir.Files {
		content := file.Content
		if file.Base64 {
			decoded, err := base64.StdEncoding.DecodeString(string(content))
			if err == nil {
				content = decoded
			}
		}
		
		tasks = append(tasks, types.Task{
			Name:   fmt.Sprintf("Deploy file %s", file.Path),
			Module: "copy",
			Args: map[string]interface{}{
				"content": string(content),
				"dest":    filepath.Join(destPath, filepath.Base(file.Path)),
				"owner":   file.Owner,
				"group":   file.Group,
				"mode":    file.Mode,
			},
		})
	}
	
	return tasks
}

// DeployTemplate creates tasks to deploy and render a template
func (ct *ContentTasks) DeployTemplate(name, destPath string, vars map[string]interface{}) []types.Task {
	file, exists := ct.files[name]
	if !exists {
		return []types.Task{{
			Name:   "Template not found",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Built-in template '%s' not found", name),
			},
		}}
	}
	
	content := file.Content
	if file.Base64 {
		decoded, err := base64.StdEncoding.DecodeString(string(content))
		if err == nil {
			content = decoded
		}
	}
	
	// First save the template to a temp location
	tempPath := fmt.Sprintf("/tmp/gosible_template_%s", name)
	tasks := []types.Task{
		{
			Name:   fmt.Sprintf("Save template %s", name),
			Module: "copy",
			Args: map[string]interface{}{
				"content": string(content),
				"dest":    tempPath,
				"mode":    "0644",
			},
		},
		{
			Name:   fmt.Sprintf("Deploy template %s", name),
			Module: "template",
			Args: map[string]interface{}{
				"src":   tempPath,
				"dest":  destPath,
				"owner": file.Owner,
				"group": file.Group,
				"mode":  file.Mode,
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
	
	return tasks
}

// ListFiles returns all registered file names
func (ct *ContentTasks) ListFiles() []string {
	names := make([]string, 0, len(ct.files))
	for name := range ct.files {
		names = append(names, name)
	}
	return names
}

// ListDirectories returns all registered directory names
func (ct *ContentTasks) ListDirectories() []string {
	names := make([]string, 0, len(ct.directories))
	for name := range ct.directories {
		names = append(names, name)
	}
	return names
}

// BulkDeploy creates tasks to deploy multiple files and directories
func (ct *ContentTasks) BulkDeploy(deployments map[string]string) []types.Task {
	var tasks []types.Task
	
	for name, dest := range deployments {
		// Check if it's a file
		if _, isFile := ct.files[name]; isFile {
			tasks = append(tasks, ct.DeployFile(name, dest)...)
		}
		// Check if it's a directory
		if _, isDir := ct.directories[name]; isDir {
			tasks = append(tasks, ct.DeployDirectory(name, dest)...)
		}
	}
	
	return tasks
}

// ValidateContent creates tasks to verify deployed content
func (ct *ContentTasks) ValidateContent(name, destPath string, checksum string) []types.Task {
	return []types.Task{
		{
			Name:   fmt.Sprintf("Check if %s exists", destPath),
			Module: "stat",
			Args: map[string]interface{}{
				"path": destPath,
			},
			Register: "content_stat",
		},
		{
			Name:   "Verify content exists",
			Module: "assert",
			Args: map[string]interface{}{
				"that": []string{
					"content_stat.stat.exists",
				},
				"fail_msg": fmt.Sprintf("Content %s was not deployed to %s", name, destPath),
			},
		},
		{
			Name:   "Verify checksum",
			Module: "assert",
			Args: map[string]interface{}{
				"that": []string{
					fmt.Sprintf("content_stat.stat.checksum == '%s'", checksum),
				},
				"fail_msg": "Content checksum does not match expected value",
			},
			When: checksum != "",
		},
	}
}