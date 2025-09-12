// Package modules provides the module system architecture for gosible.
package modules

import (
	"fmt"
	"sync"

	"github.com/liliang-cn/gosible/pkg/types"
)

// ModuleRegistry manages registered modules
type ModuleRegistry struct {
	mu      sync.RWMutex
	modules map[string]types.Module
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry() *ModuleRegistry {
	registry := &ModuleRegistry{
		modules: make(map[string]types.Module),
	}

	// Register built-in modules
	registry.registerBuiltinModules()

	return registry
}

// RegisterModule registers a module in the registry
func (r *ModuleRegistry) RegisterModule(module types.Module) error {
	if module == nil {
		return fmt.Errorf("module cannot be nil")
	}

	name := module.Name()
	if name == "" {
		return fmt.Errorf("module name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.modules[name] = module
	return nil
}

// GetModule retrieves a module by name
func (r *ModuleRegistry) GetModule(name string) (types.Module, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	module, exists := r.modules[name]
	if !exists {
		return nil, types.ErrModuleNotFound
	}

	return module, nil
}

// ListModules returns all registered module names
func (r *ModuleRegistry) ListModules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name := range r.modules {
		names = append(names, name)
	}

	return names
}

// GetModuleDocumentation returns documentation for a module
func (r *ModuleRegistry) GetModuleDocumentation(name string) (*types.ModuleDoc, error) {
	module, err := r.GetModule(name)
	if err != nil {
		return nil, err
	}

	doc := module.Documentation()
	return &doc, nil
}

// UnregisterModule removes a module from the registry
func (r *ModuleRegistry) UnregisterModule(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[name]; !exists {
		return types.ErrModuleNotFound
	}

	delete(r.modules, name)
	return nil
}

// ValidateModuleArgs validates module arguments before execution
func (r *ModuleRegistry) ValidateModuleArgs(name string, args map[string]interface{}) error {
	module, err := r.GetModule(name)
	if err != nil {
		return err
	}

	return module.Validate(args)
}

// registerBuiltinModules registers all built-in modules
func (r *ModuleRegistry) registerBuiltinModules() {
	// Register ping module
	r.RegisterModule(NewPingModule())

	// Register command module
	r.RegisterModule(NewCommandModule())

	// Register copy module
	r.RegisterModule(NewCopyModule())

	// Register template module
	r.RegisterModule(NewTemplateModule())

	// Register file module
	r.RegisterModule(NewFileModule())

	// Register setup module (fact gathering)
	r.RegisterModule(NewSetupModule())

	// Register shell module
	r.RegisterModule(NewShellModule())

	// Register debug module
	r.RegisterModule(NewDebugModule())

	// Register service module
	r.RegisterModule(NewServiceModule())

	// Register package module
	r.RegisterModule(NewPackageModule())

	// Register user module
	r.RegisterModule(NewUserModule())

	// Register group module
	r.RegisterModule(NewGroupModule())

	// Register archive module
	r.RegisterModule(NewArchiveModule())

	// Register unarchive module
	r.RegisterModule(NewUnarchiveModule())

	// Register gem module
	r.RegisterModule(NewGemModule())

	// Register mount module
	r.RegisterModule(NewMountModule())

	// Register npm module
	r.RegisterModule(NewNpmModule())

	// Register pip module
	r.RegisterModule(NewPipModule())

	// Register sysctl module
	r.RegisterModule(NewSysctlModule())

	// Register iptables module
	r.RegisterModule(NewIPTablesModule())
}

// DefaultModuleRegistry provides a default module registry instance
var DefaultModuleRegistry = NewModuleRegistry()
