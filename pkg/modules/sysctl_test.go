package modules

import (
	"context"
	"testing"

	gotest "github.com/gosinble/gosinble/pkg/testing"
)

func TestSysctlModule(t *testing.T) {
	t.Run("ModuleProperties", func(t *testing.T) {
		m := NewSysctlModule()
		if m.Name() != "sysctl" {
			t.Errorf("Expected module name 'sysctl', got %s", m.Name())
		}
	})

	t.Run("ValidationTests", func(t *testing.T) {
		m := NewSysctlModule()
		
		testCases := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name: "ValidSetValue",
				args: map[string]interface{}{
					"name":  "net.ipv4.ip_forward",
					"value": "1",
					"state": "present",
				},
				wantErr: false,
			},
			{
				name: "ValidPersistent",
				args: map[string]interface{}{
					"name":       "kernel.panic",
					"value":      "10",
					"state":      "present",
					"persistent": true,
				},
				wantErr: false,
			},
			{
				name: "ValidRemove",
				args: map[string]interface{}{
					"name":  "net.ipv4.ip_forward",
					"state": "absent",
				},
				wantErr: false,
			},
			{
				name: "MissingName",
				args: map[string]interface{}{
					"value": "1",
					"state": "present",
				},
				wantErr: true,
			},
			{
				name: "MissingValueForPresent",
				args: map[string]interface{}{
					"name":  "net.ipv4.ip_forward",
					"state": "present",
				},
				wantErr: true,
			},
			{
				name: "InvalidState",
				args: map[string]interface{}{
					"name":  "net.ipv4.ip_forward",
					"value": "1",
					"state": "invalid",
				},
				wantErr: true,
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := m.Validate(tc.args)
				if (err != nil) != tc.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("SysctlSetTests", func(t *testing.T) {
		m := NewSysctlModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("SetValue", func(t *testing.T) {
			// Check current value
			conn.ExpectCommand("sysctl -n net.ipv4.ip_forward", &gotest.CommandResponse{
				Stdout:   "0",
				ExitCode: 0,
			})
			
			// Set new value
			conn.ExpectCommand("sysctl -w net.ipv4.ip_forward=1", &gotest.CommandResponse{
				Stdout:   "net.ipv4.ip_forward = 1",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":  "net.ipv4.ip_forward",
				"value": "1",
				"state": "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("ValueAlreadySet", func(t *testing.T) {
			conn.Reset()
			// Check current value
			conn.ExpectCommand("sysctl -n net.ipv4.ip_forward", &gotest.CommandResponse{
				Stdout:   "1",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":  "net.ipv4.ip_forward",
				"value": "1",
				"state": "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertNotChanged(result)
			conn.Verify()
		})
		
		t.Run("SetPersistent", func(t *testing.T) {
			conn.Reset()
			// Check current value
			conn.ExpectCommand("sysctl -n kernel.panic", &gotest.CommandResponse{
				Stdout:   "0",
				ExitCode: 0,
			})
			
			// Set new value
			conn.ExpectCommand("sysctl -w kernel.panic=10", &gotest.CommandResponse{
				Stdout:   "kernel.panic = 10",
				ExitCode: 0,
			})
			
			// Make persistent
			conn.ExpectCommand("echo 'kernel.panic=10' >> /etc/sysctl.conf", &gotest.CommandResponse{
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":       "kernel.panic",
				"value":      "10",
				"state":      "present",
				"persistent": true,
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
	})
}