package modules

import (
	"strings"
	"testing"

	testhelper "github.com/liliang-cn/gosible/pkg/testing"
	"github.com/liliang-cn/gosiblepkg/types"
)

func TestLineInFileModule(t *testing.T) {
	module := NewLineInFileModule()
	helper := testhelper.NewModuleTestHelper(t, module)

	// Test basic module properties
	t.Run("ModuleProperties", func(t *testing.T) {
		if module.Name() != "lineinfile" {
			t.Errorf("Expected module name 'lineinfile', got %s", module.Name())
		}

		caps := module.Capabilities()
		if !caps.CheckMode {
			t.Error("Expected lineinfile module to support check mode")
		}
		if !caps.DiffMode {
			t.Error("Expected lineinfile module to support diff mode")
		}
		if caps.Platform != "any" {
			t.Errorf("Expected platform 'any', got %s", caps.Platform)
		}
		if caps.RequiresRoot {
			t.Error("Expected lineinfile module to not require root")
		}
	})

	t.Run("ValidationTests", testLineInFileValidation)
	t.Run("PresentLineTests", func(t *testing.T) { testLineInFilePresent(t, helper) })
	t.Run("AbsentLineTests", func(t *testing.T) { testLineInFileAbsent(t, helper) })
	t.Run("RegexTests", func(t *testing.T) { testLineInFileRegex(t, helper) })
	t.Run("InsertPositionTests", func(t *testing.T) { testLineInFileInsertPosition(t, helper) })
	t.Run("CheckModeTests", func(t *testing.T) { testLineInFileCheckMode(t, helper) })
	t.Run("DiffModeTests", func(t *testing.T) { testLineInFileDiffMode(t, helper) })
	t.Run("BackupTests", func(t *testing.T) { testLineInFileBackup(t, helper) })
	t.Run("ErrorHandlingTests", func(t *testing.T) { testLineInFileErrorHandling(t, helper) })
}

func testLineInFileValidation(t *testing.T) {
	module := NewLineInFileModule()

	tests := []struct {
		name        string
		args        map[string]interface{}
		shouldFail  bool
		expectedErr string
	}{
		{
			name: "ValidPresentLine",
			args: map[string]interface{}{
				"path": "/etc/hosts",
				"line": "127.0.0.1 localhost",
			},
			shouldFail: false,
		},
		{
			name: "MissingPath",
			args: map[string]interface{}{
				"line": "test line",
			},
			shouldFail:  true,
			expectedErr: "required parameter",
		},
		{
			name: "PresentWithoutLineOrRegexp",
			args: map[string]interface{}{
				"path":  "/tmp/test",
				"state": "present",
			},
			shouldFail:  true,
			expectedErr: "either line or regexp must be provided",
		},
		{
			name: "AbsentWithoutLineOrRegexp",
			args: map[string]interface{}{
				"path":  "/tmp/test",
				"state": "absent",
			},
			shouldFail:  true,
			expectedErr: "either regexp or line must be provided",
		},
		{
			name: "InvalidState",
			args: map[string]interface{}{
				"path":  "/tmp/test",
				"line":  "test",
				"state": "invalid",
			},
			shouldFail:  true,
			expectedErr: "value must be one of",
		},
		{
			name: "InvalidRegexp",
			args: map[string]interface{}{
				"path":   "/tmp/test",
				"regexp": "[invalid",
			},
			shouldFail:  true,
			expectedErr: "invalid regular expression",
		},
		{
			name: "MutuallyExclusiveInserts",
			args: map[string]interface{}{
				"path":         "/tmp/test",
				"line":         "test line",
				"insertafter":  "pattern1",
				"insertbefore": "pattern2",
			},
			shouldFail:  true,
			expectedErr: "mutually exclusive",
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

func testLineInFilePresent(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "AddNewLine",
			Args: map[string]interface{}{
				"path": "/tmp/test.conf",
				"line": "new_setting=value",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/tmp/test.conf", "existing_line=old\n")

				// Mock cat command to read file
				h.GetConnection().ExpectCommand("cat /tmp/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "existing_line=old\n",
				})

				// Mock write operations
				h.GetConnection().ExpectCommand("echo -n 'existing_line=old\nnew_setting=value\n' > /tmp/test.conf.tmp", &testhelper.CommandResponse{
					ExitCode: 0,
				})

				h.GetConnection().ExpectCommand("mv /tmp/test.conf.tmp /tmp/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "added line")

				// Verify file operations were called
				conn := h.GetConnection()
				conn.AssertCommandCalled("cat /tmp/test.conf")
			},
		},
		{
			Name: "LineAlreadyExists",
			Args: map[string]interface{}{
				"path": "/tmp/test.conf",
				"line": "existing_line=old",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/tmp/test.conf", "existing_line=old\n")

				// Mock cat command to read file
				h.GetConnection().ExpectCommand("cat /tmp/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "existing_line=old\n",
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertNotChanged(result)
				h.AssertMessageContains(result, "already in desired state")

				conn := h.GetConnection()
				conn.AssertCommandCalled("cat /tmp/test.conf")
			},
		},
		{
			Name: "CreateNewFile",
			Args: map[string]interface{}{
				"path":   "/tmp/new_file.conf",
				"line":   "first_line=value",
				"create": true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock cat command failing (file doesn't exist)
				h.GetConnection().ExpectCommand("cat /tmp/new_file.conf", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "cat: /tmp/new_file.conf: No such file or directory",
				})

				// Mock write operations for new file
				h.GetConnection().ExpectCommand("echo -n 'first_line=value\n' > /tmp/new_file.conf.tmp", &testhelper.CommandResponse{
					ExitCode: 0,
				})

				h.GetConnection().ExpectCommand("mv /tmp/new_file.conf.tmp /tmp/new_file.conf", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "added line")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testLineInFileAbsent(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "RemoveExistingLine",
			Args: map[string]interface{}{
				"path":  "/tmp/test.conf",
				"line":  "remove_me=value",
				"state": "absent",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/tmp/test.conf", "keep_line=value\nremove_me=value\nother_line=value\n")

				// Mock cat command
				h.GetConnection().ExpectCommand("cat /tmp/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "keep_line=value\nremove_me=value\nother_line=value\n",
				})

				// Mock write operations
				h.GetConnection().ExpectCommand("echo -n 'keep_line=value\nother_line=value\n' > /tmp/test.conf.tmp", &testhelper.CommandResponse{
					ExitCode: 0,
				})

				h.GetConnection().ExpectCommand("mv /tmp/test.conf.tmp /tmp/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "removed")
			},
		},
		{
			Name: "LineNotPresent",
			Args: map[string]interface{}{
				"path":  "/tmp/test.conf",
				"line":  "not_there=value",
				"state": "absent",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/tmp/test.conf", "existing_line=value\n")

				// Mock cat command
				h.GetConnection().ExpectCommand("cat /tmp/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "existing_line=value\n",
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertNotChanged(result)
				h.AssertMessageContains(result, "already in desired state")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testLineInFileRegex(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "ReplaceLineWithRegex",
			Args: map[string]interface{}{
				"path":   "/etc/ssh/sshd_config",
				"regexp": "^#?PermitRootLogin",
				"line":   "PermitRootLogin no",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/etc/ssh/sshd_config", "#PermitRootLogin yes\nPort 22\n")

				// Mock cat command
				h.GetConnection().ExpectCommand("cat /etc/ssh/sshd_config", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "#PermitRootLogin yes\nPort 22\n",
				})

				// Mock write operations
				h.GetConnection().ExpectCommand("echo -n 'PermitRootLogin no\nPort 22\n' > /etc/ssh/sshd_config.tmp", &testhelper.CommandResponse{
					ExitCode: 0,
				})

				h.GetConnection().ExpectCommand("mv /etc/ssh/sshd_config.tmp /etc/ssh/sshd_config", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "replaced line")
			},
		},
		{
			Name: "RemoveLineWithRegex",
			Args: map[string]interface{}{
				"path":   "/tmp/hosts",
				"regexp": "192\\.168\\.1\\.",
				"state":  "absent",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/tmp/hosts", "127.0.0.1 localhost\n192.168.1.100 server\n192.168.1.101 client\n")

				// Mock cat command
				h.GetConnection().ExpectCommand("cat /tmp/hosts", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "127.0.0.1 localhost\n192.168.1.100 server\n192.168.1.101 client\n",
				})

				// Mock write operations (both lines removed)
				h.GetConnection().ExpectCommand("echo -n '127.0.0.1 localhost\n' > /tmp/hosts.tmp", &testhelper.CommandResponse{
					ExitCode: 0,
				})

				h.GetConnection().ExpectCommand("mv /tmp/hosts.tmp /tmp/hosts", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "removed 2 line")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testLineInFileInsertPosition(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "InsertAfterPattern",
			Args: map[string]interface{}{
				"path":        "/etc/fstab",
				"line":        "/dev/sdb1 /mnt/backup ext4 defaults 0 2",
				"insertafter": "# /etc/fstab",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/etc/fstab", "# /etc/fstab\n# Static information about the filesystems\n/dev/sda1 / ext4 defaults 0 1\n")

				// Mock cat command
				h.GetConnection().ExpectCommand("cat /etc/fstab", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "# /etc/fstab\n# Static information about the filesystems\n/dev/sda1 / ext4 defaults 0 1\n",
				})

				// Mock write operations (line inserted after first line)
				h.GetConnection().ExpectCommand("echo -n '# /etc/fstab\n/dev/sdb1 /mnt/backup ext4 defaults 0 2\n# Static information about the filesystems\n/dev/sda1 / ext4 defaults 0 1\n' > /etc/fstab.tmp", &testhelper.CommandResponse{
					ExitCode: 0,
				})

				h.GetConnection().ExpectCommand("mv /etc/fstab.tmp /etc/fstab", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "added line at position 2")
			},
		},
		{
			Name: "InsertBeforePattern",
			Args: map[string]interface{}{
				"path":         "/tmp/config",
				"line":         "new_setting=value",
				"insertbefore": "# End of config",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/tmp/config", "setting1=value1\nsetting2=value2\n# End of config\n")

				// Mock cat command
				h.GetConnection().ExpectCommand("cat /tmp/config", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "setting1=value1\nsetting2=value2\n# End of config\n",
				})

				// Mock write operations (line inserted before last line)
				h.GetConnection().ExpectCommand("echo -n 'setting1=value1\nsetting2=value2\nnew_setting=value\n# End of config\n' > /tmp/config.tmp", &testhelper.CommandResponse{
					ExitCode: 0,
				})

				h.GetConnection().ExpectCommand("mv /tmp/config.tmp /tmp/config", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertMessageContains(result, "added line at position 3")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testLineInFileCheckMode(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name:      "AddLineCheckMode",
			CheckMode: true,
			Args: map[string]interface{}{
				"path": "/tmp/test.conf",
				"line": "new_setting=value",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/tmp/test.conf", "existing_line=old\n")

				// Mock cat command to read file
				h.GetConnection().ExpectCommand("cat /tmp/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "existing_line=old\n",
				})
				// Note: No write operations should be called in check mode
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertSimulated(result)
				h.AssertCheckModeSimulated(result)
				h.AssertMessageContains(result, "Would modify")

				conn := h.GetConnection()
				conn.AssertCommandCalled("cat /tmp/test.conf")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testLineInFileDiffMode(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name:     "AddLineDiffMode",
			DiffMode: true,
			Args: map[string]interface{}{
				"path": "/tmp/test.conf",
				"line": "new_setting=value",
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/tmp/test.conf", "existing_line=old\n")

				// Mock cat command
				h.GetConnection().ExpectCommand("cat /tmp/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "existing_line=old\n",
				})

				// Mock write operations
				h.GetConnection().ExpectCommand("echo -n 'existing_line=old\nnew_setting=value\n' > /tmp/test.conf.tmp", &testhelper.CommandResponse{
					ExitCode: 0,
				})

				h.GetConnection().ExpectCommand("mv /tmp/test.conf.tmp /tmp/test.conf", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertDiffPresent(result)

				// Check that diff contains file changes
				if result.Diff != nil && !strings.Contains(result.Diff.Before, "existing_line=old") {
					h.AssertMessageContains(result, "Modified file") // fallback assertion
				}
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testLineInFileBackup(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "CreateBackup",
			Args: map[string]interface{}{
				"path":   "/tmp/important.conf",
				"line":   "new_setting=value",
				"backup": true,
			},
			Setup: func(h *testhelper.ModuleTestHelper) {
				fileHelper := h.GetFileHelper()
				fileHelper.CreateTestFile("/tmp/important.conf", "existing_line=old\n")

				// Mock cat command
				h.GetConnection().ExpectCommand("cat /tmp/important.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "existing_line=old\n",
				})

				// Mock backup creation (use regex pattern for timestamp)
				h.GetConnection().ExpectCommandPattern("cp /tmp/important.conf /tmp/important.conf\\.[0-9]{8}-[0-9]{6}\\.backup", &testhelper.CommandResponse{
					ExitCode: 0,
				})

				// Mock write operations
				h.GetConnection().ExpectCommand("echo -n 'existing_line=old\nnew_setting=value\n' > /tmp/important.conf.tmp", &testhelper.CommandResponse{
					ExitCode: 0,
				})

				h.GetConnection().ExpectCommand("mv /tmp/important.conf.tmp /tmp/important.conf", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			Assertions: func(h *testhelper.ModuleTestHelper, result *types.Result) {
				h.AssertSuccess(result)
				h.AssertChanged(result)
				h.AssertDataContainsKey(result, "backup_file")
			},
		},
	}

	helper.RunTestCases(testCases)
}

func testLineInFileErrorHandling(t *testing.T, helper *testhelper.ModuleTestHelper) {
	testCases := []testhelper.TestCase{
		{
			Name: "FileNotExists",
			Args: map[string]interface{}{
				"path": "/tmp/nonexistent.conf",
				"line": "test_line=value",
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock cat command failing (file doesn't exist)
				h.GetConnection().ExpectCommand("cat /tmp/nonexistent.conf", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "cat: /tmp/nonexistent.conf: No such file or directory",
				})
			},
		},
		{
			Name: "WritePermissionDenied",
			Args: map[string]interface{}{
				"path": "/etc/readonly.conf",
				"line": "test_line=value",
			},
			ExpectError: true,
			Setup: func(h *testhelper.ModuleTestHelper) {
				// Mock cat command succeeding
				h.GetConnection().ExpectCommand("cat /etc/readonly.conf", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:   "existing_line=old\n",
				})

				// Mock write operation failing (permission denied)
				h.GetConnection().ExpectCommand("echo -n 'existing_line=old\ntest_line=value\n' > /etc/readonly.conf.tmp", &testhelper.CommandResponse{
					ExitCode: 1,
					Stderr:   "bash: /etc/readonly.conf.tmp: Permission denied",
				})
			},
		},
	}

	helper.RunTestCases(testCases)
}
