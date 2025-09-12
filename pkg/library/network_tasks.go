package library

import (
	"fmt"
	"github.com/liliang-cn/gosible/pkg/types"
)

// NetworkTasks provides common network and firewall operations
type NetworkTasks struct{}

// NewNetworkTasks creates a new NetworkTasks instance
func NewNetworkTasks() *NetworkTasks {
	return &NetworkTasks{}
}

// ConfigureFirewall creates tasks to manage firewall rules
func (nt *NetworkTasks) ConfigureFirewall(port int, protocol, action string) []types.Task {
	return []types.Task{
		{
			Name:   "Install UFW (Ubuntu)",
			Module: "apt",
			Args: map[string]interface{}{
				"name":  "ufw",
				"state": "present",
			},
			When: "ansible_distribution == 'Ubuntu'",
		},
		{
			Name:   "Configure UFW rule",
			Module: "ufw",
			Args: map[string]interface{}{
				"rule":  action,
				"port":  port,
				"proto": protocol,
			},
			When: "ansible_distribution == 'Ubuntu'",
		},
		{
			Name:   "Configure firewalld rule",
			Module: "firewalld",
			Args: map[string]interface{}{
				"port":      fmt.Sprintf("%d/%s", port, protocol),
				"permanent": true,
				"state":     "enabled",
				"immediate": true,
			},
			When: "ansible_os_family == 'RedHat'",
		},
		{
			Name:   "Configure iptables rule",
			Module: "iptables",
			Args: map[string]interface{}{
				"chain":    "INPUT",
				"protocol": protocol,
				"destination_port": port,
				"jump":     "ACCEPT",
				"state":    "present",
			},
			When: "ansible_os_family == 'Debian' and ansible_distribution != 'Ubuntu'",
		},
	}
}

// SetupSSHSecurity hardens SSH configuration
func (nt *NetworkTasks) SetupSSHSecurity(permitRoot bool, passwordAuth bool, port int) []types.Task {
	return []types.Task{
		{
			Name:   "Backup SSH config",
			Module: "copy",
			Args: map[string]interface{}{
				"src":  "/etc/ssh/sshd_config",
				"dest": "/etc/ssh/sshd_config.bak",
				"remote_src": true,
			},
		},
		{
			Name:   "Disable root login",
			Module: "lineinfile",
			Args: map[string]interface{}{
				"path":   "/etc/ssh/sshd_config",
				"regexp": "^#?PermitRootLogin",
				"line":   "PermitRootLogin " + map[bool]string{true: "yes", false: "no"}[permitRoot],
			},
			Notify: []string{"restart sshd"},
		},
		{
			Name:   "Configure password authentication",
			Module: "lineinfile",
			Args: map[string]interface{}{
				"path":   "/etc/ssh/sshd_config",
				"regexp": "^#?PasswordAuthentication",
				"line":   "PasswordAuthentication " + map[bool]string{true: "yes", false: "no"}[passwordAuth],
			},
			Notify: []string{"restart sshd"},
		},
		{
			Name:   "Set SSH port",
			Module: "lineinfile",
			Args: map[string]interface{}{
				"path":   "/etc/ssh/sshd_config",
				"regexp": "^#?Port",
				"line":   fmt.Sprintf("Port %d", port),
			},
			Notify: []string{"restart sshd"},
			When: "port != 22",
		},
		{
			Name:   "Enable SSH service",
			Module: "service",
			Args: map[string]interface{}{
				"name":    "sshd",
				"state":   "started",
				"enabled": true,
			},
		},
	}
}

// ConfigureNetworkInterface sets up a network interface
func (nt *NetworkTasks) ConfigureNetworkInterface(iface, ipaddr, netmask, gateway string) []types.Task {
	return []types.Task{
		// For Ubuntu/Debian with netplan
		{
			Name:   "Configure network with netplan",
			Module: "template",
			Args: map[string]interface{}{
				"dest": "/etc/netplan/01-netcfg.yaml",
				"content": `network:
  version: 2
  ethernets:
    ` + iface + `:
      addresses: [` + ipaddr + `/` + netmask + `]
      gateway4: ` + gateway + `
      nameservers:
        addresses: [8.8.8.8, 8.8.4.4]`,
			},
			When: "ansible_distribution == 'Ubuntu' and ansible_distribution_major_version | int >= 18",
			Notify: []string{"apply netplan"},
		},
		// For older systems or RedHat
		{
			Name:   "Configure network interface (traditional)",
			Module: "nmcli",
			Args: map[string]interface{}{
				"conn_name": iface,
				"ifname":    iface,
				"type":      "ethernet",
				"ip4":       ipaddr + "/" + netmask,
				"gw4":       gateway,
				"state":     "present",
			},
			When: "ansible_os_family == 'RedHat'",
		},
	}
}

// SetupDNS configures DNS settings
func (nt *NetworkTasks) SetupDNS(nameservers []string, searchDomains []string) []types.Task {
	resolvContent := ""
	for _, ns := range nameservers {
		resolvContent += "nameserver " + ns + "\n"
	}
	if len(searchDomains) > 0 {
		resolvContent += "search"
		for _, domain := range searchDomains {
			resolvContent += " " + domain
		}
		resolvContent += "\n"
	}
	
	return []types.Task{
		{
			Name:   "Configure DNS resolvers",
			Module: "copy",
			Args: map[string]interface{}{
				"content": resolvContent,
				"dest":    "/etc/resolv.conf",
				"backup":  true,
			},
		},
		{
			Name:   "Prevent resolv.conf from being overwritten",
			Module: "file",
			Args: map[string]interface{}{
				"path":       "/etc/resolv.conf",
				"attributes": "+i",
			},
			When: "lock_resolv_conf | default(false)",
		},
	}
}

// CheckConnectivity creates tasks to verify network connectivity
func (nt *NetworkTasks) CheckConnectivity(hosts []string) []types.Task {
	tasks := []types.Task{}
	
	for _, host := range hosts {
		tasks = append(tasks, types.Task{
			Name:   "Check connectivity to " + host,
			Module: "wait_for",
			Args: map[string]interface{}{
				"host":    host,
				"port":    443,
				"timeout": 10,
			},
			Register: "connectivity_" + host,
		})
	}
	
	return tasks
}

// SetupVPN creates tasks to configure OpenVPN client
func (nt *NetworkTasks) SetupVPN(configFile, authFile string) []types.Task {
	return []types.Task{
		{
			Name:   "Install OpenVPN",
			Module: "package",
			Args: map[string]interface{}{
				"name":  "openvpn",
				"state": "present",
			},
		},
		{
			Name:   "Copy VPN configuration",
			Module: "copy",
			Args: map[string]interface{}{
				"src":  configFile,
				"dest": "/etc/openvpn/client.conf",
				"mode": "0600",
			},
		},
		{
			Name:   "Copy VPN credentials",
			Module: "copy",
			Args: map[string]interface{}{
				"src":  authFile,
				"dest": "/etc/openvpn/auth.txt",
				"mode": "0600",
			},
			When: "authFile is defined",
		},
		{
			Name:   "Enable and start OpenVPN",
			Module: "systemd",
			Args: map[string]interface{}{
				"name":    "openvpn@client",
				"state":   "started",
				"enabled": true,
			},
		},
	}
}

// ConfigureProxy sets up system-wide proxy settings
func (nt *NetworkTasks) ConfigureProxy(httpProxy, httpsProxy, noProxy string) []types.Task {
	proxyContent := ""
	if httpProxy != "" {
		proxyContent += "export http_proxy=" + httpProxy + "\n"
		proxyContent += "export HTTP_PROXY=" + httpProxy + "\n"
	}
	if httpsProxy != "" {
		proxyContent += "export https_proxy=" + httpsProxy + "\n"
		proxyContent += "export HTTPS_PROXY=" + httpsProxy + "\n"
	}
	if noProxy != "" {
		proxyContent += "export no_proxy=" + noProxy + "\n"
		proxyContent += "export NO_PROXY=" + noProxy + "\n"
	}
	
	return []types.Task{
		{
			Name:   "Configure system-wide proxy",
			Module: "copy",
			Args: map[string]interface{}{
				"content": proxyContent,
				"dest":    "/etc/profile.d/proxy.sh",
				"mode":    "0644",
			},
		},
		{
			Name:   "Configure APT proxy",
			Module: "copy",
			Args: map[string]interface{}{
				"content": "Acquire::http::Proxy \"" + httpProxy + "\";\n" +
				          "Acquire::https::Proxy \"" + httpsProxy + "\";\n",
				"dest": "/etc/apt/apt.conf.d/95proxies",
			},
			When: "ansible_os_family == 'Debian'",
		},
	}
}