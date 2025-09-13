package main

import (
	"fmt"
	"log"

	"github.com/liliang-cn/gosible/pkg/modules"
)

func main() {
	// Create a registry and register built-in modules
	registry := modules.NewModuleRegistry()
	registry.RegisterBuiltinModules()

	// Check if systemd module is registered
	systemdModule, err := registry.GetModule("systemd")
	if err != nil {
		log.Fatalf("Failed to get systemd module: %v", err)
	}

	fmt.Printf("✓ Systemd module found: %s\n", systemdModule.Name())
	
	// List all registered modules
	fmt.Println("\nAll registered modules:")
	for _, name := range registry.ListModules() {
		fmt.Printf("  - %s\n", name)
	}
}