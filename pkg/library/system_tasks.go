package library

import (
	"github.com/liliang-cn/gosinble/pkg/types"
)

// SystemTasks provides common system configuration operations
type SystemTasks struct{}

// NewSystemTasks creates a new SystemTasks instance
func NewSystemTasks() *SystemTasks {
	return &SystemTasks{}
}

// SetSysctl creates tasks to set a sysctl parameter
func (st *SystemTasks) SetSysctl(name, value string, persistent bool) []types.Task {
	return []types.Task{
		{
			Name:   "Set sysctl parameter",
			Module: "sysctl",
			Args: map[string]interface{}{
				"name":       name,
				"value":      value,
				"state":      "present",
				"persistent": persistent,
			},
		},
	}
}

// EnableIPForwarding creates tasks to enable IP forwarding
func (st *SystemTasks) EnableIPForwarding(persistent bool) []types.Task {
	return []types.Task{
		{
			Name:   "Enable IPv4 forwarding",
			Module: "sysctl",
			Args: map[string]interface{}{
				"name":       "net.ipv4.ip_forward",
				"value":      "1",
				"state":      "present",
				"persistent": persistent,
			},
		},
		{
			Name:   "Enable IPv6 forwarding",
			Module: "sysctl",
			Args: map[string]interface{}{
				"name":       "net.ipv6.conf.all.forwarding",
				"value":      "1",
				"state":      "present",
				"persistent": persistent,
			},
		},
	}
}

// MountFilesystem creates tasks to mount a filesystem
func (st *SystemTasks) MountFilesystem(src, path, fstype string, opts []string) []types.Task {
	args := map[string]interface{}{
		"path":   path,
		"src":    src,
		"fstype": fstype,
		"state":  "mounted",
	}
	
	if len(opts) > 0 {
		args["opts"] = opts
	}
	
	return []types.Task{
		{
			Name:   "Mount filesystem",
			Module: "mount",
			Args:   args,
		},
	}
}

// UnmountFilesystem creates tasks to unmount a filesystem
func (st *SystemTasks) UnmountFilesystem(path string) []types.Task {
	return []types.Task{
		{
			Name:   "Unmount filesystem",
			Module: "mount",
			Args: map[string]interface{}{
				"path":  path,
				"state": "unmounted",
			},
		},
	}
}

// AddFirewallRule creates tasks to add an iptables rule
func (st *SystemTasks) AddFirewallRule(chain, protocol, dport, jump string) []types.Task {
	return []types.Task{
		{
			Name:   "Add firewall rule",
			Module: "iptables",
			Args: map[string]interface{}{
				"chain":    chain,
				"protocol": protocol,
				"dport":    dport,
				"jump":     jump,
				"state":    "present",
			},
		},
	}
}

// AllowSSH creates tasks to allow SSH traffic
func (st *SystemTasks) AllowSSH() []types.Task {
	return []types.Task{
		{
			Name:   "Allow SSH on port 22",
			Module: "iptables",
			Args: map[string]interface{}{
				"chain":    "INPUT",
				"protocol": "tcp",
				"dport":    "22",
				"jump":     "ACCEPT",
				"state":    "present",
			},
		},
	}
}

// AllowHTTP creates tasks to allow HTTP and HTTPS traffic
func (st *SystemTasks) AllowHTTP() []types.Task {
	return []types.Task{
		{
			Name:   "Allow HTTP on port 80",
			Module: "iptables",
			Args: map[string]interface{}{
				"chain":    "INPUT",
				"protocol": "tcp",
				"dport":    "80",
				"jump":     "ACCEPT",
				"state":    "present",
			},
		},
		{
			Name:   "Allow HTTPS on port 443",
			Module: "iptables",
			Args: map[string]interface{}{
				"chain":    "INPUT",
				"protocol": "tcp",
				"dport":    "443",
				"jump":     "ACCEPT",
				"state":    "present",
			},
		},
	}
}

// BlockIP creates tasks to block traffic from a specific IP
func (st *SystemTasks) BlockIP(ip string) []types.Task {
	return []types.Task{
		{
			Name:   "Block traffic from IP",
			Module: "iptables",
			Args: map[string]interface{}{
				"chain":  "INPUT",
				"source": ip,
				"jump":   "DROP",
				"state":  "present",
			},
		},
	}
}

// EnableNAT creates tasks to enable NAT/masquerading
func (st *SystemTasks) EnableNAT(iface string) []types.Task {
	return []types.Task{
		{
			Name:   "Enable IP forwarding",
			Module: "sysctl",
			Args: map[string]interface{}{
				"name":       "net.ipv4.ip_forward",
				"value":      "1",
				"state":      "present",
				"persistent": true,
			},
		},
		{
			Name:   "Enable NAT masquerading",
			Module: "iptables",
			Args: map[string]interface{}{
				"table":     "nat",
				"chain":     "POSTROUTING",
				"out_iface": iface,
				"jump":      "MASQUERADE",
				"state":     "present",
			},
		},
	}
}

// SetupSwap creates tasks to setup swap space
func (st *SystemTasks) SetupSwap(path, size string) []types.Task {
	return []types.Task{
		{
			Name:   "Create swap file",
			Module: "command",
			Args: map[string]interface{}{
				"cmd":     "fallocate -l " + size + " " + path,
				"creates": path,
			},
		},
		{
			Name:   "Set swap file permissions",
			Module: "file",
			Args: map[string]interface{}{
				"path": path,
				"mode": "0600",
			},
		},
		{
			Name:   "Make swap",
			Module: "command",
			Args: map[string]interface{}{
				"cmd": "mkswap " + path,
			},
		},
		{
			Name:   "Enable swap",
			Module: "command",
			Args: map[string]interface{}{
				"cmd": "swapon " + path,
			},
		},
		{
			Name:   "Add swap to fstab",
			Module: "mount",
			Args: map[string]interface{}{
				"src":    path,
				"path":   "none",
				"fstype": "swap",
				"opts":   []string{"sw"},
				"state":  "present",
			},
		},
	}
}