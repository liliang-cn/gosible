package library

import (
	"github.com/gosinble/gosinble/pkg/types"
)

// PackageTasks provides common package management operations
type PackageTasks struct{}

// NewPackageTasks creates a new PackageTasks instance
func NewPackageTasks() *PackageTasks {
	return &PackageTasks{}
}

// ManagePackages creates tasks for package management across different systems
func (pt *PackageTasks) ManagePackages(packages []string, state string) []types.Task {
	return []types.Task{
		{
			Name:   "Update package cache (Debian/Ubuntu)",
			Module: "apt",
			Args: map[string]interface{}{
				"update_cache": true,
				"cache_valid_time": 3600,
			},
			When: "ansible_os_family == 'Debian'",
		},
		{
			Name:   "Install packages (Debian/Ubuntu)",
			Module: "apt",
			Args: map[string]interface{}{
				"name":  packages,
				"state": state,
			},
			When: "ansible_os_family == 'Debian'",
		},
		{
			Name:   "Install packages (RedHat/CentOS)",
			Module: "yum",
			Args: map[string]interface{}{
				"name":  packages,
				"state": state,
			},
			When: "ansible_os_family == 'RedHat'",
		},
		{
			Name:   "Install packages (Fedora)",
			Module: "dnf",
			Args: map[string]interface{}{
				"name":  packages,
				"state": state,
			},
			When: "ansible_distribution == 'Fedora'",
		},
		{
			Name:   "Install packages (Alpine)",
			Module: "apk",
			Args: map[string]interface{}{
				"name":  packages,
				"state": state,
			},
			When: "ansible_os_family == 'Alpine'",
		},
	}
}

// InstallFromURL downloads and installs a package from URL
func (pt *PackageTasks) InstallFromURL(url, dest string, packageManager string) []types.Task {
	tasks := []types.Task{
		{
			Name:   "Download package",
			Module: "get_url",
			Args: map[string]interface{}{
				"url":  url,
				"dest": dest,
				"mode": "0644",
			},
			Register: "package_download",
		},
	}
	
	switch packageManager {
	case "dpkg":
		tasks = append(tasks, types.Task{
			Name:   "Install deb package",
			Module: "apt",
			Args: map[string]interface{}{
				"deb": dest,
			},
			When: "package_download.changed",
		})
	case "rpm":
		tasks = append(tasks, types.Task{
			Name:   "Install rpm package",
			Module: "yum",
			Args: map[string]interface{}{
				"name": dest,
				"state": "present",
			},
			When: "package_download.changed",
		})
	default:
		tasks = append(tasks, types.Task{
			Name:   "Install package",
			Module: "package",
			Args: map[string]interface{}{
				"name": dest,
				"state": "present",
			},
			When: "package_download.changed",
		})
	}
	
	return tasks
}

// AddRepository adds a package repository
func (pt *PackageTasks) AddRepository(repo string, key string) []types.Task {
	return []types.Task{
		{
			Name:   "Add GPG key (Debian/Ubuntu)",
			Module: "apt_key",
			Args: map[string]interface{}{
				"url":   key,
				"state": "present",
			},
			When: "ansible_os_family == 'Debian' and key is defined",
		},
		{
			Name:   "Add APT repository (Debian/Ubuntu)",
			Module: "apt_repository",
			Args: map[string]interface{}{
				"repo":  repo,
				"state": "present",
			},
			When: "ansible_os_family == 'Debian'",
		},
		{
			Name:   "Add YUM repository (RedHat/CentOS)",
			Module: "yum_repository",
			Args: map[string]interface{}{
				"name":        "custom_repo",
				"description": "Custom Repository",
				"baseurl":     repo,
				"gpgcheck":    false,
			},
			When: "ansible_os_family == 'RedHat'",
		},
	}
}

// UpgradeSystem creates tasks to upgrade the entire system
func (pt *PackageTasks) UpgradeSystem() []types.Task {
	return []types.Task{
		{
			Name:   "Update package cache",
			Module: "apt",
			Args: map[string]interface{}{
				"update_cache": true,
			},
			When: "ansible_os_family == 'Debian'",
		},
		{
			Name:   "Upgrade all packages (Debian/Ubuntu)",
			Module: "apt",
			Args: map[string]interface{}{
				"upgrade": "dist",
			},
			When: "ansible_os_family == 'Debian'",
		},
		{
			Name:   "Upgrade all packages (RedHat/CentOS)",
			Module: "yum",
			Args: map[string]interface{}{
				"name":  "*",
				"state": "latest",
			},
			When: "ansible_os_family == 'RedHat'",
		},
		{
			Name:   "Remove orphaned packages",
			Module: "apt",
			Args: map[string]interface{}{
				"autoremove": true,
			},
			When: "ansible_os_family == 'Debian'",
		},
		{
			Name:   "Clean package cache",
			Module: "apt",
			Args: map[string]interface{}{
				"autoclean": true,
			},
			When: "ansible_os_family == 'Debian'",
		},
	}
}

// InstallPythonPackages installs Python packages via pip
func (pt *PackageTasks) InstallPythonPackages(packages []string, virtualenv string) []types.Task {
	tasks := []types.Task{
		{
			Name:   "Ensure pip is installed",
			Module: "package",
			Args: map[string]interface{}{
				"name": []string{"python3-pip", "python3-setuptools"},
				"state": "present",
			},
		},
	}
	
	if virtualenv != "" {
		tasks = append(tasks, 
			types.Task{
				Name:   "Create virtual environment",
				Module: "pip",
				Args: map[string]interface{}{
					"name":       "virtualenv",
					"executable": "pip3",
				},
			},
			types.Task{
				Name:   "Install packages in virtualenv",
				Module: "pip",
				Args: map[string]interface{}{
					"name":       packages,
					"virtualenv": virtualenv,
				},
			},
		)
	} else {
		tasks = append(tasks, types.Task{
			Name:   "Install Python packages",
			Module: "pip",
			Args: map[string]interface{}{
				"name":       packages,
				"executable": "pip3",
			},
		})
	}
	
	return tasks
}

// InstallNodePackages installs Node.js packages via npm
func (pt *PackageTasks) InstallNodePackages(packages []string, global bool) []types.Task {
	return []types.Task{
		{
			Name:   "Ensure Node.js and npm are installed",
			Module: "package",
			Args: map[string]interface{}{
				"name":  []string{"nodejs", "npm"},
				"state": "present",
			},
		},
		{
			Name:   "Install Node.js packages",
			Module: "npm",
			Args: map[string]interface{}{
				"name":   packages,
				"global": global,
			},
		},
	}
}

// InstallSnapPackages installs packages via Snap
func (pt *PackageTasks) InstallSnapPackages(packages []string, classic bool) []types.Task {
	return []types.Task{
		{
			Name:   "Ensure snapd is installed",
			Module: "package",
			Args: map[string]interface{}{
				"name":  "snapd",
				"state": "present",
			},
		},
		{
			Name:   "Install snap packages",
			Module: "snap",
			Args: map[string]interface{}{
				"name":    packages,
				"classic": classic,
			},
		},
	}
}