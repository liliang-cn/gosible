package modules

import (
	"testing"
)

func TestMountModule(t *testing.T) {
	t.Run("ModuleProperties", func(t *testing.T) {
		m := NewMountModule()
		if m.Name() != "mount" {
			t.Errorf("Expected module name 'mount', got %s", m.Name())
		}
	})

	t.Run("ValidationTests", func(t *testing.T) {
		m := NewMountModule()
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name: "ValidMount",
				args: map[string]interface{}{
					"path":   "/mnt/data",
					"src":    "/dev/sdb1",
					"fstype": "ext4",
					"state":  "mounted",
				},
				wantErr: false,
			},
			{
				name: "ValidUnmount",
				args: map[string]interface{}{
					"path":  "/mnt/data",
					"state": "unmounted",
				},
				wantErr: false,
			},
			{
				name: "ValidRemount",
				args: map[string]interface{}{
					"path":  "/mnt/data",
					"state": "remounted",
				},
				wantErr: false,
			},
			{
				name: "ValidAbsent",
				args: map[string]interface{}{
					"path":  "/mnt/data",
					"state": "absent",
				},
				wantErr: false,
			},
			{
				name: "MissingPath",
				args: map[string]interface{}{
					"state": "mounted",
				},
				wantErr: true,
			},
			{
				name: "MissingSrcForMount",
				args: map[string]interface{}{
					"path":  "/mnt/data",
					"state": "mounted",
				},
				wantErr: true,
			},
			{
				name: "InvalidState",
				args: map[string]interface{}{
					"path":  "/mnt/data",
					"src":   "/dev/sdb1",
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