package modules

import (
	"strings"
	"testing"

	testhelper "github.com/liliang-cn/gosible/pkg/testing"
	"github.com/liliang-cn/gosiblepkg/types"
)

func TestReplaceModule(t *testing.T) {
	module := NewReplaceModule()
	helper := testhelper.NewModuleTestHelper(t, module)

	// Test basic module properties
	t.Run("ModuleProperties", func(t *testing.T) {
		if module.Name() != "replace" {
			t.Errorf("Expected module name 'replace', got %s", module.Name())
		}

		caps := module.Capabilities()
		if !caps.CheckMode {
			t.Error("Expected replace module to support check mode")
		}
		if !caps.DiffMode {
			t.Error("Expected replace module to support diff mode")
		}
		if caps.Platform != "all" {
			t.Errorf("Expected platform 'all', got %s", caps.Platform)
		}
		if caps.RequiresRoot {
			t.Error("Expected replace module to not require root")
		}
	})

	t.Run("ValidationTests", testReplaceValidation)
	t.Run("ReplaceOperationTests", func(t *testing.T) { testReplaceOperations(t, helper) })
	t.Run("BackupTests", func(t *testing.T) { testReplaceBackup(t, helper) })
	t.Run("CheckModeTests", func(t *testing.T) { testReplaceCheckMode(t, helper) })
	t.Run("DiffModeTests", func(t *testing.T) { testReplaceDiffMode(t, helper) })
	t.Run("ErrorHandlingTests", func(t *testing.T) { testReplaceErrorHandling(t, helper) })
}

func testReplaceValidation(t *testing.T) {
	module := NewReplaceModule()

	tests := []struct {
		name        string
		args        map[string]interface{}
		shouldFail  bool
		expectedErr string
	}{
		{
			name: "ValidReplaceOperation",
			args: map[string]interface{}{
				"path":    "/etc/config.txt",
				"regexp":  "old_value",
				"replace": "new_value",
			},
			shouldFail: false,
		},
		{
			name: "MissingPath",
			args: map[string]interface{}{
				"regexp":  "old_value",
				"replace": "new_value",
			},
			shouldFail:  true,
			expectedErr: "path parameter is required",
		},
		{
			name: "MissingRegexp",
			args: map[string]interface{}{
				"path":    "/etc/config.txt",
				"replace": "new_value",
			},
			shouldFail:  true,
			expectedErr: "regexp parameter is required",
		},
		{
			name: "InvalidRegexp",
			args: map[string]interface{}{
				"path":    "/etc/config.txt",
				"regexp":  "[invalid",
				"replace": "new_value",
			},
			shouldFail:  true,
			expectedErr: "invalid regular expression",
		},
		{
			name: "ValidRegexpWithGroups",
			args: map[string]interface{}{
				"path":    "/etc/config.txt",
				"regexp":  `(\w+)=(\w+)`,
				"replace": "$1=updated_$2",
			},
			shouldFail: false,
		},
		{
			name: "EmptyReplace",
			args: map[string]interface{}{
				"path":    "/etc/config.txt",
				"regexp":  "to_remove",
				"replace": "",
			},
			shouldFail: false,
		},
		{
			name: "WithBackup",
			args: map[string]interface{}{
				"path":    "/etc/config.txt",
				"regexp":  "old_value",
				"replace": "new_value",
				"backup":  true,
			},
			shouldFail: false,
		},
		{
			name: "WithValidation",
			args: map[string]interface{}{
				"path":     "/etc/config.txt",
				"regexp":   "old_value",
				"replace":  "new_value",
				"validate": "configtest %s",
			},
			shouldFail: false,
		},
		{
			name: "InvalidEncoding",
			args: map[string]interface{}{
				"path":     "/etc/config.txt",
				"regexp":   "old_value",
				"replace":  "new_value",
				"encoding": "invalid",
			},
			shouldFail:  true,
			expectedErr: "encoding must be one of",
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

func testReplaceOperations(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "SimpleStringReplace",
			Args: map[string]interface{}{
				"path":    "/etc/config.txt",
				"regexp":  "old_value",
				"replace": "new_value",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read
				h.GetConnection().ExpectCommand("cat /etc/config.txt", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "setting=old_value\nother=keep",
				})

				// Mock file write operations
				h.GetConnection().ExpectCommandPattern(`echo -n '[\s\S]*' > /etc/config\.txt\.tmp\.\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`mv /etc/config\.txt\.tmp\.\d+ /etc/config\.txt`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "replaced 1 occurrence")
			},
		},
		{
			Name: "RegexReplaceWithGroups",
			Args: map[string]interface{}{
				"path":    "/etc/hosts",
				"regexp":  `^(\d+\.\d+\.\d+\.\d+)\s+localhost`,
				"replace": "$1 new-localhost",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read
				h.GetConnection().ExpectCommand("cat /etc/hosts", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "127.0.0.1 localhost\n10.0.0.1 server",
				})

				// Mock file write operations
				h.GetConnection().ExpectCommandPattern(`echo -n '[\s\S]*' > /etc/hosts\.tmp\.\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`mv /etc/hosts\.tmp\.\d+ /etc/hosts`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "replaced 1 occurrence")
			},
		},
		{
			Name: "MultipleOccurrences",
			Args: map[string]interface{}{
				"path":    "/var/log/test.log",
				"regexp":  "ERROR",
				"replace": "WARNING",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read
				h.GetConnection().ExpectCommand("cat /var/log/test.log", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "INFO: start\nERROR: failed\nERROR: timeout\nINFO: end",
				})

				// Mock file write operations
				h.GetConnection().ExpectCommandPattern(`echo -n '[\s\S]*' > /var/log/test\.log\.tmp\.\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`mv /var/log/test\.log\.tmp\.\d+ /var/log/test\.log`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "replaced 2 occurrence")
			},
		},
		{
			Name: "RemoveText",
			Args: map[string]interface{}{
				"path":    "/etc/config.txt",
				"regexp":  `#.*\n`,
				"replace": "",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read
				h.GetConnection().ExpectCommand("cat /etc/config.txt", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "# Comment line\nsetting=value\n# Another comment\n",
				})

				// Mock file write operations
				h.GetConnection().ExpectCommandPattern(`echo -n '[\s\S]*' > /etc/config\.txt\.tmp\.\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`mv /etc/config\.txt\.tmp\.\d+ /etc/config\.txt`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "replaced 2 occurrence")
			},
		},
		{
			Name: "NoChangesNeeded",
			Args: map[string]interface{}{
				"path":    "/etc/config.txt",
				"regexp":  "not_found",
				"replace": "replacement",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read
				h.GetConnection().ExpectCommand("cat /etc/config.txt", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "setting=value\nother=data",
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertNotChanged(result)
				h.AssertMessageContains(result, "No changes needed")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testReplaceBackup(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "CreateBackup",
			Args: map[string]interface{}{
				"path":    "/etc/important.conf",
				"regexp":  "old",
				"replace": "new",
				"backup":  true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read
				h.GetConnection().ExpectCommand("cat /etc/important.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "value=old",
				})

				// Mock backup creation
				h.GetConnection().ExpectCommandPattern(`cp /etc/important\.conf /etc/important\.conf\.backup\.\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				// Mock file write operations
				h.GetConnection().ExpectCommandPattern(`echo -n '[\s\S]*' > /etc/important\.conf\.tmp\.\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`mv /etc/important\.conf\.tmp\.\d+ /etc/important\.conf`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				if result.Data["backup_file"] == nil {
					t.Error("Expected backup_file in result data")
				}
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testReplaceCheckMode(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name:      "ReplaceCheckMode",
			CheckMode: true,
			Args: map[string]interface{}{
				"path":    "/etc/test.conf",
				"regexp":  "test",
				"replace": "prod",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read
				h.GetConnection().ExpectCommand("cat /etc/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "env=test\nmode=test",
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

func testReplaceDiffMode(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name:     "ReplaceDiffMode",
			DiffMode: true,
			Args: map[string]interface{}{
				"path":    "/etc/app.conf",
				"regexp":  "debug=true",
				"replace": "debug=false",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read
				h.GetConnection().ExpectCommand("cat /etc/app.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "name=myapp\ndebug=true\nport=8080",
				})

				// Mock file write operations
				h.GetConnection().ExpectCommandPattern(`echo -n '[\s\S]*' > /etc/app\.conf\.tmp\.\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`mv /etc/app\.conf\.tmp\.\d+ /etc/app\.conf`, &testhelper.CommandResponse{
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

func testReplaceErrorHandling(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "FileNotExists",
			Args: map[string]interface{}{
				"path":    "/tmp/nonexistent.txt",
				"regexp":  "test",
				"replace": "prod",
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read failure
				h.GetConnection().ExpectCommand("cat /tmp/nonexistent.txt", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "cat: /tmp/nonexistent.txt: No such file or directory",
				})
			},
		},
		{
			Name: "WritePermissionDenied",
			Args: map[string]interface{}{
				"path":    "/etc/readonly.conf",
				"regexp":  "old",
				"replace": "new",
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read success
				h.GetConnection().ExpectCommand("cat /etc/readonly.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "setting=old",
				})

				// Mock write failure
				h.GetConnection().ExpectCommandPattern(`echo -n '[\s\S]*' > /etc/readonly\.conf\.tmp\.\d+`, &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "Permission denied",
				}).AllowMultipleCalls()
			},
		},
		{
			Name: "BackupCreationFailed",
			Args: map[string]interface{}{
				"path":    "/etc/test.conf",
				"regexp":  "old",
				"replace": "new",
				"backup":  true,
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read success
				h.GetConnection().ExpectCommand("cat /etc/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "setting=old",
				})

				// Mock backup creation failure
				h.GetConnection().ExpectCommandPattern(`cp /etc/test\.conf /etc/test\.conf\.backup\.\d+`, &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "cp: cannot create backup: Permission denied",
				}).AllowMultipleCalls()
			},
		},
		{
			Name: "ValidationFailed",
			Args: map[string]interface{}{
				"path":     "/etc/config.xml",
				"regexp":   "<old>",
				"replace":  "<new>",
				"validate": "xmllint %s",
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read success
				h.GetConnection().ExpectCommand("cat /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "<config><old>value</old></config>",
				})

				// Mock file write operations
				h.GetConnection().ExpectCommandPattern(`echo -n '[\s\S]*' > /etc/config\.xml\.tmp\.\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`mv /etc/config\.xml\.tmp\.\d+ /etc/config\.xml`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				// Mock validation failure
				h.GetConnection().ExpectCommand("xmllint /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "XML validation failed",
				})
			},
		},
	}

	helper.RunTestCases(testCases)
}
