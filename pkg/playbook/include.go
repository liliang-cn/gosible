package playbook

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liliang-cn/gosible/pkg/types"
	"gopkg.in/yaml.v3"
)

// IncludeType represents the type of include operation
type IncludeType string

const (
	IncludeStatic  IncludeType = "static"  // import_tasks, import_playbook
	IncludeDynamic IncludeType = "dynamic" // include_tasks, include_role
)

// IncludeTask represents an include or import task
type IncludeTask struct {
	Type        IncludeType            `yaml:"-"`
	File        string                 `yaml:"file,omitempty"`
	Name        string                 `yaml:"name,omitempty"`
	Vars        map[string]interface{} `yaml:"vars,omitempty"`
	Tags        []string               `yaml:"tags,omitempty"`
	When        string                 `yaml:"when,omitempty"`
	Loop        interface{}            `yaml:"loop,omitempty"`
	LoopControl *LoopControl           `yaml:"loop_control,omitempty"`
	Tasks       []types.Task           `yaml:"-"` // Loaded tasks
}

// LoopControl provides loop control options
type LoopControl struct {
	LoopVar   string `yaml:"loop_var,omitempty"`
	IndexVar  string `yaml:"index_var,omitempty"`
	Label     string `yaml:"label,omitempty"`
	PauseTime int    `yaml:"pause,omitempty"`
}

// IncludeManager manages task includes and imports
type IncludeManager struct {
	basePath  string
	taskCache map[string][]types.Task
}

// NewIncludeManager creates a new include manager
func NewIncludeManager(basePath string) *IncludeManager {
	return &IncludeManager{
		basePath:  basePath,
		taskCache: make(map[string][]types.Task),
	}
}

// ProcessInclude processes an include or import directive
func (im *IncludeManager) ProcessInclude(ctx context.Context, includeTask *IncludeTask) ([]types.Task, error) {
	// Resolve the file path
	filePath := im.resolveFilePath(includeTask.File)
	
	// Check cache for static includes
	if includeTask.Type == IncludeStatic {
		if cachedTasks, exists := im.taskCache[filePath]; exists {
			return im.applyIncludeVars(cachedTasks, includeTask.Vars), nil
		}
	}

	// Load tasks from file
	tasks, err := im.loadTasksFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load tasks from %s: %w", filePath, err)
	}

	// Apply include-level variables
	tasks = im.applyIncludeVars(tasks, includeTask.Vars)

	// Apply tags if specified
	if len(includeTask.Tags) > 0 {
		tasks = im.applyTags(tasks, includeTask.Tags)
	}

	// Apply when condition if specified
	if includeTask.When != "" {
		tasks = im.applyWhenCondition(tasks, includeTask.When)
	}

	// Handle loop if specified
	if includeTask.Loop != nil {
		tasks = im.expandLoop(tasks, includeTask.Loop, includeTask.LoopControl)
	}

	// Cache static includes
	if includeTask.Type == IncludeStatic {
		im.taskCache[filePath] = tasks
	}

	return tasks, nil
}

// ImportTasks imports tasks statically (at parse time)
func (im *IncludeManager) ImportTasks(file string, vars map[string]interface{}) ([]types.Task, error) {
	includeTask := &IncludeTask{
		Type: IncludeStatic,
		File: file,
		Vars: vars,
	}
	return im.ProcessInclude(context.Background(), includeTask)
}

// IncludeTasks includes tasks dynamically (at runtime)
func (im *IncludeManager) IncludeTasks(ctx context.Context, file string, vars map[string]interface{}) ([]types.Task, error) {
	includeTask := &IncludeTask{
		Type: IncludeDynamic,
		File: file,
		Vars: vars,
	}
	return im.ProcessInclude(ctx, includeTask)
}

// ImportPlaybook imports an entire playbook
func (im *IncludeManager) ImportPlaybook(file string, vars map[string]interface{}) (*types.Playbook, error) {
	filePath := im.resolveFilePath(file)
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read playbook file %s: %w", filePath, err)
	}

	var playbooks []types.Playbook
	if err := yaml.Unmarshal(data, &playbooks); err != nil {
		// Try single playbook format
		var playbook types.Playbook
		if err := yaml.Unmarshal(data, &playbook); err != nil {
			return nil, fmt.Errorf("failed to parse playbook %s: %w", filePath, err)
		}
		playbooks = []types.Playbook{playbook}
	}

	if len(playbooks) == 0 {
		return nil, fmt.Errorf("no playbooks found in %s", filePath)
	}

	// Apply vars to the first playbook
	playbook := &playbooks[0]
	if playbook.Vars == nil {
		playbook.Vars = make(map[string]interface{})
	}
	for k, v := range vars {
		playbook.Vars[k] = v
	}

	return playbook, nil
}

// IncludeRole includes a role dynamically
func (im *IncludeManager) IncludeRole(ctx context.Context, roleName string, vars map[string]interface{}, tasks string) ([]types.Task, error) {
	// This would integrate with the role manager
	// For now, return a placeholder
	return []types.Task{
		{
			Name:   fmt.Sprintf("Include role: %s", roleName),
			Module: "include_role",
			Args: map[string]interface{}{
				"name":  roleName,
				"vars":  vars,
				"tasks": tasks,
			},
		},
	}, nil
}

// ImportRole imports a role statically
func (im *IncludeManager) ImportRole(roleName string, vars map[string]interface{}) ([]types.Task, error) {
	// This would integrate with the role manager
	// For now, return a placeholder
	return []types.Task{
		{
			Name:   fmt.Sprintf("Import role: %s", roleName),
			Module: "import_role",
			Args: map[string]interface{}{
				"name": roleName,
				"vars": vars,
			},
		},
	}, nil
}

// Helper methods

func (im *IncludeManager) resolveFilePath(file string) string {
	// If absolute path, use as-is
	if filepath.IsAbs(file) {
		return file
	}
	
	// Try relative to base path
	fullPath := filepath.Join(im.basePath, file)
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath
	}
	
	// Try common directories
	searchPaths := []string{
		filepath.Join(im.basePath, "tasks", file),
		filepath.Join(im.basePath, "includes", file),
		filepath.Join(im.basePath, file),
	}
	
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	
	// Return original if not found (will error later)
	return file
}

func (im *IncludeManager) loadTasksFromFile(file string) ([]types.Task, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var tasks []types.Task
	if err := yaml.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (im *IncludeManager) applyIncludeVars(tasks []types.Task, includeVars map[string]interface{}) []types.Task {
	if includeVars == nil || len(includeVars) == 0 {
		return tasks
	}

	// Deep copy tasks and apply vars
	result := make([]types.Task, len(tasks))
	for i, task := range tasks {
		result[i] = task
		if result[i].Vars == nil {
			result[i].Vars = make(map[string]interface{})
		}
		// Include vars have lower priority than task vars
		for k, v := range includeVars {
			if _, exists := result[i].Vars[k]; !exists {
				result[i].Vars[k] = v
			}
		}
	}
	
	return result
}

func (im *IncludeManager) applyTags(tasks []types.Task, tags []string) []types.Task {
	result := make([]types.Task, len(tasks))
	for i, task := range tasks {
		result[i] = task
		// Append tags to existing tags
		result[i].Tags = append(result[i].Tags, tags...)
	}
	return result
}

func (im *IncludeManager) applyWhenCondition(tasks []types.Task, when string) []types.Task {
	result := make([]types.Task, len(tasks))
	for i, task := range tasks {
		result[i] = task
		// Combine conditions with AND
		if result[i].When != nil {
			result[i].When = fmt.Sprintf("(%v) and (%s)", result[i].When, when)
		} else {
			result[i].When = when
		}
	}
	return result
}

func (im *IncludeManager) expandLoop(tasks []types.Task, loop interface{}, control *LoopControl) []types.Task {
	// This would expand tasks for each loop item
	// For now, mark tasks as having a loop
	result := make([]types.Task, len(tasks))
	for i, task := range tasks {
		result[i] = task
		result[i].Loop = loop
		if control != nil {
			// Apply loop control settings
			if result[i].Args == nil {
				result[i].Args = make(map[string]interface{})
			}
			result[i].Args["_loop_control"] = control
		}
	}
	return result
}

// ParseIncludeTask parses an include task from a map
func ParseIncludeTask(data map[string]interface{}) (*IncludeTask, error) {
	include := &IncludeTask{
		Vars: make(map[string]interface{}),
	}

	// Determine type based on module name
	if module, ok := data["import_tasks"].(string); ok {
		include.Type = IncludeStatic
		include.File = module
	} else if module, ok := data["include_tasks"].(string); ok {
		include.Type = IncludeDynamic
		include.File = module
	} else if module, ok := data["include"].(string); ok {
		include.Type = IncludeDynamic
		include.File = module
	} else {
		return nil, fmt.Errorf("no include directive found")
	}

	// Parse other fields
	if name, ok := data["name"].(string); ok {
		include.Name = name
	}

	if vars, ok := data["vars"].(map[string]interface{}); ok {
		include.Vars = vars
	}

	if tags, ok := data["tags"].([]interface{}); ok {
		include.Tags = make([]string, len(tags))
		for i, tag := range tags {
			include.Tags[i] = fmt.Sprintf("%v", tag)
		}
	}

	if when, ok := data["when"].(string); ok {
		include.When = when
	}

	if loop, ok := data["loop"]; ok {
		include.Loop = loop
	}

	if control, ok := data["loop_control"].(map[string]interface{}); ok {
		include.LoopControl = &LoopControl{}
		if loopVar, ok := control["loop_var"].(string); ok {
			include.LoopControl.LoopVar = loopVar
		}
		if indexVar, ok := control["index_var"].(string); ok {
			include.LoopControl.IndexVar = indexVar
		}
		if label, ok := control["label"].(string); ok {
			include.LoopControl.Label = label
		}
		if pause, ok := control["pause"].(int); ok {
			include.LoopControl.PauseTime = pause
		}
	}

	return include, nil
}

// IsIncludeTask checks if a task map represents an include directive
func IsIncludeTask(data map[string]interface{}) bool {
	includeKeys := []string{"include", "include_tasks", "import_tasks", "include_role", "import_role", "import_playbook"}
	for _, key := range includeKeys {
		if _, exists := data[key]; exists {
			return true
		}
	}
	return false
}