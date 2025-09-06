// Package vars provides variable management and fact gathering functionality.
package vars

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gosinble/gosinble/pkg/types"
)

// VarManager implements variable management with proper precedence
type VarManager struct {
	mu        sync.RWMutex
	variables map[string]interface{}
	facts     map[string]interface{}
}

// NewVarManager creates a new variable manager
func NewVarManager() *VarManager {
	return &VarManager{
		variables: make(map[string]interface{}),
		facts:     make(map[string]interface{}),
	}
}

// SetVar sets a variable
func (vm *VarManager) SetVar(key string, value interface{}) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.variables[key] = value
}

// GetVar gets a variable
func (vm *VarManager) GetVar(key string) (interface{}, bool) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	
	// Check variables first
	if value, exists := vm.variables[key]; exists {
		return value, true
	}
	
	// Then check facts
	if value, exists := vm.facts[key]; exists {
		return value, true
	}
	
	return nil, false
}

// SetVars sets multiple variables
func (vm *VarManager) SetVars(vars map[string]interface{}) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	for k, v := range vars {
		vm.variables[k] = v
	}
}

// GetVars returns all variables (including facts)
func (vm *VarManager) GetVars() map[string]interface{} {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	
	result := make(map[string]interface{})
	
	// Add facts first (lower precedence)
	for k, v := range vm.facts {
		result[k] = v
	}
	
	// Add variables (higher precedence)
	for k, v := range vm.variables {
		result[k] = v
	}
	
	return result
}

// GatherFacts collects system facts from a host
func (vm *VarManager) GatherFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})
	
	// Gather system information
	systemFacts, err := vm.gatherSystemFacts(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("failed to gather system facts: %w", err)
	}
	vm.mergeMaps(facts, systemFacts)
	
	// Gather network information
	networkFacts, err := vm.gatherNetworkFacts(ctx, conn)
	if err != nil {
		// Don't fail completely if network facts fail
		fmt.Printf("Warning: failed to gather network facts: %v\n", err)
	} else {
		vm.mergeMaps(facts, networkFacts)
	}
	
	// Gather hardware information
	hardwareFacts, err := vm.gatherHardwareFacts(ctx, conn)
	if err != nil {
		// Don't fail completely if hardware facts fail
		fmt.Printf("Warning: failed to gather hardware facts: %v\n", err)
	} else {
		vm.mergeMaps(facts, hardwareFacts)
	}
	
	// Gather environment information
	envFacts, err := vm.gatherEnvironmentFacts(ctx, conn)
	if err != nil {
		// Don't fail completely if environment facts fail
		fmt.Printf("Warning: failed to gather environment facts: %v\n", err)
	} else {
		vm.mergeMaps(facts, envFacts)
	}
	
	// Store facts in the manager
	vm.mu.Lock()
	vm.facts = facts
	vm.mu.Unlock()
	
	return facts, nil
}

// MergeVars merges variables with proper precedence
func (vm *VarManager) MergeVars(base, override map[string]interface{}) map[string]interface{} {
	return types.DeepMergeInterfaceMaps(base, override)
}

// gatherSystemFacts gathers basic system facts
func (vm *VarManager) gatherSystemFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})
	
	// Get hostname
	if result, err := conn.Execute(ctx, "hostname", types.ExecuteOptions{}); err == nil && result.Success {
		hostname := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_hostname"] = hostname
		facts["ansible_nodename"] = hostname
		facts["inventory_hostname"] = hostname
		
		// Extract short hostname
		if parts := strings.Split(hostname, "."); len(parts) > 0 {
			facts["inventory_hostname_short"] = parts[0]
		}
	}
	
	// Get FQDN
	if result, err := conn.Execute(ctx, "hostname -f 2>/dev/null || hostname", types.ExecuteOptions{}); err == nil && result.Success {
		fqdn := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_fqdn"] = fqdn
	}
	
	// Get system information
	if result, err := conn.Execute(ctx, "uname -s", types.ExecuteOptions{}); err == nil && result.Success {
		system := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_system"] = system
	}
	
	if result, err := conn.Execute(ctx, "uname -r", types.ExecuteOptions{}); err == nil && result.Success {
		kernel := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_kernel"] = kernel
	}
	
	if result, err := conn.Execute(ctx, "uname -m", types.ExecuteOptions{}); err == nil && result.Success {
		arch := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_architecture"] = arch
		facts["ansible_machine"] = arch
	}
	
	// Get OS distribution information
	if distFacts, err := vm.getDistributionFacts(ctx, conn); err == nil {
		vm.mergeMaps(facts, distFacts)
	}
	
	return facts, nil
}

// gatherNetworkFacts gathers network-related facts
func (vm *VarManager) gatherNetworkFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})
	
	// Get default IPv4 information
	if result, err := conn.Execute(ctx, "ip route get 1.1.1.1 2>/dev/null | head -1", types.ExecuteOptions{}); err == nil && result.Success {
		routeInfo := strings.TrimSpace(result.Data["stdout"].(string))
		if routeInfo != "" {
			defaultIPv4 := vm.parseDefaultRoute(routeInfo)
			if len(defaultIPv4) > 0 {
				facts["ansible_default_ipv4"] = defaultIPv4
			}
		}
	}
	
	// Get network interfaces
	if result, err := conn.Execute(ctx, "ip -o link show | awk -F': ' '{print $2}' | grep -v lo", types.ExecuteOptions{}); err == nil && result.Success {
		interfacesStr := strings.TrimSpace(result.Data["stdout"].(string))
		if interfacesStr != "" {
			interfaces := strings.Split(interfacesStr, "\n")
			facts["ansible_interfaces"] = interfaces
			
			// Get information for each interface
			for _, iface := range interfaces {
				iface = strings.TrimSpace(iface)
				if iface == "" {
					continue
				}
				
				if ifaceInfo, err := vm.getInterfaceInfo(ctx, conn, iface); err == nil && len(ifaceInfo) > 0 {
					facts[fmt.Sprintf("ansible_%s", iface)] = ifaceInfo
				}
			}
		}
	}
	
	return facts, nil
}

// gatherHardwareFacts gathers hardware-related facts
func (vm *VarManager) gatherHardwareFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})
	
	// Get CPU information
	if result, err := conn.Execute(ctx, "nproc 2>/dev/null || echo 1", types.ExecuteOptions{}); err == nil && result.Success {
		cpuCountStr := strings.TrimSpace(result.Data["stdout"].(string))
		if cpuCount, err := types.ConvertToInt(cpuCountStr); err == nil {
			facts["ansible_processor_vcpus"] = cpuCount
			facts["ansible_processor_count"] = cpuCount
		}
	}
	
	// Get CPU details
	if result, err := conn.Execute(ctx, "cat /proc/cpuinfo 2>/dev/null | grep 'model name' | head -1 | cut -d: -f2 | xargs", types.ExecuteOptions{}); err == nil && result.Success {
		cpuModel := strings.TrimSpace(result.Data["stdout"].(string))
		if cpuModel != "" {
			facts["ansible_processor"] = []string{cpuModel}
		}
	}
	
	// Get memory information
	if memFacts, err := vm.getMemoryFacts(ctx, conn); err == nil {
		vm.mergeMaps(facts, memFacts)
	}
	
	// Get disk/mount information
	if result, err := conn.Execute(ctx, "df -P", types.ExecuteOptions{}); err == nil && result.Success {
		mounts := vm.parseMounts(result.Data["stdout"].(string))
		if len(mounts) > 0 {
			facts["ansible_mounts"] = mounts
		}
	}
	
	return facts, nil
}

// gatherEnvironmentFacts gathers environment-related facts
func (vm *VarManager) gatherEnvironmentFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})
	
	// Get current user
	if result, err := conn.Execute(ctx, "whoami", types.ExecuteOptions{}); err == nil && result.Success {
		user := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_user_id"] = user
		facts["ansible_user"] = user
	}
	
	// Get user information
	if result, err := conn.Execute(ctx, "id", types.ExecuteOptions{}); err == nil && result.Success {
		idOutput := strings.TrimSpace(result.Data["stdout"].(string))
		if userInfo := vm.parseIdOutput(idOutput); len(userInfo) > 0 {
			vm.mergeMaps(facts, userInfo)
		}
	}
	
	// Get home directory
	if result, err := conn.Execute(ctx, "echo $HOME", types.ExecuteOptions{}); err == nil && result.Success {
		home := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_user_dir"] = home
	}
	
	// Get shell
	if result, err := conn.Execute(ctx, "echo $SHELL", types.ExecuteOptions{}); err == nil && result.Success {
		shell := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_user_shell"] = shell
	}
	
	// Get environment variables
	envVars := make(map[string]string)
	if result, err := conn.Execute(ctx, "echo $PATH", types.ExecuteOptions{}); err == nil && result.Success {
		path := strings.TrimSpace(result.Data["stdout"].(string))
		envVars["PATH"] = path
	}
	
	if len(envVars) > 0 {
		facts["ansible_env"] = envVars
	}
	
	return facts, nil
}

// getDistributionFacts gets OS distribution information
func (vm *VarManager) getDistributionFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})
	
	// Try /etc/os-release first (most modern systems)
	if result, err := conn.Execute(ctx, "cat /etc/os-release 2>/dev/null", types.ExecuteOptions{}); err == nil && result.Success {
		osRelease := result.Data["stdout"].(string)
		if distInfo := vm.parseOSRelease(osRelease); len(distInfo) > 0 {
			vm.mergeMaps(facts, distInfo)
			return facts, nil
		}
	}
	
	// Try other release files
	releaseFiles := []string{
		"/etc/redhat-release",
		"/etc/centos-release",
		"/etc/debian_version",
		"/etc/ubuntu-release",
	}
	
	for _, file := range releaseFiles {
		if result, err := conn.Execute(ctx, fmt.Sprintf("cat %s 2>/dev/null", file), types.ExecuteOptions{}); err == nil && result.Success {
			content := strings.TrimSpace(result.Data["stdout"].(string))
			if content != "" {
				facts["ansible_distribution"] = vm.guessDistributionFromFile(file, content)
				facts["ansible_distribution_version"] = vm.extractVersionFromContent(content)
				break
			}
		}
	}
	
	return facts, nil
}

// getMemoryFacts gets memory information
func (vm *VarManager) getMemoryFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})
	
	if result, err := conn.Execute(ctx, "cat /proc/meminfo 2>/dev/null", types.ExecuteOptions{}); err == nil && result.Success {
		memInfo := result.Data["stdout"].(string)
		
		// Parse memory information
		lines := strings.Split(memInfo, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			
			key := strings.TrimSuffix(parts[0], ":")
			valueStr := parts[1]
			
			if value, err := types.ConvertToInt(valueStr); err == nil {
				switch key {
				case "MemTotal":
					facts["ansible_memtotal_mb"] = value / 1024
				case "MemFree":
					facts["ansible_memfree_mb"] = value / 1024
				case "MemAvailable":
					facts["ansible_memavailable_mb"] = value / 1024
				case "SwapTotal":
					facts["ansible_swaptotal_mb"] = value / 1024
				case "SwapFree":
					facts["ansible_swapfree_mb"] = value / 1024
				}
			}
		}
	}
	
	return facts, nil
}

// getInterfaceInfo gets detailed information about a network interface
func (vm *VarManager) getInterfaceInfo(ctx context.Context, conn types.Connection, iface string) (map[string]interface{}, error) {
	info := make(map[string]interface{})
	
	// Get IP address information
	if result, err := conn.Execute(ctx, fmt.Sprintf("ip addr show %s 2>/dev/null", iface), types.ExecuteOptions{}); err == nil && result.Success {
		addrOutput := result.Data["stdout"].(string)
		
		// Check if interface is up
		info["active"] = strings.Contains(addrOutput, "state UP")
		
		// Parse IPv4 addresses
		if ipv4Info := vm.parseIPv4FromAddr(addrOutput); len(ipv4Info) > 0 {
			info["ipv4"] = ipv4Info
		}
		
		// Parse IPv6 addresses
		if ipv6Info := vm.parseIPv6FromAddr(addrOutput); len(ipv6Info) > 0 {
			info["ipv6"] = ipv6Info
		}
		
		// Parse MAC address
		if macAddr := vm.parseMACFromAddr(addrOutput); macAddr != "" {
			info["macaddress"] = macAddr
		}
	}
	
	return info, nil
}

// Helper functions for parsing command outputs

func (vm *VarManager) parseDefaultRoute(routeInfo string) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Example: "1.1.1.1 via 192.168.1.1 dev eth0 src 192.168.1.100 uid 1000"
	parts := strings.Fields(routeInfo)
	for i, part := range parts {
		if part == "src" && i+1 < len(parts) {
			result["address"] = parts[i+1]
		}
		if part == "dev" && i+1 < len(parts) {
			result["interface"] = parts[i+1]
		}
		if part == "via" && i+1 < len(parts) {
			result["gateway"] = parts[i+1]
		}
	}
	
	return result
}

func (vm *VarManager) parseOSRelease(content string) map[string]interface{} {
	result := make(map[string]interface{})
	
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "=") {
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := parts[0]
		value := strings.Trim(parts[1], "\"")
		
		switch key {
		case "NAME":
			result["ansible_distribution"] = value
		case "VERSION_ID":
			result["ansible_distribution_version"] = value
		case "VERSION_CODENAME":
			result["ansible_distribution_release"] = value
		case "ID":
			result["ansible_os_family"] = value
		}
	}
	
	return result
}

func (vm *VarManager) parseMounts(dfOutput string) []map[string]interface{} {
	var mounts []map[string]interface{}
	
	lines := strings.Split(dfOutput, "\n")
	if len(lines) < 2 {
		return mounts
	}
	
	// Skip header line
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		
		mount := map[string]interface{}{
			"device":         fields[0],
			"size_total":     fields[1],
			"size_used":      fields[2],
			"size_available": fields[3],
			"mount":          fields[5],
		}
		
		// Calculate percentage used
		if used, err := types.ConvertToInt(fields[2]); err == nil {
			if total, err := types.ConvertToInt(fields[1]); err == nil && total > 0 {
				percentage := (used * 100) / total
				mount["size_percent"] = percentage
			}
		}
		
		mounts = append(mounts, mount)
	}
	
	return mounts
}

func (vm *VarManager) parseIdOutput(idOutput string) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Example: "uid=1000(user) gid=1000(user) groups=1000(user),4(adm),24(cdrom)"
	parts := strings.Fields(idOutput)
	for _, part := range parts {
		if strings.HasPrefix(part, "uid=") {
			if uid := vm.extractNumberFromIDString(part); uid >= 0 {
				result["ansible_user_uid"] = uid
			}
		}
		if strings.HasPrefix(part, "gid=") {
			if gid := vm.extractNumberFromIDString(part); gid >= 0 {
				result["ansible_user_gid"] = gid
			}
		}
	}
	
	return result
}

func (vm *VarManager) parseIPv4FromAddr(addrOutput string) map[string]interface{} {
	lines := strings.Split(addrOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "inet ") && !strings.Contains(line, "127.0.0.1") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ipWithMask := parts[1]
				if strings.Contains(ipWithMask, "/") {
					ipParts := strings.Split(ipWithMask, "/")
					return map[string]interface{}{
						"address": ipParts[0],
						"netmask": vm.cidrToNetmask(ipParts[1]),
					}
				}
			}
		}
	}
	return nil
}

func (vm *VarManager) parseIPv6FromAddr(addrOutput string) []map[string]interface{} {
	var ipv6Addrs []map[string]interface{}
	
	lines := strings.Split(addrOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "inet6 ") && !strings.Contains(line, "::1") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ipWithMask := parts[1]
				if strings.Contains(ipWithMask, "/") {
					ipParts := strings.Split(ipWithMask, "/")
					ipv6Addrs = append(ipv6Addrs, map[string]interface{}{
						"address": ipParts[0],
						"prefix":  ipParts[1],
					})
				}
			}
		}
	}
	
	return ipv6Addrs
}

func (vm *VarManager) parseMACFromAddr(addrOutput string) string {
	lines := strings.Split(addrOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "link/ether") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "link/ether" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}
	return ""
}

// Helper utility functions

func (vm *VarManager) guessDistributionFromFile(filename, content string) string {
	switch filename {
	case "/etc/redhat-release":
		if strings.Contains(strings.ToLower(content), "centos") {
			return "CentOS"
		}
		return "RedHat"
	case "/etc/centos-release":
		return "CentOS"
	case "/etc/debian_version":
		return "Debian"
	case "/etc/ubuntu-release":
		return "Ubuntu"
	default:
		return "Unknown"
	}
}

func (vm *VarManager) extractVersionFromContent(content string) string {
	// Simple version extraction - look for patterns like "X.Y" or "X.Y.Z"
	parts := strings.Fields(content)
	for _, part := range parts {
		if strings.Contains(part, ".") {
			// Check if it looks like a version number
			versionParts := strings.Split(part, ".")
			if len(versionParts) >= 2 {
				return part
			}
		}
	}
	return ""
}

func (vm *VarManager) extractNumberFromIDString(idStr string) int {
	// Extract number from strings like "uid=1000(user)"
	if idx := strings.Index(idStr, "("); idx > 0 {
		numberStr := idStr[strings.Index(idStr, "=")+1 : idx]
		if number, err := types.ConvertToInt(numberStr); err == nil {
			return number
		}
	}
	return -1
}

func (vm *VarManager) cidrToNetmask(cidr string) string {
	// Simple CIDR to netmask conversion for common cases
	switch cidr {
	case "8":
		return "255.0.0.0"
	case "16":
		return "255.255.0.0"
	case "24":
		return "255.255.255.0"
	case "32":
		return "255.255.255.255"
	default:
		return cidr // Return CIDR if we can't convert
	}
}

func (vm *VarManager) mergeMaps(dest, src map[string]interface{}) {
	for k, v := range src {
		dest[k] = v
	}
}