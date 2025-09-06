// Package library provides built-in playbooks, roles, and task collections
package library

import (
	// "embed"  // Uncomment when we have embedded content
	"fmt"
	// "io/fs"  // Uncomment when we have embedded content
	// "path/filepath"  // Uncomment when we have embedded content
	"strings"
	
	// "gopkg.in/yaml.v3"  // Uncomment when we have embedded content
	"github.com/gosinble/gosinble/pkg/types"
)

// Commented out until we have actual embedded content
// //go:embed playbooks/*.yml tasks/*.yml roles/*/tasks/*.yml roles/*/handlers/*.yml roles/*/defaults/*.yml
// var builtinContent embed.FS

// Library provides access to built-in playbook components
type Library struct {
	playbooks map[string]*types.Playbook
	tasks     map[string][]types.Task
	roles     map[string]*Role
}

// Role represents a reusable role with tasks, handlers, defaults, etc.
type Role struct {
	Name      string
	Tasks     []types.Task
	Handlers  []types.Task
	Defaults  map[string]interface{}
	Vars      map[string]interface{}
	Meta      RoleMeta
}

// RoleMeta contains role metadata
type RoleMeta struct {
	Description  string
	Author       string
	Dependencies []string
	MinVersion   string
	Tags         []string
}

var defaultLibrary *Library

func init() {
	defaultLibrary = NewLibrary()
	defaultLibrary.Load()
}

// GetDefault returns the default library instance
func GetDefault() *Library {
	return defaultLibrary
}

// NewLibrary creates a new library instance
func NewLibrary() *Library {
	return &Library{
		playbooks: make(map[string]*types.Playbook),
		tasks:     make(map[string][]types.Task),
		roles:     make(map[string]*Role),
	}
}

// Load loads all built-in content
func (l *Library) Load() error {
	// Load playbooks
	if err := l.loadPlaybooks(); err != nil {
		return fmt.Errorf("failed to load playbooks: %w", err)
	}
	
	// Load task collections
	if err := l.loadTasks(); err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}
	
	// Load roles
	if err := l.loadRoles(); err != nil {
		return fmt.Errorf("failed to load roles: %w", err)
	}
	
	return nil
}

// GetPlaybook returns a built-in playbook by name
func (l *Library) GetPlaybook(name string) (*types.Playbook, error) {
	// Handle gosinble:// prefix
	name = strings.TrimPrefix(name, "gosinble://")
	name = strings.TrimSuffix(name, ".yml")
	
	playbook, exists := l.playbooks[name]
	if !exists {
		return nil, fmt.Errorf("playbook '%s' not found in library", name)
	}
	
	return playbook, nil
}

// GetTasks returns a built-in task collection by name
func (l *Library) GetTasks(name string) ([]types.Task, error) {
	// Handle gosinble:// prefix
	name = strings.TrimPrefix(name, "gosinble://")
	name = strings.TrimPrefix(name, "tasks/")
	name = strings.TrimSuffix(name, ".yml")
	
	tasks, exists := l.tasks[name]
	if !exists {
		return nil, fmt.Errorf("task collection '%s' not found in library", name)
	}
	
	return tasks, nil
}

// GetRole returns a built-in role by name
func (l *Library) GetRole(name string) (*Role, error) {
	// Handle gosinble. prefix
	name = strings.TrimPrefix(name, "gosinble.")
	
	role, exists := l.roles[name]
	if !exists {
		return nil, fmt.Errorf("role '%s' not found in library", name)
	}
	
	return role, nil
}

// ListPlaybooks returns all available playbook names
func (l *Library) ListPlaybooks() []string {
	names := make([]string, 0, len(l.playbooks))
	for name := range l.playbooks {
		names = append(names, name)
	}
	return names
}

// ListTasks returns all available task collection names
func (l *Library) ListTasks() []string {
	names := make([]string, 0, len(l.tasks))
	for name := range l.tasks {
		names = append(names, name)
	}
	return names
}

// ListRoles returns all available role names
func (l *Library) ListRoles() []string {
	names := make([]string, 0, len(l.roles))
	for name := range l.roles {
		names = append(names, name)
	}
	return names
}

// loadPlaybooks loads built-in playbooks from embedded files
func (l *Library) loadPlaybooks() error {
	// TODO: Implement when we have actual embedded content
	return nil
	
	/* Original implementation - uncomment when builtinContent is available:
	entries, err := fs.ReadDir(builtinContent, "playbooks")
	if err != nil {
		return nil // No playbooks directory is OK
	}
	
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		
		data, err := builtinContent.ReadFile(filepath.Join("playbooks", entry.Name()))
		if err != nil {
			continue
		}
		
		var plays []types.Play
		if err := yaml.Unmarshal(data, &plays); err != nil {
			continue
		}
		
		name := strings.TrimSuffix(entry.Name(), ".yml")
		l.playbooks[name] = &types.Playbook{
			Plays: plays,
		}
	}
	*/
	
	// return nil
}

// loadTasks loads built-in task collections from embedded files
func (l *Library) loadTasks() error {
	// TODO: Implement when we have actual embedded content
	return nil
	
	/* Original implementation - uncomment when builtinContent is available:
	entries, err := fs.ReadDir(builtinContent, "tasks")
	if err != nil {
		return nil // No tasks directory is OK
	}
	
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		
		data, err := builtinContent.ReadFile(filepath.Join("tasks", entry.Name()))
		if err != nil {
			continue
		}
		
		var tasks []types.Task
		if err := yaml.Unmarshal(data, &tasks); err != nil {
			continue
		}
		
		name := strings.TrimSuffix(entry.Name(), ".yml")
		l.tasks[name] = tasks
	}
	*/
	
	// return nil
}

// loadRoles loads built-in roles from embedded files
func (l *Library) loadRoles() error {
	// TODO: Implement when we have actual embedded content
	return nil
	
	/* Original implementation - uncomment when builtinContent is available:
	// Read roles directory
	roleEntries, err := fs.ReadDir(builtinContent, "roles")
	if err != nil {
		return nil // No roles directory is OK
	}
	
	for _, roleEntry := range roleEntries {
		if !roleEntry.IsDir() {
			continue
		}
		
		roleName := roleEntry.Name()
		role := &Role{
			Name:     roleName,
			Defaults: make(map[string]interface{}),
			Vars:     make(map[string]interface{}),
		}
		
		// Load tasks
		tasksFile := filepath.Join("roles", roleName, "tasks", "main.yml")
		if data, err := builtinContent.ReadFile(tasksFile); err == nil {
			var tasks []types.Task
			if err := yaml.Unmarshal(data, &tasks); err == nil {
				role.Tasks = tasks
			}
		}
		
		// Load handlers
		handlersFile := filepath.Join("roles", roleName, "handlers", "main.yml")
		if data, err := builtinContent.ReadFile(handlersFile); err == nil {
			var handlers []types.Task
			if err := yaml.Unmarshal(data, &handlers); err == nil {
				role.Handlers = handlers
			}
		}
		
		// Load defaults
		defaultsFile := filepath.Join("roles", roleName, "defaults", "main.yml")
		if data, err := builtinContent.ReadFile(defaultsFile); err == nil {
			if err := yaml.Unmarshal(data, &role.Defaults); err == nil {
				// Defaults loaded
			}
		}
		
		l.roles[roleName] = role
	}
	*/
	
	// return nil
}

// ApplyRole applies a role's tasks and handlers to a play
func (l *Library) ApplyRole(roleName string, play *types.Play) error {
	role, err := l.GetRole(roleName)
	if err != nil {
		return err
	}
	
	// Add role defaults to play vars
	if play.Vars == nil {
		play.Vars = make(map[string]interface{})
	}
	for k, v := range role.Defaults {
		if _, exists := play.Vars[k]; !exists {
			play.Vars[k] = v
		}
	}
	
	// Add role tasks to play
	play.Tasks = append(play.Tasks, role.Tasks...)
	
	// Add role handlers to play
	play.Handlers = append(play.Handlers, role.Handlers...)
	
	return nil
}

// IncludeTasks includes a task collection at the current position
func (l *Library) IncludeTasks(name string, vars map[string]interface{}) ([]types.Task, error) {
	tasks, err := l.GetTasks(name)
	if err != nil {
		return nil, err
	}
	
	// Apply provided variables to tasks
	if vars != nil {
		for i := range tasks {
			if tasks[i].Vars == nil {
				tasks[i].Vars = make(map[string]interface{})
			}
			for k, v := range vars {
				tasks[i].Vars[k] = v
			}
		}
	}
	
	return tasks, nil
}

// GeneratePlaybook generates a playbook from a template
func (l *Library) GeneratePlaybook(template string, params map[string]interface{}) (*types.Playbook, error) {
	// Get template playbook
	basePlaybook, err := l.GetPlaybook(template)
	if err != nil {
		return nil, err
	}
	
	// Deep copy the playbook
	playbook := &types.Playbook{
		Plays: make([]types.Play, len(basePlaybook.Plays)),
		Vars:  make(map[string]interface{}),
	}
	
	// Copy and customize plays
	for i, play := range basePlaybook.Plays {
		playbook.Plays[i] = play
		
		// Apply parameters
		if playbook.Plays[i].Vars == nil {
			playbook.Plays[i].Vars = make(map[string]interface{})
		}
		for k, v := range params {
			playbook.Plays[i].Vars[k] = v
		}
	}
	
	return playbook, nil
}