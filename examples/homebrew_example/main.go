package main

import (
	"context"
	"fmt"
	"log"

	"github.com/liliang-cn/gosible/pkg/connection"
	"github.com/liliang-cn/gosible/pkg/modules"
	"github.com/liliang-cn/gosible/pkg/types"
)

func main() {
	fmt.Println("=== Homebrew Module Example ===")
	
	// Create a local connection (for macOS)
	conn := connection.NewLocalConnection()
	ctx := context.Background()
	
	// Connect
	err := conn.Connect(ctx, types.ConnectionInfo{
		Host: "localhost",
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// Create homebrew module
	homebrewModule := modules.NewHomebrewModule()
	
	// Example 1: Install a package
	fmt.Println("\n1. Installing wget...")
	result, err := homebrewModule.Run(ctx, conn, map[string]interface{}{
		"name":  "wget",
		"state": "present",
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("   Success: %v, Changed: %v\n", result.Success, result.Changed)
		fmt.Printf("   Message: %s\n", result.Message)
	}
	
	// Example 2: Install multiple packages
	fmt.Println("\n2. Installing multiple packages...")
	result, err = homebrewModule.Run(ctx, conn, map[string]interface{}{
		"names": []interface{}{"jq", "tree", "htop"},
		"state": "present",
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("   Success: %v, Changed: %v\n", result.Success, result.Changed)
		fmt.Printf("   Message: %s\n", result.Message)
	}
	
	// Example 3: Install a cask application
	fmt.Println("\n3. Installing Visual Studio Code (cask)...")
	result, err = homebrewModule.Run(ctx, conn, map[string]interface{}{
		"name":  "visual-studio-code",
		"state": "present",
		"cask":  true,
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("   Success: %v, Changed: %v\n", result.Success, result.Changed)
		fmt.Printf("   Message: %s\n", result.Message)
	}
	
	// Example 4: Update homebrew
	fmt.Println("\n4. Updating Homebrew...")
	result, err = homebrewModule.Run(ctx, conn, map[string]interface{}{
		"update_homebrew": true,
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("   Success: %v, Changed: %v\n", result.Success, result.Changed)
		fmt.Printf("   Message: %s\n", result.Message)
	}
	
	// Example 5: Upgrade all packages
	fmt.Println("\n5. Upgrading all packages...")
	result, err = homebrewModule.Run(ctx, conn, map[string]interface{}{
		"upgrade_all": true,
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("   Success: %v, Changed: %v\n", result.Success, result.Changed)
		fmt.Printf("   Message: %s\n", result.Message)
	}
	
	// Example 6: Remove a package
	fmt.Println("\n6. Removing a package...")
	result, err = homebrewModule.Run(ctx, conn, map[string]interface{}{
		"name":  "wget",
		"state": "absent",
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("   Success: %v, Changed: %v\n", result.Success, result.Changed)
		fmt.Printf("   Message: %s\n", result.Message)
	}
	
	fmt.Println("\n=== Example Complete ===")
}