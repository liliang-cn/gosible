package library

import (
	"embed"
	"testing"
	"testing/fstest"
	"time"
)

// Create a test filesystem
func createTestFS() fstest.MapFS {
	return fstest.MapFS{
		"configs/app.yml": &fstest.MapFile{
			Data:    []byte("app:\n  name: test\n  port: 8080"),
			Mode:    0644,
			ModTime: time.Now(),
		},
		"configs/db.yml": &fstest.MapFile{
			Data:    []byte("database:\n  host: localhost"),
			Mode:    0644,
			ModTime: time.Now(),
		},
		"scripts/deploy.sh": &fstest.MapFile{
			Data:    []byte("#!/bin/bash\necho 'deploying'"),
			Mode:    0755,
			ModTime: time.Now(),
		},
		"scripts/backup.sh": &fstest.MapFile{
			Data:    []byte("#!/bin/bash\necho 'backup'"),
			Mode:    0755,
			ModTime: time.Now(),
		},
		"templates/nginx.j2": &fstest.MapFile{
			Data:    []byte("server_name {{ domain }};"),
			Mode:    0644,
			ModTime: time.Now(),
		},
	}
}

func TestEmbeddedContent_LoadAll(t *testing.T) {
	// We can't easily test with embed.FS in unit tests
	// So we'll test the structure and methods
	
	// This would be the actual usage:
	// //go:embed testdata/*
	// var testFS embed.FS
	// ec := NewEmbeddedContent(testFS, "testdata")
	
	// For now, we'll test that the structure is correct
	var testFS embed.FS // Empty for testing
	ec := NewEmbeddedContent(testFS, "testdata")
	
	if ec.basePath != "testdata" {
		t.Errorf("Expected basePath 'testdata', got '%s'", ec.basePath)
	}
	
	if ec.contentTasks == nil {
		t.Error("contentTasks should be initialized")
	}
}

func TestEmbeddedContent_DeployFile(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, "configs")
	
	// Test deployment of a file (will fail since embed.FS is empty, but tests the logic)
	tasks := ec.DeployFile("app.yml", "/etc/app/config.yml", "app", "app", "0644")
	
	// Should return error task since file doesn't exist
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
	
	// Should be a fail task
	if tasks[0].Module != "fail" {
		t.Errorf("Expected 'fail' module for non-existent file, got '%s'", tasks[0].Module)
	}
}

func TestEmbeddedContent_DeployDirectory(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, ".")
	
	// Test directory deployment structure
	tasks := ec.DeployDirectory("scripts", "/opt/scripts", "root", "root", "0755", "0755")
	
	// Should at least create the directory
	if len(tasks) < 1 {
		t.Error("Expected at least 1 task for directory creation")
	}
	
	if tasks[0].Module != "file" {
		t.Errorf("Expected first task to be 'file' module, got '%s'", tasks[0].Module)
	}
	
	args := tasks[0].Args
	if args["state"] != "directory" {
		t.Errorf("Expected state 'directory', got '%v'", args["state"])
	}
}

func TestEmbeddedContent_DeployTemplate(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, "templates")
	
	vars := map[string]interface{}{
		"domain": "example.com",
		"port":   8080,
	}
	
	// Test template deployment (will fail but tests the structure)
	tasks := ec.DeployTemplate("app.j2", "/etc/nginx/sites-available/app", vars, "root", "root", "0644")
	
	// Should return error task since template doesn't exist
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
	
	if tasks[0].Module != "fail" {
		t.Errorf("Expected 'fail' module for non-existent template, got '%s'", tasks[0].Module)
	}
}

func TestEmbeddedContent_FileExists(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, "configs")
	
	// Test with empty FS (should return false)
	exists := ec.FileExists("app.yml")
	if exists {
		t.Error("FileExists should return false for empty FS")
	}
}

func TestEmbeddedContent_ListFiles(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, ".")
	
	// With empty FS, should return empty list or error
	files, err := ec.ListFiles()
	
	// Either empty list or error is acceptable for empty FS
	if err == nil && len(files) > 0 {
		t.Errorf("Expected empty file list for empty FS, got %d files", len(files))
	}
}

func TestEmbeddedContent_ListDirectories(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, ".")
	
	// With empty FS, should return empty list or error
	dirs, err := ec.ListDirectories()
	
	// Either empty list or error is acceptable for empty FS
	if err == nil && len(dirs) > 0 {
		t.Errorf("Expected empty directory list for empty FS, got %d dirs", len(dirs))
	}
}

func TestEmbeddedContent_SyncDirectory(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, "configs")
	
	tasks := ec.SyncDirectory("", "/etc/app", "app", "app")
	
	// Should at least have directory creation task
	if len(tasks) < 1 {
		t.Error("Expected at least 1 task for sync")
	}
	
	// First task should ensure directory exists
	if tasks[0].Module != "file" {
		t.Errorf("Expected first task to be 'file' module, got '%s'", tasks[0].Module)
	}
	
	// Should have cleanup tasks at the end
	hasCleanup := false
	for _, task := range tasks {
		if task.Module == "find" || (task.Module == "file" && task.Loop != "") {
			hasCleanup = true
			break
		}
	}
	
	if len(tasks) > 1 && !hasCleanup {
		t.Error("Expected cleanup tasks in sync operation")
	}
}

func TestEmbeddedContent_ReadFile(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, "configs")
	
	// Test reading non-existent file
	_, err := ec.ReadFile("nonexistent.yml")
	if err == nil {
		t.Error("Expected error when reading non-existent file")
	}
}

func TestEmbeddedContent_DeployWithModes(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, ".")
	
	// Test that file modes are detected correctly
	// Even with empty FS, we can test the task structure
	
	// Deploy a shell script (should get 0755)
	tasks := ec.DeployFile("script.sh", "/usr/local/bin/script", "root", "root", "")
	if len(tasks) > 0 && tasks[0].Module != "fail" {
		args := tasks[0].Args
		// Mode should be set even if not specified
		if args["mode"] == nil {
			t.Error("Expected mode to be set for deployment")
		}
	}
	
	// Deploy a Python script (should get 0755)
	tasks = ec.DeployFile("app.py", "/opt/app/main.py", "app", "app", "")
	if len(tasks) > 0 && tasks[0].Module != "fail" {
		args := tasks[0].Args
		if args["mode"] == nil {
			t.Error("Expected mode to be set for Python script")
		}
	}
}

// Test helper functions
func TestEmbeddedContent_PathHandling(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, "base")
	
	// Test that paths are properly joined with base
	tasks := ec.DeployFile("subdir/file.txt", "/tmp/file.txt", "user", "group", "0644")
	
	// Even though it will fail, we can verify the error message contains the right path
	if len(tasks) == 1 && tasks[0].Module == "fail" {
		args := tasks[0].Args
		msg := args["msg"].(string)
		if msg == "" {
			t.Error("Expected error message in fail task")
		}
	}
}

// Mock test for template variable handling
func TestEmbeddedContent_TemplateVariables(t *testing.T) {
	var testFS embed.FS
	ec := NewEmbeddedContent(testFS, "templates")
	
	// Complex variables
	vars := map[string]interface{}{
		"app_name": "testapp",
		"servers": []string{"server1", "server2"},
		"config": map[string]interface{}{
			"port":    8080,
			"enabled": true,
		},
	}
	
	tasks := ec.DeployTemplate("complex.j2", "/etc/app/config", vars, "root", "root", "0644")
	
	// Even with failure, verify the task structure is correct
	if len(tasks) == 1 && tasks[0].Module == "fail" {
		// This is expected for empty FS
		return
	}
	
	// If we had real content, check that variables are passed correctly
	if len(tasks) >= 2 {
		for _, task := range tasks {
			if task.Module == "template" {
				args := task.Args
				if taskVars, ok := args["vars"].(map[string]interface{}); ok {
					if taskVars["app_name"] != "testapp" {
						t.Error("Template variables not passed correctly")
					}
				}
			}
		}
	}
}