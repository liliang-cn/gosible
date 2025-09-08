package modules

import (
	"context"
	"testing"
	
	"github.com/liliang-cn/gosinble/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestPackageModule_Validate(t *testing.T) {
	module := NewPackageModule()
	
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing name",
			args:    map[string]interface{}{},
			wantErr: true,
			errMsg:  "required field is missing",
		},
		{
			name: "valid present state",
			args: map[string]interface{}{
				"name":  "nginx",
				"state": "present",
			},
			wantErr: false,
		},
		{
			name: "valid absent state",
			args: map[string]interface{}{
				"name":  "nginx",
				"state": "absent",
			},
			wantErr: false,
		},
		{
			name: "valid latest state",
			args: map[string]interface{}{
				"name":  "nginx",
				"state": "latest",
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			args: map[string]interface{}{
				"name":  "nginx",
				"state": "invalid",
			},
			wantErr: true,
			errMsg:  "must be one of",
		},
		{
			name: "multiple packages",
			args: map[string]interface{}{
				"name": "git,vim,curl",
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPackageModule_Run_InstallPackage(t *testing.T) {
	module := NewPackageModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":  "nginx",
		"state": "present",
	}
	
	// Mock: Detect package manager (apt)
	mockConn.On("Execute", ctx, "which apt-get", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "/usr/bin/apt-get",
	}, nil)
	
	// Mock: Check if package is installed
	mockConn.On("Execute", ctx, "dpkg -l nginx 2>/dev/null | grep -q '^ii'", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: false,
	}, nil)
	
	// Mock: Install package
	mockConn.On("Execute", ctx, "DEBIAN_FRONTEND=noninteractive apt-get install -y nginx", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Contains(t, result.Message, "state changed")
	assert.Equal(t, "apt", result.Data["package_manager"])
	
	mockConn.AssertExpectations(t)
}

func TestPackageModule_Run_RemovePackage(t *testing.T) {
	module := NewPackageModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":  "nginx",
		"state": "absent",
	}
	
	// Mock: Detect package manager (apt)
	mockConn.On("Execute", ctx, "which apt-get", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "/usr/bin/apt-get",
	}, nil)
	
	// Mock: Check if package is installed (it is)
	mockConn.On("Execute", ctx, "dpkg -l nginx 2>/dev/null | grep -q '^ii'", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Remove package
	mockConn.On("Execute", ctx, "DEBIAN_FRONTEND=noninteractive apt-get remove -y nginx", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	
	mockConn.AssertExpectations(t)
}

func TestPackageModule_Run_UpdateCache(t *testing.T) {
	module := NewPackageModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":         "nginx",
		"state":        "present",
		"update_cache": true,
	}
	
	// Mock: Detect package manager (apt)
	mockConn.On("Execute", ctx, "which apt-get", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "/usr/bin/apt-get",
	}, nil)
	
	// Mock: Update cache
	mockConn.On("Execute", ctx, "apt-get update", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Check if package is installed (already installed)
	mockConn.On("Execute", ctx, "dpkg -l nginx 2>/dev/null | grep -q '^ii'", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed) // Changed because cache was updated
	
	mockConn.AssertExpectations(t)
}

func TestPackageModule_ParsePackageList(t *testing.T) {
	module := NewPackageModule()
	
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single package",
			input:    "nginx",
			expected: []string{"nginx"},
		},
		{
			name:     "comma separated",
			input:    "git,vim,curl",
			expected: []string{"git", "vim", "curl"},
		},
		{
			name:     "space separated",
			input:    "git vim curl",
			expected: []string{"git", "vim", "curl"},
		},
		{
			name:     "mixed separators",
			input:    "git, vim curl",
			expected: []string{"git", "vim", "curl"},
		},
		{
			name:     "with extra spaces",
			input:    "  git  ,  vim  ,  curl  ",
			expected: []string{"git", "vim", "curl"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := module.parsePackageList(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}