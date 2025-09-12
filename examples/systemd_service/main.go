package main

import (
	"context"
	"fmt"
	"log"

	"github.com/liliang-cn/gosible/pkg/connection"
	"github.com/liliang-cn/gosiblepkg/modules"
	"github.com/liliang-cn/gosiblepkg/types"
)

func main() {
	// Create a local connection
	conn := connection.NewLocalConnection()
	ctx := context.Background()

	// Connect
	err := conn.Connect(ctx, types.ConnectionInfo{
		Host: "localhost",
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Example 1: Check systemd service status
	fmt.Println("=== Checking Docker Service Status ===")
	systemdModule := modules.NewSystemdModule()
	
	result, err := systemdModule.Run(ctx, conn, map[string]interface{}{
		"name": "docker",
		"state": "started",
		"_check_mode": true,  // Run in check mode to avoid changes
	})
	
	if err != nil {
		log.Printf("Error checking docker service: %v", err)
	} else {
		fmt.Printf("Success: %v\n", result.Success)
		fmt.Printf("Changed: %v\n", result.Changed)
		fmt.Printf("Message: %s\n", result.Message)
		if result.Data != nil {
			fmt.Printf("Service State: %v\n", result.Data["state"])
		}
	}

	// Example 2: Manage a file with LineInFile module
	fmt.Println("\n=== Managing Configuration File ===")
	lineModule := modules.NewLineInFileModule()
	
	result, err = lineModule.Run(ctx, conn, map[string]interface{}{
		"path": "/tmp/test_config.txt",
		"line": "# Configuration managed by gosible,
		"create": true,
		"state": "present",
	})
	
	if err != nil {
		log.Printf("Error managing file: %v", err)
	} else {
		fmt.Printf("Success: %v\n", result.Success)
		fmt.Printf("Changed: %v\n", result.Changed)
		fmt.Printf("Message: %s\n", result.Message)
	}

	// Example 3: Install a package with pip
	fmt.Println("\n=== Python Package Management ===")
	pipModule := modules.NewPipModule()
	
	result, err = pipModule.Run(ctx, conn, map[string]interface{}{
		"name": "requests",
		"state": "present",
		"_check_mode": true,  // Check mode to avoid actual installation
	})
	
	if err != nil {
		log.Printf("Error with pip module: %v", err)
	} else {
		fmt.Printf("Success: %v\n", result.Success)
		fmt.Printf("Changed: %v\n", result.Changed)
		fmt.Printf("Message: %s\n", result.Message)
	}

	// Example 4: Create a cron job
	fmt.Println("\n=== Cron Job Management ===")
	cronModule := modules.NewCronModule()
	
	result, err = cronModule.Run(ctx, conn, map[string]interface{}{
		"name": "backup_database",
		"minute": "0",
		"hour": "2",
		"job": "/usr/local/bin/backup.sh",
		"state": "present",
		"_check_mode": true,  // Check mode
	})
	
	if err != nil {
		log.Printf("Error with cron module: %v", err)
	} else {
		fmt.Printf("Success: %v\n", result.Success)
		fmt.Printf("Changed: %v\n", result.Changed)
		fmt.Printf("Message: %s\n", result.Message)
	}

	// Example 5: Archive creation
	fmt.Println("\n=== Archive Creation ===")
	archiveModule := modules.NewArchiveModule()
	
	result, err = archiveModule.Run(ctx, conn, map[string]interface{}{
		"path": "/tmp/test_config.txt",
		"dest": "/tmp/config_backup.tar.gz",
		"format": "gz",
		"_check_mode": true,
	})
	
	if err != nil {
		log.Printf("Error with archive module: %v", err)
	} else {
		fmt.Printf("Success: %v\n", result.Success)
		fmt.Printf("Changed: %v\n", result.Changed)
		fmt.Printf("Message: %s\n", result.Message)
	}

	fmt.Println("\n=== Example Complete ===")
}