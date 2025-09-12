package modules

import (
	"strings"
	"testing"

	testhelper "github.com/liliang-cn/gosible/pkg/testing"
	"github.com/liliang-cn/gosiblepkg/types"
)

func TestBlockInFileModule(t *testing.T) {
	module := NewBlockInFileModule()
	helper := testhelper.NewModuleTestHelper(t, module)

	// Test basic module properties
	t.Run("ModuleProperties", func(t *testing.T) {
		if module.Name() != "blockinfile" {
			t.Errorf("Expected module name 'blockinfile', got %s", module.Name())
		}

		caps := module.Capabilities()
		if !caps.CheckMode {
			t.Error("Expected blockinfile module to support check mode")
		}
		if !caps.DiffMode {
			t.Error("Expected blockinfile module to support diff mode")
		}
		if caps.Platform != "all" {
			t.Errorf("Expected platform 'all', got %s", caps.Platform)
		}
		if caps.RequiresRoot {
			t.Error("Expected blockinfile module to not require root")
		}
	})

	t.Run("ValidationTests", testBlockInFileValidation)
	t.Run("BlockOperationTests", func(t *testing.T) { testBlockOperations(t, helper) })
	t.Run("MarkerTests", func(t *testing.T) { testBlockMarkers(t, helper) })
	t.Run("InsertPositionTests", func(t *testing.T) { testInsertPositions(t, helper) })
	t.Run("CheckModeTests", func(t *testing.T) { testBlockInFileCheckMode(t, helper) })
	t.Run("DiffModeTests", func(t *testing.T) { testBlockInFileDiffMode(t, helper) })
	t.Run("ErrorHandlingTests", func(t *testing.T) { testBlockInFileErrorHandling(t, helper) })
}

func testBlockInFileValidation(t *testing.T) {
	module := NewBlockInFileModule()

	tests := []struct {
		name        string
		args        map[string]interface{}
		shouldFail  bool
		expectedErr string
	}{
		{
			name: "ValidBlockOperation",
			args: map[string]interface{}{
				"path":  "/etc/config.txt",
				"block": "# This is a test block\nsetting=value",
			},
			shouldFail: false,
		},
		{
			name: "MissingPath",
			args: map[string]interface{}{
				"block": "test content",
			},
			shouldFail:  true,
			expectedErr: "path parameter is required",
		},
		{
			name: "MissingBlockWhenPresent",
			args: map[string]interface{}{
				"path":  "/etc/config.txt",
				"state": "present",
			},
			shouldFail:  true,
			expectedErr: "block parameter is required when state is present",
		},
		{
			name: "ValidAbsentState",
			args: map[string]interface{}{
				"path":  "/etc/config.txt",
				"state": "absent",
			},
			shouldFail: false,
		},
		{
			name: "InvalidState",
			args: map[string]interface{}{
				"path":  "/etc/config.txt",
				"state": "invalid",
			},
			shouldFail:  true,
			expectedErr: "state must be one of",
		},
		{
			name: "InvalidMarker",
			args: map[string]interface{}{
				"path":   "/etc/config.txt",
				"block":  "test",
				"marker": "# STATIC MARKER",
			},
			shouldFail:  true,
			expectedErr: "marker must contain {mark} placeholder",
		},
		{
			name: "ValidCustomMarker",
			args: map[string]interface{}{
				"path":   "/etc/config.txt",
				"block":  "test",
				"marker": "// {mark} CUSTOM BLOCK",
			},
			shouldFail: false,
		},
		{
			name: "MutuallyExclusiveInserts",
			args: map[string]interface{}{
				"path":         "/etc/config.txt",
				"block":        "test",
				"insertafter":  "line1",
				"insertbefore": "line2",
			},
			shouldFail:  true,
			expectedErr: "insertafter and insertbefore are mutually exclusive",
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

func testBlockOperations(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "InsertNewBlock",
			Args: map[string]interface{}{
				"path":  "/etc/hosts",
				"block": "127.0.0.1 test.local\n192.168.1.100 app.local",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read (no existing block)
				h.GetConnection().ExpectCommand("cat /etc/hosts", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "127.0.0.1 localhost\n::1 localhost",
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
				h.AssertMessageContains(result, "inserted block into file")
			},
		},
		{
			Name: "UpdateExistingBlock",
			Args: map[string]interface{}{
				"path":  "/etc/config.txt",
				"block": "new_setting=new_value\nother_setting=updated",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read (with existing block)
				h.GetConnection().ExpectCommand("cat /etc/config.txt", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "# Some config\n# BEGIN ANSIBLE MANAGED BLOCK\nold_setting=old_value\n# END ANSIBLE MANAGED BLOCK\n# More config",
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
				h.AssertMessageContains(result, "updated block in file")
			},
		},
		{
			Name: "RemoveExistingBlock",
			Args: map[string]interface{}{
				"path":  "/etc/config.txt",
				"state": "absent",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read (with existing block)
				h.GetConnection().ExpectCommand("cat /etc/config.txt", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "# Some config\n# BEGIN ANSIBLE MANAGED BLOCK\nsetting=value\n# END ANSIBLE MANAGED BLOCK\n# More config",
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
				h.AssertMessageContains(result, "removed block from file")
			},
		},
		{
			Name: "BlockAlreadyPresent",
			Args: map[string]interface{}{
				"path":  "/etc/config.txt",
				"block": "existing_setting=value",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read (with identical existing block)
				h.GetConnection().ExpectCommand("cat /etc/config.txt", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "# Some config\n# BEGIN ANSIBLE MANAGED BLOCK\nexisting_setting=value\n# END ANSIBLE MANAGED BLOCK\n# More config",
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertNotChanged(result)
				h.AssertMessageContains(result, "already in desired state")
			},
		},
		{
			Name: "BlockAlreadyAbsent",
			Args: map[string]interface{}{
				"path":  "/etc/config.txt",
				"state": "absent",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read (no existing block)
				h.GetConnection().ExpectCommand("cat /etc/config.txt", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "# Some config\n# More config",
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertNotChanged(result)
				h.AssertMessageContains(result, "already in desired state")
			},
		},
		{
			Name: "CreateNewFile",
			Args: map[string]interface{}{
				"path":   "/tmp/newfile.txt",
				"block":  "new_content=value",
				"create": true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read (file doesn't exist)
				h.GetConnection().ExpectCommand("cat /tmp/newfile.txt", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "cat: /tmp/newfile.txt: No such file or directory",
				})

				// Mock file write operations
				h.GetConnection().ExpectCommandPattern(`echo -n '[\s\S]*' > /tmp/newfile\.txt\.tmp\.\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`mv /tmp/newfile\.txt\.tmp\.\d+ /tmp/newfile\.txt`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "inserted block into file")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testBlockMarkers(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "CustomMarkers",
			Args: map[string]interface{}{
				"path":   "/etc/app.conf",
				"block":  "custom_setting=value",
				"marker": "// {mark} CUSTOM BLOCK",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read (no existing block)
				h.GetConnection().ExpectCommand("cat /etc/app.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "// Main config\ndefault_setting=default",
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
				h.AssertMessageContains(result, "inserted block into file")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testInsertPositions(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "InsertAfterPattern",
			Args: map[string]interface{}{
				"path":        "/etc/config.txt",
				"block":       "inserted_setting=value",
				"insertafter": "# Configuration section",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read
				h.GetConnection().ExpectCommand("cat /etc/config.txt", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "# Header\n# Configuration section\nexisting_setting=value\n# Footer",
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
				h.AssertMessageContains(result, "inserted block into file")
			},
		},
		{
			Name: "InsertBeforePattern",
			Args: map[string]interface{}{
				"path":         "/etc/config.txt",
				"block":        "inserted_setting=value",
				"insertbefore": "# Footer",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read
				h.GetConnection().ExpectCommand("cat /etc/config.txt", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "# Header\nexisting_setting=value\n# Footer",
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
				h.AssertMessageContains(result, "inserted block into file")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testBlockInFileCheckMode(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name:      "InsertBlockCheckMode",
			CheckMode: true,
			Args: map[string]interface{}{
				"path":  "/etc/test.conf",
				"block": "test_setting=check_mode_value",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read (no existing block)
				h.GetConnection().ExpectCommand("cat /etc/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "existing_content=value",
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

func testBlockInFileDiffMode(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name:     "InsertBlockDiffMode",
			DiffMode: true,
			Args: map[string]interface{}{
				"path":  "/etc/app.conf",
				"block": "debug=true\nverbose=false",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read (no existing block)
				h.GetConnection().ExpectCommand("cat /etc/app.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "name=myapp\nport=8080",
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

func testBlockInFileErrorHandling(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "FileNotExistsNoCreate",
			Args: map[string]interface{}{
				"path":  "/tmp/nonexistent.txt",
				"block": "test_content",
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
				"path":  "/etc/readonly.conf",
				"block": "new_setting=value",
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read success
				h.GetConnection().ExpectCommand("cat /etc/readonly.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "existing_setting=value",
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
				"path":   "/etc/test.conf",
				"block":  "new_setting=value",
				"backup": true,
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read success
				h.GetConnection().ExpectCommand("cat /etc/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "existing_setting=old",
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
				"path":     "/etc/nginx/nginx.conf",
				"block":    "server { listen 80; }",
				"validate": "nginx -t -c %s",
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock file read success
				h.GetConnection().ExpectCommand("cat /etc/nginx/nginx.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "user nginx;\nevents { worker_connections 1024; }",
				})

				// Mock file write operations
				h.GetConnection().ExpectCommandPattern(`echo -n '[\s\S]*' > /etc/nginx/nginx\.conf\.tmp\.\d+`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				h.GetConnection().ExpectCommandPattern(`mv /etc/nginx/nginx\.conf\.tmp\.\d+ /etc/nginx/nginx\.conf`, &testhelper.CommandResponse{
					ExitCode: 0,
				}).AllowMultipleCalls()

				// Mock validation failure
				h.GetConnection().ExpectCommand("nginx -t -c /etc/nginx/nginx.conf", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "nginx: configuration file test failed",
				})
			},
		},
	}

	helper.RunTestCases(testCases)
}
