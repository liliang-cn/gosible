package runner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
	"github.com/liliang-cn/gosinble/pkg/connection"
	"github.com/liliang-cn/gosinble/pkg/modules"
	"github.com/liliang-cn/gosinble/pkg/vars"
)

func TestNewTaskRunner(t *testing.T) {
	runner := NewTaskRunner()
	if runner == nil {
		t.Fatal("NewTaskRunner returned nil")
	}

	if runner.maxConcurrency != 5 {
		t.Errorf("expected default maxConcurrency 5, got %d", runner.maxConcurrency)
	}

	if runner.moduleRegistry == nil {
		t.Error("moduleRegistry should not be nil")
	}

	if runner.connectionMgr == nil {
		t.Error("connectionMgr should not be nil")
	}

	if runner.connections == nil {
		t.Error("connections map should not be nil")
	}
}

func TestNewTaskRunnerWithDependencies(t *testing.T) {
	moduleRegistry := modules.NewModuleRegistry()
	connectionMgr := connection.NewConnectionManager()
	varMgr := vars.NewVarManager()

	runner := NewTaskRunnerWithDependencies(moduleRegistry, connectionMgr, varMgr)
	if runner == nil {
		t.Fatal("NewTaskRunnerWithDependencies returned nil")
	}

	if runner.moduleRegistry != moduleRegistry {
		t.Error("moduleRegistry not set correctly")
	}

	if runner.connectionMgr != connectionMgr {
		t.Error("connectionMgr not set correctly")
	}

	if runner.varManager != varMgr {
		t.Error("varManager not set correctly")
	}
}

func TestTaskRunnerSetMaxConcurrency(t *testing.T) {
	runner := NewTaskRunner()

	// Test setting valid concurrency
	runner.SetMaxConcurrency(10)
	if runner.maxConcurrency != 10 {
		t.Errorf("expected maxConcurrency 10, got %d", runner.maxConcurrency)
	}

	// Test setting zero concurrency (should default to 1)
	runner.SetMaxConcurrency(0)
	if runner.maxConcurrency != 1 {
		t.Errorf("expected maxConcurrency 1 for zero input, got %d", runner.maxConcurrency)
	}

	// Test setting negative concurrency (should default to 1)
	runner.SetMaxConcurrency(-5)
	if runner.maxConcurrency != 1 {
		t.Errorf("expected maxConcurrency 1 for negative input, got %d", runner.maxConcurrency)
	}
}

func TestTaskRunnerRegisterGetModule(t *testing.T) {
	runner := NewTaskRunner()

	// Test getting existing module
	module, err := runner.GetModule("command")
	if err != nil {
		t.Fatalf("GetModule failed for built-in module: %v", err)
	}
	if module.Name() != "command" {
		t.Errorf("expected module name 'command', got %s", module.Name())
	}

	// Test getting non-existent module
	_, err = runner.GetModule("nonexistent")
	if err == nil {
		t.Error("GetModule should fail for non-existent module")
	}

	// Test registering custom module
	customModule := modules.NewDebugModule() // Use debug module as test
	err = runner.RegisterModule(customModule)
	if err != nil {
		t.Fatalf("RegisterModule failed: %v", err)
	}

	// Verify we can get the registered module
	retrieved, err := runner.GetModule("debug")
	if err != nil {
		t.Fatalf("GetModule failed for registered module: %v", err)
	}
	if retrieved.Name() != "debug" {
		t.Errorf("expected retrieved module name 'debug', got %s", retrieved.Name())
	}
}

func TestTaskRunnerRun(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()

	// Create test hosts
	hosts := []types.Host{
		{
			Name:    "localhost",
			Address: "localhost",
			Port:    22,
		},
	}

	// Test simple command task
	task := types.Task{
		Name:   "Test Command",
		Module: "command",
		Args: map[string]interface{}{
			"cmd": "echo 'test'",
		},
	}

	vars := map[string]interface{}{
		"test_var": "test_value",
	}

	results, err := runner.Run(ctx, task, hosts, vars)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
		return
	}

	result := results[0]
	if result.Host != "localhost" {
		t.Errorf("expected host 'localhost', got %s", result.Host)
	}

	if result.ModuleName != "command" {
		t.Errorf("expected module 'command', got %s", result.ModuleName)
	}

	if result.TaskName != "Test Command" {
		t.Errorf("expected task name 'Test Command', got %s", result.TaskName)
	}
}

func TestTaskRunnerRunWithMultipleHosts(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()

	// Create multiple test hosts (all localhost for testing)
	hosts := []types.Host{
		{Name: "host1", Address: "localhost", Port: 22},
		{Name: "host2", Address: "localhost", Port: 22},
		{Name: "host3", Address: "localhost", Port: 22},
	}

	task := types.Task{
		Name:   "Multi-host Test",
		Module: "debug",
		Args: map[string]interface{}{
			"msg": "Hello from {{inventory_hostname}}",
		},
	}

	results, err := runner.Run(ctx, task, hosts, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
		return
	}

	// Check that each host got its result
	hostNames := make(map[string]bool)
	for _, result := range results {
		hostNames[result.Host] = true
		if !result.Success {
			t.Errorf("result for host %s should be successful", result.Host)
		}
	}

	expectedHosts := []string{"host1", "host2", "host3"}
	for _, expectedHost := range expectedHosts {
		if !hostNames[expectedHost] {
			t.Errorf("missing result for host %s", expectedHost)
		}
	}
}

func TestTaskRunnerRunWithEmptyHosts(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()

	task := types.Task{
		Name:   "Empty Hosts Test",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "test"},
	}

	results, err := runner.Run(ctx, task, []types.Host{}, nil)
	if err != nil {
		t.Fatalf("Run with empty hosts failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty hosts, got %d", len(results))
	}
}

func TestTaskRunnerRunWithInvalidModule(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()

	hosts := []types.Host{
		{Name: "localhost", Address: "127.0.0.1"},
	}

	task := types.Task{
		Name:   "Invalid Module Test",
		Module: "nonexistent_module",
		Args:   map[string]interface{}{},
	}

	_, err := runner.Run(ctx, task, hosts, nil)
	if err == nil {
		t.Error("Run should fail with invalid module")
	}
}

func TestTaskRunnerExpandTaskArguments(t *testing.T) {
	runner := NewTaskRunner()

	args := map[string]interface{}{
		"message": "Hello {{user}}",
		"count":   "{{number}}",
		"nested": map[string]interface{}{
			"key": "value-{{suffix}}",
		},
		"list": []interface{}{"item1", "{{item2}}", "item3"},
	}

	vars := map[string]interface{}{
		"user":   "Alice",
		"number": "42",
		"suffix": "test",
		"item2":  "dynamic_item",
	}

	expanded := runner.expandTaskArguments(args, vars)

	if expanded["message"] != "Hello Alice" {
		t.Errorf("expected 'Hello Alice', got %v", expanded["message"])
	}

	if expanded["count"] != "42" {
		t.Errorf("expected '42', got %v", expanded["count"])
	}

	nested, ok := expanded["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("nested should be a map")
	}
	if nested["key"] != "value-test" {
		t.Errorf("expected 'value-test', got %v", nested["key"])
	}

	list, ok := expanded["list"].([]interface{})
	if !ok {
		t.Fatal("list should be a slice")
	}
	if len(list) != 3 {
		t.Errorf("expected list length 3, got %d", len(list))
	}
	if list[1] != "dynamic_item" {
		t.Errorf("expected 'dynamic_item', got %v", list[1])
	}
}

func TestTaskRunnerGetHostVariables(t *testing.T) {
	runner := NewTaskRunner()

	host := types.Host{
		Name:    "testhost",
		Address: "192.168.1.100",
		Port:    2222,
		User:    "testuser",
		Variables: map[string]interface{}{
			"custom_var": "custom_value",
		},
	}

	taskVars := map[string]interface{}{
		"task_var": "task_value",
	}

	hostVars, err := runner.getHostVariables(host, taskVars)
	if err != nil {
		t.Fatalf("getHostVariables failed: %v", err)
	}

	// Check task variables
	if hostVars["task_var"] != "task_value" {
		t.Errorf("expected task_var 'task_value', got %v", hostVars["task_var"])
	}

	// Check host variables
	if hostVars["custom_var"] != "custom_value" {
		t.Errorf("expected custom_var 'custom_value', got %v", hostVars["custom_var"])
	}

	// Check built-in variables
	if hostVars["inventory_hostname"] != "testhost" {
		t.Errorf("expected inventory_hostname 'testhost', got %v", hostVars["inventory_hostname"])
	}

	if hostVars["ansible_host"] != "192.168.1.100" {
		t.Errorf("expected ansible_host '192.168.1.100', got %v", hostVars["ansible_host"])
	}

	if hostVars["ansible_port"] != 2222 {
		t.Errorf("expected ansible_port 2222, got %v", hostVars["ansible_port"])
	}

	if hostVars["ansible_user"] != "testuser" {
		t.Errorf("expected ansible_user 'testuser', got %v", hostVars["ansible_user"])
	}
}

func TestTaskRunnerValidateTask(t *testing.T) {
	runner := NewTaskRunner()

	// Test valid task
	validTask := types.Task{
		Name:   "Valid Task",
		Module: "command",
		Args: map[string]interface{}{
			"cmd": "echo test",
		},
	}

	err := runner.ValidateTask(validTask)
	if err != nil {
		t.Errorf("ValidateTask should succeed for valid task: %v", err)
	}

	// Test invalid task (missing required argument)
	invalidTask := types.Task{
		Name:   "Invalid Task",
		Module: "command",
		Args:   map[string]interface{}{}, // Missing 'cmd' argument
	}

	err = runner.ValidateTask(invalidTask)
	if err == nil {
		t.Error("ValidateTask should fail for invalid task")
	}

	// Test task with non-existent module
	nonExistentTask := types.Task{
		Name:   "Non-existent Module Task",
		Module: "nonexistent",
		Args:   map[string]interface{}{},
	}

	err = runner.ValidateTask(nonExistentTask)
	if err == nil {
		t.Error("ValidateTask should fail for non-existent module")
	}
}

func TestTaskRunnerExecuteTask(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()

	hosts := []types.Host{
		{Name: "localhost", Address: "127.0.0.1"},
	}

	results, err := runner.ExecuteTask(ctx, "Test Task", "debug", 
		map[string]interface{}{"msg": "Hello World"}, hosts, nil)
	
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
		return
	}

	result := results[0]
	if result.TaskName != "Test Task" {
		t.Errorf("expected task name 'Test Task', got %s", result.TaskName)
	}

	if result.ModuleName != "debug" {
		t.Errorf("expected module name 'debug', got %s", result.ModuleName)
	}
}

func TestTaskRunnerGetStats(t *testing.T) {
	runner := NewTaskRunner()
	runner.SetMaxConcurrency(10)
	runner.SetConnectionTTL(1 * time.Hour)

	stats := runner.GetStats()

	if stats["max_concurrency"] != 10 {
		t.Errorf("expected max_concurrency 10, got %v", stats["max_concurrency"])
	}

	if stats["active_connections"] != 0 {
		t.Errorf("expected 0 active_connections initially, got %v", stats["active_connections"])
	}

	if stats["connection_ttl_mins"] != 60 {
		t.Errorf("expected connection_ttl_mins 60, got %v", stats["connection_ttl_mins"])
	}

	// Check that registered_modules is reasonable
	moduleCount, ok := stats["registered_modules"].(int)
	if !ok || moduleCount <= 0 {
		t.Errorf("expected positive registered_modules count, got %v", stats["registered_modules"])
	}
}

func TestTaskRunnerListModules(t *testing.T) {
	runner := NewTaskRunner()

	modules := runner.ListModules()
	if len(modules) == 0 {
		t.Error("ListModules should return at least some modules")
	}

	// Check for some expected built-in modules
	expectedModules := []string{"command", "debug", "shell", "setup"}
	for _, expected := range expectedModules {
		found := false
		for _, module := range modules {
			if module == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected module %s not found in list", expected)
		}
	}
}

func TestTaskRunnerGetModuleDocumentation(t *testing.T) {
	runner := NewTaskRunner()

	doc, err := runner.GetModuleDocumentation("command")
	if err != nil {
		t.Fatalf("GetModuleDocumentation failed: %v", err)
	}

	if doc.Name != "command" {
		t.Errorf("expected doc name 'command', got %s", doc.Name)
	}

	if doc.Description == "" {
		t.Error("documentation should have a description")
	}

	if len(doc.Parameters) == 0 {
		t.Error("command module should have parameters")
	}

	// Test non-existent module
	_, err = runner.GetModuleDocumentation("nonexistent")
	if err == nil {
		t.Error("GetModuleDocumentation should fail for non-existent module")
	}
}

func TestTaskRunnerClose(t *testing.T) {
	runner := NewTaskRunner()

	// Add a mock connection
	host := types.Host{Name: "testhost", Address: "127.0.0.1"}
	ctx := context.Background()
	
	// This should create a connection
	_, err := runner.getConnection(ctx, host)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}

	if runner.GetConnectionCount() == 0 {
		t.Error("should have at least one connection")
	}

	// Close all connections
	err = runner.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if runner.GetConnectionCount() != 0 {
		t.Errorf("expected 0 connections after Close, got %d", runner.GetConnectionCount())
	}
}

// Benchmark tests
func BenchmarkTaskRunnerRun(b *testing.B) {
	runner := NewTaskRunner()
	ctx := context.Background()

	hosts := []types.Host{
		{Name: "localhost", Address: "127.0.0.1"},
	}

	task := types.Task{
		Name:   "Benchmark Task",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "benchmark"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := runner.Run(ctx, task, hosts, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTaskRunnerRunParallel(b *testing.B) {
	runner := NewTaskRunner()
	runner.SetMaxConcurrency(10)
	ctx := context.Background()

	// Create multiple hosts for parallel execution
	hosts := make([]types.Host, 5)
	for i := 0; i < 5; i++ {
		hosts[i] = types.Host{
			Name:    fmt.Sprintf("host%d", i),
			Address: "127.0.0.1",
		}
	}

	task := types.Task{
		Name:   "Parallel Benchmark Task",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "parallel benchmark"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := runner.Run(ctx, task, hosts, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}