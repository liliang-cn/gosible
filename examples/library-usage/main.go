// Package main provides examples of how to use the gosible library.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/liliang-cn/gosiblepkg/config"
	"github.com/liliang-cn/gosiblepkg/inventory"
	"github.com/liliang-cn/gosiblepkg/playbook"
	"github.com/liliang-cn/gosiblepkg/runner"
	"github.com/liliang-cn/gosiblepkg/template"
	"github.com/liliang-cn/gosiblepkg/types"
	"github.com/liliang-cn/gosiblepkg/vars"
)

func main() {
	fmt.Println("=== gosible Library Examples ===")

	// Run all examples
	runInventoryExample()
	runModuleExample()
	runTemplateExample()
	runConfigExample()
	runPlaybookExample()
	runFactsExample()
}

// runInventoryExample demonstrates inventory management
func runInventoryExample() {
	fmt.Println("1. Inventory Management Example")
	fmt.Println("------------------------------")

	// Create inventory
	inv := inventory.NewStaticInventory()

	// Add hosts
	webHosts := []types.Host{
		{Name: "web1", Address: "10.43.3.109", Port: 22, User: "root", Groups: []string{"webservers"}},
		{Name: "web2", Address: "10.43.3.110", Port: 22, User: "root", Groups: []string{"webservers"}},
	}

	dbHosts := []types.Host{
		{Name: "db1", Address: "10.43.3.111", Port: 22, User: "root", Groups: []string{"databases"}},
		{Name: "localhost", Address: "127.0.0.1", Port: 22, Groups: []string{"local"}},
	}

	for _, host := range append(webHosts, dbHosts...) {
		if err := inv.AddHost(host); err != nil {
			log.Printf("Failed to add host %s: %v", host.Name, err)
		}
	}

	// Query hosts
	allHosts, _ := inv.GetHosts("*")
	fmt.Printf("Total hosts: %d\n", len(allHosts))

	webServers, _ := inv.GetHosts("webservers")
	fmt.Printf("Web servers: %d\n", len(webServers))

	// Show host variables
	if len(webServers) > 0 {
		hostVars, _ := inv.GetHostVars(webServers[0].Name)
		fmt.Printf("Host %s variables: %d\n", webServers[0].Name, len(hostVars))
	}

	fmt.Println()
}

// runModuleExample demonstrates module execution
func runModuleExample() {
	fmt.Println("2. Module Execution Example")
	fmt.Println("---------------------------")

	// Create task runner
	taskRunner := runner.NewTaskRunner()

	// Create localhost host
	localhost := []types.Host{
		{Name: "localhost", Address: "127.0.0.1"},
	}

	// Execute command module
	ctx := context.Background()
	results, err := taskRunner.ExecuteTask(ctx,
		"Get hostname",
		"command",
		map[string]interface{}{"cmd": "hostname"},
		localhost,
		nil,
	)

	if err != nil {
		log.Printf("Command execution failed: %v", err)
	} else if len(results) > 0 {
		result := results[0]
		fmt.Printf("Command result: Success=%v, Host=%s\n", result.Success, result.Host)
		if result.Data != nil {
			if stdout, ok := result.Data["stdout"].(string); ok {
				fmt.Printf("Output: %s", stdout)
			}
		}
	}

	// Execute debug module
	debugResults, err := taskRunner.ExecuteTask(ctx,
		"Debug message",
		"debug",
		map[string]interface{}{"msg": "Hello from gosible!"},
		localhost,
		map[string]interface{}{"custom_var": "custom_value"},
	)

	if err != nil {
		log.Printf("Debug execution failed: %v", err)
	} else if len(debugResults) > 0 {
		result := debugResults[0]
		fmt.Printf("Debug result: Success=%v, Message=%s\n", result.Success, result.Message)
	}

	fmt.Println()
}

// runTemplateExample demonstrates template rendering
func runTemplateExample() {
	fmt.Println("3. Template Engine Example")
	fmt.Println("--------------------------")

	engine := template.NewEngine()

	templateStr := `
Hello {{.name | title}}!

Your details:
- Email: {{.email | lower}}
- Active: {{ternary .active "Yes" "No"}}
- Items: {{join ", " .items}}

{{if .admin}}You have admin privileges.{{end}}
`

	vars := map[string]interface{}{
		"name":   "alice johnson",
		"email":  "ALICE@EXAMPLE.COM",
		"active": true,
		"admin":  false,
		"items":  []string{"item1", "item2", "item3"},
	}

	result, err := engine.Render(templateStr, vars)
	if err != nil {
		log.Printf("Template rendering failed: %v", err)
	} else {
		fmt.Printf("Rendered template:\n%s\n", result)
	}
}

// runConfigExample demonstrates configuration management
func runConfigExample() {
	fmt.Println("4. Configuration Management Example")
	fmt.Println("-----------------------------------")

	cfg := config.NewConfig()

	// Show some default values
	fmt.Printf("Default timeout: %d seconds\n", cfg.GetInt("timeout"))
	fmt.Printf("Default forks: %d\n", cfg.GetInt("forks"))
	fmt.Printf("Gather facts: %v\n", cfg.GetBool("gather_facts"))

	// Set custom values
	cfg.SetString("custom_setting", "example_value")
	cfg.SetInt("custom_timeout", 60)

	fmt.Printf("Custom setting: %s\n", cfg.GetString("custom_setting"))
	fmt.Printf("Custom timeout: %d\n", cfg.GetInt("custom_timeout"))

	// Show validation
	if err := cfg.Validate(); err != nil {
		log.Printf("Configuration validation failed: %v", err)
	} else {
		fmt.Println("Configuration is valid")
	}

	fmt.Println()
}

// runPlaybookExample demonstrates playbook parsing and execution
func runPlaybookExample() {
	fmt.Println("5. Playbook Example")
	fmt.Println("-------------------")

	// Create a simple playbook in memory
	playbookYAML := `---
- name: Simple test playbook
  hosts: localhost
  vars:
    message: "Hello from playbook"
  tasks:
    - name: Show message
      debug:
        msg: "{{message}}"
    
    - name: Get system info
      command:
        cmd: "uname -a"
`

	// Parse the playbook
	parser := playbook.NewParser()
	pb, err := parser.Parse([]byte(playbookYAML), "example.yml")
	if err != nil {
		log.Printf("Playbook parsing failed: %v", err)
		fmt.Println()
		return
	}

	fmt.Printf("Parsed playbook with %d plays\n", len(pb.Plays))
	if len(pb.Plays) > 0 {
		play := pb.Plays[0]
		fmt.Printf("First play: %s (%d tasks)\n", play.Name, len(play.Tasks))

		// Show task names
		for i, task := range play.Tasks {
			fmt.Printf("  Task %d: %s (%s module)\n", i+1, task.Name, task.Module)
		}
	}

	fmt.Println()
}

// runFactsExample demonstrates fact gathering
func runFactsExample() {
	fmt.Println("6. Facts Gathering Example")
	fmt.Println("--------------------------")

	varMgr := vars.NewVarManager()

	// Set some variables
	varMgr.SetVar("environment", "development")
	varMgr.SetVar("app_version", "1.0.0")

	// Get all variables (would include facts if gathered)
	allVars := varMgr.GetVars()
	fmt.Printf("Total variables: %d\n", len(allVars))

	// Show some variables
	for key, value := range allVars {
		if key == "environment" || key == "app_version" {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	// Demonstrate variable merging
	base := map[string]interface{}{
		"db_host": "localhost",
		"db_port": 5432,
	}

	override := map[string]interface{}{
		"db_port": 5433,    // Override port
		"db_name": "myapp", // Add database name
	}

	merged := varMgr.MergeVars(base, override)
	fmt.Println("\nMerged variables:")
	for key, value := range merged {
		fmt.Printf("  %s: %v\n", key, value)
	}

	fmt.Println()
}

// Demonstration of how to create a complete automation workflow
func runCompleteExample() {
	fmt.Println("7. Complete Workflow Example")
	fmt.Println("----------------------------")

	// 1. Load configuration
	cfg := config.NewConfig()
	cfg.SetInt("forks", 3) // Parallel execution

	// 2. Create inventory
	inv := inventory.NewStaticInventory()

	// Add test hosts - these are the VMs provided in the requirements
	testHosts := []types.Host{
		{Name: "obfy11", Address: "10.43.3.109", User: "root"},
		{Name: "obfy12", Address: "10.43.3.110", User: "root"},
		{Name: "localhost", Address: "127.0.0.1"},
	}

	for _, host := range testHosts {
		inv.AddHost(host)
	}

	// 3. Create task runner with configuration
	taskRunner := runner.NewTaskRunner()
	taskRunner.SetMaxConcurrency(cfg.GetInt("forks"))

	// 4. Define automation tasks
	tasks := []types.Task{
		{
			Name:   "Gather system facts",
			Module: "setup",
			Args:   map[string]interface{}{},
		},
		{
			Name:   "Check disk space",
			Module: "command",
			Args:   map[string]interface{}{"cmd": "df -h /"},
		},
		{
			Name:   "Show hostname",
			Module: "debug",
			Args:   map[string]interface{}{"msg": "Host: {{inventory_hostname}}"},
		},
	}

	// 5. Execute tasks
	ctx := context.Background()
	hosts, _ := inv.GetHosts("*")

	fmt.Printf("Executing automation on %d hosts:\n", len(hosts))

	for _, task := range tasks {
		fmt.Printf("Running task: %s\n", task.Name)

		results, err := taskRunner.Run(ctx, task, hosts, map[string]interface{}{
			"environment": "example",
		})

		if err != nil {
			log.Printf("Task failed: %v", err)
			continue
		}

		// Show results summary
		successful := 0
		for _, result := range results {
			if result.Success {
				successful++
			}
		}
		fmt.Printf("  Results: %d/%d successful\n", successful, len(results))
	}

	// 6. Cleanup
	taskRunner.Close()

	fmt.Println("Workflow completed!")
}

// Helper function to create example files
func createExampleFiles() {
	// Create example inventory file
	inventoryYAML := `---
all:
  hosts:
    web1:
      address: 10.43.3.109
      user: root
      vars:
        role: webserver
    web2:
      address: 10.43.3.110
      user: root
      vars:
        role: webserver
    db1:
      address: 10.43.3.111
      user: root
      vars:
        role: database
  children:
    webservers:
      hosts:
        - web1
        - web2
      vars:
        http_port: 80
    databases:
      hosts:
        - db1
      vars:
        db_port: 5432
`

	if err := os.WriteFile("example_inventory.yml", []byte(inventoryYAML), 0644); err != nil {
		log.Printf("Failed to create example inventory: %v", err)
	}

	// Create example playbook
	playbookYAML := `---
- name: Web server setup
  hosts: webservers
  vars:
    app_name: myapp
  tasks:
    - name: Install web server
      debug:
        msg: "Installing web server on {{inventory_hostname}}"
    
    - name: Configure web server
      template:
        src: config.j2
        dest: /etc/app/config.conf
    
    - name: Start services
      command:
        cmd: "systemctl start nginx"
      become: yes

- name: Database setup
  hosts: databases
  tasks:
    - name: Install database
      debug:
        msg: "Installing database on {{inventory_hostname}}"
`

	if err := os.WriteFile("example_playbook.yml", []byte(playbookYAML), 0644); err != nil {
		log.Printf("Failed to create example playbook: %v", err)
	}

	// Create example template
	templateContent := `# Configuration for {{app_name}}
server {
    listen {{http_port | default 80}};
    server_name {{inventory_hostname}};
    
    location / {
        root /var/www/{{app_name}};
        index index.html;
    }
}
`

	if err := os.WriteFile("config.j2", []byte(templateContent), 0644); err != nil {
		log.Printf("Failed to create example template: %v", err)
	}

	fmt.Println("Example files created:")
	fmt.Println("  - example_inventory.yml")
	fmt.Println("  - example_playbook.yml")
	fmt.Println("  - config.j2")
}
