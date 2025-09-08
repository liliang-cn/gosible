package modules

import (
	"testing"
)

func TestUnarchiveModule(t *testing.T) {
	t.Run("ModuleProperties", func(t *testing.T) {
		m := NewUnarchiveModule()
		if m.Name() != "unarchive" {
			t.Errorf("Expected module name 'unarchive', got %s", m.Name())
		}
	})

	t.Run("ValidationTests", func(t *testing.T) {
		m := NewUnarchiveModule()
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name: "ValidExtractTarGz",
				args: map[string]interface{}{
					"src":  "/tmp/archive.tar.gz",
					"dest": "/opt/app",
				},
				wantErr: false,
			},
			{
				name: "ValidExtractZip",
				args: map[string]interface{}{
					"src":  "/tmp/archive.zip",
					"dest": "/var/www",
				},
				wantErr: false,
			},
			{
				name: "ValidRemoteSource",
				args: map[string]interface{}{
					"src":    "https://example.com/archive.tar.gz",
					"dest":   "/opt/app",
					"remote": true,
				},
				wantErr: false,
			},
			{
				name: "ValidWithOwner",
				args: map[string]interface{}{
					"src":   "/tmp/archive.tar.gz",
					"dest":  "/opt/app",
					"owner": "appuser",
					"group": "appgroup",
				},
				wantErr: false,
			},
			{
				name: "MissingSrc",
				args: map[string]interface{}{
					"dest": "/opt/app",
				},
				wantErr: true,
			},
			{
				name: "MissingDest",
				args: map[string]interface{}{
					"src": "/tmp/archive.tar.gz",
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