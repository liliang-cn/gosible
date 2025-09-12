package playbook

import (
	"testing"

	"github.com/liliang-cn/gosible/pkg/types"
)

func TestBasicFunctionality(t *testing.T) {
	t.Run("Parser_Creation", func(t *testing.T) {
		parser := NewParser()
		if parser == nil {
			t.Fatal("NewParser() returned nil")
		}
	})

	t.Run("IncludeManager_Creation", func(t *testing.T) {
		manager := NewIncludeManager("/test")
		if manager == nil {
			t.Fatal("NewIncludeManager() returned nil")
		}
		if manager.basePath != "/test" {
			t.Errorf("Expected basePath '/test', got '%s'", manager.basePath)
		}
	})

	t.Run("IncludeTypes", func(t *testing.T) {
		if IncludeStatic != "static" {
			t.Errorf("Expected IncludeStatic 'static', got '%s'", IncludeStatic)
		}
		if IncludeDynamic != "dynamic" {
			t.Errorf("Expected IncludeDynamic 'dynamic', got '%s'", IncludeDynamic)
		}
	})

	t.Run("IncludeTask_Creation", func(t *testing.T) {
		task := &IncludeTask{
			Type: IncludeStatic,
			File: "tasks.yml",
			Name: "Include tasks",
			Vars: map[string]interface{}{"var1": "value1"},
		}
		
		if task.Type != IncludeStatic {
			t.Errorf("Expected type IncludeStatic, got %v", task.Type)
		}
		if task.File != "tasks.yml" {
			t.Errorf("Expected file 'tasks.yml', got '%s'", task.File)
		}
		if task.Name != "Include tasks" {
			t.Errorf("Expected name 'Include tasks', got '%s'", task.Name)
		}
	})

	t.Run("LoopControl_Creation", func(t *testing.T) {
		control := &LoopControl{
			LoopVar:   "item",
			IndexVar:  "idx",
			Label:     "{{ item.name }}",
			PauseTime: 5,
		}
		
		if control.LoopVar != "item" {
			t.Errorf("Expected loop_var 'item', got '%s'", control.LoopVar)
		}
		if control.IndexVar != "idx" {
			t.Errorf("Expected index_var 'idx', got '%s'", control.IndexVar)
		}
		if control.PauseTime != 5 {
			t.Errorf("Expected pause 5, got %d", control.PauseTime)
		}
	})

	t.Run("Parser_InvalidYAML", func(t *testing.T) {
		parser := NewParser()
		invalidYaml := "invalid: yaml: [content"
		
		_, err := parser.Parse([]byte(invalidYaml), "test.yml")
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
	})

	t.Run("Parser_EmptyPlaybook", func(t *testing.T) {
		parser := NewParser()
		emptyYaml := "---\n"
		
		_, err := parser.Parse([]byte(emptyYaml), "empty.yml")
		if err == nil {
			t.Error("Expected error for empty playbook")
		}
	})

	t.Run("Parser_ValidPlaybook", func(t *testing.T) {
		parser := NewParser()
		validYaml := `
- name: Test play
  hosts: localhost
  tasks:
    - name: Test task
      debug:
        msg: "hello"
`
		
		playbook, err := parser.Parse([]byte(validYaml), "test.yml")
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		
		if len(playbook.Plays) != 1 {
			t.Errorf("Expected 1 play, got %d", len(playbook.Plays))
		}
		
		if playbook.Plays[0].Name != "Test play" {
			t.Errorf("Expected play name 'Test play', got '%s'", playbook.Plays[0].Name)
		}
	})

	t.Run("IncludeManager_Cache", func(t *testing.T) {
		manager := NewIncludeManager("/test")
		
		// Test cache initialization
		if manager.taskCache == nil {
			t.Error("Task cache should be initialized")
		}
		
		// Add to cache
		manager.taskCache["test"] = []types.Task{{Name: "test"}}
		
		if len(manager.taskCache) == 0 {
			t.Error("Cache should have content after adding")
		}
		
		if task := manager.taskCache["test"]; len(task) == 0 || task[0].Name != "test" {
			t.Error("Cache content not correct")
		}
	})
}