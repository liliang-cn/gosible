package library

import (
	"fmt"
	"github.com/liliang-cn/gosible/pkg/types"
)

// TaskBuilder provides a fluent interface for building complex task sequences
type TaskBuilder struct {
	tasks []types.Task
	ct    *CommonTasks
}

// NewTaskBuilder creates a new task builder
func NewTaskBuilder() *TaskBuilder {
	return &TaskBuilder{
		tasks: []types.Task{},
		ct:    NewCommonTasks(),
	}
}

// AddTask adds a custom task
func (tb *TaskBuilder) AddTask(task types.Task) *TaskBuilder {
	tb.tasks = append(tb.tasks, task)
	return tb
}

// AddTasks adds multiple tasks
func (tb *TaskBuilder) AddTasks(tasks []types.Task) *TaskBuilder {
	tb.tasks = append(tb.tasks, tasks...)
	return tb
}

// WithFile ensures a file exists with content
func (tb *TaskBuilder) WithFile(path, content string) *TaskBuilder {
	tb.tasks = append(tb.tasks, tb.ct.EnsureFile(path, content, "root", "root", "0644")...)
	return tb
}

// WithDirectory ensures a directory exists
func (tb *TaskBuilder) WithDirectory(path string) *TaskBuilder {
	tb.tasks = append(tb.tasks, tb.ct.EnsureDirectory(path, "root", "root", "0755", true)...)
	return tb
}

// WithPackages installs packages
func (tb *TaskBuilder) WithPackages(packages ...string) *TaskBuilder {
	tb.tasks = append(tb.tasks, tb.ct.ManagePackages(packages, "present")...)
	return tb
}

// WithService installs and manages a service from a binary
func (tb *TaskBuilder) WithService(binaryUrl, serviceName string) *TaskBuilder {
	binaryPath := "/usr/local/bin/" + serviceName
	tb.tasks = append(tb.tasks, tb.ct.InstallBinaryAsService(
		binaryUrl, 
		binaryPath, 
		serviceName, 
		serviceName,
		map[string]interface{}{},
	)...)
	return tb
}

// WithGitRepo clones or updates a git repository
func (tb *TaskBuilder) WithGitRepo(repo, dest string) *TaskBuilder {
	tb.tasks = append(tb.tasks, tb.ct.GitCloneOrUpdate(repo, dest, "HEAD")...)
	return tb
}

// WithBackup backs up a file before modification
func (tb *TaskBuilder) WithBackup(path string) *TaskBuilder {
	tb.tasks = append(tb.tasks, tb.ct.BackupFile(path)...)
	return tb
}

// WithDocker runs a Docker container
func (tb *TaskBuilder) WithDocker(name, image string, ports []string) *TaskBuilder {
	tb.tasks = append(tb.tasks, tb.ct.DockerContainer(name, image, ports, nil, nil)...)
	return tb
}

// WithCron adds a cron job
func (tb *TaskBuilder) WithCron(name, schedule, command string) *TaskBuilder {
	// Parse schedule (e.g., "*/5 * * * *")
	parts := []string{"*", "*", "*", "*", "*"}
	// Simple parsing - in production would be more robust
	tb.tasks = append(tb.tasks, tb.ct.CronJob(name, "root", command, 
		parts[0], parts[1], parts[2], parts[3], parts[4])...)
	return tb
}

// WithUser creates a user
func (tb *TaskBuilder) WithUser(username string, sudoer bool) *TaskBuilder {
	tb.tasks = append(tb.tasks, tb.ct.CreateUserWithSSHKey(username, []string{}, "", sudoer)...)
	return tb
}

// Build returns the built task list
func (tb *TaskBuilder) Build() []types.Task {
	return tb.tasks
}

// ToPlaybook converts tasks to a playbook
func (tb *TaskBuilder) ToPlaybook(name string, hosts string) *types.Playbook {
	return &types.Playbook{
		Plays: []types.Play{
			{
				Name:  name,
				Hosts: hosts,
				Tasks: tb.tasks,
			},
		},
	}
}

// QuickTasks provides quick one-liner task creators
type QuickTasks struct{}

// NewQuickTasks creates a new QuickTasks instance
func NewQuickTasks() *QuickTasks {
	return &QuickTasks{}
}

// Command creates a simple command task
func (qt *QuickTasks) Command(name, cmd string) types.Task {
	return types.Task{
		Name:   name,
		Module: "command",
		Args:   map[string]interface{}{"cmd": cmd},
	}
}

// Shell creates a shell task
func (qt *QuickTasks) Shell(name, cmd string) types.Task {
	return types.Task{
		Name:   name,
		Module: "shell",
		Args:   map[string]interface{}{"cmd": cmd},
	}
}

// File creates a file task
func (qt *QuickTasks) File(path, state string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("Manage file %s", path),
		Module: "file",
		Args: map[string]interface{}{
			"path":  path,
			"state": state,
		},
	}
}

// Copy creates a copy task
func (qt *QuickTasks) Copy(src, dest string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("Copy %s to %s", src, dest),
		Module: "copy",
		Args: map[string]interface{}{
			"src":  src,
			"dest": dest,
		},
	}
}

// Service creates a service task
func (qt *QuickTasks) Service(name, state string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("Manage service %s", name),
		Module: "service",
		Args: map[string]interface{}{
			"name":  name,
			"state": state,
		},
	}
}

// Package creates a package task
func (qt *QuickTasks) Package(name, state string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("Manage package %s", name),
		Module: "package",
		Args: map[string]interface{}{
			"name":  name,
			"state": state,
		},
	}
}

// Debug creates a debug task
func (qt *QuickTasks) Debug(msg string) types.Task {
	return types.Task{
		Name:   "Debug message",
		Module: "debug",
		Args:   map[string]interface{}{"msg": msg},
	}
}

// Wait creates a wait/pause task
func (qt *QuickTasks) Wait(seconds int) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("Wait %d seconds", seconds),
		Module: "pause",
		Args:   map[string]interface{}{"seconds": seconds},
	}
}

// Reboot creates a reboot task
func (qt *QuickTasks) Reboot() types.Task {
	return types.Task{
		Name:   "Reboot system",
		Module: "reboot",
		Args:   map[string]interface{}{
			"reboot_timeout": 300,
			"msg": "Rebooting system",
		},
	}
}

// LineInFile adds/modifies a line in a file
func (qt *QuickTasks) LineInFile(path, line string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("Ensure line in %s", path),
		Module: "lineinfile",
		Args: map[string]interface{}{
			"path": path,
			"line": line,
		},
	}
}

// Replace performs text replacement in a file
func (qt *QuickTasks) Replace(path, regexp, replace string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("Replace text in %s", path),
		Module: "replace",
		Args: map[string]interface{}{
			"path":    path,
			"regexp":  regexp,
			"replace": replace,
		},
	}
}

// GetUrl downloads a file
func (qt *QuickTasks) GetUrl(url, dest string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("Download %s", url),
		Module: "get_url",
		Args: map[string]interface{}{
			"url":  url,
			"dest": dest,
		},
	}
}

// Unarchive extracts an archive
func (qt *QuickTasks) Unarchive(src, dest string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("Extract %s", src),
		Module: "unarchive",
		Args: map[string]interface{}{
			"src":  src,
			"dest": dest,
		},
	}
}

// Template applies a template
func (qt *QuickTasks) Template(src, dest string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("Apply template %s", src),
		Module: "template",
		Args: map[string]interface{}{
			"src":  src,
			"dest": dest,
		},
	}
}

// Handlers provides common handler patterns
type Handlers struct{}

// NewHandlers creates a new Handlers instance
func NewHandlers() *Handlers {
	return &Handlers{}
}

// RestartService creates a service restart handler
func (h *Handlers) RestartService(name string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("restart %s", name),
		Module: "service",
		Args: map[string]interface{}{
			"name":  name,
			"state": "restarted",
		},
		Listen: fmt.Sprintf("restart %s", name),
	}
}

// ReloadService creates a service reload handler
func (h *Handlers) ReloadService(name string) types.Task {
	return types.Task{
		Name:   fmt.Sprintf("reload %s", name),
		Module: "service",
		Args: map[string]interface{}{
			"name":  name,
			"state": "reloaded",
		},
		Listen: fmt.Sprintf("reload %s", name),
	}
}

// ReloadSystemd creates a systemd daemon-reload handler
func (h *Handlers) ReloadSystemd() types.Task {
	return types.Task{
		Name:   "reload systemd",
		Module: "systemd",
		Args: map[string]interface{}{
			"daemon_reload": true,
		},
		Listen: "reload systemd",
	}
}