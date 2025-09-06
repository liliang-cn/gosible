package integration

import (
	"context"
	"testing"

	"github.com/gosinble/gosinble/pkg/inventory"
	"github.com/gosinble/gosinble/pkg/modules"
	"github.com/gosinble/gosinble/pkg/runner"
	"github.com/gosinble/gosinble/pkg/types"
)

// TestModule is a test module that supports check and diff modes
type TestModule struct {
	*modules.BaseModule
}

func NewTestModule() *TestModule {
	base := modules.NewBaseModule("test_module", types.ModuleDoc{
		Name:        "test_module",
		Description: "A module used to test check and diff mode functionality",
	})
	
	// Set capabilities to support both check and diff modes
	base.SetCapabilities(&types.ModuleCapability{
		CheckMode: true,
		DiffMode:  true,
		Platform:  "all",
	})
	
	return &TestModule{
		BaseModule: base,
	}
}

func (m *TestModule) Validate(args map[string]interface{}) error {
	return m.ValidateRequired(args, []string{"content"})
}

func (m *TestModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	host := m.GetHostFromConnection(conn)
	content := m.GetStringArg(args, "content", "")
	
	// Check if we're in check mode
	if m.CheckMode(args) {
		// In check mode, simulate what would happen
		result := m.CreateCheckModeResult(host, true, "Would update content", map[string]interface{}{
			"content": content,
		})
		
		// If diff mode is also enabled, add diff
		if m.DiffMode(args) {
			diff := m.GenerateDiff("old content", content)
			result.Diff = diff
		}
		
		return result, nil
	}
	
	// Normal execution
	result := m.CreateSuccessResult(host, true, "Content updated", map[string]interface{}{
		"content": content,
	})
	
	// If diff mode is enabled, add diff
	if m.DiffMode(args) {
		diff := m.GenerateDiff("old content", content)
		result.Diff = diff
	}
	
	return result, nil
}

func TestCheckMode(t *testing.T) {
	// Create inventory with local host
	inv := inventory.NewStaticInventory()
	inv.AddHost(types.Host{
		Name:    "localhost",
		Address: "localhost",
	})
	
	// Create runner
	taskRunner := runner.NewTaskRunner()
	
	// Register test module
	testModule := NewTestModule()
	if err := taskRunner.RegisterModule(testModule); err != nil {
		t.Fatalf("Failed to register test module: %v", err)
	}
	
	// Get hosts
	hosts, err := inv.GetHosts("localhost")
	if err != nil {
		t.Fatalf("Failed to get hosts: %v", err)
	}
	
	// Test 1: Normal execution (no check mode)
	t.Run("NormalExecution", func(t *testing.T) {
		task := types.Task{
			Name:   "Test normal execution",
			Module: types.ModuleType("test_module"),
			Args: map[string]interface{}{
				"content": "new content",
			},
		}
		
		vars := map[string]interface{}{}
		
		results, err := taskRunner.Run(context.Background(), task, hosts, vars)
		if err != nil {
			t.Fatalf("Task execution failed: %v", err)
		}
		
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		
		result := results[0]
		if !result.Success {
			t.Error("Expected success")
		}
		if !result.Changed {
			t.Error("Expected changed=true")
		}
		if result.Simulated {
			t.Error("Expected simulated=false in normal mode")
		}
	})
	
	// Test 2: Check mode execution
	t.Run("CheckMode", func(t *testing.T) {
		task := types.Task{
			Name:   "Test check mode",
			Module: types.ModuleType("test_module"),
			Args: map[string]interface{}{
				"content": "new content",
			},
		}
		
		vars := map[string]interface{}{
			"ansible_check_mode": true,
		}
		
		results, err := taskRunner.Run(context.Background(), task, hosts, vars)
		if err != nil {
			t.Fatalf("Task execution failed: %v", err)
		}
		
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		
		result := results[0]
		if !result.Success {
			t.Error("Expected success")
		}
		if !result.Changed {
			t.Error("Expected changed=true in check mode when would change")
		}
		if !result.Simulated {
			t.Error("Expected simulated=true in check mode")
		}
		if result.Data["check_mode"] != true {
			t.Error("Expected check_mode=true in data")
		}
		if result.Data["would_change"] != true {
			t.Error("Expected would_change=true in data")
		}
	})
	
	// Test 3: Diff mode execution
	t.Run("DiffMode", func(t *testing.T) {
		task := types.Task{
			Name:   "Test diff mode",
			Module: types.ModuleType("test_module"),
			Args: map[string]interface{}{
				"content": "new content",
			},
		}
		
		vars := map[string]interface{}{
			"ansible_diff_mode": true,
		}
		
		results, err := taskRunner.Run(context.Background(), task, hosts, vars)
		if err != nil {
			t.Fatalf("Task execution failed: %v", err)
		}
		
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		
		result := results[0]
		if !result.Success {
			t.Error("Expected success")
		}
		if result.Diff == nil {
			t.Error("Expected diff to be present")
		} else {
			if result.Diff.Before != "old content" {
				t.Errorf("Expected diff.Before='old content', got %s", result.Diff.Before)
			}
			if result.Diff.After != "new content" {
				t.Errorf("Expected diff.After='new content', got %s", result.Diff.After)
			}
			if !result.Diff.Prepared {
				t.Error("Expected diff.Prepared=true")
			}
		}
	})
	
	// Test 4: Check + Diff mode together
	t.Run("CheckAndDiffMode", func(t *testing.T) {
		task := types.Task{
			Name:   "Test check and diff mode",
			Module: types.ModuleType("test_module"),
			Args: map[string]interface{}{
				"content": "new content",
			},
		}
		
		vars := map[string]interface{}{
			"ansible_check_mode": true,
			"ansible_diff_mode":  true,
		}
		
		results, err := taskRunner.Run(context.Background(), task, hosts, vars)
		if err != nil {
			t.Fatalf("Task execution failed: %v", err)
		}
		
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		
		result := results[0]
		if !result.Success {
			t.Error("Expected success")
		}
		if !result.Simulated {
			t.Error("Expected simulated=true in check mode")
		}
		if result.Diff == nil {
			t.Error("Expected diff to be present")
		}
		if result.Data["check_mode"] != true {
			t.Error("Expected check_mode=true in data")
		}
	})
}

// TestModuleWithoutCheckSupport tests behavior with modules that don't support check mode
func TestModuleWithoutCheckSupport(t *testing.T) {
	// Create a module without check mode support
	base := modules.NewBaseModule("no_check_module", types.ModuleDoc{
		Name:        "no_check_module",
		Description: "Module without check mode support",
	})
	base.SetCapabilities(&types.ModuleCapability{
		CheckMode: false, // No check mode support
		DiffMode:  false,
		Platform:  "all",
	})
	
	// Create inventory
	inv := inventory.NewStaticInventory()
	inv.AddHost(types.Host{
		Name:    "localhost",
		Address: "localhost",
	})
	
	// Create runner
	taskRunner := runner.NewTaskRunner()
	
	// Register module wrapper
	moduleWrapper := &ModuleWrapper{BaseModule: base}
	if err := taskRunner.RegisterModule(moduleWrapper); err != nil {
		t.Fatalf("Failed to register module: %v", err)
	}
	
	// Get hosts
	hosts, err := inv.GetHosts("localhost")
	if err != nil {
		t.Fatalf("Failed to get hosts: %v", err)
	}
	
	// Test: Module without check mode support
	task := types.Task{
		Name:   "Test module without check support",
		Module: types.ModuleType("no_check_module"),
		Args:   map[string]interface{}{},
	}
	
	vars := map[string]interface{}{
		"ansible_check_mode": true,
	}
	
	results, err := taskRunner.Run(context.Background(), task, hosts, vars)
	if err != nil {
		t.Fatalf("Task execution failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	
	result := results[0]
	if !result.Success {
		t.Error("Expected success (module should be skipped)")
	}
	if result.Changed {
		t.Error("Expected changed=false for skipped module")
	}
	if !result.Simulated {
		t.Error("Expected simulated=true for skipped module in check mode")
	}
	if result.Data["skipped"] != true {
		t.Error("Expected skipped=true in data")
	}
	if result.Data["reason"] != "module_no_check_support" {
		t.Errorf("Expected reason='module_no_check_support', got %v", result.Data["reason"])
	}
}

// ModuleWrapper wraps BaseModule to implement the Module interface
type ModuleWrapper struct {
	*modules.BaseModule
}

func (m *ModuleWrapper) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	host := m.GetHostFromConnection(conn)
	return m.CreateSuccessResult(host, false, "Module executed", nil), nil
}

func (m *ModuleWrapper) Validate(args map[string]interface{}) error {
	return nil
}