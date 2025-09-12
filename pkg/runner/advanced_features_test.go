package runner

import (
	"context"
	"testing"
	"time"
	
	"github.com/liliang-cn/gosible/pkg/types"
)

func TestTaskRunnerTags(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()
	
	// Create test hosts
	hosts := []types.Host{
		{Name: "localhost", Address: "localhost"},
	}
	
	// Test task with tags
	taskWithTags := types.Task{
		Name:   "Tagged Task",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "Tagged task"},
		Tags:   []string{"web", "config"},
	}
	
	// Test task without tags
	taskWithoutTags := types.Task{
		Name:   "Untagged Task",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "Untagged task"},
	}
	
	// Test task with 'always' tag
	taskAlways := types.Task{
		Name:   "Always Task",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "Always run"},
		Tags:   []string{"always"},
	}
	
	// Test 1: No tags set - all tasks should run
	results, err := runner.Run(ctx, taskWithTags, hosts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Data["skipped"] == true {
		t.Error("task should not be skipped when no tags are set")
	}
	
	// Test 2: Set tags - only matching tasks should run
	runner.SetTags([]string{"web"})
	
	results, err = runner.Run(ctx, taskWithTags, hosts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Data["skipped"] == true {
		t.Error("task with matching tag should not be skipped")
	}
	
	results, err = runner.Run(ctx, taskWithoutTags, hosts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Data["skipped"] != true {
		t.Error("task without tags should be skipped when tags are set")
	}
	
	// Test 3: Always tag should always run
	runner.SetTags([]string{"other"})
	
	results, err = runner.Run(ctx, taskAlways, hosts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Data["skipped"] == true {
		t.Error("task with 'always' tag should never be skipped")
	}
}

func TestTaskRunnerWhenCondition(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()
	
	hosts := []types.Host{
		{Name: "localhost", Address: "localhost"},
	}
	
	// Test task with true condition
	taskTrue := types.Task{
		Name:   "Conditional Task",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "Should run"},
		When:   "os_family == 'linux'",
	}
	
	vars := map[string]interface{}{
		"os_family": "linux",
	}
	
	results, err := runner.Run(ctx, taskTrue, hosts, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Data["skipped"] == true {
		t.Error("task should run when condition is true")
	}
	
	// Test task with false condition
	taskFalse := types.Task{
		Name:   "Conditional Task",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "Should not run"},
		When:   "os_family == 'windows'",
	}
	
	results, err = runner.Run(ctx, taskFalse, hosts, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Data["skipped"] != true {
		t.Error("task should be skipped when condition is false")
	}
}

func TestTaskRunnerLoop(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()
	
	hosts := []types.Host{
		{Name: "localhost", Address: "localhost"},
	}
	
	// Test task with loop
	task := types.Task{
		Name:   "Loop Task",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "Processing {{item}}"},
		Loop:   []interface{}{"item1", "item2", "item3"},
	}
	
	results, err := runner.Run(ctx, task, hosts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should have 3 results (one per loop item)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	
	// Check loop metadata
	for i, result := range results {
		loopData, ok := result.Data["ansible_loop"].(map[string]interface{})
		if !ok {
			t.Error("expected ansible_loop data in result")
			continue
		}
		
		if loopData["index"] != i {
			t.Errorf("expected index %d, got %v", i, loopData["index"])
		}
		
		if loopData["first"] != (i == 0) {
			t.Errorf("expected first=%v for index %d", i == 0, i)
		}
		
		if loopData["last"] != (i == 2) {
			t.Errorf("expected last=%v for index %d", i == 2, i)
		}
		
		expectedItem := []string{"item1", "item2", "item3"}[i]
		if loopData["item"] != expectedItem {
			t.Errorf("expected item %s, got %v", expectedItem, loopData["item"])
		}
	}
}

func TestTaskRunnerWithItems(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()
	
	hosts := []types.Host{
		{Name: "localhost", Address: "localhost"},
	}
	
	// Test task with with_items (legacy syntax)
	task := types.Task{
		Name:      "WithItems Task",
		Module:    "debug",
		Args:      map[string]interface{}{"msg": "Item: {{item}}"},
		WithItems: []interface{}{"a", "b", "c"},
	}
	
	results, err := runner.Run(ctx, task, hosts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestTaskRunnerLoopControl(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()
	
	hosts := []types.Host{
		{Name: "localhost", Address: "localhost"},
	}
	
	// Test task with custom loop variable
	task := types.Task{
		Name:   "Custom Loop Var",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "User: {{user}}"},
		Loop:   []interface{}{"alice", "bob", "charlie"},
		LoopControl: map[string]interface{}{
			"loop_var":  "user",
			"index_var": "user_index",
		},
	}
	
	results, err := runner.Run(ctx, task, hosts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	
	// Check custom loop variable
	for i, result := range results {
		loopData, ok := result.Data["ansible_loop"].(map[string]interface{})
		if !ok {
			t.Error("expected ansible_loop data in result")
			continue
		}
		
		expectedUser := []string{"alice", "bob", "charlie"}[i]
		if loopData["user"] != expectedUser {
			t.Errorf("expected user %s, got %v", expectedUser, loopData["user"])
		}
	}
}

func TestTaskRunnerNotify(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()
	
	// Register a handler
	handler := types.Task{
		Name:   "restart_service",
		Module: "debug",
		Args:   map[string]interface{}{"msg": "Service restarted"},
	}
	
	err := runner.GetHandlerManager().RegisterHandler(handler)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}
	
	hosts := []types.Host{
		{Name: "localhost", Address: "localhost"},
	}
	
	// Test task that notifies handler on change
	task := types.Task{
		Name:        "Config Task",
		Module:      "debug",
		Args:        map[string]interface{}{"msg": "Config updated"},
		Notify:      []string{"restart_service"},
		ChangedWhen: true, // Force changed status
	}
	
	_, err2 := runner.Run(ctx, task, hosts, nil)
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}
	
	// Check that handler was notified
	pending := runner.GetHandlerManager().GetPendingHandlers()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending handler, got %d", len(pending))
	}
	if pending[0].Name != "restart_service" {
		t.Errorf("expected handler 'restart_service', got '%s'", pending[0].Name)
	}
}

func TestTaskRunnerIgnoreErrors(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()
	
	hosts := []types.Host{
		{Name: "localhost", Address: "localhost"},
	}
	
	// Test task that would fail but has ignore_errors
	task := types.Task{
		Name:         "Failing Task",
		Module:       "command",
		Args:         map[string]interface{}{"cmd": "false"}, // This command always fails
		IgnoreErrors: true,
	}
	
	results, err := runner.Run(ctx, task, hosts, nil)
	if err != nil {
		t.Fatalf("unexpected error despite ignore_errors: %v", err)
	}
	
	// Task should complete despite the failure
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestTaskRunnerRetries(t *testing.T) {
	runner := NewTaskRunner()
	ctx := context.Background()
	
	hosts := []types.Host{
		{Name: "localhost", Address: "localhost"},
	}
	
	// Test task with retries (using debug module which always succeeds)
	task := types.Task{
		Name:    "Retry Task",
		Module:  "debug",
		Args:    map[string]interface{}{"msg": "Attempt"},
		Retries: 3,
		Delay:   1,
		Until:   "false", // This will cause retries to continue
	}
	
	// Use a short timeout to avoid long test
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	results, err := runner.Run(ctxWithTimeout, task, hosts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Result should exist (task will retry until timeout or max retries)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}