package main

import (
	"context"
	"fmt"
	"log"

	"github.com/liliang-cn/gosible/pkg/inventory"
	"github.com/liliang-cn/gosiblepkg/library"
	"github.com/liliang-cn/gosiblepkg/runner"
	"github.com/liliang-cn/gosiblepkg/types"
)

func main() {
	// Example 1: Using TaskBuilder for complex operations
	fmt.Println("=== Example 1: Installing Prometheus as a Service ===")
	installPrometheusExample()

	// Example 2: Using QuickTasks for simple operations
	fmt.Println("\n=== Example 2: Quick Tasks ===")
	quickTasksExample()

	// Example 3: Common patterns
	fmt.Println("\n=== Example 3: Common Patterns ===")
	commonPatternsExample()
}

func installPrometheusExample() {
	// Build a complete task sequence to install Prometheus
	tb := library.NewTaskBuilder()

	tasks := tb.
		// Ensure directories exist
		WithDirectory("/opt/prometheus").
		WithDirectory("/etc/prometheus").
		WithDirectory("/var/lib/prometheus").

		// Install Prometheus as a service
		WithService(
			"https://github.com/prometheus/prometheus/releases/download/v2.40.0/prometheus-2.40.0.linux-amd64.tar.gz",
			"prometheus",
		).

		// Add configuration file
		WithFile("/etc/prometheus/prometheus.yml", `
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: 'prometheus'
    static_configs:
    - targets: ['localhost:9090']
`).
		// Add backup before modifying
		WithBackup("/etc/prometheus/prometheus.yml").

		// Build the task list
		Build()

	// Convert to playbook
	_ = tb.ToPlaybook("Install Prometheus", "monitoring_servers")

	fmt.Printf("Generated %d tasks for Prometheus installation\n", len(tasks))

	// In real usage, you would execute this:
	// runner.RunPlaybook(ctx, playbook, inventory, vars)
}

func quickTasksExample() {
	qt := library.NewQuickTasks()

	// Create simple tasks quickly
	tasks := []types.Task{
		qt.Package("nginx", "present"),
		qt.File("/var/www/html", "directory"),
		qt.Copy("index.html", "/var/www/html/index.html"),
		qt.Service("nginx", "started"),
		qt.LineInFile("/etc/nginx/nginx.conf", "server_tokens off;"),
		qt.GetUrl("https://example.com/app.tar.gz", "/tmp/app.tar.gz"),
		qt.Unarchive("/tmp/app.tar.gz", "/opt/"),
		qt.Template("config.j2", "/etc/app/config.yml"),
		qt.Debug("Installation complete"),
	}

	fmt.Printf("Created %d quick tasks\n", len(tasks))

	// Show task names
	for _, task := range tasks {
		fmt.Printf("  - %s\n", task.Name)
	}
}

func commonPatternsExample() {
	ct := library.NewCommonTasks()

	// Pattern 1: Deploy configuration with validation and restart
	fmt.Println("\nPattern 1: Deploy config with validation")
	configTasks := ct.TemplateConfig(
		"nginx.conf.j2",         // template
		"/etc/nginx/nginx.conf", // destination
		"root",                  // owner
		"root",                  // group
		"0644",                  // mode
		"nginx -t",              // validation command
		"nginx",                 // service to restart
	)
	fmt.Printf("  Generated %d tasks for config deployment\n", len(configTasks))

	// Pattern 2: Git deployment
	fmt.Println("\nPattern 2: Git repository deployment")
	gitTasks := ct.GitCloneOrUpdate(
		"https://github.com/example/app.git",
		"/opt/app",
		"main",
	)
	fmt.Printf("  Generated %d tasks for git deployment\n", len(gitTasks))

	// Pattern 3: User creation with SSH access
	fmt.Println("\nPattern 3: User with SSH access")
	userTasks := ct.CreateUserWithSSHKey(
		"deploy",
		[]string{"docker", "sudo"},
		"ssh-rsa AAAAB3NzaC1yc2E... user@example.com",
		true, // sudoer
	)
	fmt.Printf("  Generated %d tasks for user creation\n", len(userTasks))

	// Pattern 4: Docker container deployment
	fmt.Println("\nPattern 4: Docker container")
	dockerTasks := ct.DockerContainer(
		"myapp",
		"myapp:latest",
		[]string{"8080:80"},
		map[string]string{
			"DB_HOST": "localhost",
			"DB_PASS": "secret",
		},
		[]string{"/data:/var/lib/app"},
	)
	fmt.Printf("  Generated %d tasks for Docker deployment\n", len(dockerTasks))

	// Pattern 5: Cron job setup
	fmt.Println("\nPattern 5: Cron job")
	cronTasks := ct.CronJob(
		"backup",
		"root",
		"/usr/local/bin/backup.sh",
		"0", // minute
		"2", // hour
		"*", // day
		"*", // month
		"*", // weekday
	)
	fmt.Printf("  Generated %d tasks for cron job\n", len(cronTasks))

	// Pattern 6: Firewall configuration
	fmt.Println("\nPattern 6: Firewall rules")
	firewallTasks := ct.ConfigureFirewall(443, "tcp", "allow")
	fmt.Printf("  Generated %d tasks for firewall configuration\n", len(firewallTasks))

	// Pattern 7: Run script with checks
	fmt.Println("\nPattern 7: Script execution with checks")
	scriptTasks := ct.RunScriptWithCheck(
		"/opt/scripts/deploy.sh",
		"/opt/app/deployed.flag", // creates this file
		"test -f /opt/app/ready", // only run if this exists
	)
	fmt.Printf("  Generated %d tasks for script execution\n", len(scriptTasks))

	// Pattern 8: Using specialized task types directly
	fmt.Println("\nPattern 8: Direct access to specialized task types")

	// Direct file operations
	fileTasks := ct.File.SetPermissions(
		"/var/www/html",
		"www-data",
		"www-data",
		"0644", // file mode
		"0755", // dir mode
	)
	fmt.Printf("  Generated %d tasks for file permissions\n", len(fileTasks))

	// Direct service operations
	serviceTasks := ct.Service.ServiceHealthCheck(
		"nginx",
		"curl -f http://localhost/health",
	)
	fmt.Printf("  Generated %d tasks for service health check\n", len(serviceTasks))

	// Direct package operations
	pkgTasks := ct.Package.InstallPythonPackages(
		[]string{"flask", "requests", "pytest"},
		"/opt/venv",
	)
	fmt.Printf("  Generated %d tasks for Python packages\n", len(pkgTasks))

	// Direct network operations
	netTasks := ct.Network.SetupVPN(
		"/etc/openvpn/client.ovpn",
		"/etc/openvpn/auth.txt",
	)
	fmt.Printf("  Generated %d tasks for VPN setup\n", len(netTasks))
}

// Example of using common patterns in actual execution
func executeCommonTasks() {
	// Create inventory
	inv, _ := inventory.NewFromYAML([]byte(`
all:
  hosts:
    server1:
      ansible_host: 10.0.0.1
`))

	// Create runner
	r := runner.NewTaskRunner()

	// Use TaskBuilder to create and execute tasks
	tb := library.NewTaskBuilder()
	tasks := tb.
		WithPackages("git", "curl", "wget").
		WithDirectory("/opt/myapp").
		WithGitRepo("https://github.com/example/app.git", "/opt/myapp").
		WithFile("/etc/myapp/config.yml", "port: 8080\nhost: 0.0.0.0").
		Build()

	// Execute tasks
	ctx := context.Background()
	hosts, _ := inv.GetHosts("all")

	for _, task := range tasks {
		results, err := r.Run(ctx, task, hosts, nil)
		if err != nil {
			log.Printf("Task failed: %v", err)
			continue
		}

		for _, result := range results {
			if result.Success {
				fmt.Printf("✓ %s on %s\n", task.Name, result.Host)
			} else {
				fmt.Printf("✗ %s on %s: %v\n", task.Name, result.Host, result.Error)
			}
		}
	}
}

// Example of programmatic playbook generation
func generatePlaybook() {
	tb := library.NewTaskBuilder()
	h := library.NewHandlers()

	// Build a complete application deployment playbook
	playbook := &types.Playbook{
		Plays: []types.Play{
			{
				Name:  "Deploy Web Application",
				Hosts: "webservers",
				Vars: map[string]interface{}{
					"app_name":    "myapp",
					"app_port":    8080,
					"app_version": "v1.2.3",
				},
				Tasks: tb.
					WithPackages("nginx", "python3", "python3-pip").
					WithDirectory("/opt/{{ app_name }}").
					WithGitRepo("https://github.com/example/{{ app_name }}.git", "/opt/{{ app_name }}").
					WithFile("/etc/systemd/system/{{ app_name }}.service", `
[Unit]
Description={{ app_name }}
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/{{ app_name }}
ExecStart=/usr/bin/python3 /opt/{{ app_name }}/app.py
Restart=always

[Install]
WantedBy=multi-user.target
`).
					Build(),
				Handlers: []types.Task{
					h.RestartService("{{ app_name }}"),
					h.ReloadService("nginx"),
					h.ReloadSystemd(),
				},
			},
		},
	}

	fmt.Println("Generated complete deployment playbook")
	_ = playbook // Prevent unused variable warning

	// This playbook can now be:
	// 1. Saved to a file
	// 2. Executed directly
	// 3. Modified programmatically
	// 4. Combined with other playbooks
}
