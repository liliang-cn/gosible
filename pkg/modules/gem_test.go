package modules

import (
	"context"
	"testing"

	gotest "github.com/liliang-cn/gosinble/pkg/testing"
)

func TestGemModule(t *testing.T) {
	t.Run("ModuleProperties", func(t *testing.T) {
		m := NewGemModule()
		if m.Name() != "gem" {
			t.Errorf("Expected module name 'gem', got %s", m.Name())
		}
	})

	t.Run("ValidationTests", func(t *testing.T) {
		m := NewGemModule()
		
		testCases := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name: "ValidInstall",
				args: map[string]interface{}{
					"name":  "bundler",
					"state": "present",
				},
				wantErr: false,
			},
			{
				name: "ValidWithVersion",
				args: map[string]interface{}{
					"name":    "rails",
					"version": "7.0.0",
					"state":   "present",
				},
				wantErr: false,
			},
			{
				name: "ValidRemove",
				args: map[string]interface{}{
					"name":  "rspec",
					"state": "absent",
				},
				wantErr: false,
			},
			{
				name: "ValidWithUserInstall",
				args: map[string]interface{}{
					"name":         "pry",
					"user_install": true,
					"state":        "present",
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
					"name":  "bundler",
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

	t.Run("GemInstallationTests", func(t *testing.T) {
		m := NewGemModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("InstallGem", func(t *testing.T) {
			// Check if gem is installed
			conn.ExpectCommand("gem list --local bundler 2>/dev/null", &gotest.CommandResponse{
				Stdout:   "", // Not installed - empty output
				ExitCode: 0,
			})
			
			// Install gem
			conn.ExpectCommand("gem install --user-install bundler", &gotest.CommandResponse{
				Stdout:   "Successfully installed bundler-2.4.0",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":  "bundler",
				"state": "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("GemAlreadyInstalled", func(t *testing.T) {
			conn.Reset()
			// Check if gem is installed
			conn.ExpectCommand("gem list --local rails 2>/dev/null", &gotest.CommandResponse{
				Stdout:   "rails (7.0.4)\n", // Already installed
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":  "rails",
				"state": "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertNotChanged(result)
			conn.Verify()
		})
		
		t.Run("InstallWithVersion", func(t *testing.T) {
			conn.Reset()
			// Check if specific version is installed
			conn.ExpectCommand("gem list --local rails 2>/dev/null", &gotest.CommandResponse{
				Stdout:   "rails (6.1.0)\n", // Wrong version installed
				ExitCode: 0,
			})
			
			// Install specific version
			conn.ExpectCommand("gem install --user-install rails --version 7.0.0", &gotest.CommandResponse{
				Stdout:   "Successfully installed rails-7.0.0",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":    "rails",
				"version": "7.0.0",
				"state":   "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
	})

	t.Run("GemRemovalTests", func(t *testing.T) {
		m := NewGemModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("RemoveGem", func(t *testing.T) {
			// Check if gem is installed
			conn.ExpectCommand("gem list --local rspec 2>/dev/null", &gotest.CommandResponse{
				Stdout:   "rspec (3.12.0)\n", // Installed
				ExitCode: 0,
			})
			
			// Uninstall gem
			conn.ExpectCommand("gem uninstall -x rspec --all", &gotest.CommandResponse{
				Stdout:   "Successfully uninstalled rspec-3.12.0",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":  "rspec",
				"state": "absent",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("GemNotInstalled", func(t *testing.T) {
			conn.Reset()
			// Check if gem is installed
			conn.ExpectCommand("gem list --local nonexistent 2>/dev/null", &gotest.CommandResponse{
				Stdout:   "", // Not installed
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":  "nonexistent",
				"state": "absent",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertNotChanged(result)
			conn.Verify()
		})
	})
}