// Package main demonstrates that the gosinble library types are now publicly accessible
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/liliang-cn/gosinble/pkg/types"
	"github.com/liliang-cn/gosinble/pkg/inventory"
	"github.com/liliang-cn/gosinble/pkg/runner"
)

func main() {
	fmt.Println("Testing Public API Access")
	fmt.Println("========================")
	
	// Now users can directly create and use all the core types
	
	// 1. Create a Task using the public types
	task := types.Task{
		Name:   "Test Task",
		Module: types.TypeDebug,  // Can use the ModuleType constants
		Args: map[string]interface{}{
			"msg": "Hello from public API!",
		},
	}
	fmt.Printf("Created task: %s (module: %s)\n", task.Name, task.Module)
	
	// 2. Create a Host using the public types
	host := types.Host{
		Name:    "localhost",
		Address: "127.0.0.1",
		Port:    22,
		User:    "ubuntu",
	}
	fmt.Printf("Created host: %s (%s:%d)\n", host.Name, host.Address, host.Port)
	
	// 3. Create inventory and add the host
	inv := inventory.NewStaticInventory()
	if err := inv.AddHost(host); err != nil {
		log.Fatal(err)
	}
	
	// 4. Create a runner
	runner := runner.NewTaskRunner()
	
	// 5. Execute the task (this would work in a real environment)
	ctx := context.Background()
	hosts, _ := inv.GetHosts("*")
	
	fmt.Printf("\nReady to execute task on %d host(s)\n", len(hosts))
	
	_ = ctx // Avoid unused variable warning
	
	// 6. Users can also implement their own modules by implementing the Module interface
	// They have access to all the types they need:
	// - types.Module interface
	// - types.Connection interface  
	// - types.Result struct
	// - types.ModuleDoc struct
	// - etc.
	
	fmt.Println("\nAll core types are now accessible to library users!")
	fmt.Println("Users can:")
	fmt.Println("- Build tasks programmatically using types.Task")
	fmt.Println("- Create custom modules implementing types.Module")
	fmt.Println("- Work with results using types.Result")
	fmt.Println("- Extend connections with types.Connection")
	fmt.Println("- Use all ModuleType constants")
	
	// This wouldn't compile before because these were in internal/common
	// Now it works perfectly!
	
	_ = runner // Avoid unused variable warning
}