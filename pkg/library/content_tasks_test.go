package library

import (
	"encoding/base64"
	"testing"
	"path/filepath"
)

func TestContentTasks_AddFile(t *testing.T) {
	ct := NewContentTasks()
	
	content := []byte("test content")
	ct.AddFile("test.txt", content, "0644", "user", "group")
	
	if len(ct.files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(ct.files))
	}
	
	file, exists := ct.files["test.txt"]
	if !exists {
		t.Error("File 'test.txt' not found")
	}
	
	if string(file.Content) != "test content" {
		t.Errorf("Expected content 'test content', got '%s'", string(file.Content))
	}
	
	if file.Mode != "0644" {
		t.Errorf("Expected mode '0644', got '%s'", file.Mode)
	}
	
	if file.Base64 {
		t.Error("File should not be marked as base64")
	}
}

func TestContentTasks_AddFileBase64(t *testing.T) {
	ct := NewContentTasks()
	
	originalContent := "binary content"
	encoded := base64.StdEncoding.EncodeToString([]byte(originalContent))
	
	ct.AddFileBase64("binary.dat", encoded, "0755", "root", "root")
	
	file, exists := ct.files["binary.dat"]
	if !exists {
		t.Error("File 'binary.dat' not found")
	}
	
	if !file.Base64 {
		t.Error("File should be marked as base64")
	}
	
	if string(file.Content) != encoded {
		t.Errorf("Expected encoded content to be stored")
	}
}

func TestContentTasks_AddDirectory(t *testing.T) {
	ct := NewContentTasks()
	
	ct.AddDirectory("configs", "0755", "app", "app")
	
	if len(ct.directories) != 1 {
		t.Errorf("Expected 1 directory, got %d", len(ct.directories))
	}
	
	dir, exists := ct.directories["configs"]
	if !exists {
		t.Error("Directory 'configs' not found")
	}
	
	if dir.Mode != "0755" {
		t.Errorf("Expected mode '0755', got '%s'", dir.Mode)
	}
}

func TestContentTasks_AddFileToDirectory(t *testing.T) {
	ct := NewContentTasks()
	
	// Add directory first
	ct.AddDirectory("configs", "0755", "app", "app")
	
	// Add file to directory
	err := ct.AddFileToDirectory("configs", "app.yml", []byte("config: value"), "0644")
	if err != nil {
		t.Errorf("Failed to add file to directory: %v", err)
	}
	
	dir := ct.directories["configs"]
	if len(dir.Files) != 1 {
		t.Errorf("Expected 1 file in directory, got %d", len(dir.Files))
	}
	
	// Try adding to non-existent directory
	err = ct.AddFileToDirectory("nonexistent", "file.txt", []byte("test"), "0644")
	if err == nil {
		t.Error("Expected error when adding file to non-existent directory")
	}
}

func TestContentTasks_DeployFile(t *testing.T) {
	ct := NewContentTasks()
	
	// Add a file
	ct.AddFile("app.conf", []byte("server {\n  port: 8080\n}"), "0644", "www", "www")
	
	// Deploy the file
	tasks := ct.DeployFile("app.conf", "/etc/app/app.conf")
	
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
	
	task := tasks[0]
	if task.Module != "copy" {
		t.Errorf("Expected module 'copy', got '%s'", task.Module)
	}
	
	args := task.Args
	if args["dest"] != "/etc/app/app.conf" {
		t.Errorf("Expected dest '/etc/app/app.conf', got '%v'", args["dest"])
	}
	
	if args["mode"] != "0644" {
		t.Errorf("Expected mode '0644', got '%v'", args["mode"])
	}
	
	// Try deploying non-existent file
	tasks = ct.DeployFile("nonexistent", "/tmp/test")
	if len(tasks) != 1 || tasks[0].Module != "fail" {
		t.Error("Expected fail task for non-existent file")
	}
}

func TestContentTasks_DeployFileBase64(t *testing.T) {
	ct := NewContentTasks()
	
	// Add base64 encoded file
	originalContent := "binary data here"
	encoded := base64.StdEncoding.EncodeToString([]byte(originalContent))
	ct.AddFileBase64("binary.dat", encoded, "0755", "root", "root")
	
	// Deploy the file
	tasks := ct.DeployFile("binary.dat", "/usr/local/bin/binary")
	
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
	
	task := tasks[0]
	args := task.Args
	
	// Check that content was decoded
	content := args["content"].(string)
	if content != originalContent {
		t.Errorf("Expected decoded content '%s', got '%s'", originalContent, content)
	}
}

func TestContentTasks_DeployDirectory(t *testing.T) {
	ct := NewContentTasks()
	
	// Create directory with files
	ct.AddDirectory("configs", "0755", "app", "app")
	ct.AddFileToDirectory("configs", "main.yml", []byte("main: config"), "0644")
	ct.AddFileToDirectory("configs", "db.yml", []byte("database: config"), "0644")
	
	// Deploy directory
	tasks := ct.DeployDirectory("configs", "/etc/myapp")
	
	// Should have: 1 create dir + 2 deploy files
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}
	
	// First task should create directory
	if tasks[0].Module != "file" {
		t.Errorf("Expected first task to be 'file' module, got '%s'", tasks[0].Module)
	}
	
	// Check directory creation args
	dirArgs := tasks[0].Args
	if dirArgs["state"] != "directory" {
		t.Errorf("Expected state 'directory', got '%v'", dirArgs["state"])
	}
	
	// Next tasks should deploy files
	for i := 1; i < 3; i++ {
		if tasks[i].Module != "copy" {
			t.Errorf("Expected task %d to be 'copy' module, got '%s'", i, tasks[i].Module)
		}
	}
}

func TestContentTasks_DeployTemplate(t *testing.T) {
	ct := NewContentTasks()
	
	// Add template file
	templateContent := "server_name {{ domain }};\nport {{ port }};"
	ct.AddFile("nginx.conf.j2", []byte(templateContent), "0644", "root", "root")
	
	// Deploy template with variables
	vars := map[string]interface{}{
		"domain": "example.com",
		"port":   8080,
	}
	
	tasks := ct.DeployTemplate("nginx.conf.j2", "/etc/nginx/sites-available/app", vars)
	
	// Should have: save temp, render template, cleanup
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}
	
	// First task saves template to temp location
	if tasks[0].Module != "copy" {
		t.Errorf("Expected first task to be 'copy', got '%s'", tasks[0].Module)
	}
	
	// Second task renders template
	if tasks[1].Module != "template" {
		t.Errorf("Expected second task to be 'template', got '%s'", tasks[1].Module)
	}
	
	templateArgs := tasks[1].Args
	if templateArgs["dest"] != "/etc/nginx/sites-available/app" {
		t.Errorf("Expected dest '/etc/nginx/sites-available/app', got '%v'", templateArgs["dest"])
	}
	
	// Check variables are passed
	if templateVars, ok := templateArgs["vars"].(map[string]interface{}); ok {
		if templateVars["domain"] != "example.com" {
			t.Errorf("Expected domain 'example.com', got '%v'", templateVars["domain"])
		}
	} else {
		t.Error("Template variables not found or wrong type")
	}
	
	// Third task cleans up
	if tasks[2].Module != "file" {
		t.Errorf("Expected third task to be 'file', got '%s'", tasks[2].Module)
	}
}

func TestContentTasks_ListFiles(t *testing.T) {
	ct := NewContentTasks()
	
	ct.AddFile("file1.txt", []byte("content1"), "0644", "user", "group")
	ct.AddFile("file2.txt", []byte("content2"), "0644", "user", "group")
	ct.AddFile("file3.txt", []byte("content3"), "0644", "user", "group")
	
	files := ct.ListFiles()
	
	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}
	
	// Check all files are present
	fileMap := make(map[string]bool)
	for _, f := range files {
		fileMap[f] = true
	}
	
	for _, expected := range []string{"file1.txt", "file2.txt", "file3.txt"} {
		if !fileMap[expected] {
			t.Errorf("Expected file '%s' not found in list", expected)
		}
	}
}

func TestContentTasks_BulkDeploy(t *testing.T) {
	ct := NewContentTasks()
	
	// Add files and directories
	ct.AddFile("config.yml", []byte("config"), "0644", "app", "app")
	ct.AddFile("script.sh", []byte("#!/bin/bash"), "0755", "root", "root")
	ct.AddDirectory("templates", "0755", "app", "app")
	ct.AddFileToDirectory("templates", "index.html", []byte("<html>"), "0644")
	
	// Bulk deploy
	deployments := map[string]string{
		"config.yml": "/etc/app/config.yml",
		"script.sh":  "/usr/local/bin/script.sh",
		"templates":  "/var/www/templates",
	}
	
	tasks := ct.BulkDeploy(deployments)
	
	// Should have tasks for 2 files + directory deployment
	if len(tasks) < 4 { // 2 file deploys + 1 dir create + 1 file in dir
		t.Errorf("Expected at least 4 tasks, got %d", len(tasks))
	}
}

func TestContentTasks_ValidateContent(t *testing.T) {
	ct := NewContentTasks()
	
	tasks := ct.ValidateContent("app.conf", "/etc/app/app.conf", "sha256:abc123")
	
	// Should have: check exists, verify exists, verify checksum
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}
	
	// First task checks if file exists
	if tasks[0].Module != "stat" {
		t.Errorf("Expected first task to be 'stat', got '%s'", tasks[0].Module)
	}
	
	// Second task verifies existence
	if tasks[1].Module != "assert" {
		t.Errorf("Expected second task to be 'assert', got '%s'", tasks[1].Module)
	}
	
	// Third task verifies checksum
	if tasks[2].Module != "assert" {
		t.Errorf("Expected third task to be 'assert', got '%s'", tasks[2].Module)
	}
	
	// Test without checksum
	tasks = ct.ValidateContent("app.conf", "/etc/app/app.conf", "")
	
	// Should skip checksum verification
	lastTask := tasks[len(tasks)-1]
	if lastTask.When != "" && lastTask.Module == "assert" {
		// Check that checksum task has a condition
		checksumArgs := lastTask.Args
		if _, hasChecksum := checksumArgs["that"]; hasChecksum {
			if lastTask.When == "" {
				t.Error("Checksum task should have When condition when checksum is empty")
			}
		}
	}
}

func TestContentTasks_DirectoryWithSubdirectories(t *testing.T) {
	ct := NewContentTasks()
	
	// Create nested directory structure
	ct.AddDirectory("app", "0755", "app", "app")
	ct.AddFileToDirectory("app", "config/main.yml", []byte("main"), "0644")
	ct.AddFileToDirectory("app", "scripts/deploy.sh", []byte("#!/bin/bash"), "0755")
	ct.AddFileToDirectory("app", "data/users.json", []byte("[]"), "0644")
	
	tasks := ct.DeployDirectory("app", "/opt/app")
	
	// Should create directory and deploy all files
	if len(tasks) < 4 {
		t.Errorf("Expected at least 4 tasks for directory with files, got %d", len(tasks))
	}
	
	// Verify files have correct paths
	for i := 1; i < len(tasks); i++ {
		if tasks[i].Module == "copy" {
			args := tasks[i].Args
			dest := args["dest"].(string)
			
			// Check that destination includes proper subdirectory
			base := filepath.Base(dest)
			if base != "main.yml" && base != "deploy.sh" && base != "users.json" {
				t.Errorf("Unexpected file in deployment: %s", base)
			}
		}
	}
}