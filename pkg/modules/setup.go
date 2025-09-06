package modules

import (
	"context"
	"fmt"
	"strings"

	"github.com/gosinble/gosinble/pkg/types"
)

// SetupModule implements the setup module for fact gathering
type SetupModule struct {
	*BaseModule
}

// NewSetupModule creates a new setup module
func NewSetupModule() *SetupModule {
	doc := types.ModuleDoc{
		Name:        "setup",
		Description: "Gather facts about remote hosts",
		Parameters: map[string]types.ParamDoc{
			"fact_path": {
				Description: "Path to additional facts directory",
				Required:    false,
				Type:        "string",
			},
			"filter": {
				Description: "If supplied, only return facts that match this shell-style glob",
				Required:    false,
				Type:        "string",
				Default:     "*",
			},
			"gather_subset": {
				Description: "If supplied, restrict the additional facts collected to the given subset",
				Required:    false,
				Type:        "slice",
				Default:     []string{"all"},
			},
			"gather_timeout": {
				Description: "Set the default timeout in seconds for individual fact gathering",
				Required:    false,
				Type:        "int",
				Default:     10,
			},
		},
		Examples: []string{
			`- name: Gather all facts
  setup:`,
			`- name: Gather only network facts
  setup:
    gather_subset:
      - network`,
			`- name: Filter facts by pattern
  setup:
    filter: ansible_*`,
		},
		Returns: map[string]string{
			"ansible_facts": "Dictionary containing all the facts that were gathered",
		},
	}

	return &SetupModule{
		BaseModule: NewBaseModule("setup", doc),
	}
}

// Validate validates the module arguments
func (m *SetupModule) Validate(args map[string]interface{}) error {
	// Validate field types
	fieldTypes := map[string]string{
		"fact_path":       "string",
		"filter":          "string",
		"gather_subset":   "slice",
		"gather_timeout":  "int",
	}
	return m.ValidateTypes(args, fieldTypes)
}

// Run executes the setup module
func (m *SetupModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	return m.ExecuteWithTiming(ctx, conn, args, func() (*types.Result, error) {
		host := m.GetHostFromConnection(conn)

		// Get parameters
		factPath := m.GetStringArg(args, "fact_path", "")
		filter := m.GetStringArg(args, "filter", "*")
		gatherSubset := m.GetSliceArg(args, "gather_subset")
		_, _ = m.GetIntArg(args, "gather_timeout", 10) // TODO: implement timeouts

		if gatherSubset == nil {
			gatherSubset = []interface{}{"all"}
		}

		// Check mode handling - setup module always runs to gather facts
		facts := make(map[string]interface{})

		// Gather basic system facts
		if m.shouldGatherSubset(gatherSubset, "hardware") || m.shouldGatherSubset(gatherSubset, "all") {
			hardwareFacts, err := m.gatherHardwareFacts(ctx, conn)
			if err != nil {
				m.LogWarn("Failed to gather hardware facts: %v", err)
			} else {
				m.mergeFacts(facts, hardwareFacts)
			}
		}

		// Gather network facts
		if m.shouldGatherSubset(gatherSubset, "network") || m.shouldGatherSubset(gatherSubset, "all") {
			networkFacts, err := m.gatherNetworkFacts(ctx, conn)
			if err != nil {
				m.LogWarn("Failed to gather network facts: %v", err)
			} else {
				m.mergeFacts(facts, networkFacts)
			}
		}

		// Gather OS facts
		if m.shouldGatherSubset(gatherSubset, "virtual") || m.shouldGatherSubset(gatherSubset, "all") {
			osFacts, err := m.gatherOSFacts(ctx, conn)
			if err != nil {
				m.LogWarn("Failed to gather OS facts: %v", err)
			} else {
				m.mergeFacts(facts, osFacts)
			}
		}

		// Gather environment facts
		if m.shouldGatherSubset(gatherSubset, "env") || m.shouldGatherSubset(gatherSubset, "all") {
			envFacts, err := m.gatherEnvironmentFacts(ctx, conn)
			if err != nil {
				m.LogWarn("Failed to gather environment facts: %v", err)
			} else {
				m.mergeFacts(facts, envFacts)
			}
		}

		// Gather custom facts from fact_path
		if factPath != "" {
			customFacts, err := m.gatherCustomFacts(ctx, conn, factPath)
			if err != nil {
				m.LogWarn("Failed to gather custom facts from %s: %v", factPath, err)
			} else {
				m.mergeFacts(facts, customFacts)
			}
		}

		// Filter facts based on pattern
		if filter != "*" && filter != "" {
			facts = m.filterFacts(facts, filter)
		}

		// Create result
		resultData := map[string]interface{}{
			"ansible_facts": facts,
		}

		return m.CreateSuccessResult(host, false, "Facts gathered successfully", resultData), nil
	})
}

// shouldGatherSubset checks if a subset should be gathered
func (m *SetupModule) shouldGatherSubset(gatherSubset []interface{}, subset string) bool {
	for _, s := range gatherSubset {
		if types.ConvertToString(s) == subset || types.ConvertToString(s) == "all" {
			return true
		}
	}
	return false
}

// gatherHardwareFacts gathers hardware-related facts
func (m *SetupModule) gatherHardwareFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})

	// Get CPU information
	if result, err := conn.Execute(ctx, "nproc", types.ExecuteOptions{}); err == nil && result.Success {
		if cpuCount := strings.TrimSpace(result.Data["stdout"].(string)); cpuCount != "" {
			if count, err := types.ConvertToInt(cpuCount); err == nil {
				facts["ansible_processor_vcpus"] = count
				facts["ansible_processor_count"] = count
			}
		}
	}

	// Get memory information
	if result, err := conn.Execute(ctx, "cat /proc/meminfo | head -2", types.ExecuteOptions{}); err == nil && result.Success {
		memInfo := strings.TrimSpace(result.Data["stdout"].(string))
		if memInfo != "" {
			facts["ansible_memtotal_mb"] = m.parseMemoryInfo(memInfo, "MemTotal")
			facts["ansible_memfree_mb"] = m.parseMemoryInfo(memInfo, "MemFree")
		}
	}

	// Get disk information
	if result, err := conn.Execute(ctx, "df -h /", types.ExecuteOptions{}); err == nil && result.Success {
		diskInfo := strings.TrimSpace(result.Data["stdout"].(string))
		if diskInfo != "" {
			facts["ansible_mounts"] = m.parseDiskInfo(diskInfo)
		}
	}

	return facts, nil
}

// gatherNetworkFacts gathers network-related facts
func (m *SetupModule) gatherNetworkFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})

	// Get hostname
	if result, err := conn.Execute(ctx, "hostname", types.ExecuteOptions{}); err == nil && result.Success {
		hostname := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_hostname"] = hostname
		facts["ansible_nodename"] = hostname
	}

	// Get FQDN
	if result, err := conn.Execute(ctx, "hostname -f", types.ExecuteOptions{}); err == nil && result.Success {
		fqdn := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_fqdn"] = fqdn
	}

	// Get default IPv4 address
	if result, err := conn.Execute(ctx, "ip route get 1.1.1.1 | head -1 | awk '{print $7}'", types.ExecuteOptions{}); err == nil && result.Success {
		defaultIP := strings.TrimSpace(result.Data["stdout"].(string))
		if defaultIP != "" {
			facts["ansible_default_ipv4"] = map[string]interface{}{
				"address": defaultIP,
			}
		}
	}

	// Get network interfaces
	if result, err := conn.Execute(ctx, "ip -o link show | awk -F': ' '{print $2}'", types.ExecuteOptions{}); err == nil && result.Success {
		interfaces := strings.Split(strings.TrimSpace(result.Data["stdout"].(string)), "\n")
		facts["ansible_interfaces"] = interfaces

		// Get details for each interface
		for _, iface := range interfaces {
			if strings.TrimSpace(iface) == "" {
				continue
			}
			if ifaceInfo, err := m.getInterfaceInfo(ctx, conn, iface); err == nil {
				facts[fmt.Sprintf("ansible_%s", iface)] = ifaceInfo
			}
		}
	}

	return facts, nil
}

// gatherOSFacts gathers operating system facts
func (m *SetupModule) gatherOSFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})

	// Get OS information from uname
	if result, err := conn.Execute(ctx, "uname -s", types.ExecuteOptions{}); err == nil && result.Success {
		osName := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_system"] = osName
	}

	if result, err := conn.Execute(ctx, "uname -r", types.ExecuteOptions{}); err == nil && result.Success {
		kernel := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_kernel"] = kernel
	}

	if result, err := conn.Execute(ctx, "uname -m", types.ExecuteOptions{}); err == nil && result.Success {
		arch := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_architecture"] = arch
	}

	// Get distribution information
	if result, err := conn.Execute(ctx, "cat /etc/os-release", types.ExecuteOptions{}); err == nil && result.Success {
		osRelease := result.Data["stdout"].(string)
		facts["ansible_distribution"] = m.parseOSRelease(osRelease, "NAME")
		facts["ansible_distribution_version"] = m.parseOSRelease(osRelease, "VERSION_ID")
		facts["ansible_distribution_release"] = m.parseOSRelease(osRelease, "VERSION_CODENAME")
	}

	// Get Python version (if available)
	if result, err := conn.Execute(ctx, "python3 --version 2>&1", types.ExecuteOptions{}); err == nil && result.Success {
		pythonVersion := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_python_version"] = pythonVersion
	}

	return facts, nil
}

// gatherEnvironmentFacts gathers environment-related facts
func (m *SetupModule) gatherEnvironmentFacts(ctx context.Context, conn types.Connection) (map[string]interface{}, error) {
	facts := make(map[string]interface{})

	// Get current user
	if result, err := conn.Execute(ctx, "whoami", types.ExecuteOptions{}); err == nil && result.Success {
		user := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_user_id"] = user
	}

	// Get user home directory
	if result, err := conn.Execute(ctx, "echo $HOME", types.ExecuteOptions{}); err == nil && result.Success {
		home := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_user_dir"] = home
	}

	// Get shell
	if result, err := conn.Execute(ctx, "echo $SHELL", types.ExecuteOptions{}); err == nil && result.Success {
		shell := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_user_shell"] = shell
	}

	// Get PATH
	if result, err := conn.Execute(ctx, "echo $PATH", types.ExecuteOptions{}); err == nil && result.Success {
		path := strings.TrimSpace(result.Data["stdout"].(string))
		facts["ansible_env"] = map[string]interface{}{
			"PATH": path,
		}
	}

	return facts, nil
}

// gatherCustomFacts gathers custom facts from specified directory
func (m *SetupModule) gatherCustomFacts(ctx context.Context, conn types.Connection, factPath string) (map[string]interface{}, error) {
	facts := make(map[string]interface{})

	// List files in fact directory
	result, err := conn.Execute(ctx, fmt.Sprintf("find %s -type f -executable 2>/dev/null", factPath), types.ExecuteOptions{})
	if err != nil || !result.Success {
		return facts, nil // No custom facts directory or no executable files
	}

	factFiles := strings.Split(strings.TrimSpace(result.Data["stdout"].(string)), "\n")
	for _, factFile := range factFiles {
		factFile = strings.TrimSpace(factFile)
		if factFile == "" {
			continue
		}

		// Execute custom fact script
		if result, err := conn.Execute(ctx, factFile, types.ExecuteOptions{}); err == nil && result.Success {
			factName := fmt.Sprintf("ansible_local_%s", strings.Replace(factFile, factPath+"/", "", 1))
			facts[factName] = strings.TrimSpace(result.Data["stdout"].(string))
		}
	}

	return facts, nil
}

// mergeFacts merges source facts into destination facts
func (m *SetupModule) mergeFacts(dest, src map[string]interface{}) {
	for k, v := range src {
		dest[k] = v
	}
}

// filterFacts filters facts based on pattern
func (m *SetupModule) filterFacts(facts map[string]interface{}, pattern string) map[string]interface{} {
	filtered := make(map[string]interface{})
	
	for key, value := range facts {
		if types.MatchPattern(pattern, key) {
			filtered[key] = value
		}
	}
	
	return filtered
}

// parseMemoryInfo parses memory information from /proc/meminfo
func (m *SetupModule) parseMemoryInfo(memInfo, field string) int {
	lines := strings.Split(memInfo, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, field) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if kb, err := types.ConvertToInt(parts[1]); err == nil {
					return kb / 1024 // Convert KB to MB
				}
			}
		}
	}
	return 0
}

// parseDiskInfo parses disk information from df command
func (m *SetupModule) parseDiskInfo(diskInfo string) []map[string]interface{} {
	var mounts []map[string]interface{}
	
	lines := strings.Split(diskInfo, "\n")
	if len(lines) < 2 {
		return mounts
	}
	
	// Skip header line
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			mount := map[string]interface{}{
				"device":     fields[0],
				"size_total": fields[1],
				"size_used":  fields[2],
				"size_available": fields[3],
				"mount":      fields[5],
			}
			mounts = append(mounts, mount)
		}
	}
	
	return mounts
}

// parseOSRelease parses a field from /etc/os-release
func (m *SetupModule) parseOSRelease(content, field string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+"=") {
			value := strings.TrimPrefix(line, field+"=")
			value = strings.Trim(value, "\"")
			return value
		}
	}
	return ""
}

// getInterfaceInfo gets detailed information about a network interface
func (m *SetupModule) getInterfaceInfo(ctx context.Context, conn types.Connection, iface string) (map[string]interface{}, error) {
	info := make(map[string]interface{})
	
	// Get IP addresses
	if result, err := conn.Execute(ctx, fmt.Sprintf("ip addr show %s", iface), types.ExecuteOptions{}); err == nil && result.Success {
		output := result.Data["stdout"].(string)
		
		// Parse IPv4 addresses
		if strings.Contains(output, "inet ") {
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "inet ") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						ipWithMask := fields[1]
						if strings.Contains(ipWithMask, "/") {
							ip := strings.Split(ipWithMask, "/")[0]
							info["ipv4"] = map[string]interface{}{
								"address": ip,
							}
						}
					}
				}
			}
		}
		
		// Check if interface is up
		info["active"] = strings.Contains(output, "state UP")
	}
	
	return info, nil
}