package modules

import (
	"testing"
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
		
		tests := []struct {
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
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := m.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}