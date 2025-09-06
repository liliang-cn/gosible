package modules

import (
	"testing"

	testhelper "github.com/gosinble/gosinble/pkg/testing"
	"github.com/gosinble/gosinble/pkg/types"
)

func TestSystemdModule(t *testing.T) {
	module := NewSystemdModule()
	helper := testhelper.NewModuleTestHelper(t, module)

	// Test basic module properties
	t.Run("ModuleProperties", func(t *testing.T) {
		if module.Name() != "systemd" {
			t.Errorf("Expected module name 'systemd', got %s", module.Name())
		}

		caps := module.Capabilities()
		if !caps.CheckMode {
			t.Error("Expected systemd module to support check mode")
		}
		if !caps.DiffMode {
			t.Error("Expected systemd module to support diff mode")
		}
		if caps.Platform != "linux" {
			t.Errorf("Expected platform 'linux', got %s", caps.Platform)
		}
		if !caps.RequiresRoot {
			t.Error("Expected systemd module to require root")
		}
	})

	t.Run("ValidationTests", testSystemdValidation)
	t.Run("ServiceStateTests", func(t *testing.T) { testSystemdServiceStates(t, helper) })
	t.Run("EnabledStateTests", func(t *testing.T) { testSystemdEnabledStates(t, helper) })
	t.Run("MaskingTests", func(t *testing.T) { testSystemdMasking(t, helper) })
	t.Run("DaemonReloadTests", func(t *testing.T) { testSystemdDaemonReload(t, helper) })
	t.Run("CheckModeTests", func(t *testing.T) { testSystemdCheckMode(t, helper) })
	t.Run("DiffModeTests", func(t *testing.T) { testSystemdDiffMode(t, helper) })
	t.Run("ErrorHandlingTests", func(t *testing.T) { testSystemdErrorHandling(t, helper) })
	t.Run("ComplexScenarios", func(t *testing.T) { testSystemdComplexScenarios(t, helper) })
}

func testSystemdValidation(t *testing.T) {
	module := NewSystemdModule()

	tests := []struct {
		name        string
		args        map[string]interface{}
		shouldFail  bool
		expectedErr string
	}{
		{
			name:       "ValidArgs",
			args:       map[string]interface{}{"name": "nginx", "state": "started", "enabled": true},
			shouldFail: false,
		},
		{
			name:        "MissingName",
			args:        map[string]interface{}{"state": "started"},
			shouldFail:  true,
			expectedErr: "required parameter",
		},
		{
			name:        "EmptyName",
			args:        map[string]interface{}{"name": "", "state": "started"},
			shouldFail:  true,
			expectedErr: "required parameter",
		},
		{
			name:        "InvalidState",
			args:        map[string]interface{}{"name": "nginx", "state": "invalid"},
			shouldFail:  true,
			expectedErr: "must be one of",
		},
		{
			name:        "InvalidServiceName",
			args:        map[string]interface{}{"name": "nginx@#$", "state": "started"},
			shouldFail:  true,
			expectedErr: "invalid service name format",
		},
		{
			name:        "InvalidBooleanEnabled",
			args:        map[string]interface{}{"name": "nginx", "enabled": "maybe"},
			shouldFail:  true,
			expectedErr: "must be a boolean value",
		},
		{
			name:       "ValidBooleanStrings",
			args:       map[string]interface{}{"name": "nginx", "enabled": "yes", "daemon_reload": "1", "masked": "false"},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if tt.shouldFail {
				if err == nil {
					t.Error("Expected validation to fail, but it passed")
				} else if tt.expectedErr != "" && !contains(err.Error(), tt.expectedErr) {
					t.Errorf("Expected error containing %q, got %q", tt.expectedErr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected validation to pass, got error: %v", err)
				}
			}
		})
	}
}

func testSystemdServiceStates(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.ModuleTestCase{
		{
			Name: "StartInactiveService",
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": "started",
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				// Mock service as inactive
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "inactive",
					EnabledState: "disabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
					Since:        "2023-01-01 12:00:00",
				})
				
				// Mock start operation
				h.GetSystemdHelper().MockSystemdOperations("nginx", testhelper.SystemdOperations{
					AllowStart: true,
				})
				
				// Mock systemctl show for detailed state
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=inactive
SubState=dead
UnitFileState=disabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "started service nginx",
				Commands: []string{
					"systemctl show nginx --no-page",
					"systemctl start nginx",
				},
			},
		},
		{
			Name: "StartAlreadyActiveService",
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": "started",
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				// Mock service as already active
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "active",
					EnabledState: "enabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
					Since:        "2023-01-01 12:00:00",
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(false),
				MessageContains: "already in desired state",
			},
		},
		{
			Name: "StopActiveService",
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": "stopped",
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "active",
					EnabledState: "enabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
					Since:        "2023-01-01 12:00:00",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("nginx", testhelper.SystemdOperations{
					AllowStop: true,
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "stopped service nginx",
				Commands: []string{
					"systemctl show nginx --no-page",
					"systemctl stop nginx",
				},
			},
		},
		{
			Name: "RestartService",
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": "restarted",
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "active",
					EnabledState: "enabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
					Since:        "2023-01-01 12:00:00",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("nginx", testhelper.SystemdOperations{
					AllowRestart: true,
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "restarted service nginx",
				Commands: []string{
					"systemctl show nginx --no-page",
					"systemctl restart nginx",
				},
			},
		},
		{
			Name: "ReloadService",
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": "reloaded",
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "active",
					EnabledState: "enabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
					Since:        "2023-01-01 12:00:00",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("nginx", testhelper.SystemdOperations{
					AllowReload: true,
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "reloaded service nginx",
				Commands: []string{
					"systemctl show nginx --no-page",
					"systemctl reload nginx",
				},
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testSystemdEnabledStates(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.ModuleTestCase{
		{
			Name: "EnableDisabledService",
			Args: map[string]interface{}{
				"name":    "nginx",
				"enabled": true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "inactive",
					EnabledState: "disabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("nginx", testhelper.SystemdOperations{
					AllowEnable: true,
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=inactive
SubState=dead
UnitFileState=disabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "enabled service nginx",
			},
		},
		{
			Name: "DisableEnabledService",
			Args: map[string]interface{}{
				"name":    "nginx",
				"enabled": false,
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "active",
					EnabledState: "enabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("nginx", testhelper.SystemdOperations{
					AllowDisable: true,
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "disabled service nginx",
			},
		},
		{
			Name: "StartAndEnableService",
			Args: map[string]interface{}{
				"name":    "nginx",
				"state":   "started",
				"enabled": true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "inactive",
					EnabledState: "disabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("nginx", testhelper.DefaultSystemdOperations())
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=inactive
SubState=dead
UnitFileState=disabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "started service nginx",
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testSystemdMasking(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.ModuleTestCase{
		{
			Name: "MaskService",
			Args: map[string]interface{}{
				"name":   "unwanted-service",
				"masked": true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("unwanted-service", testhelper.SystemdServiceConfig{
					ActiveState:  "inactive",
					EnabledState: "disabled",
					LoadState:    "loaded",
					Description:  "Unwanted Service",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("unwanted-service", testhelper.SystemdOperations{
					AllowMask: true,
				})
				
				h.GetConnection().ExpectCommand("systemctl show unwanted-service --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=inactive
SubState=dead
UnitFileState=disabled
FragmentPath=/lib/systemd/system/unwanted-service.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "masked service unwanted-service",
			},
		},
		{
			Name: "UnmaskService",
			Args: map[string]interface{}{
				"name":   "masked-service",
				"masked": false,
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("masked-service", testhelper.SystemdServiceConfig{
					ActiveState:  "inactive",
					EnabledState: "masked",
					LoadState:    "masked",
					Description:  "Masked Service",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("masked-service", testhelper.SystemdOperations{
					AllowUnmask: true,
				})
				
				h.GetConnection().ExpectCommand("systemctl show masked-service --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=masked
ActiveState=inactive
SubState=dead
UnitFileState=masked
FragmentPath=`,
					ExitCode: 0,
				})
				
				// After unmask, need to refresh state
				h.GetConnection().ExpectCommand("systemctl show masked-service --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=inactive
SubState=dead
UnitFileState=disabled
FragmentPath=/lib/systemd/system/masked-service.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "unmasked service masked-service",
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testSystemdDaemonReload(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.ModuleTestCase{
		{
			Name: "DaemonReloadOnly",
			Args: map[string]interface{}{
				"name":          "nginx",
				"daemon_reload": true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "active",
					EnabledState: "enabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("nginx", testhelper.SystemdOperations{
					AllowDaemonReload: true,
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "reloaded systemd daemon",
				Commands: []string{
					"systemctl show nginx --no-page",
					"systemctl daemon-reload",
				},
			},
		},
		{
			Name: "DaemonReloadWithServiceStart",
			Args: map[string]interface{}{
				"name":          "nginx",
				"state":         "started",
				"daemon_reload": true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "inactive",
					EnabledState: "disabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("nginx", testhelper.SystemdOperations{
					AllowDaemonReload: true,
					AllowStart:        true,
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=inactive
SubState=dead
UnitFileState=disabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "reloaded systemd daemon",
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testSystemdCheckMode(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.ModuleTestCase{
		{
			Name:      "CheckModeStartService",
			CheckMode: true,
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": "started",
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "inactive",
					EnabledState: "disabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=inactive
SubState=dead
UnitFileState=disabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "Would make changes",
				DataChecks: map[string]interface{}{
					"check_mode": true,
				},
				CustomAssertions: []func(*types.Result, *testing.T){
					func(result *types.Result, t *testing.T) {
						if !result.Simulated {
							t.Error("Expected simulated=true for check mode")
						}
					},
				},
			},
		},
		{
			Name:      "CheckModeNoChanges",
			CheckMode: true,
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": "started",
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "active",
					EnabledState: "enabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(false),
				MessageContains: "already in desired state",
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testSystemdDiffMode(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.ModuleTestCase{
		{
			Name:     "DiffModeStartService",
			DiffMode: true,
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": "started",
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("nginx", testhelper.SystemdServiceConfig{
					ActiveState:  "inactive",
					EnabledState: "disabled",
					LoadState:    "loaded",
					Description:  "Nginx HTTP Server",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("nginx", testhelper.SystemdOperations{
					AllowStart: true,
				})
				
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=inactive
SubState=dead
UnitFileState=disabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				// Mock final state check after start
				h.GetConnection().ExpectCommand("systemctl show nginx --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=disabled
FragmentPath=/lib/systemd/system/nginx.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "started service nginx",
				CustomAssertions: []func(*types.Result, *testing.T){
					func(result *types.Result, t *testing.T) {
						if result.Diff == nil {
							t.Error("Expected diff result in diff mode")
						} else {
							if !contains(result.Diff.Diff, "active_state") {
								t.Error("Expected diff to contain active_state change")
							}
						}
					},
				},
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testSystemdErrorHandling(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.ModuleTestCase{
		{
			Name: "ServiceNotFound",
			Args: map[string]interface{}{
				"name":  "nonexistent-service",
				"state": "started",
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				// Mock systemctl show failing
				h.GetConnection().ExpectCommand("systemctl show nonexistent-service --no-page", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "Unit nonexistent-service.service could not be found.",
				})
				
				// Mock fallback is-active command
				h.GetConnection().ExpectCommand("systemctl is-active nonexistent-service", &testhelper.CommandResponse{
					ExitCode: 3,
					Stdout:   "inactive",
				})
				
				// Mock is-enabled command
				h.GetConnection().ExpectCommand("systemctl is-enabled nonexistent-service 2>/dev/null", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "Failed to get unit file state for nonexistent-service.service: No such file or directory",
				})
				
				// Mock status command
				h.GetConnection().ExpectCommand("systemctl status nonexistent-service", &testhelper.CommandResponse{
					ExitCode: 4,
					Stderr:   "Unit nonexistent-service.service could not be found.",
				})
				
				// Mock start command that will fail
				h.GetConnection().ExpectCommand("systemctl start nonexistent-service", &testhelper.CommandResponse{
					ExitCode: 5,
					Stderr:   "Failed to start nonexistent-service.service: Unit nonexistent-service.service not found.",
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(false),
				ErrorContains: "failed to start service",
			},
		},
		{
			Name: "PermissionDenied",
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": "started",
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().SetupScenario(testhelper.ScenarioPermissionDenied, "nginx")
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(false),
				ErrorContains: "Permission denied",
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testSystemdComplexScenarios(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.ModuleTestCase{
		{
			Name: "CompleteServiceManagement",
			Args: map[string]interface{}{
				"name":          "myapp",
				"state":         "started",
				"enabled":       true,
				"daemon_reload": true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				// Mock service as completely unmanaged initially
				h.GetSystemdHelper().MockSystemdService("myapp", testhelper.SystemdServiceConfig{
					ActiveState:  "inactive",
					EnabledState: "disabled",
					LoadState:    "loaded",
					Description:  "My Application",
				})
				
				h.GetSystemdHelper().MockSystemdOperations("myapp", testhelper.DefaultSystemdOperations())
				
				h.GetConnection().ExpectCommand("systemctl show myapp --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=inactive
SubState=dead
UnitFileState=disabled
FragmentPath=/lib/systemd/system/myapp.service`,
					ExitCode: 0,
				})
				
				// Mock final state after all operations
				h.GetConnection().ExpectCommand("systemctl show myapp --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
FragmentPath=/lib/systemd/system/myapp.service`,
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "reloaded systemd daemon",
				Commands: []string{
					"systemctl show myapp --no-page",
					"systemctl daemon-reload",
					"systemctl start myapp",
					"systemctl enable myapp",
					"systemctl show myapp --no-page",
				},
			},
		},
		{
			Name: "ForceRestartWithNoBlock",
			Args: map[string]interface{}{
				"name":     "stubborn-service",
				"state":    "restarted",
				"force":    true,
				"no_block": true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) error {
				h.GetSystemdHelper().MockSystemdService("stubborn-service", testhelper.SystemdServiceConfig{
					ActiveState:  "active",
					EnabledState: "enabled",
					LoadState:    "loaded",
					Description:  "Stubborn Service",
				})
				
				h.GetConnection().ExpectCommand("systemctl show stubborn-service --no-page", &testhelper.CommandResponse{
					Stdout: `LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
FragmentPath=/lib/systemd/system/stubborn-service.service`,
					ExitCode: 0,
				})
				
				// Mock restart with no-block
				h.GetConnection().ExpectCommand("systemctl restart stubborn-service --no-block", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				
				return nil
			},
			ExpectedResult: &testhelper.ExpectedResult{
				Success: testhelper.BoolPtr(true),
				Changed: testhelper.BoolPtr(true),
				MessageContains: "restarted service stubborn-service",
			},
		},
	}

	helper.RunTestCases(testCases)
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}