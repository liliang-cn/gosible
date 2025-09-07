package modules

import (
	"context"
	"testing"

	gotest "github.com/gosinble/gosinble/pkg/testing"
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
		
		testCases := []struct {
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
				name: "MissingName",
				args: map[string]interface{}{
					"state": "present",
				},
				wantErr: true,
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
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := m.Validate(tc.args)
				if (err != nil) != tc.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("NpmInstallationTests", func(t *testing.T) {
		m := NewNpmModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("InstallPackage", func(t *testing.T) {
			// Check if package is installed
			conn.ExpectCommand("npm list express", &gotest.CommandResponse{
				ExitCode: 1, // Not installed
			})
			
			// Install package
			conn.ExpectCommand("npm install express", &gotest.CommandResponse{
				Stdout:   "added 57 packages in 2s",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":  "express",
				"state": "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("PackageAlreadyInstalled", func(t *testing.T) {
			conn.Reset()
			// Check if package is installed
			conn.ExpectCommand("npm list react", &gotest.CommandResponse{
				Stdout:   "react@18.2.0",
				ExitCode: 0, // Already installed
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":  "react",
				"state": "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertNotChanged(result)
			conn.Verify()
		})
		
		t.Run("GlobalInstall", func(t *testing.T) {
			conn.Reset()
			// Check if package is installed globally
			conn.ExpectCommand("npm list -g typescript", &gotest.CommandResponse{
				ExitCode: 1, // Not installed
			})
			
			// Install globally
			conn.ExpectCommand("npm install -g typescript", &gotest.CommandResponse{
				Stdout:   "added 1 package in 1s",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":   "typescript",
				"global": true,
				"state":  "present",
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