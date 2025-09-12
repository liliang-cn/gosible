// Example demonstrating the new type-safe module API
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/liliang-cn/gosible/pkg/types"
	"github.com/liliang-cn/gosible/pkg/inventory"
	"github.com/liliang-cn/gosible/pkg/runner"
)

func main() {
	ctx := context.Background()

	// Create inventory
	inv := inventory.NewStaticInventory()
	localhost := &types.Host{
		Name:    "localhost",
		Address: "127.0.0.1",
		User:    "root",
		Variables: map[string]interface{}{
			"env": "development",
		},
	}
	inv.AddHost(*localhost)

	// Create task runner
	taskRunner := runner.NewTaskRunner()

	// Example 1: Type-safe task creation with constants
	tasks := []types.Task{
		{
			Name:   "Create directory",
			Module: types.TypeFile, // Type-safe module reference
			Args: map[string]interface{}{
				"path":  "/tmp/gosibletest",
				"state": types.StateDirectory, // Type-safe state constant
				"mode":  "0755",
			},
		},
		{
			Name:   "Copy configuration file",
			Module: types.TypeCopy,
			Args: map[string]interface{}{
				"content": "# gosible test configuration\nversion: 1.0\n",
				"dest":    "/tmp/gosibletest/config.yml",
				"mode":    "0644",
			},
		},
		{
			Name:   "Execute test command",
			Module: types.TypeCommand,
			Args: map[string]interface{}{
				"cmd": "ls -la /tmp/gosibletest",
			},
		},
		{
			Name:   "Display results",
			Module: types.TypeDebug,
			Args: map[string]interface{}{
				"msg": "Type-safe deployment completed successfully!",
			},
		},
	}

	// Execute tasks
	fmt.Println("🚀 Executing type-safe tasks...")

	for i, task := range tasks {
		fmt.Printf("\n[%d/4] %s (Module: %s)\n", i+1, task.Name, task.Module.String())

		// Validate module type
		if !task.Module.IsValid() {
			log.Printf("⚠️  Warning: Invalid module type: %s", task.Module)
			continue
		}

		hosts, _ := inv.GetHosts("all")
		results, err := taskRunner.Run(ctx, task, hosts, nil)
		if err != nil {
			log.Printf("❌ Task failed: %v", err)
			continue
		}

		for _, result := range results {
			if result.Success {
				if result.Changed {
					fmt.Printf("✅ Changed: %s\n", result.Message)
				} else {
					fmt.Printf("✨ OK: %s\n", result.Message)
				}
			} else {
				fmt.Printf("❌ Failed: %v\n", result.Error)
			}
		}
	}

	// Example 2: Using module validation
	fmt.Println("\n📋 Module Type Validation Examples:")

	testModules := []types.ModuleType{
		types.TypeService,
		types.TypePackage,
		"invalid_module", // This will be invalid
		types.TypeTemplate,
	}

	for _, moduleType := range testModules {
		if moduleType.IsValid() {
			fmt.Printf("✅ %s is a valid module type\n", moduleType)
		} else {
			fmt.Printf("❌ %s is NOT a valid module type\n", moduleType)
		}
	}

	// Example 3: List all available module types
	fmt.Println("\n📦 Available Module Types:")
	for _, moduleType := range types.AllModuleTypes() {
		fmt.Printf("  - %s\n", moduleType.String())
	}

	// Example 4: OBFY RustFS deployment example using type-safe modules
	fmt.Println("\n🦀 OBFY RustFS Deployment Example:")

	rustfsDeploymentTasks := createRustFSDeploymentTasks()

	for _, task := range rustfsDeploymentTasks {
		fmt.Printf("📋 Task: %s (Module: %s)\n", task.Name, task.Module.String())
	}
}

// createRustFSDeploymentTasks creates type-safe tasks for OBFY RustFS deployment
func createRustFSDeploymentTasks() []types.Task {
	return []types.Task{
		{
			Name:   "Test connectivity",
			Module: types.TypePing,
			Args:   map[string]interface{}{},
		},
		{
			Name:   "Create RustFS directories",
			Module: types.TypeFile,
			Args: map[string]interface{}{
				"path":  "{{ item }}",
				"state": types.StateDirectory,
				"mode":  "0750",
			},
			Loop: []string{"/data/sda", "/data/vda", "/data/nvme0n1", "/var/log/rustfs", "/opt/tls"},
		},
		{
			Name:   "Copy RustFS binary",
			Module: types.TypeCopy,
			Args: map[string]interface{}{
				"src":   "./rustfs",
				"dest":  "/usr/local/bin/rustfs",
				"mode":  "0755",
				"owner": "root",
				"group": "root",
			},
		},
		{
			Name:   "Format data disks",
			Module: types.TypeShell,
			Args: map[string]interface{}{
				"cmd": "mkfs.xfs -i size=512 -n ftype=1 -L {{ item | basename }} {{ item }}",
			},
			Loop: "{{ data_disks }}",
		},
		{
			Name:   "Generate RustFS configuration",
			Module: types.TypeTemplate,
			Args: map[string]interface{}{
				"src":  "rustfs.conf.j2",
				"dest": "/etc/default/rustfs",
				"mode": "0644",
			},
		},
		{
			Name:   "Create systemd service file",
			Module: types.TypeTemplate,
			Args: map[string]interface{}{
				"src":  "rustfs.service.j2",
				"dest": "/etc/systemd/system/rustfs.service",
				"mode": "0644",
			},
		},
		{
			Name:   "Start RustFS service",
			Module: types.TypeService,
			Args: map[string]interface{}{
				"name":          "rustfs",
				"state":         types.StateStarted,
				"enabled":       true,
				"daemon_reload": true,
			},
		},
		{
			Name:   "Verify RustFS is running",
			Module: types.TypeCommand,
			Args: map[string]interface{}{
				"cmd": "systemctl is-active rustfs",
			},
		},
		{
			Name:   "Display deployment status",
			Module: types.TypeDebug,
			Args: map[string]interface{}{
				"msg": "RustFS deployment completed! Service is {{ ansible_facts['services']['rustfs']['state'] }}",
			},
		},
	}
}
