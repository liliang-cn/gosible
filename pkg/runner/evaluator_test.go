package runner

import (
	"testing"
	
	"github.com/liliang-cn/gosinble/pkg/types"
)

func TestEvaluateWhen(t *testing.T) {
	tests := []struct {
		name      string
		condition interface{}
		vars      map[string]interface{}
		expected  bool
		expectErr bool
	}{
		{
			name:      "nil condition returns true",
			condition: nil,
			vars:      map[string]interface{}{},
			expected:  true,
		},
		{
			name:      "boolean true",
			condition: true,
			vars:      map[string]interface{}{},
			expected:  true,
		},
		{
			name:      "boolean false",
			condition: false,
			vars:      map[string]interface{}{},
			expected:  false,
		},
		{
			name:      "string true",
			condition: "true",
			vars:      map[string]interface{}{},
			expected:  true,
		},
		{
			name:      "string false",
			condition: "false",
			vars:      map[string]interface{}{},
			expected:  false,
		},
		{
			name:      "variable exists",
			condition: "my_var is defined",
			vars:      map[string]interface{}{"my_var": "value"},
			expected:  true,
		},
		{
			name:      "variable undefined",
			condition: "my_var is undefined",
			vars:      map[string]interface{}{},
			expected:  true,
		},
		{
			name:      "equality check",
			condition: "my_var == 'test'",
			vars:      map[string]interface{}{"my_var": "test"},
			expected:  true,
		},
		{
			name:      "inequality check",
			condition: "my_var != 'test'",
			vars:      map[string]interface{}{"my_var": "other"},
			expected:  true,
		},
		{
			name:      "numeric comparison",
			condition: "count > 5",
			vars:      map[string]interface{}{"count": 10},
			expected:  true,
		},
		{
			name:      "in operator",
			condition: "'prod' in environments",
			vars:      map[string]interface{}{"environments": []interface{}{"dev", "test", "prod"}},
			expected:  true,
		},
		{
			name:      "not in operator",
			condition: "'staging' not in environments",
			vars:      map[string]interface{}{"environments": []interface{}{"dev", "test", "prod"}},
			expected:  true,
		},
		{
			name:      "and operator",
			condition: "os == 'linux' and arch == 'x86_64'",
			vars:      map[string]interface{}{"os": "linux", "arch": "x86_64"},
			expected:  true,
		},
		{
			name:      "or operator",
			condition: "env == 'prod' or env == 'staging'",
			vars:      map[string]interface{}{"env": "prod"},
			expected:  true,
		},
		{
			name:      "not operator",
			condition: "not debug_mode",
			vars:      map[string]interface{}{"debug_mode": false},
			expected:  true,
		},
		{
			name:      "nested object access",
			condition: "config.database.host == 'localhost'",
			vars: map[string]interface{}{
				"config": map[string]interface{}{
					"database": map[string]interface{}{
						"host": "localhost",
					},
				},
			},
			expected: true,
		},
		{
			name:      "array index access",
			condition: "items[0] == 'first'",
			vars:      map[string]interface{}{"items": []interface{}{"first", "second", "third"}},
			expected:  true,
		},
		{
			name:      "multiple conditions in list",
			condition: []interface{}{"os == 'linux'", "arch == 'x86_64'"},
			vars:      map[string]interface{}{"os": "linux", "arch": "x86_64"},
			expected:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewConditionEvaluator(tt.vars)
			result, err := evaluator.EvaluateWhen(tt.condition)
			
			if tt.expectErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateFailedWhen(t *testing.T) {
	tests := []struct {
		name      string
		condition interface{}
		result    *types.Result
		vars      map[string]interface{}
		expected  bool
	}{
		{
			name:      "nil condition with success",
			condition: nil,
			result:    &types.Result{Success: true},
			expected:  false,
		},
		{
			name:      "nil condition with failure",
			condition: nil,
			result:    &types.Result{Success: false},
			expected:  true,
		},
		{
			name:      "check exit code",
			condition: "rc != 0",
			result:    &types.Result{Data: map[string]interface{}{"exit_code": 1}},
			expected:  true,
		},
		{
			name:      "check stderr",
			condition: "'error' in stderr",
			result:    &types.Result{Data: map[string]interface{}{"stderr": "error occurred"}},
			expected:  true,
		},
		{
			name:      "check stdout",
			condition: "'failed' in stdout",
			result:    &types.Result{Data: map[string]interface{}{"stdout": "operation failed"}},
			expected:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewConditionEvaluator(tt.vars)
			result, err := evaluator.EvaluateFailedWhen(tt.condition, tt.result)
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateChangedWhen(t *testing.T) {
	tests := []struct {
		name      string
		condition interface{}
		result    *types.Result
		vars      map[string]interface{}
		expected  bool
	}{
		{
			name:      "nil condition uses module changed",
			condition: nil,
			result:    &types.Result{Changed: true},
			expected:  true,
		},
		{
			name:      "false means never changed",
			condition: false,
			result:    &types.Result{Changed: true},
			expected:  false,
		},
		{
			name:      "check exit code",
			condition: "rc == 0",
			result:    &types.Result{Data: map[string]interface{}{"exit_code": 0}},
			expected:  true,
		},
		{
			name:      "check stdout content",
			condition: "'created' in stdout",
			result:    &types.Result{Data: map[string]interface{}{"stdout": "file created"}},
			expected:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewConditionEvaluator(tt.vars)
			result, err := evaluator.EvaluateChangedWhen(tt.condition, tt.result)
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateLoopItems(t *testing.T) {
	tests := []struct {
		name     string
		loop     interface{}
		vars     map[string]interface{}
		expected []interface{}
	}{
		{
			name:     "nil loop returns nil",
			loop:     nil,
			expected: nil,
		},
		{
			name:     "direct array",
			loop:     []interface{}{"a", "b", "c"},
			expected: []interface{}{"a", "b", "c"},
		},
		{
			name:     "variable reference",
			loop:     "my_items",
			vars:     map[string]interface{}{"my_items": []interface{}{1, 2, 3}},
			expected: []interface{}{1, 2, 3},
		},
		{
			name:     "range expression",
			loop:     "1-5",
			expected: []interface{}{1, 2, 3, 4, 5},
		},
		{
			name:     "single value",
			loop:     "single_item",
			vars:     map[string]interface{}{"single_item": "value"},
			expected: []interface{}{"value"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewConditionEvaluator(tt.vars)
			result, err := evaluator.EvaluateLoopItems(tt.loop)
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d items, got %d", len(tt.expected), len(result))
				return
			}
			for i, item := range result {
				if item != tt.expected[i] {
					t.Errorf("item %d: expected %v, got %v", i, tt.expected[i], item)
				}
			}
		})
	}
}