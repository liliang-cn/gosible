package roles

import (
	"fmt"
	"sort"
)

// DependencyResolver resolves role dependencies in correct order
type DependencyResolver struct {
	roles      map[string]*Role
	resolved   map[string]bool
	inProgress map[string]bool
	order      []string
}

// NewDependencyResolver creates a new dependency resolver
func NewDependencyResolver() *DependencyResolver {
	return &DependencyResolver{
		roles:      make(map[string]*Role),
		resolved:   make(map[string]bool),
		inProgress: make(map[string]bool),
		order:      []string{},
	}
}

// AddRole adds a role to the resolver
func (dr *DependencyResolver) AddRole(role *Role) {
	dr.roles[role.Name] = role
}

// Resolve returns roles in dependency order
func (dr *DependencyResolver) Resolve() ([]*Role, error) {
	// Reset state
	dr.resolved = make(map[string]bool)
	dr.inProgress = make(map[string]bool)
	dr.order = []string{}

	// Sort role names for deterministic order
	var roleNames []string
	for name := range dr.roles {
		roleNames = append(roleNames, name)
	}
	sort.Strings(roleNames)

	// Resolve each role
	for _, name := range roleNames {
		if !dr.resolved[name] {
			if err := dr.visit(name); err != nil {
				return nil, err
			}
		}
	}

	// Build result in resolved order
	var result []*Role
	for _, name := range dr.order {
		if role, exists := dr.roles[name]; exists {
			result = append(result, role)
		}
	}

	return result, nil
}

// visit performs depth-first traversal for dependency resolution
func (dr *DependencyResolver) visit(name string) error {
	if dr.inProgress[name] {
		return fmt.Errorf("circular dependency detected involving role '%s'", name)
	}

	if dr.resolved[name] {
		return nil
	}

	dr.inProgress[name] = true

	role, exists := dr.roles[name]
	if !exists {
		return fmt.Errorf("role '%s' not found", name)
	}

	// Visit dependencies first
	for _, dep := range role.Dependencies {
		// Check if dependency exists
		if _, exists := dr.roles[dep.Role]; !exists {
			// Try to load the dependency
			// This would typically call back to RoleManager.LoadRole
			return fmt.Errorf("dependency '%s' of role '%s' not found", dep.Role, name)
		}

		if err := dr.visit(dep.Role); err != nil {
			return err
		}
	}

	dr.inProgress[name] = false
	dr.resolved[name] = true
	dr.order = append(dr.order, name)

	return nil
}

// GetExecutionOrder returns the execution order for roles
func (dr *DependencyResolver) GetExecutionOrder() []string {
	return dr.order
}

// CheckCircularDependencies checks for circular dependencies
func (dr *DependencyResolver) CheckCircularDependencies() error {
	for name := range dr.roles {
		visited := make(map[string]bool)
		path := []string{}
		if err := dr.checkCircular(name, visited, path); err != nil {
			return err
		}
	}
	return nil
}

// checkCircular performs DFS to detect circular dependencies
func (dr *DependencyResolver) checkCircular(name string, visited map[string]bool, path []string) error {
	// Check if we've seen this role in the current path
	for _, p := range path {
		if p == name {
			return fmt.Errorf("circular dependency detected: %v -> %s", path, name)
		}
	}

	if visited[name] {
		return nil
	}

	role, exists := dr.roles[name]
	if !exists {
		return nil // Skip missing roles during circular check
	}

	newPath := append(path, name)

	for _, dep := range role.Dependencies {
		if err := dr.checkCircular(dep.Role, visited, newPath); err != nil {
			return err
		}
	}

	visited[name] = true
	return nil
}

// GetDependencyGraph returns a map of role to its dependencies
func (dr *DependencyResolver) GetDependencyGraph() map[string][]string {
	graph := make(map[string][]string)
	
	for name, role := range dr.roles {
		var deps []string
		for _, dep := range role.Dependencies {
			deps = append(deps, dep.Role)
		}
		graph[name] = deps
	}
	
	return graph
}

// GetDependents returns roles that depend on the given role
func (dr *DependencyResolver) GetDependents(roleName string) []string {
	var dependents []string
	
	for name, role := range dr.roles {
		if name == roleName {
			continue
		}
		
		for _, dep := range role.Dependencies {
			if dep.Role == roleName {
				dependents = append(dependents, name)
				break
			}
		}
	}
	
	return dependents
}