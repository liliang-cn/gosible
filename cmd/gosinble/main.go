package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	
	"github.com/gosinble/gosinble/pkg/inventory"
	"github.com/gosinble/gosinble/pkg/playbook"
	"github.com/gosinble/gosinble/pkg/runner"
	"github.com/gosinble/gosinble/pkg/types"
	"gopkg.in/yaml.v3"
)

var (
	version = "1.0.0"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	var (
		inventoryFile = flag.String("i", "", "Inventory file (required)")
		playbookFile  = flag.String("p", "", "Playbook file to execute")
		moduleCmd     = flag.String("m", "", "Module to execute")
		moduleArgs    = flag.String("a", "", "Module arguments (key=value pairs)")
		hosts         = flag.String("hosts", "all", "Host pattern to match")
		check         = flag.Bool("check", false, "Run in check mode (dry run)")
		diff          = flag.Bool("diff", false, "Show differences")
		verbose       = flag.Bool("v", false, "Verbose output")
		versionFlag   = flag.Bool("version", false, "Show version information")
		listHosts     = flag.Bool("list-hosts", false, "List matching hosts")
		listTasks     = flag.Bool("list-tasks", false, "List tasks in playbook")
		become        = flag.Bool("b", false, "Run with become (sudo)")
		becomeUser    = flag.String("become-user", "root", "User to become")
		forks         = flag.Int("f", 5, "Number of parallel processes")
		extraVars     = flag.String("e", "", "Extra variables (key=value pairs or @file.yml)")
	)
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Gosinble - Ansible-compatible automation tool in Go\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s -i INVENTORY -p PLAYBOOK [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -i INVENTORY -m MODULE -a ARGS [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Run a playbook\n")
		fmt.Fprintf(os.Stderr, "  %s -i inventory.yml -p playbook.yml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  # Run an ad-hoc command\n")
		fmt.Fprintf(os.Stderr, "  %s -i inventory.yml -m ping\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  # Install a package\n")
		fmt.Fprintf(os.Stderr, "  %s -i inventory.yml -m apt -a \"name=nginx state=present\"\n", os.Args[0])
	}
	
	flag.Parse()
	
	// Show version
	if *versionFlag {
		fmt.Printf("Gosinble version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}
	
	// Validate required arguments
	if *inventoryFile == "" {
		fmt.Fprintf(os.Stderr, "Error: inventory file is required (-i)\n\n")
		flag.Usage()
		os.Exit(1)
	}
	
	if *playbookFile == "" && *moduleCmd == "" {
		fmt.Fprintf(os.Stderr, "Error: either playbook (-p) or module (-m) is required\n\n")
		flag.Usage()
		os.Exit(1)
	}
	
	// Load inventory
	inv, err := loadInventory(*inventoryFile)
	if err != nil {
		log.Fatalf("Failed to load inventory: %v", err)
	}
	
	// List hosts if requested
	if *listHosts {
		matchedHosts, err := inv.GetHosts(*hosts)
		if err != nil {
			log.Fatalf("Failed to get hosts: %v", err)
		}
		fmt.Printf("Matched hosts (%d):\n", len(matchedHosts))
		for _, host := range matchedHosts {
			fmt.Printf("  %s\n", host.Name)
		}
		os.Exit(0)
	}
	
	// Parse extra variables
	vars := make(map[string]interface{})
	if *extraVars != "" {
		vars = parseExtraVars(*extraVars)
	}
	
	// Add runtime variables
	vars["ansible_check_mode"] = *check
	vars["ansible_diff_mode"] = *diff
	vars["ansible_become"] = *become
	vars["ansible_become_user"] = *becomeUser
	vars["ansible_forks"] = *forks
	
	ctx := context.Background()
	
	if *playbookFile != "" {
		// Execute playbook
		if err := runPlaybook(ctx, *playbookFile, inv, vars, *listTasks, *verbose); err != nil {
			log.Fatalf("Playbook execution failed: %v", err)
		}
	} else if *moduleCmd != "" {
		// Execute ad-hoc command
		if err := runAdHoc(ctx, *moduleCmd, *moduleArgs, *hosts, inv, vars, *verbose); err != nil {
			log.Fatalf("Ad-hoc command failed: %v", err)
		}
	}
}

// loadInventory loads inventory from a file
func loadInventory(filename string) (*inventory.StaticInventory, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read inventory file: %w", err)
	}
	
	// Try YAML format
	inv, err := inventory.NewFromYAML(data)
	if err == nil {
		return inv, nil
	}
	
	// Try INI format (not implemented yet, but would go here)
	return nil, fmt.Errorf("failed to parse inventory: %w", err)
}

// runPlaybook executes a playbook
func runPlaybook(ctx context.Context, filename string, inv *inventory.StaticInventory, vars map[string]interface{}, listTasks, verbose bool) error {
	// Read playbook file
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read playbook: %w", err)
	}
	
	// Parse playbook
	var pb types.Playbook
	if err := yaml.Unmarshal(data, &pb); err != nil {
		// Try parsing as list of plays
		var plays []types.Play
		if err := yaml.Unmarshal(data, &plays); err != nil {
			return fmt.Errorf("failed to parse playbook: %w", err)
		}
		pb.Plays = plays
	}
	
	// List tasks if requested
	if listTasks {
		fmt.Printf("Playbook: %s\n\n", filename)
		for i, play := range pb.Plays {
			fmt.Printf("Play #%d: %s\n", i+1, play.Name)
			fmt.Printf("  Hosts: %s\n", play.Hosts)
			fmt.Printf("  Tasks:\n")
			for j, task := range play.Tasks {
				fmt.Printf("    %d. %s\n", j+1, task.Name)
			}
			fmt.Println()
		}
		return nil
	}
	
	// Create playbook executor
	taskRunner := runner.NewTaskRunner()
	executor := playbook.NewExecutor(taskRunner, inv, nil)
	
	// Execute playbook
	if verbose {
		fmt.Printf("Executing playbook: %s\n", filename)
	}
	
	results, err := executor.Execute(ctx, &pb, vars)
	if err != nil {
		return fmt.Errorf("playbook execution failed: %w", err)
	}
	
	// Display results
	displayResults(results, verbose)
	
	// Check for failures
	for _, result := range results {
		if !result.Success {
			return fmt.Errorf("playbook execution had failures")
		}
	}
	
	return nil
}

// runAdHoc executes an ad-hoc command
func runAdHoc(ctx context.Context, module, args, hostPattern string, inv *inventory.StaticInventory, vars map[string]interface{}, verbose bool) error {
	// Get matching hosts
	hosts, err := inv.GetHosts(hostPattern)
	if err != nil {
		return fmt.Errorf("failed to get hosts: %w", err)
	}
	
	if len(hosts) == 0 {
		return fmt.Errorf("no hosts matched pattern: %s", hostPattern)
	}
	
	// Parse module arguments
	moduleArgs := parseModuleArgs(args)
	
	// Create task
	task := types.Task{
		Name:   fmt.Sprintf("Ad-hoc: %s", module),
		Module: types.ModuleType(module),
		Args:   moduleArgs,
	}
	
	// Create runner
	taskRunner := runner.NewTaskRunner()
	
	// Execute task
	if verbose {
		fmt.Printf("Executing module '%s' on %d hosts\n", module, len(hosts))
	}
	
	results, err := taskRunner.Run(ctx, task, hosts, vars)
	if err != nil {
		return fmt.Errorf("task execution failed: %w", err)
	}
	
	// Display results
	displayResults(results, verbose)
	
	// Check for failures
	for _, result := range results {
		if !result.Success {
			return fmt.Errorf("task execution had failures")
		}
	}
	
	return nil
}

// parseModuleArgs parses module arguments from string
func parseModuleArgs(args string) map[string]interface{} {
	result := make(map[string]interface{})
	
	if args == "" {
		return result
	}
	
	// Simple key=value parsing
	pairs := strings.Fields(args)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, "\"'")
			result[key] = value
		}
	}
	
	return result
}

// parseExtraVars parses extra variables
func parseExtraVars(vars string) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Check if it's a file reference
	if strings.HasPrefix(vars, "@") {
		filename := vars[1:]
		data, err := os.ReadFile(filename)
		if err != nil {
			log.Printf("Warning: failed to read vars file %s: %v", filename, err)
			return result
		}
		
		if err := yaml.Unmarshal(data, &result); err != nil {
			log.Printf("Warning: failed to parse vars file %s: %v", filename, err)
		}
		return result
	}
	
	// Parse as key=value pairs
	return parseModuleArgs(vars)
}

// displayResults displays task execution results
func displayResults(results []types.Result, verbose bool) {
	successCount := 0
	failureCount := 0
	changedCount := 0
	
	for _, result := range results {
		if result.Success {
			successCount++
			if result.Changed {
				changedCount++
				fmt.Printf("changed: [%s] => %s\n", result.Host, result.TaskName)
			} else {
				if verbose {
					fmt.Printf("ok: [%s] => %s\n", result.Host, result.TaskName)
				}
			}
		} else {
			failureCount++
			fmt.Printf("failed: [%s] => %s: %v\n", result.Host, result.TaskName, result.Error)
		}
		
		// Show output if verbose
		if verbose && result.Message != "" {
			fmt.Printf("  Output: %s\n", result.Message)
		}
	}
	
	// Summary
	fmt.Printf("\nPLAY RECAP *********************************************************************\n")
	hostSummary := make(map[string]struct {
		ok      int
		changed int
		failed  int
	})
	
	for _, result := range results {
		summary := hostSummary[result.Host]
		if result.Success {
			summary.ok++
			if result.Changed {
				summary.changed++
			}
		} else {
			summary.failed++
		}
		hostSummary[result.Host] = summary
	}
	
	for host, summary := range hostSummary {
		fmt.Printf("%-20s : ok=%-3d changed=%-3d unreachable=%-3d failed=%-3d\n",
			host, summary.ok, summary.changed, 0, summary.failed)
	}
}

// Built-in help command
func showBuiltinModules() {
	fmt.Println("Built-in modules:")
	fmt.Println("  ping         - Test connectivity")
	fmt.Println("  command      - Execute shell commands")
	fmt.Println("  shell        - Execute shell commands (with shell features)")
	fmt.Println("  copy         - Copy files to remote hosts")
	fmt.Println("  file         - Manage files and directories")
	fmt.Println("  template     - Deploy files from templates")
	fmt.Println("  apt          - Manage apt packages (Debian/Ubuntu)")
	fmt.Println("  yum          - Manage yum packages (RedHat/CentOS)")
	fmt.Println("  service      - Manage services")
	fmt.Println("  systemd      - Manage systemd services")
	fmt.Println("  user         - Manage user accounts")
	fmt.Println("  group        - Manage groups")
	fmt.Println("  lineinfile   - Manage lines in files")
	fmt.Println("  debug        - Print debug messages")
	fmt.Println("  setup        - Gather facts about hosts")
}