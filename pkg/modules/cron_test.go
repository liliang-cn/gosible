package modules

import (
	"strings"
	"testing"

	testhelper "github.com/liliang-cn/gosible/pkg/testing"
	"github.com/liliang-cn/gosiblepkg/types"
)

func TestCronModule(t *testing.T) {
	module := NewCronModule()
	helper := testhelper.NewModuleTestHelper(t, module)

	// Test basic module properties
	t.Run("ModuleProperties", func(t *testing.T) {
		if module.Name() != "cron" {
			t.Errorf("Expected module name 'cron', got %s", module.Name())
		}

		caps := module.Capabilities()
		if !caps.CheckMode {
			t.Error("Expected cron module to support check mode")
		}
		if !caps.DiffMode {
			t.Error("Expected cron module to support diff mode")
		}
		if caps.Platform != "linux" {
			t.Errorf("Expected platform 'linux', got %s", caps.Platform)
		}
		if caps.RequiresRoot {
			t.Error("Expected cron module to not require root")
		}
	})

	t.Run("ValidationTests", testCronValidation)
	t.Run("CronJobTests", func(t *testing.T) { testCronJobs(t, helper) })
	t.Run("CronTimeValidation", func(t *testing.T) { testCronTimeValidation(t, helper) })
	t.Run("CheckModeTests", func(t *testing.T) { testCronCheckMode(t, helper) })
	t.Run("DiffModeTests", func(t *testing.T) { testCronDiffMode(t, helper) })
	t.Run("ErrorHandlingTests", func(t *testing.T) { testCronErrorHandling(t, helper) })
}

func testCronValidation(t *testing.T) {
	module := NewCronModule()

	tests := []struct {
		name        string
		args        map[string]interface{}
		shouldFail  bool
		expectedErr string
	}{
		{
			name: "ValidJobParameter",
			args: map[string]interface{}{
				"job": "0 2 * * * /usr/bin/backup.sh",
			},
			shouldFail: false,
		},
		{
			name: "ValidNameParameter",
			args: map[string]interface{}{
				"name":   "backup job",
				"job":    "/usr/bin/backup.sh",
				"minute": "0",
				"hour":   "2",
			},
			shouldFail: false,
		},
		{
			name: "MissingJobAndName",
			args: map[string]interface{}{
				"state": "present",
			},
			shouldFail:  true,
			expectedErr: "either job or name parameter is required",
		},
		{
			name: "InvalidState",
			args: map[string]interface{}{
				"name":  "test job",
				"state": "invalid",
			},
			shouldFail:  true,
			expectedErr: "value must be one of",
		},
		{
			name: "ValidCronTimes",
			args: map[string]interface{}{
				"name":    "test job",
				"job":     "/bin/echo test",
				"minute":  "*/15",
				"hour":    "9-17",
				"day":     "1,15",
				"month":   "*",
				"weekday": "1-5",
			},
			shouldFail: false,
		},
		{
			name: "InvalidMinute",
			args: map[string]interface{}{
				"name":   "test job",
				"job":    "/bin/echo test",
				"minute": "60",
			},
			shouldFail:  true,
			expectedErr: "value must be between 0 and 59",
		},
		{
			name: "InvalidHour",
			args: map[string]interface{}{
				"name": "test job",
				"job":  "/bin/echo test",
				"hour": "25",
			},
			shouldFail:  true,
			expectedErr: "value must be between 0 and 23",
		},
		{
			name: "InvalidUser",
			args: map[string]interface{}{
				"name": "test job",
				"job":  "/bin/echo test",
				"user": "invalid@user",
			},
			shouldFail:  true,
			expectedErr: "invalid username format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if tt.shouldFail {
				if err == nil {
					t.Error("Expected validation to fail, but it passed")
				} else if tt.expectedErr != "" && !strings.Contains(err.Error(), tt.expectedErr) {
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

func testCronJobs(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "AddNewCronJob",
			Args: map[string]interface{}{
				"name":    "backup job",
				"job":     "/usr/bin/backup.sh",
				"minute":  "0",
				"hour":    "2",
				"day":     "*",
				"month":   "*",
				"weekday": "*",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock empty crontab
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "no crontab for user",
				})

				// Mock successful crontab write
				h.GetConnection().ExpectCommandPattern(`echo '[\s\S]*' > /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`crontab /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`rm -f /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "added cron job")
			},
		},
		{
			Name: "UpdateExistingCronJob",
			Args: map[string]interface{}{
				"name":   "backup job",
				"job":    "/usr/bin/backup.sh",
				"minute": "30",
				"hour":   "3",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock existing crontab
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "0 2 * * * /usr/bin/backup.sh # Ansible: backup job\n",
				})

				// Mock successful crontab write
				h.GetConnection().ExpectCommandPattern(`echo '[\s\S]*' > /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`crontab /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`rm -f /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "updated cron job")
			},
		},
		{
			Name: "RemoveCronJob",
			Args: map[string]interface{}{
				"name":  "backup job",
				"state": "absent",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock existing crontab
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "0 2 * * * /usr/bin/backup.sh # Ansible: backup job\n",
				})

				// Mock successful crontab write (empty)
				h.GetConnection().ExpectCommandPattern(`echo '[\s\S]*' > /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`crontab /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`rm -f /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "removed cron job")
			},
		},
		{
			Name: "JobAlreadyExists",
			Args: map[string]interface{}{
				"name":    "backup job",
				"job":     "/usr/bin/backup.sh",
				"minute":  "0",
				"hour":    "2",
				"day":     "*",
				"month":   "*",
				"weekday": "*",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock existing identical crontab
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "0 2 * * * /usr/bin/backup.sh # Ansible: backup job\n",
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertNotChanged(result)
				h.AssertMessageContains(result, "already in desired state")
			},
		},
		{
			Name: "CronJobWithUser",
			Args: map[string]interface{}{
				"name": "user backup",
				"job":  "/home/backup.sh",
				"user": "testuser",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock empty crontab for user
				h.GetConnection().ExpectCommand("crontab -u testuser -l", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "no crontab for testuser",
				})

				// Mock successful crontab write for user
				h.GetConnection().ExpectCommandPattern(`echo '[\s\S]*' > /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`crontab -u testuser /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`rm -f /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "added cron job")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testCronTimeValidation(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "ValidStepValues",
			Args: map[string]interface{}{
				"name":    "step test",
				"job":     "/bin/echo step",
				"minute":  "*/15",
				"hour":    "*/2",
				"weekday": "*/1",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "no crontab for user",
				})
				h.GetConnection().ExpectCommandPattern(`echo '[\s\S]*' > /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
				h.GetConnection().ExpectCommandPattern(`crontab /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
				h.GetConnection().ExpectCommandPattern(`rm -f /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
			},
		},
		{
			Name: "ValidRangeValues",
			Args: map[string]interface{}{
				"name":    "range test",
				"job":     "/bin/echo range",
				"minute":  "0-30",
				"hour":    "9-17",
				"weekday": "1-5",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "no crontab for user",
				})
				h.GetConnection().ExpectCommandPattern(`echo '[\s\S]*' > /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
				h.GetConnection().ExpectCommandPattern(`crontab /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
				h.GetConnection().ExpectCommandPattern(`rm -f /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
			},
		},
		{
			Name: "ValidCommaValues",
			Args: map[string]interface{}{
				"name":   "comma test",
				"job":    "/bin/echo comma",
				"minute": "0,15,30,45",
				"hour":   "8,12,16",
				"day":    "1,15",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "no crontab for user",
				})
				h.GetConnection().ExpectCommandPattern(`echo '[\s\S]*' > /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
				h.GetConnection().ExpectCommandPattern(`crontab /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
				h.GetConnection().ExpectCommandPattern(`rm -f /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testCronCheckMode(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name:      "AddJobCheckMode",
			CheckMode: true,
			Args: map[string]interface{}{
				"name": "check mode job",
				"job":  "/bin/echo check",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "no crontab for user",
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertSimulated(result)
				h.AssertCheckModeSimulated(result)
				h.AssertMessageContains(result, "Would make changes")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testCronDiffMode(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name:     "AddJobDiffMode",
			DiffMode: true,
			Args: map[string]interface{}{
				"name": "diff mode job",
				"job":  "/bin/echo diff",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "no crontab for user",
				})
				h.GetConnection().ExpectCommandPattern(`echo '[\s\S]*' > /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
				h.GetConnection().ExpectCommandPattern(`crontab /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
				h.GetConnection().ExpectCommandPattern(`rm -f /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertDiffPresent(result)
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testCronErrorHandling(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "CrontabReadError",
			Args: map[string]interface{}{
				"name": "error job",
				"job":  "/bin/echo error",
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 2,
					Stderr:   "crontab: permission denied",
				})
			},
		},
		{
			Name: "CrontabWriteError",
			Args: map[string]interface{}{
				"name": "write error job",
				"job":  "/bin/echo write_error",
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				h.GetConnection().ExpectCommand("crontab -l", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "no crontab for user",
				})
				h.GetConnection().ExpectCommandPattern(`echo '[\s\S]*' > /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
				h.GetConnection().ExpectCommandPattern(`crontab /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "crontab: permission denied",
				}).AllowMultipleCalls()
				h.GetConnection().ExpectCommandPattern(`rm -f /tmp/crontab_\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
		},
	}

	helper.RunTestCases(testCases)
}
