package modules

import (
	"context"
	"testing"

	gotest "github.com/gosinble/gosinble/pkg/testing"
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
		
		testCases := []struct {
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
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := m.Validate(tc.args)
				if (err != nil) != tc.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("PipInstallationTests", func(t *testing.T) {
		m := NewPipModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("InstallPackage", func(t *testing.T) {
			// Check if package is installed
			conn.ExpectCommand("pip show django", &gotest.CommandResponse{
				Stdout:   "",
				ExitCode: 1,
			})
			
			// Install package
			conn.ExpectCommand("pip install django", &gotest.CommandResponse{
				Stdout:   "Successfully installed django-4.2.0",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":  "django",
				"state": "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("InstallWithVirtualenv", func(t *testing.T) {
			conn.Reset()
			// Check if package is installed in virtualenv
			conn.ExpectCommand("/app/venv/bin/pip show numpy", &gotest.CommandResponse{
				Stdout:   "",
				ExitCode: 1,
			})
			
			// Install in virtualenv
			conn.ExpectCommand("/app/venv/bin/pip install numpy", &gotest.CommandResponse{
				Stdout:   "Successfully installed numpy-1.24.3",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"name":       "numpy",
				"virtualenv": "/app/venv",
				"state":      "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("InstallRequirementsFile", func(t *testing.T) {
			conn.Reset()
			// Install from requirements file
			conn.ExpectCommand("pip install -r /app/requirements.txt", &gotest.CommandResponse{
				Stdout:   "Successfully installed multiple packages",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"requirements": "/app/requirements.txt",
				"state":        "present",
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