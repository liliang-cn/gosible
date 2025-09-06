package library

import (
	"github.com/gosinble/gosinble/pkg/types"
)

// ServiceTasks provides common service management operations
type ServiceTasks struct{}

// NewServiceTasks creates a new ServiceTasks instance
func NewServiceTasks() *ServiceTasks {
	return &ServiceTasks{}
}

// InstallBinaryAsService installs a binary and sets it up as a systemd service
func (st *ServiceTasks) InstallBinaryAsService(binaryUrl, binaryPath, serviceName, serviceUser string, serviceArgs map[string]interface{}) []types.Task {
	return []types.Task{
		// Download and install binary
		{
			Name:   "Create binary directory",
			Module: "file",
			Args: map[string]interface{}{
				"path":  "/usr/local/bin",
				"state": "directory",
				"mode":  "0755",
			},
		},
		{
			Name:   "Download binary",
			Module: "get_url",
			Args: map[string]interface{}{
				"url":  binaryUrl,
				"dest": binaryPath,
				"mode": "0755",
			},
			Register: "binary_download",
		},
		{
			Name:   "Verify binary is executable",
			Module: "file",
			Args: map[string]interface{}{
				"path": binaryPath,
				"mode": "0755",
			},
		},
		// Create service user
		{
			Name:   "Create service user",
			Module: "user",
			Args: map[string]interface{}{
				"name":   serviceUser,
				"system": true,
				"shell":  "/bin/false",
				"home":   "/var/lib/" + serviceName,
				"create_home": true,
			},
			When: "serviceUser != 'root'",
		},
		// Create systemd service
		{
			Name:   "Create systemd service file",
			Module: "template",
			Args: map[string]interface{}{
				"dest": "/etc/systemd/system/" + serviceName + ".service",
				"content": st.generateServiceTemplate(serviceName, binaryPath, serviceUser, serviceArgs),
			},
			Notify: []string{"reload systemd"},
		},
		{
			Name:   "Start and enable service",
			Module: "systemd",
			Args: map[string]interface{}{
				"name":    serviceName,
				"state":   "started",
				"enabled": true,
				"daemon_reload": true,
			},
		},
	}
}

// generateServiceTemplate creates a systemd service file template
func (st *ServiceTasks) generateServiceTemplate(serviceName, binaryPath, serviceUser string, args map[string]interface{}) string {
	execStart := binaryPath
	if cmdArgs, ok := args["arguments"].(string); ok {
		execStart += " " + cmdArgs
	}
	
	workingDir := "/var/lib/" + serviceName
	if wd, ok := args["working_directory"].(string); ok {
		workingDir = wd
	}
	
	environment := ""
	if env, ok := args["environment"].(map[string]string); ok {
		for k, v := range env {
			environment += "Environment=\"" + k + "=" + v + "\"\n"
		}
	}
	
	return `[Unit]
Description=` + serviceName + ` service
After=network.target

[Service]
Type=simple
User=` + serviceUser + `
WorkingDirectory=` + workingDir + `
ExecStart=` + execStart + `
` + environment + `
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target`
}

// ManageSystemdService creates tasks to manage a systemd service
func (st *ServiceTasks) ManageSystemdService(name, state string, enabled bool) []types.Task {
	return []types.Task{
		{
			Name:   "Manage systemd service",
			Module: "systemd",
			Args: map[string]interface{}{
				"name":    name,
				"state":   state,
				"enabled": enabled,
				"daemon_reload": true,
			},
		},
	}
}

// ManageSysVService creates tasks to manage a SysV init service
func (st *ServiceTasks) ManageSysVService(name, state string, enabled bool) []types.Task {
	return []types.Task{
		{
			Name:   "Manage SysV service",
			Module: "service",
			Args: map[string]interface{}{
				"name":    name,
				"state":   state,
				"enabled": enabled,
			},
		},
	}
}

// CreateServiceFromScript creates a service from a shell script
func (st *ServiceTasks) CreateServiceFromScript(scriptPath, serviceName, description string) []types.Task {
	serviceContent := `[Unit]
Description=` + description + `
After=network.target

[Service]
Type=simple
ExecStart=/bin/bash ` + scriptPath + `
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target`

	return []types.Task{
		{
			Name:   "Ensure script is executable",
			Module: "file",
			Args: map[string]interface{}{
				"path": scriptPath,
				"mode": "0755",
			},
		},
		{
			Name:   "Create systemd service file",
			Module: "copy",
			Args: map[string]interface{}{
				"content": serviceContent,
				"dest":    "/etc/systemd/system/" + serviceName + ".service",
			},
		},
		{
			Name:   "Reload systemd daemon",
			Module: "systemd",
			Args: map[string]interface{}{
				"daemon_reload": true,
			},
		},
		{
			Name:   "Enable and start service",
			Module: "systemd",
			Args: map[string]interface{}{
				"name":    serviceName,
				"state":   "started",
				"enabled": true,
			},
		},
	}
}

// ServiceHealthCheck creates tasks to check service health
func (st *ServiceTasks) ServiceHealthCheck(serviceName string, checkCommand string) []types.Task {
	tasks := []types.Task{
		{
			Name:   "Check service status",
			Module: "systemd",
			Args: map[string]interface{}{
				"name": serviceName,
			},
			Register: "service_status",
		},
		{
			Name:   "Verify service is running",
			Module: "assert",
			Args: map[string]interface{}{
				"that": []string{
					"service_status.status.ActiveState == 'active'",
				},
				"fail_msg": "Service " + serviceName + " is not running",
			},
		},
	}
	
	if checkCommand != "" {
		tasks = append(tasks, types.Task{
			Name:   "Run health check command",
			Module: "command",
			Args: map[string]interface{}{
				"cmd": checkCommand,
			},
			Register: "health_check",
		})
	}
	
	return tasks
}

// RestartServiceGracefully creates tasks for graceful service restart
func (st *ServiceTasks) RestartServiceGracefully(serviceName string, waitTime int) []types.Task {
	return []types.Task{
		{
			Name:   "Stop service gracefully",
			Module: "systemd",
			Args: map[string]interface{}{
				"name":  serviceName,
				"state": "stopped",
			},
		},
		{
			Name:   "Wait for service to stop",
			Module: "pause",
			Args: map[string]interface{}{
				"seconds": waitTime,
			},
		},
		{
			Name:   "Start service",
			Module: "systemd",
			Args: map[string]interface{}{
				"name":  serviceName,
				"state": "started",
			},
		},
		{
			Name:   "Wait for service to be ready",
			Module: "wait_for",
			Args: map[string]interface{}{
				"port":  "{{ service_port | default(omit) }}",
				"delay": 5,
				"timeout": 60,
			},
		},
	}
}