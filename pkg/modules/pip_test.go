package modules

import (
	"testing"
)

func TestPipModule(t *testing.T) {
	t.Run("ModuleProperties", func(t *testing.T) {
		m := NewPipModule()
		if m.Name() != "pip" {
			t.Errorf("Expected module name 'pip', got %s", m.Name())
		}
	})

	t.Run("ValidationTests", func(t *testing.T) {
		m := NewPipModule()
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name: "ValidInstall",
				args: map[string]interface{}{
					"name":  "django",
					"state": "present",
				},
				wantErr: false,
			},
			{
				name: "ValidWithVersion",
				args: map[string]interface{}{
					"name":    "flask",
					"version": "2.3.0",
					"state":   "present",
				},
				wantErr: false,
			},
			{
				name: "ValidRequirementsFile",
				args: map[string]interface{}{
					"requirements": "/app/requirements.txt",
					"state":        "present",
				},
				wantErr: false,
			},
			{
				name: "ValidVirtualenv",
				args: map[string]interface{}{
					"name":       "numpy",
					"virtualenv": "/app/venv",
					"state":      "present",
				},
				wantErr: false,
			},
			{
				name: "MissingNameAndRequirements",
				args: map[string]interface{}{
					"state": "present",
				},
				wantErr: true,
			},
			{
				name: "InvalidState",
				args: map[string]interface{}{
					"name":  "django",
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