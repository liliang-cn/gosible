package modules

import (
	"context"
	"testing"
	
	"github.com/liliang-cn/gosible/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestGroupModule_Validate(t *testing.T) {
	module := NewGroupModule()
	
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
				"name":  "testgroup",
				"state": "present",
			},
			wantErr: false,
		},
		{
			name: "valid absent state",
			args: map[string]interface{}{
				"name":  "testgroup",
				"state": "absent",
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			args: map[string]interface{}{
				"name":  "testgroup",
				"state": "invalid",
			},
			wantErr: true,
			errMsg:  "must be one of",
		},
		{
			name: "with gid",
			args: map[string]interface{}{
				"name": "testgroup",
				"gid":  2001,
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

func TestGroupModule_Run_CreateGroup(t *testing.T) {
	module := NewGroupModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":  "testgroup",
		"state": "present",
		"gid":   2001,
	}
	
	// Mock: Check if group exists (first call)
	mockConn.On("Execute", ctx, "getent group testgroup 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: false,
		Message: "",
	}, nil).Once()
	
	// Mock: Create group
	mockConn.On("Execute", ctx, "groupadd -g 2001 testgroup", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Get group info after creation (second call)
	mockConn.On("Execute", ctx, "getent group testgroup 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "testgroup:x:2001:",
	}, nil).Once()
	
	// Mock: Get group info for members (third call)
	mockConn.On("Execute", ctx, "getent group testgroup 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "testgroup:x:2001:",
	}, nil).Once()
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "Group testgroup created", result.Message)
	assert.Equal(t, "2001", result.Data["gid"])
	
	mockConn.AssertExpectations(t)
}

func TestGroupModule_Run_RemoveGroup(t *testing.T) {
	module := NewGroupModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":  "testgroup",
		"state": "absent",
	}
	
	// Mock: Check if group exists (it does)
	mockConn.On("Execute", ctx, "getent group testgroup 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "testgroup:x:2001:",
	}, nil)
	
	// Mock: Remove group
	mockConn.On("Execute", ctx, "groupdel testgroup", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "Group testgroup removed", result.Message)
	
	mockConn.AssertExpectations(t)
}

func TestGroupModule_Run_UpdateGID(t *testing.T) {
	module := NewGroupModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":  "testgroup",
		"state": "present",
		"gid":   2002,
	}
	
	// Mock: Check if group exists with different GID (first call)
	mockConn.On("Execute", ctx, "getent group testgroup 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "testgroup:x:2001:",
	}, nil).Once()
	
	// Mock: Update GID
	mockConn.On("Execute", ctx, "groupmod -g 2002 testgroup", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Get updated group info (second call)
	mockConn.On("Execute", ctx, "getent group testgroup 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "testgroup:x:2002:",
	}, nil).Once()
	
	// Mock: Get group info for members (third call)
	mockConn.On("Execute", ctx, "getent group testgroup 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "testgroup:x:2002:",
	}, nil).Once()
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "Group testgroup GID updated to 2002", result.Message)
	assert.Equal(t, "2002", result.Data["gid"])
	
	mockConn.AssertExpectations(t)
}

func TestGroupModule_GroupExists(t *testing.T) {
	module := NewGroupModule()
	ctx := context.Background()
	
	tests := []struct {
		name         string
		groupName    string
		cmdResult    *types.Result
		cmdError     error
		expectExists bool
		expectGID    int
	}{
		{
			name:      "group exists",
			groupName: "testgroup",
			cmdResult: &types.Result{
				Success: true,
				Message: "testgroup:x:2001:user1,user2",
			},
			expectExists: true,
			expectGID:    2001,
		},
		{
			name:      "group does not exist",
			groupName: "nogroup",
			cmdResult: &types.Result{
				Success: false,
				Message: "",
			},
			expectExists: false,
			expectGID:    0,
		},
		{
			name:      "malformed output",
			groupName: "badgroup",
			cmdResult: &types.Result{
				Success: true,
				Message: "invalid",
			},
			expectExists: true,
			expectGID:    0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := new(MockConnection)
			conn.On("Execute", ctx, "getent group "+tt.groupName+" 2>/dev/null", 
				types.ExecuteOptions{}).Return(tt.cmdResult, tt.cmdError)
			
			exists, gid := module.groupExists(ctx, conn, tt.groupName)
			assert.Equal(t, tt.expectExists, exists)
			assert.Equal(t, tt.expectGID, gid)
			
			conn.AssertExpectations(t)
		})
	}
}