package modules

import (
	"testing"

	testhelper "github.com/gosinble/gosinble/pkg/testing"
)

func TestRepositoryModule(t *testing.T) {
	module := NewRepositoryModule()
	helper := testhelper.NewModuleTestHelper(t, module)

	// Test basic module properties
	t.Run("ModuleProperties", func(t *testing.T) {
		if module.Name() != "repository" {
			t.Errorf("Expected module name 'repository', got %s", module.Name())
		}

		caps := module.Capabilities()
		if !caps.CheckMode {
			t.Error("Expected repository module to support check mode")
		}
		if !caps.DiffMode {
			t.Error("Expected repository module to support diff mode")
		}
		if caps.Platform != "linux" {
			t.Errorf("Expected platform 'linux', got %s", caps.Platform)
		}
		if !caps.RequiresRoot {
			t.Error("Expected repository module to require root")
		}
	})

	t.Run("ValidationTests", testRepositoryValidation)
	
	_ = helper // Suppress unused warning
}

func testRepositoryValidation(t *testing.T) {
	module := NewRepositoryModule()

	tests := []struct {
		name        string
		args        map[string]interface{}
		shouldFail  bool
		errorSubstr string
	}{
		{
			name: "minimal valid args with name",
			args: map[string]interface{}{
				"name": "test-repo",
			},
			shouldFail: false,
		},
		{
			name: "full args with baseurl",
			args: map[string]interface{}{
				"name":        "test-repo",
				"baseurl":     "http://example.com/repo",
				"description": "Test repository",
				"enabled":     true,
				"state":       "present",
			},
			shouldFail: false,
		},
		{
			name: "apt repository",
			args: map[string]interface{}{
				"name": "ppa:example/ppa",
				"repo": "deb http://example.com/ubuntu focal main",
			},
			shouldFail: false,
		},
		{
			name:        "missing required name and repo",
			args:        map[string]interface{}{},
			shouldFail:  true,
			errorSubstr: "either repo or name parameter is required",
		},
		{
			name: "invalid state",
			args: map[string]interface{}{
				"name":  "test-repo",
				"state": "invalid",
			},
			shouldFail:  true,
			errorSubstr: "state must be one of",
		},
		{
			name: "invalid baseurl",
			args: map[string]interface{}{
				"name":    "test-repo",
				"baseurl": "invalid-url",
			},
			shouldFail:  true,
			errorSubstr: "baseurl must be a valid URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := module.Validate(test.args)
			if test.shouldFail {
				if err == nil {
					t.Errorf("Expected validation to fail for %s", test.name)
					return
				}
				if test.errorSubstr != "" && !containsString(err.Error(), test.errorSubstr) {
					t.Errorf("Expected error containing '%s', got: %v", test.errorSubstr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error for %s: %v", test.name, err)
				}
			}
		})
	}
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}