package modules

import (
	"testing"
)

func TestNpmModule(t *testing.T) {
	t.Run("ModuleProperties", func(t *testing.T) {
		m := NewNpmModule()
		if m.Name() != "npm" {
			t.Errorf("Expected module name 'npm', got %s", m.Name())
		}
	})

	t.Run("ValidationTests", func(t *testing.T) {
		m := NewNpmModule()
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name: "ValidInstall",
				args: map[string]interface{}{
					"name":  "express",
					"state": "present",
				},
				wantErr: false,
			},
			{
				name: "ValidWithVersion",
				args: map[string]interface{}{
					"name":    "react",
					"version": "18.2.0",
					"state":   "present",
				},
				wantErr: false,
			},
			{
				name: "ValidGlobalInstall",
				args: map[string]interface{}{
					"name":   "typescript",
					"global": true,
					"state":  "present",
				},
				wantErr: false,
			},
			{
				name: "ValidRemove",
				args: map[string]interface{}{
					"name":  "lodash",
					"state": "absent",
				},
				wantErr: false,
			},
			{
				name: "ValidWithoutName",
				args: map[string]interface{}{
					"state": "present",
				},
				wantErr: false,
			},
			{
				name: "InvalidState",
				args: map[string]interface{}{
					"name":  "express",
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