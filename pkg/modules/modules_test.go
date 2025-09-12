package modules

import (
	"context"
	"testing"

	"github.com/liliang-cn/gosible/pkg/types"
	"github.com/liliang-cn/gosible/pkg/connection"
)

func TestNewModuleRegistry(t *testing.T) {
	registry := NewModuleRegistry()
	if registry == nil {
		t.Fatal("NewModuleRegistry returned nil")
	}

	modules := registry.ListModules()
	if len(modules) == 0 {
		t.Error("registry should have built-in modules registered")
	}

	// Check that key modules are registered
	expectedModules := []string{"command", "shell", "copy", "setup", "debug"}
	for _, expected := range expectedModules {
		found := false
		for _, module := range modules {
			if module == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected module %s not found in registry", expected)
		}
	}
}

func TestModuleRegistryRegisterAndGet(t *testing.T) {
	registry := NewModuleRegistry()

	// Create a test module
	testModule := NewCommandModule()

	// Register the module
	err := registry.RegisterModule(testModule)
	if err != nil {
		t.Fatalf("RegisterModule failed: %v", err)
	}

	// Get the module
	retrieved, err := registry.GetModule("command")
	if err != nil {
		t.Fatalf("GetModule failed: %v", err)
	}

	if retrieved.Name() != "command" {
		t.Errorf("retrieved module name expected command, got %s", retrieved.Name())
	}
}

func TestModuleRegistryGetNonexistent(t *testing.T) {
	registry := NewModuleRegistry()

	_, err := registry.GetModule("nonexistent")
	if err != types.ErrModuleNotFound {
		t.Errorf("expected ErrModuleNotFound, got %v", err)
	}
}

func TestCommandModuleValidation(t *testing.T) {
	module := NewCommandModule()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid args",
			args:    map[string]interface{}{"cmd": "echo hello"},
			wantErr: false,
		},
		{
			name:    "missing cmd",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "invalid timeout type",
			args:    map[string]interface{}{"cmd": "echo hello", "timeout": "invalid"},
			wantErr: true,
		},
		{
			name:    "negative timeout",
			args:    map[string]interface{}{"cmd": "echo hello", "timeout": -5},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommandModuleRun(t *testing.T) {
	module := NewCommandModule()
	conn := connection.NewLocalConnection()
	ctx := context.Background()

	// Connect first
	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		t.Fatalf("connection failed: %v", err)
	}
	defer conn.Close()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "simple echo command",
			args: map[string]interface{}{
				"cmd": "echo 'hello world'",
			},
			wantErr: false,
		},
		{
			name: "command with working directory",
			args: map[string]interface{}{
				"cmd":   "pwd",
				"chdir": "/tmp",
			},
			wantErr: false,
		},
		{
			name: "check mode",
			args: map[string]interface{}{
				"cmd":         "echo 'test'",
				"_check_mode": true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := module.Run(ctx, conn, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Error("result should not be nil")
				return
			}

			if result.ModuleName != "command" {
				t.Errorf("result.ModuleName expected command, got %s", result.ModuleName)
			}

			// Check mode should return different result
			if checkMode, ok := tt.args["_check_mode"].(bool); ok && checkMode {
				if !result.Success {
					t.Error("check mode should always succeed")
				}
			}
		})
	}
}

func TestShellModuleValidation(t *testing.T) {
	module := NewShellModule()

	validArgs := map[string]interface{}{"cmd": "echo hello | grep hello"}
	err := module.Validate(validArgs)
	if err != nil {
		t.Errorf("Validate() with valid args failed: %v", err)
	}

	invalidArgs := map[string]interface{}{}
	err = module.Validate(invalidArgs)
	if err == nil {
		t.Error("Validate() should fail with missing cmd")
	}
}

func TestCopyModuleValidation(t *testing.T) {
	module := NewCopyModule()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid with src",
			args: map[string]interface{}{
				"src":  "/tmp/source",
				"dest": "/tmp/dest",
			},
			wantErr: false,
		},
		{
			name: "valid with content",
			args: map[string]interface{}{
				"content": "hello world",
				"dest":    "/tmp/dest",
			},
			wantErr: false,
		},
		{
			name: "missing dest",
			args: map[string]interface{}{
				"src": "/tmp/source",
			},
			wantErr: true,
		},
		{
			name: "both src and content",
			args: map[string]interface{}{
				"src":     "/tmp/source",
				"content": "hello",
				"dest":    "/tmp/dest",
			},
			wantErr: true,
		},
		{
			name: "neither src nor content",
			args: map[string]interface{}{
				"dest": "/tmp/dest",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetupModuleRun(t *testing.T) {
	module := NewSetupModule()
	conn := connection.NewLocalConnection()
	ctx := context.Background()

	// Connect first
	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		t.Fatalf("connection failed: %v", err)
	}
	defer conn.Close()

	args := map[string]interface{}{}
	result, err := module.Run(ctx, conn, args)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if !result.Success {
		t.Error("setup module should always succeed")
	}

	if result.Changed {
		t.Error("setup module should never report changes")
	}

	// Check that facts were gathered
	facts, ok := result.Data["ansible_facts"].(map[string]interface{})
	if !ok {
		t.Error("result should contain ansible_facts")
	}

	if len(facts) == 0 {
		t.Error("ansible_facts should not be empty")
	}
}

func TestDebugModuleValidation(t *testing.T) {
	module := NewDebugModule()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid with msg",
			args:    map[string]interface{}{"msg": "hello world"},
			wantErr: false,
		},
		{
			name:    "valid with var",
			args:    map[string]interface{}{"var": "my_variable"},
			wantErr: false,
		},
		{
			name:    "both msg and var",
			args:    map[string]interface{}{"msg": "hello", "var": "test"},
			wantErr: true,
		},
		{
			name:    "neither msg nor var",
			args:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDebugModuleRun(t *testing.T) {
	module := NewDebugModule()
	conn := connection.NewLocalConnection()
	ctx := context.Background()

	// Connect first
	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		t.Fatalf("connection failed: %v", err)
	}
	defer conn.Close()

	// Test with message
	args := map[string]interface{}{"msg": "test message"}
	result, err := module.Run(ctx, conn, args)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if !result.Success {
		t.Error("debug module should always succeed")
	}

	if result.Changed {
		t.Error("debug module should never report changes")
	}

	if result.Message != "test message" {
		t.Errorf("result message expected 'test message', got %s", result.Message)
	}
}

func TestBaseModuleHelpers(t *testing.T) {
	base := NewBaseModule("test", types.ModuleDoc{})

	// Test GetStringArg
	args := map[string]interface{}{
		"string_field": "hello",
		"int_field":    42,
		"bool_field":   true,
	}

	if result := base.GetStringArg(args, "string_field", "default"); result != "hello" {
		t.Errorf("GetStringArg expected 'hello', got %s", result)
	}

	if result := base.GetStringArg(args, "nonexistent", "default"); result != "default" {
		t.Errorf("GetStringArg expected 'default', got %s", result)
	}

	// Test GetBoolArg
	if result := base.GetBoolArg(args, "bool_field", false); !result {
		t.Error("GetBoolArg expected true")
	}

	if result := base.GetBoolArg(args, "nonexistent", true); !result {
		t.Error("GetBoolArg expected default true")
	}

	// Test GetIntArg
	if result, _ := base.GetIntArg(args, "int_field", 0); result != 42 {
		t.Errorf("GetIntArg expected 42, got %d", result)
	}

	if result, _ := base.GetIntArg(args, "nonexistent", 10); result != 10 {
		t.Errorf("GetIntArg expected default 10, got %d", result)
	}
}

func TestModuleRegistryUnregister(t *testing.T) {
	registry := NewModuleRegistry()

	// Unregister existing module
	err := registry.UnregisterModule("command")
	if err != nil {
		t.Fatalf("UnregisterModule failed: %v", err)
	}

	// Try to get unregistered module
	_, err = registry.GetModule("command")
	if err != types.ErrModuleNotFound {
		t.Errorf("expected ErrModuleNotFound, got %v", err)
	}

	// Try to unregister non-existent module
	err = registry.UnregisterModule("nonexistent")
	if err != types.ErrModuleNotFound {
		t.Errorf("expected ErrModuleNotFound, got %v", err)
	}
}

// Benchmark tests
func BenchmarkCommandModuleRun(b *testing.B) {
	module := NewCommandModule()
	conn := connection.NewLocalConnection()
	ctx := context.Background()

	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		b.Fatalf("connection failed: %v", err)
	}
	defer conn.Close()

	args := map[string]interface{}{"cmd": "echo 'benchmark test'"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := module.Run(ctx, conn, args)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkModuleRegistryGetModule(b *testing.B) {
	registry := NewModuleRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := registry.GetModule("command")
		if err != nil {
			b.Fatal(err)
		}
	}
}
