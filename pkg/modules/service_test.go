package modules

import (
	"context"
	"testing"
	
	"github.com/gosinble/gosinble/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestServiceModule_Validate(t *testing.T) {
	module := NewServiceModule()
	
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
			name: "valid start state",
			args: map[string]interface{}{
				"name":  "nginx",
				"state": "started",
			},
			wantErr: false,
		},
		{
			name: "valid stop state",
			args: map[string]interface{}{
				"name":  "nginx",
				"state": "stopped",
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
			name: "with enabled",
			args: map[string]interface{}{
				"name":    "nginx",
				"enabled": true,
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

func TestServiceModule_Run_StartService(t *testing.T) {
	module := NewServiceModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":  "nginx",
		"state": "started",
	}
	
	// Mock: Detect init system
	mockConn.On("Execute", ctx, "which systemctl 2>/dev/null && echo systemd", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "/usr/bin/systemctl\nsystemd",
	}, nil)
	
	// Mock: Check service status (first call)
	mockConn.On("Execute", ctx, "systemctl is-active nginx 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: false,
		Message: "inactive",
	}, nil).Once()
	
	// Mock: Start service
	mockConn.On("Execute", ctx, "systemctl start nginx", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Get final status (second call)
	mockConn.On("Execute", ctx, "systemctl is-active nginx 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "active",
	}, nil).Once()
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Contains(t, result.Message, "state changed")
	assert.Equal(t, "systemd", result.Data["init_system"])
	assert.Equal(t, "active", result.Data["status"])
	
	mockConn.AssertExpectations(t)
}

func TestServiceModule_Run_StopService(t *testing.T) {
	module := NewServiceModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":  "nginx",
		"state": "stopped",
	}
	
	// Mock: Detect init system
	mockConn.On("Execute", ctx, "which systemctl 2>/dev/null && echo systemd", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "/usr/bin/systemctl\nsystemd",
	}, nil)
	
	// Mock: Check service status (running)
	mockConn.On("Execute", ctx, "systemctl is-active nginx 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "active",
	}, nil)
	
	// Mock: Stop service
	mockConn.On("Execute", ctx, "systemctl stop nginx", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Get final status
	mockConn.On("Execute", ctx, "systemctl is-active nginx 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: false,
		Message: "inactive",
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	
	mockConn.AssertExpectations(t)
}

func TestServiceModule_Run_EnableService(t *testing.T) {
	module := NewServiceModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{
		"name":    "nginx",
		"enabled": true,
	}
	
	// Mock: Detect init system
	mockConn.On("Execute", ctx, "which systemctl 2>/dev/null && echo systemd", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "/usr/bin/systemctl\nsystemd",
	}, nil)
	
	// Mock: Check if enabled
	mockConn.On("Execute", ctx, "systemctl is-enabled nginx 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: false,
		Message: "disabled",
	}, nil)
	
	// Mock: Enable service
	mockConn.On("Execute", ctx, "systemctl enable nginx", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Get final status
	mockConn.On("Execute", ctx, "systemctl is-active nginx 2>/dev/null", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "active",
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	
	mockConn.AssertExpectations(t)
}