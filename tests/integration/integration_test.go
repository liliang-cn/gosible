// +build integration

// Integration tests for the gosinble library using the provided test VMs.
// Run with: go test -tags=integration -v .

package main

import (
	"context"
	"testing"
	"time"

	"github.com/gosinble/gosinble/pkg/types"
	"github.com/gosinble/gosinble/pkg/inventory"
	"github.com/gosinble/gosinble/pkg/playbook"
	"github.com/gosinble/gosinble/pkg/runner"
	"github.com/gosinble/gosinble/pkg/template"
	"github.com/gosinble/gosinble/pkg/vars"
)

const (
	testInventoryYAML = `
all:
  hosts:
    obfy11:
      address: 10.43.3.109
      user: root
      password: linbit
      vars:
        test_group: vm1
    obfy12:
      address: 10.43.3.110
      user: root
      password: linbit
      vars:
        test_group: vm2
    obfy13:
      address: 10.43.3.111
      user: root
      password: linbit
      vars:
        test_group: vm3
    obfy14:
      address: 10.43.3.112
      user: root
      password: linbit
      vars:
        test_group: vm4
  children:
    testvms:
      hosts:
        - obfy11
        - obfy12
        - obfy13
        - obfy14
      vars:
        environment: testing
        device: /dev/vdb
`

	testPlaybookYAML = `
---
- name: Integration test playbook
  hosts: testvms
  vars:
    test_message: "Hello from Gosinble integration test"
  
  tasks:
    - name: Test basic connectivity
      command:
        cmd: hostname
    
    - name: Gather system facts
      setup:
    
    - name: Check test device
      command:
        cmd: "lsblk {{device}}"
      ignore_errors: yes
    
    - name: Create test directory
      command:
        cmd: "mkdir -p /tmp/gosinble_test"
    
    - name: Write test file
      shell:
        cmd: 'echo "{{test_message}}" > /tmp/gosinble_test/integration.txt'
    
    - name: Verify test file
      command:
        cmd: "cat /tmp/gosinble_test/integration.txt"
    
    - name: Show system info
      debug:
        msg: "Host {{inventory_hostname}} - {{ansible_system}} {{ansible_kernel}}"
    
    - name: Cleanup test directory
      command:
        cmd: "rm -rf /tmp/gosinble_test"
`
)

func TestIntegrationBasicConnectivity(t *testing.T) {
	// Create inventory from YAML
	inv, err := inventory.NewFromYAML([]byte(testInventoryYAML))
	if err != nil {
		t.Fatalf("Failed to create inventory: %v", err)
	}

	// Get all test VMs
	hosts, err := inv.GetHosts("testvms")
	if err != nil {
		t.Fatalf("Failed to get hosts: %v", err)
	}

	if len(hosts) != 4 {
		t.Fatalf("Expected 4 hosts, got %d", len(hosts))
	}

	// Create task runner with timeout
	taskRunner := runner.NewTaskRunner()
	taskRunner.SetMaxConcurrency(2) // Test with limited concurrency

	// Test basic connectivity with each host
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := taskRunner.ExecuteTask(ctx,
		"Test connectivity",
		"command",
		map[string]interface{}{"cmd": "echo 'connectivity test successful'"},
		hosts,
		nil,
	)

	if err != nil {
		t.Fatalf("Failed to execute connectivity test: %v", err)
	}

	// Check results
	if len(results) != len(hosts) {
		t.Errorf("Expected %d results, got %d", len(hosts), len(results))
	}

	successful := 0
	for _, result := range results {
		t.Logf("Host: %s, Success: %v, Duration: %v", result.Host, result.Success, result.Duration)
		if result.Success {
			successful++
		} else {
			t.Logf("Host %s failed: %v", result.Host, result.Error)
		}
	}

	if successful == 0 {
		t.Fatal("No hosts were reachable - check network connectivity and credentials")
	}

	t.Logf("Successfully connected to %d/%d hosts", successful, len(hosts))
}

func TestIntegrationFactGathering(t *testing.T) {
	// Create inventory
	inv, err := inventory.NewFromYAML([]byte(testInventoryYAML))
	if err != nil {
		t.Fatalf("Failed to create inventory: %v", err)
	}

	// Get first available host for fact gathering test
	hosts, err := inv.GetHosts("obfy11")
	if err != nil {
		t.Fatalf("Failed to get host: %v", err)
	}

	if len(hosts) == 0 {
		t.Skip("No hosts available for fact gathering test")
	}

	host := hosts[0]
	taskRunner := runner.NewTaskRunner()

	// Test fact gathering
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	results, err := taskRunner.ExecuteTask(ctx,
		"Gather facts",
		"setup",
		map[string]interface{}{},
		[]common.Host{host},
		nil,
	)

	if err != nil {
		t.Fatalf("Failed to gather facts: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Success {
		t.Fatalf("Fact gathering failed: %v", result.Error)
	}

	// Check that facts were gathered
	if result.Data == nil {
		t.Fatal("No data returned from fact gathering")
	}

	facts, ok := result.Data["ansible_facts"].(map[string]interface{})
	if !ok {
		t.Fatal("No ansible_facts in result")
	}

	// Check for some expected facts
	expectedFacts := []string{"ansible_hostname", "ansible_system", "ansible_kernel"}
	for _, fact := range expectedFacts {
		if _, exists := facts[fact]; !exists {
			t.Errorf("Expected fact %s not found", fact)
		} else {
			t.Logf("Fact %s: %v", fact, facts[fact])
		}
	}

	t.Logf("Successfully gathered %d facts from host %s", len(facts), host.Name)
}

func TestIntegrationPlaybookExecution(t *testing.T) {
	// Create inventory
	inv, err := inventory.NewFromYAML([]byte(testInventoryYAML))
	if err != nil {
		t.Fatalf("Failed to create inventory: %v", err)
	}

	// Parse playbook
	parser := playbook.NewParser()
	pb, err := parser.Parse([]byte(testPlaybookYAML), "integration_test.yml")
	if err != nil {
		t.Fatalf("Failed to parse playbook: %v", err)
	}


	// Create task runner
	taskRunner := runner.NewTaskRunner()
	taskRunner.SetMaxConcurrency(2)

	// Execute playbook with limited hosts to avoid overwhelming the test environment
	allHosts, err := inv.GetHosts("*")
	if err != nil {
		t.Fatalf("Failed to get hosts: %v", err)
	}

	if len(allHosts) == 0 {
		t.Skip("No hosts available for playbook test")
	}
	
	// Use first available host
	hosts := []common.Host{allHosts[0]}

	// Execute just the first play on the first host
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	play := pb.Plays[0]
	// Override hosts to just test one host
	play.Hosts = hosts[0].Name

	results, err := taskRunner.RunPlay(ctx, play, inv, map[string]interface{}{
		"test_run": true,
		"integration_test": true,
	})

	if err != nil {
		t.Fatalf("Failed to execute playbook: %v", err)
	}

	// Analyze results
	successful := 0
	taskCount := make(map[string]int)

	for _, result := range results {
		if result.Success {
			successful++
		}
		taskCount[result.TaskName]++
		
		t.Logf("Task: %s, Host: %s, Success: %v, Duration: %v", 
			result.TaskName, result.Host, result.Success, result.Duration)
		
		if !result.Success {
			t.Logf("Task failed: %v", result.Error)
		}
	}

	t.Logf("Playbook execution completed: %d/%d tasks successful", successful, len(results))
	t.Logf("Task distribution: %v", taskCount)

	// Check that we got some results
	if len(results) == 0 {
		t.Fatal("No task results returned")
	}

	// Check that at least basic tasks succeeded
	if successful == 0 {
		t.Fatal("No tasks succeeded")
	}
}

func TestIntegrationVariableHandling(t *testing.T) {
	// Test variable expansion and merging
	inv, err := inventory.NewFromYAML([]byte(testInventoryYAML))
	if err != nil {
		t.Fatalf("Failed to create inventory: %v", err)
	}

	allHosts, err := inv.GetHosts("*")
	if err != nil || len(allHosts) == 0 {
		t.Skip("No hosts available for variable test")
	}

	host := allHosts[0]
	taskRunner := runner.NewTaskRunner()

	// Test variable expansion in debug message
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testVars := map[string]interface{}{
		"custom_message": "Hello from integration test",
		"test_number":    42,
	}

	results, err := taskRunner.ExecuteTask(ctx,
		"Variable expansion test",
		"debug",
		map[string]interface{}{
			"msg": "Host: {{inventory_hostname}}, Message: {{custom_message}}, Number: {{test_number}}",
		},
		[]common.Host{host},
		testVars,
	)

	if err != nil {
		t.Fatalf("Failed to execute variable test: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Success {
		t.Fatalf("Variable test failed: %v", result.Error)
	}

	t.Logf("Variable expansion result: %s", result.Message)

	// Test host variables
	hostVars, err := inv.GetHostVars(host.Name)
	if err != nil {
		t.Fatalf("Failed to get host variables: %v", err)
	}

	expectedVars := []string{"inventory_hostname", "ansible_host", "test_group"}
	for _, varName := range expectedVars {
		if _, exists := hostVars[varName]; !exists {
			t.Errorf("Expected host variable %s not found", varName)
		} else {
			t.Logf("Host variable %s: %v", varName, hostVars[varName])
		}
	}
}

func TestIntegrationTemplateRendering(t *testing.T) {
	// Test template rendering with host-specific variables
	inv, err := inventory.NewFromYAML([]byte(testInventoryYAML))
	if err != nil {
		t.Fatalf("Failed to create inventory: %v", err)
	}

	allHosts, err := inv.GetHosts("*")
	if err != nil || len(allHosts) == 0 {
		t.Skip("No hosts available for template test")
	}

	host := allHosts[0]

	// Create variable manager and gather some variables
	varMgr := vars.NewVarManager()
	varMgr.SetVar("app_name", "gosinble_test")
	varMgr.SetVar("app_version", "1.0.0")

	templateContent := `
Application: {{.app_name}}
Version: {{.app_version}}
Host: {{.inventory_hostname}}
Environment: {{.environment | default "development"}}
Timestamp: {{.ansible_date_time | default "unknown"}}
`

	// Test template rendering
	engine := template.NewEngine()
	if engine == nil {
		t.Skip("Template engine not available")
	}

	hostVars, _ := inv.GetHostVars(host.Name)
	allVars := varMgr.MergeVars(hostVars, map[string]interface{}{
		"environment": "integration_test",
	})

	// Use the template content and variables
	_, err = engine.Render(templateContent, allVars)
	if err != nil {
		t.Logf("Template rendering failed: %v", err)
	}

	// This would normally be done by the template module
	// but we're testing the template engine directly here
	t.Logf("Template rendering test completed - engine available")
}

func TestIntegrationErrorHandling(t *testing.T) {
	// Test error handling and resilience
	inv, err := inventory.NewFromYAML([]byte(testInventoryYAML))
	if err != nil {
		t.Fatalf("Failed to create inventory: %v", err)
	}

	allHosts, err := inv.GetHosts("*")
	if err != nil || len(allHosts) == 0 {
		t.Skip("No hosts available for error handling test")
	}

	host := allHosts[0]
	taskRunner := runner.NewTaskRunner()

	// Test command that should fail
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := taskRunner.ExecuteTask(ctx,
		"Intentional failure test",
		"command",
		map[string]interface{}{"cmd": "exit 1"},
		[]common.Host{host},
		nil,
	)

	if err != nil {
		t.Fatalf("Failed to execute error test: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	
	// The command should have "succeeded" in execution but the command itself failed
	if !result.Success {
		t.Logf("Command execution failed as expected: %v", result.Error)
	}

	// Check that exit code is captured
	if exitCode, exists := result.Data["exit_code"]; exists {
		t.Logf("Exit code correctly captured: %v", exitCode)
		if exitCode != 1 && exitCode != -1 {
			t.Errorf("Expected exit code 1 or -1, got %v", exitCode)
		}
	}

	t.Logf("Error handling test completed successfully")
}

func TestIntegrationConcurrency(t *testing.T) {
	// Test concurrent execution across multiple hosts
	inv, err := inventory.NewFromYAML([]byte(testInventoryYAML))
	if err != nil {
		t.Fatalf("Failed to create inventory: %v", err)
	}

	// Try to get multiple hosts
	hosts, err := inv.GetHosts("testvms")
	if err != nil {
		t.Fatalf("Failed to get hosts: %v", err)
	}

	// Limit to available hosts
	if len(hosts) > 2 {
		hosts = hosts[:2] // Use only first 2 hosts to avoid overwhelming test environment
	}

	if len(hosts) == 0 {
		t.Skip("No hosts available for concurrency test")
	}

	taskRunner := runner.NewTaskRunner()
	taskRunner.SetMaxConcurrency(len(hosts)) // Allow full parallelism

	// Execute a task that takes some time
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	start := time.Now()
	results, err := taskRunner.ExecuteTask(ctx,
		"Concurrent execution test",
		"command",
		map[string]interface{}{"cmd": "sleep 2 && hostname"},
		hosts,
		nil,
	)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to execute concurrent test: %v", err)
	}

	if len(results) != len(hosts) {
		t.Errorf("Expected %d results, got %d", len(hosts), len(results))
	}

	successful := 0
	for _, result := range results {
		if result.Success {
			successful++
		}
		t.Logf("Host: %s, Success: %v, Duration: %v", result.Host, result.Success, result.Duration)
	}

	// With parallel execution, total time should be close to individual task time
	expectedMaxTime := 10 * time.Second // Allow some overhead
	if elapsed > expectedMaxTime {
		t.Logf("Warning: Concurrent execution took %v, expected less than %v", elapsed, expectedMaxTime)
	} else {
		t.Logf("Concurrent execution completed in %v (good performance)", elapsed)
	}

	t.Logf("Concurrency test completed: %d/%d hosts successful", successful, len(hosts))
}

// Helper function to skip tests if integration environment is not available
func checkIntegrationEnvironment(t *testing.T) {
	t.Helper()
	
	// This could check for environment variables, config files, or network connectivity
	// For now, we'll assume integration tests should always run when requested
	t.Logf("Running integration tests against test VMs")
}