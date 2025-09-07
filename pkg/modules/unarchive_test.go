package modules

import (
	"context"
	"testing"

	gotest "github.com/gosinble/gosinble/pkg/testing"
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
		
		testCases := []struct {
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
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := m.Validate(tc.args)
				if (err != nil) != tc.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("ExtractOperationTests", func(t *testing.T) {
		m := NewUnarchiveModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("ExtractTarGz", func(t *testing.T) {
			// Check if archive exists
			conn.ExpectCommand("test -f /tmp/archive.tar.gz", &gotest.CommandResponse{
				ExitCode: 0,
			})
			
			// Create destination directory
			conn.ExpectCommand("mkdir -p /opt/app", &gotest.CommandResponse{
				ExitCode: 0,
			})
			
			// Extract archive
			conn.ExpectCommand("tar -xzf /tmp/archive.tar.gz -C /opt/app", &gotest.CommandResponse{
				Stdout:   "",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"src":  "/tmp/archive.tar.gz",
				"dest": "/opt/app",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("ExtractZip", func(t *testing.T) {
			conn.Reset()
			// Check if archive exists
			conn.ExpectCommand("test -f /tmp/archive.zip", &gotest.CommandResponse{
				ExitCode: 0,
			})
			
			// Create destination directory
			conn.ExpectCommand("mkdir -p /var/www", &gotest.CommandResponse{
				ExitCode: 0,
			})
			
			// Extract archive
			conn.ExpectCommand("unzip -o /tmp/archive.zip -d /var/www", &gotest.CommandResponse{
				Stdout:   "Archive:  /tmp/archive.zip\n  inflating: /var/www/index.html",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"src":  "/tmp/archive.zip",
				"dest": "/var/www",
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