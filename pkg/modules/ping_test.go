package modules

import (
	"context"
	"testing"
	
	"github.com/liliang-cn/gosible/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestPingModule_Validate(t *testing.T) {
	module := NewPingModule()
	
	// Ping module doesn't require any arguments
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "no arguments",
			args:    map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "with extra arguments",
			args: map[string]interface{}{
				"extra": "value",
			},
			wantErr: false, // Should still be valid
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPingModule_Run_Success(t *testing.T) {
	module := NewPingModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{}
	
	// Mock: Connection is connected
	mockConn.On("IsConnected").Return(true)
	
	// Mock: Execute echo command
	mockConn.On("Execute", ctx, "echo pong", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "pong",
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.False(t, result.Changed)
	assert.Equal(t, "pong", result.Message)
	assert.Equal(t, "pong", result.Data["ping"])
	
	mockConn.AssertExpectations(t)
}

func TestPingModule_Run_ConnectionFailed(t *testing.T) {
	module := NewPingModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{}
	
	// Mock: Connection is connected
	mockConn.On("IsConnected").Return(true)
	
	// Mock: Execute echo command fails
	mockConn.On("Execute", ctx, "echo pong", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: false,
		Message: "Connection error",
	}, assert.AnError)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err) // Module doesn't return error, just sets result.Success
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.False(t, result.Changed)
	assert.Equal(t, "Connection test failed", result.Message)
	assert.NotNil(t, result.Error)
	
	mockConn.AssertExpectations(t)
}

func TestPingModule_Run_NotConnected(t *testing.T) {
	module := NewPingModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	args := map[string]interface{}{}
	
	// Mock: Connection is not connected
	mockConn.On("IsConnected").Return(false)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success) // Still returns success for nil connection
	assert.False(t, result.Changed)
	assert.Equal(t, "pong", result.Message)
	assert.Equal(t, "pong", result.Data["ping"])
	
	mockConn.AssertExpectations(t)
}

func TestPingModule_Run_NilConnection(t *testing.T) {
	module := NewPingModule()
	ctx := context.Background()
	
	args := map[string]interface{}{}
	
	// Test with nil connection
	result, err := module.Run(ctx, nil, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.False(t, result.Changed)
	assert.Equal(t, "pong", result.Message)
	assert.Equal(t, "pong", result.Data["ping"])
}

func TestPingModule_Documentation(t *testing.T) {
	module := NewPingModule()
	doc := module.Documentation()
	
	assert.Equal(t, "ping", doc.Name)
	assert.Contains(t, doc.Description, "connectivity")
	assert.Empty(t, doc.Parameters) // Ping doesn't require parameters
	assert.NotEmpty(t, doc.Examples)
	assert.NotEmpty(t, doc.Returns)
	assert.Contains(t, doc.Returns["ping"], "pong")
}