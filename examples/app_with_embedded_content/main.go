// Package main demonstrates a complete application with embedded content
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	
	"github.com/gosinble/gosinble/pkg/inventory"
	"github.com/gosinble/gosinble/pkg/library"
	"github.com/gosinble/gosinble/pkg/runner"
	"github.com/gosinble/gosinble/pkg/types"
)

// Embed all application content
//go:embed configs scripts templates
var appContent embed.FS

// Application deployment manager
type AppDeployment struct {
	name     string
	version  string
	ec       *library.EmbeddedContent
	ct       *library.CommonTasks
	runner   *runner.TaskRunner
}

func NewAppDeployment(name, version string) *AppDeployment {
	return &AppDeployment{
		name:    name,
		version: version,
		ec:      library.NewEmbeddedContent(appContent, "."),
		ct:      library.NewCommonTasks(),
		runner:  runner.NewTaskRunner(),
	}
}

// Deploy performs a complete application deployment
func (ad *AppDeployment) Deploy(ctx context.Context, inv *inventory.StaticInventory, environment string) error {
	fmt.Printf("Deploying %s version %s to %s environment\n", ad.name, ad.version, environment)
	
	// Get target hosts
	hosts, err := inv.GetHosts("webservers")
	if err != nil {
		return fmt.Errorf("failed to get hosts: %w", err)
	}
	
	// Build deployment tasks
	tasks := ad.buildDeploymentTasks(environment)
	
	// Execute tasks
	for _, task := range tasks {
		fmt.Printf("Executing: %s\n", task.Name)
		results, err := ad.runner.Run(ctx, task, hosts, nil)
		if err != nil {
			return fmt.Errorf("task '%s' failed: %w", task.Name, err)
		}
		
		// Check results
		for _, result := range results {
			if !result.Success {
				return fmt.Errorf("task '%s' failed on %s: %v", task.Name, result.Host, result.Error)
			}
		}
	}
	
	fmt.Println("Deployment completed successfully")
	return nil
}

// buildDeploymentTasks creates the task sequence for deployment
func (ad *AppDeployment) buildDeploymentTasks(environment string) []common.Task {
	var tasks []common.Task
	
	// 1. Pre-deployment backup
	tasks = append(tasks, ad.createBackupTasks()...)
	
	// 2. Install dependencies
	tasks = append(tasks, ad.ct.Package.ManagePackages([]string{
		"nginx",
		"nodejs",
		"npm",
		"postgresql-client",
	}, "present")...)
	
	// 3. Create application user and directories
	tasks = append(tasks, ad.createAppStructureTasks()...)
	
	// 4. Deploy application code (in real app, this would be from git or download)
	tasks = append(tasks, ad.deployApplicationTasks()...)
	
	// 5. Deploy configuration files
	tasks = append(tasks, ad.deployConfigurationTasks(environment)...)
	
	// 6. Deploy and configure services
	tasks = append(tasks, ad.deployServiceTasks()...)
	
	// 7. Configure web server
	tasks = append(tasks, ad.configureNginxTasks()...)
	
	// 8. Post-deployment validation
	tasks = append(tasks, ad.validationTasks()...)
	
	return tasks
}

// createBackupTasks creates backup tasks
func (ad *AppDeployment) createBackupTasks() []common.Task {
	// Deploy backup script
	tasks := ad.ec.DeployFile("scripts/backup.sh", "/usr/local/bin/backup.sh", "root", "root", "0755")
	
	// Run backup
	tasks = append(tasks, common.Task{
		Name:   "Create pre-deployment backup",
		Module: "command",
		Args: map[string]interface{}{
			"cmd": "/usr/local/bin/backup.sh",
		},
		IgnoreErrors: true, // Don't fail if this is first deployment
	})
	
	return tasks
}

// createAppStructureTasks creates the application user and directory structure
func (ad *AppDeployment) createAppStructureTasks() []common.Task {
	tasks := []common.Task{
		{
			Name:   "Create application user",
			Module: "user",
			Args: map[string]interface{}{
				"name":        ad.name,
				"system":      true,
				"shell":       "/bin/bash",
				"home":        fmt.Sprintf("/home/%s", ad.name),
				"create_home": true,
			},
		},
	}
	
	// Create directory structure
	dirs := []string{
		fmt.Sprintf("/opt/%s", ad.name),
		fmt.Sprintf("/etc/%s", ad.name),
		fmt.Sprintf("/var/log/%s", ad.name),
		fmt.Sprintf("/var/lib/%s", ad.name),
	}
	
	for _, dir := range dirs {
		tasks = append(tasks, ad.ct.File.EnsureDirectory(dir, ad.name, ad.name, "0755", false)...)
	}
	
	return tasks
}

// deployApplicationTasks deploys the application files
func (ad *AppDeployment) deployApplicationTasks() []common.Task {
	var tasks []common.Task
	
	// Deploy scripts
	tasks = append(tasks, ad.ec.DeployDirectory("scripts", fmt.Sprintf("/opt/%s/scripts", ad.name), 
		ad.name, ad.name, "0755", "0755")...)
	
	// In a real application, you would:
	// - Clone from git repository
	// - Download release artifacts
	// - Extract archives
	// - Install dependencies
	
	tasks = append(tasks, common.Task{
		Name:   "Placeholder for application code deployment",
		Module: "debug",
		Args: map[string]interface{}{
			"msg": fmt.Sprintf("Application %s v%s would be deployed here", ad.name, ad.version),
		},
	})
	
	return tasks
}

// deployConfigurationTasks deploys configuration files
func (ad *AppDeployment) deployConfigurationTasks(environment string) []common.Task {
	var tasks []common.Task
	
	// Deploy main configuration
	configPath := fmt.Sprintf("/etc/%s/config.yml", ad.name)
	tasks = append(tasks, ad.ec.DeployFile("configs/app.yml", configPath, ad.name, ad.name, "0644")...)
	
	// Override with environment-specific settings
	tasks = append(tasks, common.Task{
		Name:   "Set environment in configuration",
		Module: "lineinfile",
		Args: map[string]interface{}{
			"path":   configPath,
			"regexp": "^  environment:",
			"line":   fmt.Sprintf("  environment: %s", environment),
		},
	})
	
	return tasks
}

// deployServiceTasks creates and configures the systemd service
func (ad *AppDeployment) deployServiceTasks() []common.Task {
	var tasks []common.Task
	
	// Template variables for service file
	vars := map[string]interface{}{
		"app_name": ad.name,
		"app_user": ad.name,
		"app_dir":  fmt.Sprintf("/opt/%s", ad.name),
		"app_port": 3000,
		"environment": "production",
		"env_vars": map[string]string{
			"CONFIG_PATH": fmt.Sprintf("/etc/%s/config.yml", ad.name),
			"LOG_PATH":    fmt.Sprintf("/var/log/%s", ad.name),
		},
	}
	
	// Deploy service file from template
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", ad.name)
	tasks = append(tasks, ad.ec.DeployTemplate("templates/app.service.j2", servicePath, vars, "root", "root", "0644")...)
	
	// Reload systemd and start service
	tasks = append(tasks, common.Task{
		Name:   "Reload systemd daemon",
		Module: "systemd",
		Args: map[string]interface{}{
			"daemon_reload": true,
		},
	})
	
	tasks = append(tasks, common.Task{
		Name:   fmt.Sprintf("Start and enable %s service", ad.name),
		Module: "systemd",
		Args: map[string]interface{}{
			"name":    ad.name,
			"state":   "started",
			"enabled": true,
		},
	})
	
	return tasks
}

// configureNginxTasks configures nginx as reverse proxy
func (ad *AppDeployment) configureNginxTasks() []common.Task {
	var tasks []common.Task
	
	// Template variables for nginx vhost
	vars := map[string]interface{}{
		"app_name":     ad.name,
		"server_name":  fmt.Sprintf("%s.example.com", ad.name),
		"backend_servers": []string{"localhost:3000"},
		"ssl_enabled":  false,
		"static_dir":   fmt.Sprintf("/opt/%s/public", ad.name),
		"health_check_path": "/health",
	}
	
	// Deploy nginx vhost configuration
	vhostPath := fmt.Sprintf("/etc/nginx/sites-available/%s", ad.name)
	tasks = append(tasks, ad.ec.DeployTemplate("templates/nginx.vhost.j2", vhostPath, vars, "root", "root", "0644")...)
	
	// Enable the site
	tasks = append(tasks, common.Task{
		Name:   "Enable nginx site",
		Module: "file",
		Args: map[string]interface{}{
			"src":   vhostPath,
			"dest":  fmt.Sprintf("/etc/nginx/sites-enabled/%s", ad.name),
			"state": "link",
		},
	})
	
	// Test nginx configuration
	tasks = append(tasks, common.Task{
		Name:   "Test nginx configuration",
		Module: "command",
		Args: map[string]interface{}{
			"cmd": "nginx -t",
		},
	})
	
	// Reload nginx
	tasks = append(tasks, common.Task{
		Name:   "Reload nginx",
		Module: "systemd",
		Args: map[string]interface{}{
			"name":  "nginx",
			"state": "reloaded",
		},
	})
	
	return tasks
}

// validationTasks creates post-deployment validation tasks
func (ad *AppDeployment) validationTasks() []common.Task {
	return []common.Task{
		{
			Name:   "Wait for application to start",
			Module: "wait_for",
			Args: map[string]interface{}{
				"port":    3000,
				"delay":   5,
				"timeout": 30,
			},
		},
		{
			Name:   "Check application health",
			Module: "uri",
			Args: map[string]interface{}{
				"url":         "http://localhost:3000/health",
				"status_code": 200,
			},
		},
		{
			Name:   "Check nginx proxy",
			Module: "uri",
			Args: map[string]interface{}{
				"url":         fmt.Sprintf("http://%s.example.com/health", ad.name),
				"status_code": 200,
				"headers": map[string]string{
					"Host": fmt.Sprintf("%s.example.com", ad.name),
				},
			},
		},
		{
			Name:   "Display deployment summary",
			Module: "debug",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Deployment of %s v%s completed successfully", ad.name, ad.version),
			},
		},
	}
}

// Rollback performs a rollback to previous version
func (ad *AppDeployment) Rollback(ctx context.Context, inv *inventory.StaticInventory) error {
	fmt.Printf("Rolling back %s deployment\n", ad.name)
	
	hosts, err := inv.GetHosts("webservers")
	if err != nil {
		return err
	}
	
	// Rollback tasks
	tasks := []common.Task{
		{
			Name:   "Stop application service",
			Module: "systemd",
			Args: map[string]interface{}{
				"name":  ad.name,
				"state": "stopped",
			},
		},
		{
			Name:   "Restore from backup",
			Module: "shell",
			Args: map[string]interface{}{
				"cmd": fmt.Sprintf("cd /opt && tar -xzf /var/backups/%s/backup_*.tar.gz", ad.name),
			},
		},
		{
			Name:   "Restore configuration",
			Module: "shell",
			Args: map[string]interface{}{
				"cmd": fmt.Sprintf("cd /etc && tar -xzf /var/backups/%s/config_*.tar.gz", ad.name),
			},
		},
		{
			Name:   "Start application service",
			Module: "systemd",
			Args: map[string]interface{}{
				"name":  ad.name,
				"state": "started",
			},
		},
	}
	
	for _, task := range tasks {
		fmt.Printf("Executing: %s\n", task.Name)
		_, err := ad.runner.Run(ctx, task, hosts, nil)
		if err != nil {
			return fmt.Errorf("rollback task '%s' failed: %w", task.Name, err)
		}
	}
	
	fmt.Println("Rollback completed")
	return nil
}

func main() {
	var (
		appName     = flag.String("app", "myapp", "Application name")
		version     = flag.String("version", "1.0.0", "Application version")
		environment = flag.String("env", "staging", "Deployment environment")
		inventoryFile = flag.String("inventory", "hosts.yml", "Inventory file")
		rollback    = flag.Bool("rollback", false, "Perform rollback")
	)
	flag.Parse()
	
	// Create deployment manager
	deployment := NewAppDeployment(*appName, *version)
	
	// Load inventory
	inv, err := inventory.NewFromFile(*inventoryFile)
	if err != nil {
		// Use default inventory for demo
		inv, _ = inventory.NewFromYAML([]byte(`
all:
  children:
    webservers:
      hosts:
        web1:
          ansible_host: 10.0.1.10
        web2:
          ansible_host: 10.0.1.11
`))
	}
	
	ctx := context.Background()
	
	if *rollback {
		// Perform rollback
		if err := deployment.Rollback(ctx, inv); err != nil {
			log.Fatalf("Rollback failed: %v", err)
		}
	} else {
		// Perform deployment
		if err := deployment.Deploy(ctx, inv, *environment); err != nil {
			log.Fatalf("Deployment failed: %v", err)
		}
	}
}

// This example demonstrates:
// 1. Embedding configuration files, scripts, and templates
// 2. Creating a complete deployment workflow
// 3. Using embedded content for consistent deployments
// 4. Template rendering with variables
// 5. Service configuration and management
// 6. Pre-deployment backups and rollback capability
// 7. Post-deployment validation
// 8. Integration with inventory and task runner