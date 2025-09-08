// Package library provides common task patterns for frequent operations
package library

import (
	"github.com/gosinble/gosinble/pkg/types"
)

// CommonTasks provides convenient task builders for common operations
// It delegates to specialized task types for better organization
type CommonTasks struct {
	File         *FileTasks
	Service      *ServiceTasks
	Package      *PackageTasks
	Network      *NetworkTasks
	Content      *ContentTasks
	Distribution *DistributionTasks
	Archive      *ArchiveTasks
	System       *SystemTasks
	Development  *DevelopmentTasks
}

// NewCommonTasks creates a new CommonTasks instance with all task types initialized
func NewCommonTasks() *CommonTasks {
	return &CommonTasks{
		File:         NewFileTasks(),
		Service:      NewServiceTasks(),
		Package:      NewPackageTasks(),
		Network:      NewNetworkTasks(),
		Content:      NewContentTasks(),
		Distribution: NewDistributionTasks(),
		Archive:      NewArchiveTasks(),
		System:       NewSystemTasks(),
		Development:  NewDevelopmentTasks(),
	}
}

// Convenience methods that delegate to the appropriate task type

// EnsureFile creates tasks to ensure a file exists with specific content and permissions
func (ct *CommonTasks) EnsureFile(path, content, owner, group, mode string) []types.Task {
	return ct.File.EnsureFile(path, content, owner, group, mode)
}

// EnsureDirectory creates tasks to ensure a directory structure exists
func (ct *CommonTasks) EnsureDirectory(path, owner, group, mode string, recursive bool) []types.Task {
	return ct.File.EnsureDirectory(path, owner, group, mode, recursive)
}

// BackupFile creates tasks to backup a file before modification
func (ct *CommonTasks) BackupFile(path string) []types.Task {
	return ct.File.BackupFile(path)
}

// TemplateConfig creates tasks to deploy a configuration from a template
func (ct *CommonTasks) TemplateConfig(template, dest, owner, group, mode string, validateCmd string, restartService string) []types.Task {
	return ct.File.TemplateConfig(template, dest, owner, group, mode, validateCmd, restartService)
}

// InstallBinaryAsService installs a binary and sets it up as a systemd service
func (ct *CommonTasks) InstallBinaryAsService(binaryUrl, binaryPath, serviceName, serviceUser string, serviceArgs map[string]interface{}) []types.Task {
	return ct.Service.InstallBinaryAsService(binaryUrl, binaryPath, serviceName, serviceUser, serviceArgs)
}

// ManageService creates tasks to manage a service
func (ct *CommonTasks) ManageService(name, state string, enabled bool) []types.Task {
	return ct.Service.ManageSystemdService(name, state, enabled)
}

// ManagePackages creates tasks for package management across different systems
func (ct *CommonTasks) ManagePackages(packages []string, state string) []types.Task {
	return ct.Package.ManagePackages(packages, state)
}

// InstallFromURL downloads and installs a package from URL
func (ct *CommonTasks) InstallFromURL(url, dest string) []types.Task {
	return ct.Package.InstallFromURL(url, dest, "auto")
}

// ConfigureFirewall creates tasks to manage firewall rules
func (ct *CommonTasks) ConfigureFirewall(port int, protocol, action string) []types.Task {
	return ct.Network.ConfigureFirewall(port, protocol, action)
}

// SetupSSHSecurity hardens SSH configuration
func (ct *CommonTasks) SetupSSHSecurity(permitRoot bool, passwordAuth bool) []types.Task {
	return ct.Network.SetupSSHSecurity(permitRoot, passwordAuth, 22)
}

// Additional convenience methods for common combinations

// GitCloneOrUpdate creates tasks to clone or update a git repository
func (ct *CommonTasks) GitCloneOrUpdate(repo, dest, version string) []types.Task {
	return []types.Task{
		{
			Name:   "Ensure git is installed",
			Module: "package",
			Args: map[string]interface{}{
				"name":  "git",
				"state": "present",
			},
		},
		{
			Name:   "Clone or update repository",
			Module: "git",
			Args: map[string]interface{}{
				"repo":    repo,
				"dest":    dest,
				"version": version,
				"force":   true,
			},
			Register: "git_result",
		},
		{
			Name:   "Set repository permissions",
			Module: "file",
			Args: map[string]interface{}{
				"path":    dest,
				"state":   "directory",
				"recurse": true,
				"owner":   "{{ repo_owner | default(omit) }}",
				"group":   "{{ repo_group | default(omit) }}",
			},
			When: "git_result.changed",
		},
	}
}

// RunScriptWithCheck creates tasks to run a script with pre and post checks
func (ct *CommonTasks) RunScriptWithCheck(script string, creates string, onlyIf string) []types.Task {
	tasks := []types.Task{}
	
	// Add pre-check if specified
	if onlyIf != "" {
		tasks = append(tasks, types.Task{
			Name:   "Check precondition",
			Module: "command",
			Args: map[string]interface{}{
				"cmd": onlyIf,
			},
			Register:     "precondition",
			IgnoreErrors: true,
		})
	}
	
	// Main script execution
	mainTask := types.Task{
		Name:   "Execute script",
		Module: "script",
		Args: map[string]interface{}{
			"script": script,
		},
		Register: "script_result",
	}
	
	// Add creates check if specified
	if creates != "" {
		mainTask.Args["creates"] = creates
	}
	
	// Add conditional if precondition exists
	if onlyIf != "" {
		mainTask.When = "precondition.rc == 0"
	}
	
	tasks = append(tasks, mainTask)
	
	// Add post-execution verification
	tasks = append(tasks, types.Task{
		Name:   "Verify script execution",
		Module: "debug",
		Args: map[string]interface{}{
			"msg": "Script output: {{ script_result.stdout }}",
		},
		When: "script_result.changed",
	})
	
	return tasks
}

// DockerContainer creates tasks to manage a Docker container
func (ct *CommonTasks) DockerContainer(name, image string, ports []string, env map[string]string, volumes []string) []types.Task {
	return []types.Task{
		{
			Name:   "Ensure Docker is installed",
			Module: "package",
			Args: map[string]interface{}{
				"name":  "docker.io",
				"state": "present",
			},
		},
		{
			Name:   "Pull Docker image",
			Module: "docker_image",
			Args: map[string]interface{}{
				"name":   image,
				"source": "pull",
			},
		},
		{
			Name:   "Run Docker container",
			Module: "docker_container",
			Args: map[string]interface{}{
				"name":     name,
				"image":    image,
				"state":    "started",
				"restart_policy": "unless-stopped",
				"ports":    ports,
				"env":      env,
				"volumes":  volumes,
			},
		},
	}
}

// CronJob creates tasks to manage a cron job
func (ct *CommonTasks) CronJob(name, user, job, minute, hour, day, month, weekday string) []types.Task {
	return []types.Task{
		{
			Name:   "Setup cron job",
			Module: "cron",
			Args: map[string]interface{}{
				"name":    name,
				"user":    user,
				"job":     job,
				"minute":  minute,
				"hour":    hour,
				"day":     day,
				"month":   month,
				"weekday": weekday,
				"state":   "present",
			},
		},
	}
}

// CreateUserWithSSHKey creates tasks to setup a user with SSH access
func (ct *CommonTasks) CreateUserWithSSHKey(username string, groups []string, sshKey string, sudoer bool) []types.Task {
	tasks := []types.Task{
		{
			Name:   "Create user account",
			Module: "user",
			Args: map[string]interface{}{
				"name":   username,
				"groups": groups,
				"shell":  "/bin/bash",
				"create_home": true,
			},
		},
		{
			Name:   "Add SSH key",
			Module: "authorized_key",
			Args: map[string]interface{}{
				"user": username,
				"key":  sshKey,
			},
			When: "ssh_key is defined",
		},
	}
	
	if sudoer {
		tasks = append(tasks, types.Task{
			Name:   "Add user to sudoers",
			Module: "lineinfile",
			Args: map[string]interface{}{
				"path":   "/etc/sudoers.d/" + username,
				"line":   username + " ALL=(ALL) NOPASSWD:ALL",
				"create": true,
				"mode":   "0440",
			},
		})
	}
	
	return tasks
}

// Archive operations convenience methods

// CreateArchive creates a compressed archive from files/directories
func (ct *CommonTasks) CreateArchive(paths []string, dest, format string) []types.Task {
	return ct.Archive.CreateArchive(paths, dest, format, nil)
}

// ExtractArchive extracts an archive to a destination
func (ct *CommonTasks) ExtractArchive(src, dest string) []types.Task {
	return ct.Archive.ExtractArchive(src, dest, false)
}

// System configuration convenience methods

// SetSysctl sets a kernel parameter
func (ct *CommonTasks) SetSysctl(name, value string, persistent bool) []types.Task {
	return ct.System.SetSysctl(name, value, persistent)
}

// MountFilesystem mounts a filesystem
func (ct *CommonTasks) MountFilesystem(src, path, fstype string) []types.Task {
	return ct.System.MountFilesystem(src, path, fstype, nil)
}

// AllowFirewallPort opens a firewall port
func (ct *CommonTasks) AllowFirewallPort(port, protocol string) []types.Task {
	return ct.System.AddFirewallRule("INPUT", protocol, port, "ACCEPT")
}

// Development environment convenience methods

// InstallPythonPackage installs a Python package via pip
func (ct *CommonTasks) InstallPythonPackage(name string) []types.Task {
	return ct.Development.InstallPythonPackage(name, "", "")
}

// InstallNodePackage installs a Node.js package via npm
func (ct *CommonTasks) InstallNodePackage(name string, global bool) []types.Task {
	return ct.Development.InstallNodePackage(name, "", global)
}

// InstallRubyGem installs a Ruby gem
func (ct *CommonTasks) InstallRubyGem(name string) []types.Task {
	return ct.Development.InstallRubyGem(name, "", false)
}