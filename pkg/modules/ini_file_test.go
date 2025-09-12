package modules

import (
	"context"
	"testing"

	testhelper "github.com/liliang-cn/gosible/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIniFileModule_Validate(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
			errMsg:  "path is required",
		},
		{
			name: "missing section for present state",
			args: map[string]interface{}{
				"path":  "/etc/myapp.ini",
				"state": "present",
			},
			wantErr: true,
			errMsg:  "section is required when state is present",
		},
		{
			name: "missing option for present state",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"state":   "present",
			},
			wantErr: true,
			errMsg:  "option is required when state is present",
		},
		{
			name: "missing value for present state",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"option":  "host",
				"state":   "present",
			},
			wantErr: true,
			errMsg:  "value is required when state is present",
		},
		{
			name: "valid present state",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"option":  "host",
				"value":   "localhost",
				"state":   "present",
			},
			wantErr: false,
		},
		{
			name: "valid absent state with option",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"option":  "host",
				"state":   "absent",
			},
			wantErr: false,
		},
		{
			name: "valid absent state with section only",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"state":   "absent",
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			args: map[string]interface{}{
				"path":  "/etc/myapp.ini",
				"state": "invalid",
			},
			wantErr: true,
			errMsg:  "state must be 'present' or 'absent'",
		},
		{
			name: "absent state without section or option",
			args: map[string]interface{}{
				"path":  "/etc/myapp.ini",
				"state": "absent",
			},
			wantErr: true,
			errMsg:  "at least one of section or option is required when state is absent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewIniFileModule()
			err := m.Validate(tt.args)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIniFileModule_Run(t *testing.T) {
	tests := []struct {
		name          string
		args          map[string]interface{}
		mockSetup     func(*testhelper.MockConnection)
		checkMode     bool
		diffMode      bool
		expectSuccess bool
		expectChanged bool
		expectDiff    bool
	}{
		{
			name: "create new file with option",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"option":  "host",
				"value":   "localhost",
				"state":   "present",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 1,
				})
				mc.ExpectCommandPattern("cat > /tmp/ini_file_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/ini_file_temp /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "update existing option",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"option":  "host",
				"value":   "newhost",
				"state":   "present",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "[database]\nhost = localhost\nport = 3306\n",
				})
				mc.ExpectCommandPattern("cat > /tmp/ini_file_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/ini_file_temp /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "remove option",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"option":  "host",
				"state":   "absent",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "[database]\nhost = localhost\nport = 3306\n",
				})
				mc.ExpectCommandPattern("cat > /tmp/ini_file_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/ini_file_temp /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "remove section",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"state":   "absent",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "[database]\nhost = localhost\n[app]\nport = 8080\n",
				})
				mc.ExpectCommandPattern("cat > /tmp/ini_file_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/ini_file_temp /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "check mode - would create file",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"option":  "host",
				"value":   "localhost",
				"state":   "present",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 1,
				})
			},
			checkMode:     true,
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "no change when value already correct",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"option":  "host",
				"value":   "localhost",
				"state":   "present",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "[database]\nhost = localhost\nport = 3306\n",
				})
			},
			expectSuccess: true,
			expectChanged: false,
		},
		{
			name: "create directory if needed",
			args: map[string]interface{}{
				"path":    "/etc/newdir/myapp.ini",
				"section": "database",
				"option":  "host",
				"value":   "localhost",
				"state":   "present",
				"create":  true,
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/newdir/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 1,
				})
				mc.ExpectCommand("mkdir -p /etc/newdir", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommandPattern("cat > /tmp/ini_file_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/ini_file_temp /etc/newdir/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "with backup",
			args: map[string]interface{}{
				"path":    "/etc/myapp.ini",
				"section": "database",
				"option":  "host",
				"value":   "newhost",
				"state":   "present",
				"backup":  true,
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "[database]\nhost = localhost\n",
				})
				// First write for backup
				mc.ExpectCommand("cat > /tmp/ini_file_temp << 'EOF'\n[database]\nhost = localhost\n\nEOF", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/ini_file_temp /etc/myapp.ini.backup", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				// Second write for new content
				mc.ExpectCommand("cat > /tmp/ini_file_temp << 'EOF'\n[database]\nhost = newhost\n\nEOF", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/ini_file_temp /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "no extra spaces formatting",
			args: map[string]interface{}{
				"path":           "/etc/myapp.ini",
				"section":        "database",
				"option":         "host",
				"value":          "localhost",
				"state":          "present",
				"no_extra_spaces": true,
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 1,
				})
				mc.ExpectCommandPattern("cat > /tmp/ini_file_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/ini_file_temp /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "exclusive mode removes other options",
			args: map[string]interface{}{
				"path":      "/etc/myapp.ini",
				"section":   "database",
				"option":    "host",
				"value":     "localhost",
				"state":     "present",
				"exclusive": true,
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "[database]\nhost = oldhost\nport = 3306\nuser = admin\n",
				})
				mc.ExpectCommandPattern("cat > /tmp/ini_file_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/ini_file_temp /etc/myapp.ini", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := testhelper.NewMockConnection(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mc)
			}

			m := NewIniFileModule()
			if tt.checkMode {
				tt.args["_check_mode"] = true
			}
			if tt.diffMode {
				tt.args["_diff"] = true
			}

			result, err := m.Run(context.Background(), mc, tt.args)

			require.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, result.Success)
			assert.Equal(t, tt.expectChanged, result.Changed)
			if tt.expectDiff {
				assert.NotEmpty(t, result.Diff)
			}

			// mc.AssertExpectations(t)
		})
	}
}

func TestIniParser(t *testing.T) {
	t.Run("setValue adds new option to existing section", func(t *testing.T) {
		parser := &iniParser{
			content: "[database]\nhost = localhost\n[app]\nport = 8080\n",
		}
		
		newContent, changed := parser.setValue("database", "user", "admin", false)
		assert.True(t, changed)
		assert.Contains(t, newContent, "user = admin")
		assert.Contains(t, newContent, "[database]")
	})

	t.Run("setValue creates new section if not exists", func(t *testing.T) {
		parser := &iniParser{
			content: "[app]\nport = 8080\n",
		}
		
		newContent, changed := parser.setValue("database", "host", "localhost", false)
		assert.True(t, changed)
		assert.Contains(t, newContent, "[database]")
		assert.Contains(t, newContent, "host = localhost")
	})

	t.Run("setValue updates existing option", func(t *testing.T) {
		parser := &iniParser{
			content: "[database]\nhost = oldhost\nport = 3306\n",
		}
		
		newContent, changed := parser.setValue("database", "host", "newhost", false)
		assert.True(t, changed)
		assert.Contains(t, newContent, "host = newhost")
		assert.NotContains(t, newContent, "host = oldhost")
	})

	t.Run("removeOption removes specific option", func(t *testing.T) {
		parser := &iniParser{
			content: "[database]\nhost = localhost\nport = 3306\n",
		}
		
		newContent, changed := parser.removeOption("database", "host")
		assert.True(t, changed)
		assert.NotContains(t, newContent, "host = localhost")
		assert.Contains(t, newContent, "port = 3306")
	})

	t.Run("removeSection removes entire section", func(t *testing.T) {
		parser := &iniParser{
			content: "[database]\nhost = localhost\n[app]\nport = 8080\n",
		}
		
		newContent, changed := parser.removeSection("database")
		assert.True(t, changed)
		assert.NotContains(t, newContent, "[database]")
		assert.NotContains(t, newContent, "host = localhost")
		assert.Contains(t, newContent, "[app]")
		assert.Contains(t, newContent, "port = 8080")
	})

	t.Run("noExtraSpaces formatting", func(t *testing.T) {
		parser := &iniParser{
			content:       "",
			noExtraSpaces: true,
		}
		
		newContent, _ := parser.setValue("section", "option", "value", false)
		assert.Contains(t, newContent, "option=value")
		assert.NotContains(t, newContent, "option = value")
	})

	t.Run("allowNoValue option", func(t *testing.T) {
		parser := &iniParser{
			content:      "",
			allowNoValue: true,
		}
		
		newContent, _ := parser.setValue("section", "flag", "", false)
		assert.Contains(t, newContent, "flag")
		assert.NotContains(t, newContent, "flag=")
	})

	t.Run("exclusive mode removes other options", func(t *testing.T) {
		parser := &iniParser{
			content: "[database]\nhost = localhost\nport = 3306\nuser = admin\n",
		}
		
		newContent, changed := parser.setValue("database", "host", "newhost", true)
		assert.True(t, changed)
		assert.Contains(t, newContent, "host = newhost")
		assert.NotContains(t, newContent, "port = 3306")
		assert.NotContains(t, newContent, "user = admin")
	})
}