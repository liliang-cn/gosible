package testing

import (
	"fmt"
	"strings"
	"testing"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// SystemdTestHelper provides specialized helpers for testing systemd modules
type SystemdTestHelper struct {
	connection *MockConnection
	t          *testing.T
}

// NewSystemdTestHelper creates a new systemd test helper
func NewSystemdTestHelper(connection *MockConnection, t *testing.T) *SystemdTestHelper {
	return &SystemdTestHelper{
		connection: connection,
		t:          t,
	}
}

// SystemdServiceConfig represents the configuration of a systemd service for testing
type SystemdServiceConfig struct {
	LoadState     string // loaded, not-found, error, masked
	ActiveState   string // active, inactive, failed, activating, deactivating
	SubState      string // running, dead, failed, start-pre, etc.
	EnabledState  string // enabled, disabled, static, masked
	UnitFileState string // enabled, disabled, static, masked (for compatibility)
}

// SystemdOperations defines which operations are allowed for a service
type SystemdOperations struct {
	AllowStart   bool
	AllowStop    bool
	AllowRestart bool
	AllowReload  bool
	AllowEnable  bool
	AllowDisable bool
	AllowMask    bool
	AllowUnmask  bool
}

// MockSystemdService sets up comprehensive mocks for a systemd service
func (h *SystemdTestHelper) MockSystemdService(serviceName string, config SystemdServiceConfig) *SystemdTestHelper {
	// Default states if not provided
	if config.LoadState == "" {
		config.LoadState = "loaded"
	}
	if config.ActiveState == "" {
		config.ActiveState = "inactive"
	}
	if config.SubState == "" {
		config.SubState = "dead"
	}
	if config.EnabledState == "" && config.UnitFileState == "" {
		config.EnabledState = "disabled"
	}
	// Use UnitFileState if provided, otherwise use EnabledState
	enabledState := config.EnabledState
	if config.UnitFileState != "" {
		enabledState = config.UnitFileState
	}
	
	// Mock the 'systemctl show --no-page' command (primary method)
	showOutput := fmt.Sprintf("LoadState=%s\nActiveState=%s\nSubState=%s\nUnitFileState=%s\n",
		config.LoadState, config.ActiveState, config.SubState, enabledState)
	
	// Allow multiple calls to show command (module calls it at start and potentially at end)
	h.connection.ExpectCommand(fmt.Sprintf("systemctl show %s --no-page", serviceName), &CommandResponse{
		ExitCode: 0,
		Stdout:   showOutput,
	}).SetMaxCalls(3)
	
	// Note: Fallback commands (is-active, is-enabled, status) are only called 
	// when the primary show command fails, so we don't mock them for normal cases
	
	return h
}

// MockSystemdOperations sets up mocks for systemd operations
func (h *SystemdTestHelper) MockSystemdOperations(serviceName string, ops SystemdOperations) *SystemdTestHelper {
	if ops.AllowStart {
		h.connection.ExpectCommand(fmt.Sprintf("systemctl start %s", serviceName), &CommandResponse{
			ExitCode: 0,
		})
	}
	
	if ops.AllowStop {
		h.connection.ExpectCommand(fmt.Sprintf("systemctl stop %s", serviceName), &CommandResponse{
			ExitCode: 0,
		})
	}
	
	if ops.AllowRestart {
		h.connection.ExpectCommand(fmt.Sprintf("systemctl restart %s", serviceName), &CommandResponse{
			ExitCode: 0,
		})
	}
	
	if ops.AllowReload {
		h.connection.ExpectCommand(fmt.Sprintf("systemctl reload %s", serviceName), &CommandResponse{
			ExitCode: 0,
		})
	}
	
	if ops.AllowEnable {
		h.connection.ExpectCommand(fmt.Sprintf("systemctl enable %s", serviceName), &CommandResponse{
			ExitCode: 0,
		})
	}
	
	if ops.AllowDisable {
		h.connection.ExpectCommand(fmt.Sprintf("systemctl disable %s", serviceName), &CommandResponse{
			ExitCode: 0,
		})
	}
	
	if ops.AllowMask {
		h.connection.ExpectCommand(fmt.Sprintf("systemctl mask %s", serviceName), &CommandResponse{
			ExitCode: 0,
		})
	}
	
	if ops.AllowUnmask {
		h.connection.ExpectCommand(fmt.Sprintf("systemctl unmask %s", serviceName), &CommandResponse{
			ExitCode: 0,
		})
	}
	
	return h
}

// MockSystemdDaemonReload sets up mock for daemon reload
func (h *SystemdTestHelper) MockSystemdDaemonReload() *SystemdTestHelper {
	h.connection.ExpectCommand("systemctl daemon-reload", &CommandResponse{
		ExitCode: 0,
	})
	return h
}

// MockServiceNotFound sets up mock for a service that doesn't exist
func (h *SystemdTestHelper) MockServiceNotFound(serviceName string) *SystemdTestHelper {
	// Mock the show command failing, which will trigger fallback commands
	h.connection.ExpectCommand(fmt.Sprintf("systemctl show %s --no-page", serviceName), &CommandResponse{
		ExitCode: 4, // systemctl returns 4 for unit not found
		Stdout:   fmt.Sprintf("Unit %s.service could not be found.", serviceName), // Put error in stdout for Message field
		Stderr:   fmt.Sprintf("Unit %s.service could not be found.", serviceName),
	})
	
	// Mock fallback commands that will be called when show fails
	h.connection.ExpectCommand(fmt.Sprintf("systemctl is-active %s", serviceName), &CommandResponse{
		ExitCode: 3, // inactive/unknown
		Stdout:   "unknown",
	})
	
	h.connection.ExpectCommand(fmt.Sprintf("systemctl is-enabled %s 2>/dev/null", serviceName), &CommandResponse{
		ExitCode: 1,
		Stderr:   fmt.Sprintf("Failed to get unit file state for %s: No such file or directory", serviceName),
	})
	
	h.connection.ExpectCommand(fmt.Sprintf("systemctl status %s", serviceName), &CommandResponse{
		ExitCode: 4,
		Stdout:   fmt.Sprintf("Unit %s.service could not be found.", serviceName),
		Stderr:   fmt.Sprintf("Unit %s.service could not be found.", serviceName),
	})
	
	return h
}

// MockPermissionDenied sets up mock for permission denied errors
func (h *SystemdTestHelper) MockPermissionDenied(serviceName, operation string) *SystemdTestHelper {
	// Mock the operation command to fail with permission denied
	command := fmt.Sprintf("systemctl %s %s", operation, serviceName)
	h.connection.ExpectCommand(command, &CommandResponse{
		ExitCode: 1,
		Stderr:   "==== AUTHENTICATING FOR org.freedesktop.systemd1.manage-units ====\nAuthentication required\n==== AUTHENTICATION FAILED ====\nFailed to start " + serviceName + ".service: Access denied",
	})
	return h
}

// PresetSystemdServiceStates provides common service state combinations
type PresetSystemdServiceStates struct{}

// InactiveDisabled returns config for an inactive, disabled service
func (p *PresetSystemdServiceStates) InactiveDisabled() SystemdServiceConfig {
	return SystemdServiceConfig{
		LoadState:    "loaded",
		ActiveState:  "inactive",
		SubState:     "dead",
		EnabledState: "disabled",
	}
}

// ActiveEnabled returns config for an active, enabled service
func (p *PresetSystemdServiceStates) ActiveEnabled() SystemdServiceConfig {
	return SystemdServiceConfig{
		LoadState:    "loaded",
		ActiveState:  "active",
		SubState:     "running",
		EnabledState: "enabled",
	}
}

// FailedService returns config for a failed service
func (p *PresetSystemdServiceStates) FailedService() SystemdServiceConfig {
	return SystemdServiceConfig{
		LoadState:    "loaded",
		ActiveState:  "failed",
		SubState:     "failed",
		EnabledState: "enabled",
	}
}

// MaskedService returns config for a masked service
func (p *PresetSystemdServiceStates) MaskedService() SystemdServiceConfig {
	return SystemdServiceConfig{
		LoadState:    "masked",
		ActiveState:  "inactive",
		SubState:     "dead",
		EnabledState: "masked",
	}
}

// StaticService returns config for a static service (cannot be enabled/disabled)
func (p *PresetSystemdServiceStates) StaticService() SystemdServiceConfig {
	return SystemdServiceConfig{
		LoadState:    "loaded",
		ActiveState:  "inactive",
		SubState:     "dead",
		EnabledState: "static",
	}
}

// GetSystemdPresets returns common systemd service state presets
func (h *SystemdTestHelper) GetSystemdPresets() *PresetSystemdServiceStates {
	return &PresetSystemdServiceStates{}
}

// FileTestHelper provides specialized helpers for testing file modules
type FileTestHelper struct {
	filesystem *MockFileSystem
	t          *testing.T
}

// NewFileTestHelper creates a new file test helper
func NewFileTestHelper(filesystem *MockFileSystem, t *testing.T) *FileTestHelper {
	return &FileTestHelper{
		filesystem: filesystem,
		t:          t,
	}
}

// CreateTestFile creates a test file with specified content
func (h *FileTestHelper) CreateTestFile(path, content string) *FileTestHelper {
	h.filesystem.CreateFile(path, []byte(content), 0644)
	return h
}

// CreateTestDirectory creates a test directory
func (h *FileTestHelper) CreateTestDirectory(path string) *FileTestHelper {
	h.filesystem.CreateDirectory(path, 0755)
	return h
}

// SetupConfigFile creates a typical configuration file for testing
func (h *FileTestHelper) SetupConfigFile(path string, lines []string) *FileTestHelper {
	content := strings.Join(lines, "\n") + "\n"
	h.filesystem.CreateFile(path, []byte(content), 0644)
	return h
}

// SetupReadOnlyFile creates a read-only file
func (h *FileTestHelper) SetupReadOnlyFile(path, content string) *FileTestHelper {
	h.filesystem.CreateFile(path, []byte(content), 0644)
	h.filesystem.SetFileReadOnly(path, true)
	return h
}

// PackageTestHelper provides specialized helpers for testing package modules
type PackageTestHelper struct {
	connection *MockConnection
	t          *testing.T
}

// NewPackageTestHelper creates a new package test helper
func NewPackageTestHelper(connection *MockConnection, t *testing.T) *PackageTestHelper {
	return &PackageTestHelper{
		connection: connection,
		t:          t,
	}
}

// MockAptPackageInstalled sets up mocks for an installed APT package
func (h *PackageTestHelper) MockAptPackageInstalled(packageName, version string) *PackageTestHelper {
	// Mock dpkg query
	h.connection.ExpectCommand(fmt.Sprintf("dpkg-query -W -f='${Status} ${Version}' %s", packageName), &CommandResponse{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("install ok installed %s", version),
	})
	return h
}

// MockAptPackageNotInstalled sets up mocks for a package that's not installed
func (h *PackageTestHelper) MockAptPackageNotInstalled(packageName string) *PackageTestHelper {
	h.connection.ExpectCommand(fmt.Sprintf("dpkg-query -W -f='${Status} ${Version}' %s", packageName), &CommandResponse{
		ExitCode: 1,
		Stderr:   fmt.Sprintf("dpkg-query: no packages found matching %s", packageName),
	})
	return h
}

// MockAptInstall sets up mocks for APT package installation
func (h *PackageTestHelper) MockAptInstall(packageName string, success bool) *PackageTestHelper {
	exitCode := 0
	if !success {
		exitCode = 100
	}
	
	h.connection.ExpectCommand(fmt.Sprintf("apt-get install -y %s", packageName), &CommandResponse{
		ExitCode: exitCode,
	})
	return h
}

// MockYumPackageInstalled sets up mocks for an installed YUM package
func (h *PackageTestHelper) MockYumPackageInstalled(packageName, version string) *PackageTestHelper {
	h.connection.ExpectCommand(fmt.Sprintf("rpm -q %s", packageName), &CommandResponse{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("%s-%s", packageName, version),
	})
	return h
}

// MockYumPackageNotInstalled sets up mocks for a YUM package that's not installed
func (h *PackageTestHelper) MockYumPackageNotInstalled(packageName string) *PackageTestHelper {
	h.connection.ExpectCommand(fmt.Sprintf("rpm -q %s", packageName), &CommandResponse{
		ExitCode: 1,
		Stderr:   fmt.Sprintf("package %s is not installed", packageName),
	})
	return h
}

// GetSystemdHelper returns a systemd test helper
func (h *ModuleTestHelper) GetSystemdHelper() *SystemdTestHelper {
	return NewSystemdTestHelper(h.connection, h.t)
}

// GetFileHelper returns a file test helper
func (h *ModuleTestHelper) GetFileHelper() *FileTestHelper {
	return NewFileTestHelper(h.filesystem, h.t)
}

// GetPackageHelper returns a package test helper  
func (h *ModuleTestHelper) GetPackageHelper() *PackageTestHelper {
	return NewPackageTestHelper(h.connection, h.t)
}

// Common test scenario builders

// TestScenario represents a complete test scenario with setup and expectations
type TestScenario struct {
	Name        string
	Description string
	Setup       func(helper *ModuleTestHelper)
	Execute     func(helper *ModuleTestHelper) interface{}
	Verify      func(helper *ModuleTestHelper, result interface{})
}

// SystemdTestScenarios provides common systemd testing scenarios
type SystemdTestScenarios struct{}

// StartInactiveService creates a scenario for starting an inactive service
func (s *SystemdTestScenarios) StartInactiveService(serviceName string) TestScenario {
	return TestScenario{
		Name:        fmt.Sprintf("Start inactive service %s", serviceName),
		Description: "Start a service that is currently inactive",
		Setup: func(helper *ModuleTestHelper) {
			systemdHelper := helper.GetSystemdHelper()
			presets := systemdHelper.GetSystemdPresets()
			
			systemdHelper.MockSystemdService(serviceName, presets.InactiveDisabled())
			systemdHelper.MockSystemdOperations(serviceName, SystemdOperations{
				AllowStart: true,
			})
		},
		Execute: func(helper *ModuleTestHelper) interface{} {
			return helper.Execute(map[string]interface{}{
				"name":  serviceName,
				"state": "started",
			}, false, false)
		},
		Verify: func(helper *ModuleTestHelper, result interface{}) {
			r := result.(*types.Result)
			helper.AssertSuccess(r)
			helper.AssertChanged(r)
			
			conn := helper.GetConnection()
			conn.AssertCommandCalled(fmt.Sprintf("systemctl show %s", serviceName))
			conn.AssertCommandCalled(fmt.Sprintf("systemctl start %s", serviceName))
		},
	}
}

// EnableDisabledService creates a scenario for enabling a disabled service
func (s *SystemdTestScenarios) EnableDisabledService(serviceName string) TestScenario {
	return TestScenario{
		Name:        fmt.Sprintf("Enable disabled service %s", serviceName),
		Description: "Enable a service that is currently disabled",
		Setup: func(helper *ModuleTestHelper) {
			systemdHelper := helper.GetSystemdHelper()
			presets := systemdHelper.GetSystemdPresets()
			
			systemdHelper.MockSystemdService(serviceName, presets.InactiveDisabled())
			systemdHelper.MockSystemdOperations(serviceName, SystemdOperations{
				AllowEnable: true,
			})
		},
		Execute: func(helper *ModuleTestHelper) interface{} {
			return helper.Execute(map[string]interface{}{
				"name":    serviceName,
				"enabled": true,
			}, false, false)
		},
		Verify: func(helper *ModuleTestHelper, result interface{}) {
			r := result.(*types.Result)
			helper.AssertSuccess(r)
			helper.AssertChanged(r)
			
			conn := helper.GetConnection()
			conn.AssertCommandCalled(fmt.Sprintf("systemctl show %s", serviceName))
			conn.AssertCommandCalled(fmt.Sprintf("systemctl enable %s", serviceName))
		},
	}
}

// GetSystemdScenarios returns systemd test scenarios
func GetSystemdScenarios() *SystemdTestScenarios {
	return &SystemdTestScenarios{}
}