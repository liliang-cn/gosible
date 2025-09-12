package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/liliang-cn/gosible/pkg/inventory"
	"github.com/liliang-cn/gosiblepkg/library"
	"github.com/liliang-cn/gosiblepkg/runner"
	"github.com/liliang-cn/gosiblepkg/types"
)

func main() {
	fmt.Println("=== External File Distribution Examples ===")

	// Example 1: Distribute binary files
	distributeBinaryExample()

	// Example 2: Distribute large directories
	distributeDirectoryExample()

	// Example 3: Distribute archives
	distributeArchiveExample()

	// Example 4: Parallel distribution
	parallelDistributionExample()

	// Example 5: Real-world deployment scenario
	realWorldExample()
}

func distributeBinaryExample() {
	fmt.Println("Example 1: Binary Distribution")
	fmt.Println("------------------------------")

	dt := library.NewDistributionTasks()

	// Register local binaries to distribute
	// These could be compiled Go binaries, tools, etc.
	binaries := []struct {
		name string
		path string
		dest string
	}{
		{"prometheus", "/local/binaries/prometheus", "/usr/local/bin/prometheus"},
		{"node_exporter", "/local/binaries/node_exporter", "/usr/local/bin/node_exporter"},
		{"custom_tool", "/local/binaries/mytool", "/opt/tools/mytool"},
	}

	for _, bin := range binaries {
		if err := dt.AddSource(bin.name, bin.path); err != nil {
			fmt.Printf("Note: %s not found locally (expected for example)\n", bin.path)
			// For demo, create a dummy source
			dt.AddSource(bin.name, "/usr/bin/ls") // Use ls as placeholder
		}
		dt.AddDestination(bin.name, bin.dest, "root", "root", "0755")
	}

	// Distribute prometheus binary
	tasks := dt.DistributeBinary("prometheus", "", true)
	fmt.Printf("Prometheus distribution: %d tasks\n", len(tasks))
	for _, task := range tasks {
		fmt.Printf("  - %s\n", task.Name)
	}

	fmt.Println()
}

func distributeDirectoryExample() {
	fmt.Println("Example 2: Directory Distribution")
	fmt.Println("---------------------------------")

	dt := library.NewDistributionTasks()

	// Register a large directory to distribute
	// This could be application code, data files, etc.
	configDir := "/local/configs"
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		// For demo, use current directory
		configDir = "."
	}

	dt.AddSource("app_configs", configDir)
	dt.AddDestination("app_configs", "/etc/myapp", "app", "app", "0644")

	// Distribute with exclusions
	excludePatterns := []string{
		"--exclude=*.tmp",
		"--exclude=.git",
		"--exclude=node_modules",
		"--exclude=__pycache__",
	}

	tasks := dt.DistributeDirectory("app_configs", "/etc/myapp", excludePatterns)
	fmt.Printf("Directory distribution: %d tasks\n", len(tasks))
	for _, task := range tasks {
		fmt.Printf("  - %s\n", task.Name)
	}

	fmt.Println()
}

func distributeArchiveExample() {
	fmt.Println("Example 3: Archive Distribution")
	fmt.Println("-------------------------------")

	dt := library.NewDistributionTasks()

	// Register an archive to distribute and extract
	archivePath := "/local/releases/app-v1.2.3.tar.gz"
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		// For demo, create a dummy archive path
		archivePath = "/tmp/dummy.tar.gz"
	}

	dt.AddSource("app_release", archivePath)

	// Distribute and extract archive
	// Strip 1 component (removes top-level directory from archive)
	tasks := dt.DistributeArchive("app_release", "/opt/myapp", 1)
	fmt.Printf("Archive distribution: %d tasks\n", len(tasks))
	for _, task := range tasks {
		fmt.Printf("  - %s\n", task.Name)
	}

	fmt.Println()
}

func parallelDistributionExample() {
	fmt.Println("Example 4: Parallel Distribution")
	fmt.Println("--------------------------------")

	dt := library.NewDistributionTasks()

	// Register multiple files for parallel distribution
	files := []string{
		"config1.yml",
		"config2.yml",
		"binary1",
		"binary2",
		"data1.db",
		"data2.db",
	}

	for _, file := range files {
		// Register dummy sources for demo
		dt.AddSource(file, fmt.Sprintf("/tmp/%s", file))
		dt.AddDestination(file, fmt.Sprintf("/opt/distributed/%s", file), "root", "root", "0644")
	}

	// Distribute files in parallel (max 3 at a time)
	tasks := dt.ParallelDistribute(files, 3)
	fmt.Printf("Parallel distribution: %d tasks\n", len(tasks))

	// Show batch organization
	batchNum := 1
	for i, task := range tasks {
		if i%4 == 0 && i > 0 { // New batch every 4 tasks (3 copies + 1 wait)
			batchNum++
		}
		fmt.Printf("  Batch %d - %s\n", batchNum, task.Name)
	}

	fmt.Println()
}

func realWorldExample() {
	fmt.Println("Example 5: Real-World Deployment Scenario")
	fmt.Println("-----------------------------------------")

	// Create a deployment manager
	dm := NewDeploymentManager()

	// Register all components to distribute
	if err := dm.RegisterComponents(); err != nil {
		log.Printf("Failed to register components: %v\n", err)
	}

	// Create deployment tasks
	tasks := dm.CreateDeploymentTasks()

	fmt.Printf("Complete deployment: %d tasks\n", len(tasks))

	// Group tasks by phase
	phases := map[string][]types.Task{
		"preparation":  tasks[0:3],
		"distribution": tasks[3:8],
		"installation": tasks[8:12],
		"validation":   tasks[12:],
	}

	for phase, phaseTasks := range phases {
		fmt.Printf("\n%s phase (%d tasks):\n", phase, len(phaseTasks))
		for _, task := range phaseTasks {
			fmt.Printf("  - %s\n", task.Name)
		}
	}

	// Show how to execute with inventory
	fmt.Println("\nTo execute this deployment:")
	fmt.Println("1. Create inventory with target hosts")
	fmt.Println("2. Run tasks using TaskRunner")
	fmt.Println("3. Monitor progress and handle failures")
}

// DeploymentManager manages a complete deployment scenario
type DeploymentManager struct {
	dt       *library.DistributionTasks
	ct       *library.CommonTasks
	version  string
	basePath string
}

func NewDeploymentManager() *DeploymentManager {
	return &DeploymentManager{
		dt:       library.NewDistributionTasks(),
		ct:       library.NewCommonTasks(),
		version:  "1.2.3",
		basePath: "/opt/deployments",
	}
}

// RegisterComponents registers all components to distribute
func (dm *DeploymentManager) RegisterComponents() error {
	// Application binaries
	binaries := map[string]string{
		"app_server":    "/releases/v1.2.3/bin/server",
		"app_worker":    "/releases/v1.2.3/bin/worker",
		"app_cli":       "/releases/v1.2.3/bin/cli",
		"monitoring":    "/tools/prometheus/prometheus",
		"node_exporter": "/tools/prometheus/node_exporter",
	}

	for name, path := range binaries {
		// In real scenario, these files would exist
		if err := dm.dt.AddSource(name, path); err != nil {
			// For demo, use placeholder
			dm.dt.AddSource(name, "/usr/bin/env")
		}
	}

	// Configuration directories
	configs := map[string]string{
		"app_configs":   "/configs/production",
		"nginx_configs": "/configs/nginx",
		"ssl_certs":     "/configs/ssl",
	}

	for name, path := range configs {
		if err := dm.dt.AddSource(name, path); err != nil {
			// For demo, use current directory
			dm.dt.AddSource(name, ".")
		}
	}

	// Data archives
	archives := map[string]string{
		"static_assets": "/releases/v1.2.3/assets.tar.gz",
		"ml_models":     "/releases/v1.2.3/models.tar.gz",
	}

	for name, path := range archives {
		if err := dm.dt.AddSource(name, path); err != nil {
			// For demo, use placeholder
			dm.dt.AddSource(name, "/tmp/archive.tar.gz")
		}
	}

	return nil
}

// CreateDeploymentTasks creates the complete deployment task sequence
func (dm *DeploymentManager) CreateDeploymentTasks() []types.Task {
	var tasks []types.Task

	// Phase 1: Preparation
	tasks = append(tasks, dm.preparationTasks()...)

	// Phase 2: Distribution
	tasks = append(tasks, dm.distributionTasks()...)

	// Phase 3: Installation
	tasks = append(tasks, dm.installationTasks()...)

	// Phase 4: Validation
	tasks = append(tasks, dm.validationTasks()...)

	return tasks
}

func (dm *DeploymentManager) preparationTasks() []types.Task {
	return []types.Task{
		{
			Name:   "Create deployment directory",
			Module: "file",
			Args: map[string]interface{}{
				"path":  filepath.Join(dm.basePath, dm.version),
				"state": "directory",
				"owner": "deploy",
				"group": "deploy",
				"mode":  "0755",
			},
		},
		{
			Name:   "Stop application services",
			Module: "systemd",
			Args: map[string]interface{}{
				"name":  "app-server",
				"state": "stopped",
			},
			IgnoreErrors: true,
		},
		{
			Name:   "Create backup of current version",
			Module: "shell",
			Args: map[string]interface{}{
				"cmd": fmt.Sprintf("tar -czf /backups/app-backup-$(date +%%Y%%m%%d-%%H%%M%%S).tar.gz -C %s current", dm.basePath),
			},
			IgnoreErrors: true,
		},
	}
}

func (dm *DeploymentManager) distributionTasks() []types.Task {
	tasks := []types.Task{}

	// Distribute binaries
	dm.dt.AddDestination("app_server", filepath.Join(dm.basePath, dm.version, "bin/server"), "deploy", "deploy", "0755")
	dm.dt.AddDestination("app_worker", filepath.Join(dm.basePath, dm.version, "bin/worker"), "deploy", "deploy", "0755")
	dm.dt.AddDestination("app_cli", filepath.Join(dm.basePath, dm.version, "bin/cli"), "deploy", "deploy", "0755")

	tasks = append(tasks, dm.dt.DistributeBinary("app_server", "", false)...)
	tasks = append(tasks, dm.dt.DistributeBinary("app_worker", "", false)...)
	tasks = append(tasks, dm.dt.DistributeBinary("app_cli", "", false)...)

	// Distribute configs
	tasks = append(tasks, dm.dt.DistributeDirectory("app_configs",
		filepath.Join(dm.basePath, dm.version, "config"), nil)...)

	// Extract assets
	tasks = append(tasks, dm.dt.DistributeArchive("static_assets",
		filepath.Join(dm.basePath, dm.version, "static"), 0)...)

	return tasks
}

func (dm *DeploymentManager) installationTasks() []types.Task {
	versionPath := filepath.Join(dm.basePath, dm.version)
	currentPath := filepath.Join(dm.basePath, "current")

	return []types.Task{
		{
			Name:   "Remove old current symlink",
			Module: "file",
			Args: map[string]interface{}{
				"path":  currentPath,
				"state": "absent",
			},
		},
		{
			Name:   "Create current symlink to new version",
			Module: "file",
			Args: map[string]interface{}{
				"src":   versionPath,
				"dest":  currentPath,
				"state": "link",
			},
		},
		{
			Name:   "Update systemd service file",
			Module: "template",
			Args: map[string]interface{}{
				"src":  "app.service.j2",
				"dest": "/etc/systemd/system/app-server.service",
				"vars": map[string]interface{}{
					"app_path": currentPath,
					"app_user": "deploy",
				},
			},
		},
		{
			Name:   "Reload systemd daemon",
			Module: "systemd",
			Args: map[string]interface{}{
				"daemon_reload": true,
			},
		},
	}
}

func (dm *DeploymentManager) validationTasks() []types.Task {
	return []types.Task{
		{
			Name:   "Start application service",
			Module: "systemd",
			Args: map[string]interface{}{
				"name":  "app-server",
				"state": "started",
			},
		},
		{
			Name:   "Wait for application to be ready",
			Module: "wait_for",
			Args: map[string]interface{}{
				"port":    8080,
				"delay":   5,
				"timeout": 30,
			},
		},
		{
			Name:   "Perform health check",
			Module: "uri",
			Args: map[string]interface{}{
				"url":         "http://localhost:8080/health",
				"status_code": 200,
			},
		},
		{
			Name:   "Run smoke tests",
			Module: "command",
			Args: map[string]interface{}{
				"cmd": filepath.Join(dm.basePath, "current/bin/cli") + " test --smoke",
			},
		},
	}
}

// ExecuteDeployment shows how to execute the deployment
func ExecuteDeployment(ctx context.Context) error {
	// Create deployment manager
	dm := NewDeploymentManager()

	// Register components
	if err := dm.RegisterComponents(); err != nil {
		return fmt.Errorf("failed to register components: %w", err)
	}

	// Create inventory
	inv, err := inventory.NewFromYAML([]byte(`
all:
  children:
    webservers:
      hosts:
        web1:
          ansible_host: 10.0.1.10
        web2:
          ansible_host: 10.0.1.11
        web3:
          ansible_host: 10.0.1.12
    workers:
      hosts:
        worker1:
          ansible_host: 10.0.2.10
        worker2:
          ansible_host: 10.0.2.11
`))
	if err != nil {
		return fmt.Errorf("failed to create inventory: %w", err)
	}

	// Get target hosts
	hosts, err := inv.GetHosts("webservers")
	if err != nil {
		return fmt.Errorf("failed to get hosts: %w", err)
	}

	// Create task runner
	tr := runner.NewTaskRunner()

	// Execute deployment tasks
	tasks := dm.CreateDeploymentTasks()
	for _, task := range tasks {
		fmt.Printf("Executing: %s\n", task.Name)

		results, err := tr.Run(ctx, task, hosts, nil)
		if err != nil {
			return fmt.Errorf("task %s failed: %w", task.Name, err)
		}

		// Check results
		for _, result := range results {
			if !result.Success {
				return fmt.Errorf("task %s failed on %s: %v",
					task.Name, result.Host, result.Error)
			}
		}
	}

	fmt.Println("Deployment completed successfully!")
	return nil
}
