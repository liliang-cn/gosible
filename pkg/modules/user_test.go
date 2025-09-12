package modules

import (
	"context"
	"testing"
	
	"github.com/liliang-cn/gosible/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestUserModule_Validate(t *testing.T) {
	module := NewUserModule()
	
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
				"name":  "testuser",
				"state": "present",
			},
			wantErr: false,
		},
		{
			name: "valid absent state",
			args: map[string]interface{}{
				"name":  "testuser",
				"state": "absent",
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			args: map[string]interface{}{
				"name":  "testuser",
				"state": "invalid",
			},
			wantErr: true,
			errMsg:  "must be one of",
		},
		{
			name: "with uid",
			args: map[string]interface{}{
				"name": "testuser",
				"uid":  1001,
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

func TestUserModule_Run_CreateUser(t *testing.T) {
	module := NewUserModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":  "testuser",
		"state": "present",
		"uid":   1001,
		"shell": "/bin/bash",
	}
	
	// Mock: Check if user exists
	mockConn.On("Execute", ctx, "id testuser >/dev/null 2>&1", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: false,
	}, nil)
	
	// Mock: Create user
	mockConn.On("Execute", ctx, "useradd -u 1001 -s /bin/bash -m testuser", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Get user info
	mockConn.On("Execute", ctx, "getent passwd testuser", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "testuser:x:1001:1001::/home/testuser:/bin/bash",
	}, nil)
	
	mockConn.On("Execute", ctx, "groups testuser 2>/dev/null | cut -d: -f2", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: " testuser",
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "User testuser created", result.Message)
	assert.Equal(t, "1001", result.Data["uid"])
	assert.Equal(t, "/bin/bash", result.Data["shell"])
	
	mockConn.AssertExpectations(t)
}

func TestUserModule_Run_RemoveUser(t *testing.T) {
	module := NewUserModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":   "testuser",
		"state":  "absent",
		"remove": true,
	}
	
	// Mock: Check if user exists (it does)
	mockConn.On("Execute", ctx, "id testuser >/dev/null 2>&1", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Remove user with home directory
	mockConn.On("Execute", ctx, "userdel -r testuser", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "User testuser removed", result.Message)
	
	mockConn.AssertExpectations(t)
}

func TestUserModule_Run_UpdateUser(t *testing.T) {
	module := NewUserModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":  "testuser",
		"state": "present",
		"shell": "/bin/zsh",
	}
	
	// Mock: Check if user exists (it does)
	mockConn.On("Execute", ctx, "id testuser >/dev/null 2>&1", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Get current user info (first call)
	mockConn.On("Execute", ctx, "getent passwd testuser", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "testuser:x:1001:1001::/home/testuser:/bin/bash",
	}, nil).Once()
	
	mockConn.On("Execute", ctx, "groups testuser 2>/dev/null | cut -d: -f2", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: " testuser",
	}, nil).Once()
	
	// Mock: Update shell
	mockConn.On("Execute", ctx, "usermod -s /bin/zsh testuser", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Get updated user info (second call)
	mockConn.On("Execute", ctx, "getent passwd testuser", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "testuser:x:1001:1001::/home/testuser:/bin/zsh",
	}, nil).Once()
	
	mockConn.On("Execute", ctx, "groups testuser 2>/dev/null | cut -d: -f2", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: " testuser",
	}, nil).Once()
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "User testuser updated", result.Message)
	assert.Equal(t, "/bin/zsh", result.Data["shell"])
	
	mockConn.AssertExpectations(t)
}

func TestUserModule_ToInt(t *testing.T) {
	module := NewUserModule()
	
	tests := []struct {
		name     string
		input    interface{}
		expected int
		wantErr  bool
	}{
		{
			name:     "int",
			input:    42,
			expected: 42,
			wantErr:  false,
		},
		{
			name:     "int64",
			input:    int64(42),
			expected: 42,
			wantErr:  false,
		},
		{
			name:     "float64",
			input:    float64(42),
			expected: 42,
			wantErr:  false,
		},
		{
			name:     "string",
			input:    "42",
			expected: 42,
			wantErr:  false,
		},
		{
			name:     "invalid string",
			input:    "invalid",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "unsupported type",
			input:    []int{42},
			expected: 0,
			wantErr:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := module.toInt(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}