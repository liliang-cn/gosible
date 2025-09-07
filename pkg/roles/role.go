package roles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gosinble/gosinble/pkg/types"
	"gopkg.in/yaml.v3"
)

// Role represents an Ansible-like role structure
type Role struct {
	Name         string                 `yaml:"name"`
	Path         string                 `yaml:"-"`
	Tasks        []types.Task           `yaml:"-"`
	Handlers     []types.Task           `yaml:"-"`
	Defaults     map[string]interface{} `yaml:"-"`
	Vars         map[string]interface{} `yaml:"-"`
	Meta         *RoleMeta              `yaml:"-"`
	Files        []string               `yaml:"-"`
	Templates    []string               `yaml:"-"`
	Dependencies []RoleDependency       `yaml:"-"`
}

// RoleMeta contains role metadata
type RoleMeta struct {
	Author          string             `yaml:"author,omitempty"`
	Description     string             `yaml:"description,omitempty"`
	Company         string             `yaml:"company,omitempty"`
	License         string             `yaml:"license,omitempty"`
	MinAnsibleVersion string           `yaml:"min_ansible_version,omitempty"`
	Platforms       []Platform         `yaml:"platforms,omitempty"`
	Dependencies    []RoleDependency   `yaml:"dependencies,omitempty"`
	Tags            []string           `yaml:"galaxy_tags,omitempty"`
}

// Platform represents a supported platform
type Platform struct {
	Name     string   `yaml:"name"`
	Versions []string `yaml:"versions,omitempty"`
}

// RoleDependency represents a role dependency
type RoleDependency struct {
	Role    string                 `yaml:"role"`
	Src     string                 `yaml:"src,omitempty"`
	Version string                 `yaml:"version,omitempty"`
	Vars    map[string]interface{} `yaml:"vars,omitempty"`
	Tags    []string               `yaml:"tags,omitempty"`
}

// RoleManager manages roles
type RoleManager struct {
	rolesPath   []string
	loadedRoles map[string]*Role
}

// NewRoleManager creates a new role manager
func NewRoleManager(rolesPath []string) *RoleManager {
	if len(rolesPath) == 0 {
		rolesPath = []string{"roles", "/etc/ansible/roles"}
	}
	return &RoleManager{
		rolesPath:   rolesPath,
		loadedRoles: make(map[string]*Role),
	}
}

// LoadRole loads a role by name
func (rm *RoleManager) LoadRole(name string) (*Role, error) {
	// Check if already loaded
	if role, exists := rm.loadedRoles[name]; exists {
		return role, nil
	}

	// Search for role in paths
	var rolePath string
	for _, basePath := range rm.rolesPath {
		candidatePath := filepath.Join(basePath, name)
		if info, err := os.Stat(candidatePath); err == nil && info.IsDir() {
			rolePath = candidatePath
			break
		}
	}

	if rolePath == "" {
		return nil, fmt.Errorf("role '%s' not found in paths: %v", name, rm.rolesPath)
	}

	role := &Role{
		Name: name,
		Path: rolePath,
	}

	// Load role components
	if err := rm.loadRoleTasks(role); err != nil {
		return nil, fmt.Errorf("failed to load tasks for role '%s': %w", name, err)
	}

	if err := rm.loadRoleHandlers(role); err != nil {
		return nil, fmt.Errorf("failed to load handlers for role '%s': %w", name, err)
	}

	if err := rm.loadRoleVars(role); err != nil {
		return nil, fmt.Errorf("failed to load vars for role '%s': %w", name, err)
	}

	if err := rm.loadRoleDefaults(role); err != nil {
		return nil, fmt.Errorf("failed to load defaults for role '%s': %w", name, err)
	}

	if err := rm.loadRoleMeta(role); err != nil {
		return nil, fmt.Errorf("failed to load meta for role '%s': %w", name, err)
	}

	// Load file and template lists
	role.Files = rm.listRoleFiles(role, "files")
	role.Templates = rm.listRoleFiles(role, "templates")

	// Cache the loaded role
	rm.loadedRoles[name] = role

	return role, nil
}

// loadRoleTasks loads tasks from tasks/main.yml
func (rm *RoleManager) loadRoleTasks(role *Role) error {
	tasksFile := filepath.Join(role.Path, "tasks", "main.yml")
	if _, err := os.Stat(tasksFile); os.IsNotExist(err) {
		// Tasks file is optional
		return nil
	}

	data, err := os.ReadFile(tasksFile)
	if err != nil {
		return err
	}

	var tasks []types.Task
	if err := yaml.Unmarshal(data, &tasks); err != nil {
		return err
	}

	role.Tasks = tasks
	return nil
}

// loadRoleHandlers loads handlers from handlers/main.yml
func (rm *RoleManager) loadRoleHandlers(role *Role) error {
	handlersFile := filepath.Join(role.Path, "handlers", "main.yml")
	if _, err := os.Stat(handlersFile); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(handlersFile)
	if err != nil {
		return err
	}

	var handlers []types.Task
	if err := yaml.Unmarshal(data, &handlers); err != nil {
		return err
	}

	role.Handlers = handlers
	return nil
}

// loadRoleVars loads variables from vars/main.yml
func (rm *RoleManager) loadRoleVars(role *Role) error {
	varsFile := filepath.Join(role.Path, "vars", "main.yml")
	if _, err := os.Stat(varsFile); os.IsNotExist(err) {
		role.Vars = make(map[string]interface{})
		return nil
	}

	data, err := os.ReadFile(varsFile)
	if err != nil {
		return err
	}

	vars := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &vars); err != nil {
		return err
	}

	role.Vars = vars
	return nil
}

// loadRoleDefaults loads defaults from defaults/main.yml
func (rm *RoleManager) loadRoleDefaults(role *Role) error {
	defaultsFile := filepath.Join(role.Path, "defaults", "main.yml")
	if _, err := os.Stat(defaultsFile); os.IsNotExist(err) {
		role.Defaults = make(map[string]interface{})
		return nil
	}

	data, err := os.ReadFile(defaultsFile)
	if err != nil {
		return err
	}

	defaults := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &defaults); err != nil {
		return err
	}

	role.Defaults = defaults
	return nil
}

// loadRoleMeta loads metadata from meta/main.yml
func (rm *RoleManager) loadRoleMeta(role *Role) error {
	metaFile := filepath.Join(role.Path, "meta", "main.yml")
	if _, err := os.Stat(metaFile); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(metaFile)
	if err != nil {
		return err
	}

	var meta RoleMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return err
	}

	role.Meta = &meta
	role.Dependencies = meta.Dependencies
	return nil
}

// listRoleFiles lists files in a role subdirectory
func (rm *RoleManager) listRoleFiles(role *Role, subdir string) []string {
	dirPath := filepath.Join(role.Path, subdir)
	files := []string{}

	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(dirPath, path)
		files = append(files, relPath)
		return nil
	})

	return files
}

// ApplyRole applies a role to the given hosts
func (rm *RoleManager) ApplyRole(ctx context.Context, roleName string, hosts []string, extraVars map[string]interface{}) (*types.Result, error) {
	role, err := rm.LoadRole(roleName)
	if err != nil {
		return nil, err
	}

	// Merge variables (defaults -> vars -> extra vars)
	mergedVars := make(map[string]interface{})
	
	// Apply defaults first (lowest priority)
	for k, v := range role.Defaults {
		mergedVars[k] = v
	}
	
	// Apply role vars (medium priority)
	for k, v := range role.Vars {
		mergedVars[k] = v
	}
	
	// Apply extra vars (highest priority)
	for k, v := range extraVars {
		mergedVars[k] = v
	}

	// Apply dependencies first
	for _, dep := range role.Dependencies {
		depResult, err := rm.ApplyRole(ctx, dep.Role, hosts, dep.Vars)
		if err != nil {
			return nil, fmt.Errorf("failed to apply dependency '%s': %w", dep.Role, err)
		}
		if !depResult.Success {
			return nil, fmt.Errorf("dependency '%s' failed", dep.Role)
		}
	}

	// Execute role tasks
	result := &types.Result{
		Host:    strings.Join(hosts, ","),
		Success: true,
		Changed: false,
		Message: fmt.Sprintf("Applied role '%s'", roleName),
		Data: map[string]interface{}{
			"role": roleName,
			"vars": mergedVars,
		},
	}

	// TODO: Execute tasks with the merged variables
	// This would integrate with the task runner

	return result, nil
}

// GetRolePath returns the filesystem path for a role
func (rm *RoleManager) GetRolePath(roleName string) (string, error) {
	role, err := rm.LoadRole(roleName)
	if err != nil {
		return "", err
	}
	return role.Path, nil
}

// GetRoleFile returns the path to a file within a role
func (rm *RoleManager) GetRoleFile(roleName, fileType, fileName string) (string, error) {
	role, err := rm.LoadRole(roleName)
	if err != nil {
		return "", err
	}

	filePath := filepath.Join(role.Path, fileType, fileName)
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("file '%s' not found in role '%s' %s directory", fileName, roleName, fileType)
	}

	return filePath, nil
}

// ListRoles returns a list of available roles
func (rm *RoleManager) ListRoles() []string {
	roleSet := make(map[string]bool)
	
	for _, basePath := range rm.rolesPath {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			continue
		}
		
		for _, entry := range entries {
			if entry.IsDir() {
				roleSet[entry.Name()] = true
			}
		}
	}
	
	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	
	return roles
}