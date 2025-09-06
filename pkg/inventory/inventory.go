// Package inventory provides host and group management functionality for gosinble.
package inventory

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/gosinble/gosinble/pkg/types"
)

// StaticInventory implements the Inventory interface with static host and group data
type StaticInventory struct {
	mu     sync.RWMutex
	hosts  map[string]types.Host
	groups map[string]types.Group
}

// InventoryData represents the structure of inventory YAML files
type InventoryData struct {
	All struct {
		Hosts    map[string]map[string]interface{} `yaml:"hosts,omitempty"`
		Children map[string]types.Group          `yaml:"children,omitempty"`
		Vars     map[string]interface{}           `yaml:"vars,omitempty"`
	} `yaml:"all"`
}

// NewStaticInventory creates a new static inventory
func NewStaticInventory() *StaticInventory {
	return &StaticInventory{
		hosts:  make(map[string]types.Host),
		groups: make(map[string]types.Group),
	}
}

// NewFromFile creates an inventory from a YAML file
func NewFromFile(filepath string) (*StaticInventory, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, types.NewInventoryError(filepath, "failed to open file", err)
	}
	defer file.Close()

	return NewFromReader(file)
}

// NewFromReader creates an inventory from an io.Reader containing YAML data
func NewFromReader(reader io.Reader) (*StaticInventory, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, types.NewInventoryError("reader", "failed to read data", err)
	}

	return NewFromYAML(data)
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// NewFromYAML creates an inventory from YAML data
func NewFromYAML(data []byte) (*StaticInventory, error) {
	var inventoryData InventoryData
	if err := yaml.Unmarshal(data, &inventoryData); err != nil {
		return nil, types.NewInventoryError("yaml", "failed to parse YAML", err)
	}

	inv := NewStaticInventory()

	// Add hosts
	for name, hostVars := range inventoryData.All.Hosts {
		host := types.Host{
			Name:      name,
			Variables: hostVars,
		}
		
		// Map Ansible variables to host fields
		if host.Variables == nil {
			host.Variables = make(map[string]interface{})
		}
		
		// Map ansible_host or address to Address
		if ansibleHost, ok := host.Variables["ansible_host"].(string); ok {
			host.Address = ansibleHost
		} else if address, ok := host.Variables["address"].(string); ok {
			host.Address = address
		}
		
		// Map ansible_user or user to User  
		if ansibleUser, ok := host.Variables["ansible_user"].(string); ok {
			host.User = ansibleUser
		} else if user, ok := host.Variables["user"].(string); ok {
			host.User = user
		}
		
		// Map ansible_password or password to Password
		if ansiblePassword, ok := host.Variables["ansible_password"].(string); ok {
			host.Password = ansiblePassword
		} else if password, ok := host.Variables["password"].(string); ok {
			host.Password = password
		}
		
		// Map ansible_port or port to Port
		if ansiblePort, ok := host.Variables["ansible_port"]; ok {
			switch v := ansiblePort.(type) {
			case int:
				host.Port = v
			case float64:
				host.Port = int(v)
			case string:
				// Try to parse string port
				fmt.Sscanf(v, "%d", &host.Port)
			}
		} else if port, ok := host.Variables["port"]; ok {
			switch v := port.(type) {
			case int:
				host.Port = v
			case float64:
				host.Port = int(v)
			case string:
				// Try to parse string port
				fmt.Sscanf(v, "%d", &host.Port)
			}
		}
		
		if err := inv.AddHost(host); err != nil {
			return nil, err
		}
	}

	// Add groups
	for name, group := range inventoryData.All.Children {
		group.Name = name
		if err := inv.AddGroup(group); err != nil {
			return nil, err
		}
	}

	// Create "all" group with all hosts
	allHostNames := make([]string, 0, len(inventoryData.All.Hosts))
	for name := range inventoryData.All.Hosts {
		allHostNames = append(allHostNames, name)
	}
	
	allGroup := types.Group{
		Name:      "all",
		Hosts:     allHostNames,
		Variables: inventoryData.All.Vars,
	}
	
	if err := inv.AddGroup(allGroup); err != nil {
		// Group might already exist, update it
		inv.mu.Lock()
		if existingGroup, exists := inv.groups["all"]; exists {
			existingGroup.Hosts = allHostNames
			if existingGroup.Variables == nil {
				existingGroup.Variables = make(map[string]interface{})
			}
			if inventoryData.All.Vars != nil {
				for k, v := range inventoryData.All.Vars {
					existingGroup.Variables[k] = v
				}
			}
			inv.groups["all"] = existingGroup
		}
		inv.mu.Unlock()
	}
	
	// Add "all" group to each host's group list
	for name := range inventoryData.All.Hosts {
		inv.mu.Lock()
		if host, exists := inv.hosts[name]; exists {
			if !contains(host.Groups, "all") {
				host.Groups = append(host.Groups, "all")
				inv.hosts[name] = host
			}
		}
		inv.mu.Unlock()
	}

	return inv, nil
}

// GetHosts returns all hosts matching the pattern
func (inv *StaticInventory) GetHosts(pattern string) ([]types.Host, error) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	var result []types.Host

	// If pattern is empty or "*", return all hosts
	if pattern == "" || pattern == "*" {
		for _, host := range inv.hosts {
			result = append(result, host)
		}
		return result, nil
	}

	// Parse pattern to separate hosts and groups
	hostPatterns, groupPatterns := types.ParseHostPattern(pattern)

	// Collect hosts matching host patterns
	hostSet := make(map[string]bool)
	for _, hostPattern := range hostPatterns {
		for name, host := range inv.hosts {
			if types.MatchPattern(hostPattern, name) || types.MatchPattern(hostPattern, host.Address) {
				if !hostSet[name] {
					result = append(result, host)
					hostSet[name] = true
				}
			}
		}
	}

	// Collect hosts from matching groups
	for _, groupPattern := range groupPatterns {
		for groupName, group := range inv.groups {
			if types.MatchPattern(groupPattern, groupName) {
				for _, hostname := range group.Hosts {
					if host, exists := inv.hosts[hostname]; exists && !hostSet[hostname] {
						result = append(result, host)
						hostSet[hostname] = true
					}
				}
				
				// Also check child groups recursively
				childHosts, err := inv.getHostsFromChildGroups(group, hostSet)
				if err != nil {
					return nil, err
				}
				result = append(result, childHosts...)
			}
		}
	}

	return result, nil
}

// getHostsFromChildGroups recursively gets hosts from child groups
func (inv *StaticInventory) getHostsFromChildGroups(group types.Group, hostSet map[string]bool) ([]types.Host, error) {
	var result []types.Host

	for _, childGroupName := range group.Children {
		if childGroup, exists := inv.groups[childGroupName]; exists {
			// Add direct hosts from child group
			for _, hostname := range childGroup.Hosts {
				if host, exists := inv.hosts[hostname]; exists && !hostSet[hostname] {
					result = append(result, host)
					hostSet[hostname] = true
				}
			}

			// Recursively check grandchild groups
			grandchildHosts, err := inv.getHostsFromChildGroups(childGroup, hostSet)
			if err != nil {
				return nil, err
			}
			result = append(result, grandchildHosts...)
		}
	}

	return result, nil
}

// GetHost returns a specific host by name
func (inv *StaticInventory) GetHost(name string) (*types.Host, error) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	if host, exists := inv.hosts[name]; exists {
		return &host, nil
	}

	// Also try to match by address
	for _, host := range inv.hosts {
		if host.Address == name {
			return &host, nil
		}
	}

	return nil, types.ErrHostNotFound
}

// GetGroup returns a specific group by name
func (inv *StaticInventory) GetGroup(name string) (*types.Group, error) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	if group, exists := inv.groups[name]; exists {
		return &group, nil
	}

	return nil, types.ErrGroupNotFound
}

// GetGroups returns all groups
func (inv *StaticInventory) GetGroups() ([]types.Group, error) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	result := make([]types.Group, 0, len(inv.groups))
	for _, group := range inv.groups {
		result = append(result, group)
	}

	return result, nil
}

// AddHost adds a host to the inventory
func (inv *StaticInventory) AddHost(host types.Host) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if host.Name == "" {
		return types.NewValidationError("name", "", "host name cannot be empty")
	}

	// Set default values
	if host.Variables == nil {
		host.Variables = make(map[string]interface{})
	}
	if host.Groups == nil {
		host.Groups = make([]string, 0)
	}
	if host.Address == "" {
		host.Address = host.Name
	}
	if host.Port == 0 {
		host.Port = 22 // Default SSH port
	}

	inv.hosts[host.Name] = host

	// Add host to specified groups
	for _, groupName := range host.Groups {
		if group, exists := inv.groups[groupName]; exists {
			if !types.StringSliceContains(group.Hosts, host.Name) {
				group.Hosts = append(group.Hosts, host.Name)
				inv.groups[groupName] = group
			}
		} else {
			// Create group if it doesn't exist
			newGroup := types.Group{
				Name:      groupName,
				Hosts:     []string{host.Name},
				Variables: make(map[string]interface{}),
			}
			inv.groups[groupName] = newGroup
		}
	}

	return nil
}

// AddGroup adds a group to the inventory
func (inv *StaticInventory) AddGroup(group types.Group) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if group.Name == "" {
		return types.NewValidationError("name", "", "group name cannot be empty")
	}

	// Set default values
	if group.Variables == nil {
		group.Variables = make(map[string]interface{})
	}
	if group.Hosts == nil {
		group.Hosts = make([]string, 0)
	}
	if group.Children == nil {
		group.Children = make([]string, 0)
	}

	inv.groups[group.Name] = group
	return nil
}

// GetHostVars returns variables for a specific host
func (inv *StaticInventory) GetHostVars(hostname string) (map[string]interface{}, error) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	host, exists := inv.hosts[hostname]
	if !exists {
		return nil, types.ErrHostNotFound
	}

	// Start with group variables (lower precedence)
	result := make(map[string]interface{})
	for _, groupName := range host.Groups {
		if group, exists := inv.groups[groupName]; exists {
			result = types.DeepMergeInterfaceMaps(result, group.Variables)
		}
	}

	// Merge host variables (higher precedence)
	result = types.DeepMergeInterfaceMaps(result, host.Variables)

	// Add built-in variables
	result["inventory_hostname"] = host.Name
	result["inventory_hostname_short"] = strings.Split(host.Name, ".")[0]
	result["ansible_host"] = host.Address
	result["ansible_port"] = host.Port
	if host.User != "" {
		result["ansible_user"] = host.User
	}

	return result, nil
}

// GetGroupVars returns variables for a specific group
func (inv *StaticInventory) GetGroupVars(groupname string) (map[string]interface{}, error) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	group, exists := inv.groups[groupname]
	if !exists {
		return nil, types.ErrGroupNotFound
	}

	// Start with group variables
	result := make(map[string]interface{})
	for k, v := range group.Variables {
		result[k] = v
	}

	return result, nil
}

// RemoveHost removes a host from the inventory
func (inv *StaticInventory) RemoveHost(hostname string) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	host, exists := inv.hosts[hostname]
	if !exists {
		return types.ErrHostNotFound
	}

	// Remove host from all groups
	for _, groupName := range host.Groups {
		if group, exists := inv.groups[groupName]; exists {
			newHosts := make([]string, 0, len(group.Hosts))
			for _, h := range group.Hosts {
				if h != hostname {
					newHosts = append(newHosts, h)
				}
			}
			group.Hosts = newHosts
			inv.groups[groupName] = group
		}
	}

	delete(inv.hosts, hostname)
	return nil
}

// RemoveGroup removes a group from the inventory
func (inv *StaticInventory) RemoveGroup(groupname string) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if _, exists := inv.groups[groupname]; !exists {
		return types.ErrGroupNotFound
	}

	// Remove group from all hosts
	for hostname, host := range inv.hosts {
		newGroups := make([]string, 0, len(host.Groups))
		for _, g := range host.Groups {
			if g != groupname {
				newGroups = append(newGroups, g)
			}
		}
		host.Groups = newGroups
		inv.hosts[hostname] = host
	}

	// Remove group from children of other groups
	for gname, group := range inv.groups {
		if gname == groupname {
			continue
		}
		newChildren := make([]string, 0, len(group.Children))
		for _, child := range group.Children {
			if child != groupname {
				newChildren = append(newChildren, child)
			}
		}
		group.Children = newChildren
		inv.groups[gname] = group
	}

	delete(inv.groups, groupname)
	return nil
}

// ToYAML exports the inventory to YAML format
func (inv *StaticInventory) ToYAML() ([]byte, error) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	data := InventoryData{}
	data.All.Hosts = make(map[string]map[string]interface{})
	data.All.Children = make(map[string]types.Group)

	// Export hosts
	for name, host := range inv.hosts {
		hostVars := make(map[string]interface{})
		
		// Add ansible variables
		if host.Address != "" && host.Address != name {
			hostVars["ansible_host"] = host.Address
		}
		if host.User != "" {
			hostVars["ansible_user"] = host.User
		}
		if host.Password != "" {
			hostVars["ansible_password"] = host.Password
		}
		if host.Port != 0 && host.Port != 22 {
			hostVars["ansible_port"] = host.Port
		}
		
		// Add other variables
		for k, v := range host.Variables {
			hostVars[k] = v
		}
		
		data.All.Hosts[name] = hostVars
	}

	// Export groups
	for name, group := range inv.groups {
		data.All.Children[name] = group
	}

	return yaml.Marshal(data)
}

// SaveToFile saves the inventory to a YAML file
func (inv *StaticInventory) SaveToFile(filePath string) error {
	data, err := inv.ToYAML()
	if err != nil {
		return types.NewInventoryError(filePath, "failed to serialize inventory", err)
	}

	// Create directory if it doesn't exist
	if dir := filepath.Dir(filePath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return types.NewInventoryError(filePath, "failed to create directory", err)
		}
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return types.NewInventoryError(filePath, "failed to write file", err)
	}

	return nil
}

// ExpandPattern expands inventory patterns like "web[1:5].example.com"
func (inv *StaticInventory) ExpandPattern(pattern string) ([]string, error) {
	// Handle range patterns like "web[1:5].example.com" or "db[01:10].local"
	rangeRegex := regexp.MustCompile(`^(.+)\[(\d+):(\d+)\](.*)$`)
	if matches := rangeRegex.FindStringSubmatch(pattern); matches != nil {
		prefix := matches[1]
		startStr := matches[2]
		endStr := matches[3]
		suffix := matches[4]

		start, err := types.ConvertToInt(startStr)
		if err != nil {
			return nil, fmt.Errorf("invalid range start: %s", startStr)
		}

		end, err := types.ConvertToInt(endStr)
		if err != nil {
			return nil, fmt.Errorf("invalid range end: %s", endStr)
		}

		if start > end {
			return nil, fmt.Errorf("range start (%d) cannot be greater than end (%d)", start, end)
		}

		var result []string
		leadingZeros := len(startStr) > 1 && strings.HasPrefix(startStr, "0")
		width := len(startStr)

		for i := start; i <= end; i++ {
			var hostname string
			if leadingZeros {
				hostname = fmt.Sprintf("%s%0*d%s", prefix, width, i, suffix)
			} else {
				hostname = fmt.Sprintf("%s%d%s", prefix, i, suffix)
			}
			result = append(result, hostname)
		}

		return result, nil
	}

	// Handle list patterns like "web{1,2,3}.example.com"
	listRegex := regexp.MustCompile(`^(.+)\{([^}]+)\}(.*)$`)
	if matches := listRegex.FindStringSubmatch(pattern); matches != nil {
		prefix := matches[1]
		listStr := matches[2]
		suffix := matches[3]

		items := strings.Split(listStr, ",")
		var result []string
		for _, item := range items {
			hostname := fmt.Sprintf("%s%s%s", prefix, strings.TrimSpace(item), suffix)
			result = append(result, hostname)
		}

		return result, nil
	}

	// Single pattern - return as-is
	return []string{pattern}, nil
}