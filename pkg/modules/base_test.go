package modules

import (
	"context"
	"testing"

	"github.com/gosinble/gosinble/pkg/types"
)

func TestBaseModule_CheckMode(t *testing.T) {
	base := NewBaseModule("test", types.ModuleDoc{})
	
	tests := []struct {
		name     string
		args     map[string]interface{}
		expected bool
	}{
		{
			name:     "check mode enabled",
			args:     map[string]interface{}{"_check_mode": true},
			expected: true,
		},
		{
			name:     "check mode disabled",
			args:     map[string]interface{}{"_check_mode": false},
			expected: false,
		},
		{
			name:     "check mode not set",
			args:     map[string]interface{}{},
			expected: false,
		},
		{
			name:     "check mode wrong type",
			args:     map[string]interface{}{"_check_mode": "true"},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base.CheckMode(tt.args)
			if result != tt.expected {
				t.Errorf("CheckMode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBaseModule_DiffMode(t *testing.T) {
	base := NewBaseModule("test", types.ModuleDoc{})
	
	tests := []struct {
		name     string
		args     map[string]interface{}
		expected bool
	}{
		{
			name:     "diff mode enabled",
			args:     map[string]interface{}{"_diff": true},
			expected: true,
		},
		{
			name:     "diff mode disabled",
			args:     map[string]interface{}{"_diff": false},
			expected: false,
		},
		{
			name:     "diff mode not set",
			args:     map[string]interface{}{},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base.DiffMode(tt.args)
			if result != tt.expected {
				t.Errorf("DiffMode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBaseModule_CreateCheckModeResult(t *testing.T) {
	base := NewBaseModule("test", types.ModuleDoc{})
	
	result := base.CreateCheckModeResult("testhost", true, "Would install package", map[string]interface{}{
		"package": "nginx",
	})
	
	if !result.Success {
		t.Error("Check mode result should be successful")
	}
	
	if !result.Changed {
		t.Error("Check mode result should show changed=true when would change")
	}
	
	if !result.Simulated {
		t.Error("Check mode result should have Simulated=true")
	}
	
	if result.Data["check_mode"] != true {
		t.Error("Check mode result should have check_mode=true in data")
	}
	
	if result.Data["would_change"] != true {
		t.Error("Check mode result should have would_change=true in data")
	}
}

func TestBaseModule_GenerateDiff(t *testing.T) {
	base := NewBaseModule("test", types.ModuleDoc{})
	
	tests := []struct {
		name   string
		before string
		after  string
		wantNil bool
	}{
		{
			name:    "different content",
			before:  "line1\nline2\n",
			after:   "line1\nline2\nline3\n",
			wantNil: false,
		},
		{
			name:    "same content",
			before:  "same",
			after:   "same",
			wantNil: true,
		},
		{
			name:    "empty to content",
			before:  "",
			after:   "new content",
			wantNil: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := base.GenerateDiff(tt.before, tt.after)
			if tt.wantNil && diff != nil {
				t.Error("Expected nil diff for same content")
			}
			if !tt.wantNil && diff == nil {
				t.Error("Expected diff for different content")
			}
			if diff != nil {
				if diff.Before != tt.before {
					t.Errorf("Diff.Before = %v, want %v", diff.Before, tt.before)
				}
				if diff.After != tt.after {
					t.Errorf("Diff.After = %v, want %v", diff.After, tt.after)
				}
				if !diff.Prepared {
					t.Error("Diff should be marked as prepared")
				}
			}
		})
	}
}

func TestBaseModule_Capabilities(t *testing.T) {
	base := NewBaseModule("test", types.ModuleDoc{})
	
	// Test default capabilities
	caps := base.Capabilities()
	if caps == nil {
		t.Fatal("Expected default capabilities")
	}
	
	if !caps.CheckMode {
		t.Error("Default capabilities should support check mode")
	}
	
	if caps.DiffMode {
		t.Error("Default capabilities should not support diff mode by default")
	}
	
	// Test setting custom capabilities
	customCaps := &types.ModuleCapability{
		CheckMode: true,
		DiffMode:  true,
		Platform:  "linux",
	}
	
	base.SetCapabilities(customCaps)
	
	caps = base.Capabilities()
	if !caps.DiffMode {
		t.Error("Custom capabilities should support diff mode")
	}
	
	if caps.Platform != "linux" {
		t.Errorf("Expected platform=linux, got %s", caps.Platform)
	}
}

func TestBaseModule_RunWithModes(t *testing.T) {
	base := NewBaseModule("test", types.ModuleDoc{})
	
	// Create a mock module
	mockModule := &MockModule{
		BaseModule: base,
		runFunc: func(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
			// Check if modes were injected
			if args["_check_mode"] == true {
				return base.CreateCheckModeResult("testhost", true, "Would change", nil), nil
			}
			return base.CreateSuccessResult("testhost", true, "Changed", nil), nil
		},
	}
	
	// Test check mode injection
	opts := types.ExecuteOptions{
		CheckMode: true,
		DiffMode:  false,
	}
	
	args := make(map[string]interface{})
	result, err := base.RunWithModes(context.Background(), mockModule, nil, args, opts)
	
	if err != nil {
		t.Errorf("RunWithModes failed: %v", err)
	}
	
	if args["_check_mode"] != true {
		t.Error("Check mode flag not injected into args")
	}
	
	if result != nil && !result.Simulated {
		t.Error("Result should be simulated in check mode")
	}
	
	// Test diff mode injection
	opts = types.ExecuteOptions{
		CheckMode: false,
		DiffMode:  true,
	}
	
	// Set capabilities to support diff mode
	base.SetCapabilities(&types.ModuleCapability{
		CheckMode: true,
		DiffMode:  true,
	})
	
	args = make(map[string]interface{})
	_, err = base.RunWithModes(context.Background(), mockModule, nil, args, opts)
	
	if err != nil {
		t.Errorf("RunWithModes failed: %v", err)
	}
	
	if args["_diff"] != true {
		t.Error("Diff mode flag not injected into args")
	}
}

// MockModule for testing
type MockModule struct {
	*BaseModule
	runFunc func(context.Context, types.Connection, map[string]interface{}) (*types.Result, error)
}

func (m *MockModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, conn, args)
	}
	return m.CreateSuccessResult("testhost", false, "No changes", nil), nil
}

func (m *MockModule) Validate(args map[string]interface{}) error {
	return nil
}