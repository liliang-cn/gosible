package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// DynamicInventory represents a dynamic inventory source
type DynamicInventory struct {
	source     DynamicSource
	cache      *InventoryCache
	staticInv  *StaticInventory
	lastUpdate time.Time
}

// DynamicSource is an interface for dynamic inventory sources
type DynamicSource interface {
	// GetInventory fetches the current inventory
	GetInventory(ctx context.Context) (*DynamicInventoryData, error)
	// GetHost fetches data for a specific host
	GetHost(ctx context.Context, hostname string) (map[string]interface{}, error)
	// Name returns the source name
	Name() string
	// Type returns the source type (script, plugin, etc)
	Type() string
}

// DynamicInventoryData represents the structure returned by dynamic inventory
type DynamicInventoryData struct {
	All      *GroupData             `json:"_meta,omitempty"`
	Groups   map[string]*GroupData  `json:",inline"`
	HostVars map[string]interface{} `json:"hostvars,omitempty"`
}

// GroupData represents a group in dynamic inventory
type GroupData struct {
	Hosts    []string               `json:"hosts,omitempty"`
	Children []string               `json:"children,omitempty"`
	Vars     map[string]interface{} `json:"vars,omitempty"`
}

// InventoryCache caches dynamic inventory data
type InventoryCache struct {
	data       *DynamicInventoryData
	expiration time.Time
	ttl        time.Duration
}

// NewDynamicInventory creates a new dynamic inventory
func NewDynamicInventory(source DynamicSource, cacheTTL time.Duration) *DynamicInventory {
	return &DynamicInventory{
		source: source,
		cache: &InventoryCache{
			ttl: cacheTTL,
		},
		staticInv: NewStaticInventory(),
	}
}

// Refresh updates the inventory from the dynamic source
func (di *DynamicInventory) Refresh(ctx context.Context) error {
	// Check cache
	if di.cache.IsValid() {
		return nil
	}

	// Fetch new inventory
	data, err := di.source.GetInventory(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch inventory from %s: %w", di.source.Name(), err)
	}

	// Update cache
	di.cache.data = data
	di.cache.expiration = time.Now().Add(di.cache.ttl)
	di.lastUpdate = time.Now()

	// Convert to static inventory
	return di.updateStaticInventory(data)
}

// updateStaticInventory converts dynamic data to static inventory
func (di *DynamicInventory) updateStaticInventory(data *DynamicInventoryData) error {
	// Clear existing inventory
	di.staticInv = NewStaticInventory()

	// Add all hosts with their variables
	if data.HostVars != nil {
		for hostname, vars := range data.HostVars {
			varsMap, ok := vars.(map[string]interface{})
			if !ok {
				varsMap = make(map[string]interface{})
			}
			
			host := types.Host{
				Name:      hostname,
				Variables: varsMap,
			}
			
			// Extract connection info if present
			if addr, ok := varsMap["ansible_host"].(string); ok {
				host.Address = addr
			} else {
				host.Address = hostname
			}
			
			if port, ok := varsMap["ansible_port"].(float64); ok {
				host.Port = int(port)
			}
			
			if user, ok := varsMap["ansible_user"].(string); ok {
				host.User = user
			}
			
			di.staticInv.AddHost(host)
		}
	}

	// Add groups
	for groupName, groupData := range data.Groups {
		if groupName == "_meta" {
			continue
		}

		group := types.Group{
			Name:      groupName,
			Hosts:     groupData.Hosts,
			Children:  groupData.Children,
			Variables: groupData.Vars,
		}
		
		di.staticInv.AddGroup(group)
	}

	return nil
}

// GetHosts returns hosts matching the pattern
func (di *DynamicInventory) GetHosts(pattern string) ([]types.Host, error) {
	ctx := context.Background()
	if err := di.Refresh(ctx); err != nil {
		return nil, err
	}
	return di.staticInv.GetHosts(pattern)
}

// GetHost returns a specific host
func (di *DynamicInventory) GetHost(name string) (*types.Host, error) {
	ctx := context.Background()
	if err := di.Refresh(ctx); err != nil {
		return nil, err
	}
	return di.staticInv.GetHost(name)
}

// GetGroups returns all groups
func (di *DynamicInventory) GetGroups() ([]types.Group, error) {
	ctx := context.Background()
	if err := di.Refresh(ctx); err != nil {
		return nil, err
	}
	return di.staticInv.GetGroups()
}

// GetGroup returns a specific group
func (di *DynamicInventory) GetGroup(name string) (*types.Group, error) {
	ctx := context.Background()
	if err := di.Refresh(ctx); err != nil {
		return nil, err
	}
	return di.staticInv.GetGroup(name)
}

// GetHostVars returns variables for a host
func (di *DynamicInventory) GetHostVars(hostname string) (map[string]interface{}, error) {
	ctx := context.Background()
	
	// Try to get from source first
	vars, err := di.source.GetHost(ctx, hostname)
	if err == nil && vars != nil {
		return vars, nil
	}
	
	// Fall back to cached data
	if err := di.Refresh(ctx); err != nil {
		return nil, err
	}
	return di.staticInv.GetHostVars(hostname)
}

// GetGroupVars returns variables for a group
func (di *DynamicInventory) GetGroupVars(groupname string) (map[string]interface{}, error) {
	ctx := context.Background()
	if err := di.Refresh(ctx); err != nil {
		return nil, err
	}
	return di.staticInv.GetGroupVars(groupname)
}

// AddHost is not supported for dynamic inventory
func (di *DynamicInventory) AddHost(host types.Host) error {
	return fmt.Errorf("cannot add host to dynamic inventory")
}

// AddGroup is not supported for dynamic inventory
func (di *DynamicInventory) AddGroup(group types.Group) error {
	return fmt.Errorf("cannot add group to dynamic inventory")
}

// IsValid checks if the cache is still valid
func (c *InventoryCache) IsValid() bool {
	if c.data == nil {
		return false
	}
	return time.Now().Before(c.expiration)
}

// ScriptInventorySource uses an external script as inventory source
type ScriptInventorySource struct {
	scriptPath string
	name       string
	timeout    time.Duration
}

// NewScriptInventorySource creates a new script-based inventory source
func NewScriptInventorySource(scriptPath, name string, timeout time.Duration) *ScriptInventorySource {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &ScriptInventorySource{
		scriptPath: scriptPath,
		name:       name,
		timeout:    timeout,
	}
}

// GetInventory executes the script to get inventory
func (s *ScriptInventorySource) GetInventory(ctx context.Context) (*DynamicInventoryData, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.scriptPath, "--list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute inventory script: %w", err)
	}

	var data DynamicInventoryData
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse inventory JSON: %w", err)
	}

	return &data, nil
}

// GetHost executes the script to get host data
func (s *ScriptInventorySource) GetHost(ctx context.Context, hostname string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.scriptPath, "--host", hostname)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute inventory script for host %s: %w", hostname, err)
	}

	var vars map[string]interface{}
	if err := json.Unmarshal(output, &vars); err != nil {
		return nil, fmt.Errorf("failed to parse host JSON: %w", err)
	}

	return vars, nil
}

// Name returns the source name
func (s *ScriptInventorySource) Name() string {
	return s.name
}

// Type returns "script"
func (s *ScriptInventorySource) Type() string {
	return "script"
}

// PluginInventorySource represents a plugin-based inventory source
type PluginInventorySource struct {
	plugin InventoryPlugin
	name   string
	config map[string]interface{}
}

// InventoryPlugin interface for inventory plugins
type InventoryPlugin interface {
	// Name returns the plugin name
	Name() string
	// Initialize sets up the plugin with configuration
	Initialize(config map[string]interface{}) error
	// GetInventory fetches the inventory
	GetInventory(ctx context.Context) (*DynamicInventoryData, error)
	// GetHost fetches host-specific data
	GetHost(ctx context.Context, hostname string) (map[string]interface{}, error)
}

// NewPluginInventorySource creates a new plugin-based inventory source
func NewPluginInventorySource(plugin InventoryPlugin, name string, config map[string]interface{}) (*PluginInventorySource, error) {
	if err := plugin.Initialize(config); err != nil {
		return nil, fmt.Errorf("failed to initialize plugin %s: %w", name, err)
	}
	
	return &PluginInventorySource{
		plugin: plugin,
		name:   name,
		config: config,
	}, nil
}

// GetInventory delegates to the plugin
func (p *PluginInventorySource) GetInventory(ctx context.Context) (*DynamicInventoryData, error) {
	return p.plugin.GetInventory(ctx)
}

// GetHost delegates to the plugin
func (p *PluginInventorySource) GetHost(ctx context.Context, hostname string) (map[string]interface{}, error) {
	return p.plugin.GetHost(ctx, hostname)
}

// Name returns the source name
func (p *PluginInventorySource) Name() string {
	return p.name
}

// Type returns "plugin"
func (p *PluginInventorySource) Type() string {
	return "plugin"
}