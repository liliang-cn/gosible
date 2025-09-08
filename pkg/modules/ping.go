package modules

import (
	"context"
	
	"github.com/liliang-cn/gosinble/pkg/types"
)

// PingModule implements a simple connectivity test
type PingModule struct {
	BaseModule
}

// NewPingModule creates a new ping module instance
func NewPingModule() *PingModule {
	return &PingModule{
		BaseModule: BaseModule{
			name: "ping",
		},
	}
}

// Run executes the ping module
func (m *PingModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	result := &types.Result{
		Success: true,
		Changed: false,
		Message: "pong",
		Data: map[string]interface{}{
			"ping": "pong",
		},
	}
	
	// Test connection by running a simple command
	if conn != nil && conn.IsConnected() {
		// Try to execute a simple echo command to verify connection works
		testResult, err := conn.Execute(ctx, "echo pong", types.ExecuteOptions{})
		if err != nil {
			result.Success = false
			result.Error = err
			result.Message = "Connection test failed"
		} else {
			result.Success = testResult.Success
			if testResult.Success {
				result.Message = "pong"
			}
		}
	}
	
	return result, nil
}

// Validate checks if the module arguments are valid
func (m *PingModule) Validate(args map[string]interface{}) error {
	// Ping module doesn't require any arguments
	return nil
}

// Documentation returns the module documentation
func (m *PingModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "ping",
		Description: "Test connectivity to hosts",
		Parameters:  map[string]types.ParamDoc{},
		Examples: []string{
			"- name: Test connectivity\n  ping:",
		},
		Returns: map[string]string{
			"ping": "Returns 'pong' when successful",
		},
	}
}