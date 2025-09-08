package modules

import (
	"context"
	"testing"

	testhelper "github.com/liliang-cn/gosinble/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXMLModule_Validate(t *testing.T) {
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
			name: "missing xpath",
			args: map[string]interface{}{
				"path": "/etc/config.xml",
			},
			wantErr: true,
			errMsg:  "xpath is required",
		},
		{
			name: "missing value for present state",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"state": "present",
			},
			wantErr: true,
			errMsg:  "one of value, add_children, set_children, or attribute is required when state is present",
		},
		{
			name: "valid present state with value",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"value": "content",
				"state": "present",
			},
			wantErr: false,
		},
		{
			name: "valid present state with attribute",
			args: map[string]interface{}{
				"path":            "/etc/config.xml",
				"xpath":           "/root/element",
				"attribute":       "name",
				"attribute_value": "value",
				"state":           "present",
			},
			wantErr: false,
		},
		{
			name: "valid present state with add_children",
			args: map[string]interface{}{
				"path":         "/etc/config.xml",
				"xpath":        "/root/element",
				"add_children": []interface{}{"<child>value</child>"},
				"state":        "present",
			},
			wantErr: false,
		},
		{
			name: "valid present state with set_children",
			args: map[string]interface{}{
				"path":         "/etc/config.xml",
				"xpath":        "/root/element",
				"set_children": []interface{}{"<child>value</child>"},
				"state":        "present",
			},
			wantErr: false,
		},
		{
			name: "valid absent state",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"state": "absent",
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"state": "invalid",
			},
			wantErr: true,
			errMsg:  "state must be 'present' or 'absent'",
		},
		{
			name: "count with absent state",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"state": "absent",
				"count": true,
			},
			wantErr: true,
			errMsg:  "count can only be used with state=present",
		},
		{
			name: "valid namespaces",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/ns:root/ns:element",
				"value": "content",
				"state": "present",
				"namespaces": map[string]interface{}{
					"ns": "http://example.com/namespace",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid namespace type",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/ns:root/ns:element",
				"value": "content",
				"state": "present",
				"namespaces": map[string]interface{}{
					"ns": 123,
				},
			},
			wantErr: true,
			errMsg:  "namespace ns must have a string URI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewXMLModule()
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

func TestXMLModule_Run(t *testing.T) {
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
			name: "create new file with element",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"value": "content",
				"state": "present",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 1,
				})
				mc.ExpectCommandPattern("cat > /tmp/xml_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/xml_temp /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "update existing element value",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"value": "new_content",
				"state": "present",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "<?xml version=\"1.0\"?>\n<root><element>old_content</element></root>",
				})
				mc.ExpectCommandPattern("cat > /tmp/xml_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/xml_temp /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "remove element",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"state": "absent",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "<?xml version=\"1.0\"?>\n<root><element>content</element></root>",
				})
				mc.ExpectCommandPattern("cat > /tmp/xml_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/xml_temp /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "check mode - would create file",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"value": "content",
				"state": "present",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 1,
				})
			},
			checkMode:     true,
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "count matches",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"state": "present",
				"count": true,
				"value": "dummy",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "<?xml version=\"1.0\"?>\n<root><element>1</element><element>2</element></root>",
				})
			},
			expectSuccess: true,
			expectChanged: false,
		},
		{
			name: "print matches",
			args: map[string]interface{}{
				"path":        "/etc/config.xml",
				"xpath":       "/root/element",
				"state":       "present",
				"print_match": true,
				"value":       "dummy",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "<?xml version=\"1.0\"?>\n<root><element>content</element></root>",
				})
			},
			expectSuccess: true,
			expectChanged: false,
		},
		{
			name: "create directory if needed",
			args: map[string]interface{}{
				"path":   "/etc/newdir/config.xml",
				"xpath":  "/root/element",
				"value":  "content",
				"state":  "present",
				"create": true,
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/newdir/config.xml", &testhelper.CommandResponse{
					ExitCode: 1,
				})
				mc.ExpectCommand("mkdir -p /etc/newdir", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommandPattern("cat > /tmp/xml_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/xml_temp /etc/newdir/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "with backup",
			args: map[string]interface{}{
				"path":   "/etc/config.xml",
				"xpath":  "/root/element",
				"value":  "new_content",
				"state":  "present",
				"backup": true,
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "<?xml version=\"1.0\"?>\n<root><element>old</element></root>",
				})
				// First write for backup (original content)
				mc.ExpectCommand("cat > /tmp/xml_temp << 'EOF'\n<?xml version=\"1.0\"?>\n<root><element>old</element></root>\nEOF", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/xml_temp /etc/config.xml.backup", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				// Second write for new content
				mc.ExpectCommand("cat > /tmp/xml_temp << 'EOF'\n<?xml version=\"1.0\"?>\n<root><element>new_content</element></root>\nEOF", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/xml_temp /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "set attribute",
			args: map[string]interface{}{
				"path":            "/etc/config.xml",
				"xpath":           "/root/element",
				"attribute":       "id",
				"attribute_value": "123",
				"state":           "present",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("cat /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
					Stdout:  "<?xml version=\"1.0\"?>\n<root><element>content</element></root>",
				})
				mc.ExpectCommandPattern("cat > /tmp/xml_temp.*", &testhelper.CommandResponse{
					ExitCode: 0,
				})
				mc.ExpectCommand("mv /tmp/xml_temp /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 0,
				})
			},
			expectSuccess: true,
			expectChanged: true,
		},
		{
			name: "file not exists for absent state",
			args: map[string]interface{}{
				"path":  "/etc/config.xml",
				"xpath": "/root/element",
				"state": "absent",
			},
			mockSetup: func(mc *testhelper.MockConnection) {
				mc.ExpectCommand("test -f /etc/config.xml", &testhelper.CommandResponse{
					ExitCode: 1,
				})
			},
			expectSuccess: true,
			expectChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := testhelper.NewMockConnection(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mc)
			}

			m := NewXMLModule()
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

func TestXMLProcessor(t *testing.T) {
	t.Run("parse valid XML", func(t *testing.T) {
		processor := &xmlProcessor{
			content: "<?xml version=\"1.0\"?><root><element>content</element></root>",
		}
		
		err := processor.parse()
		assert.NoError(t, err)
		assert.NotNil(t, processor.doc)
	})

	t.Run("parse invalid XML", func(t *testing.T) {
		processor := &xmlProcessor{
			content: "not valid xml",
		}
		
		err := processor.parse()
		assert.Error(t, err)
	})

	t.Run("createNewXMLContent", func(t *testing.T) {
		m := NewXMLModule()
		content := m.createNewXMLContent("/root/element", map[string]interface{}{
			"value": "test_content",
		})
		
		assert.Contains(t, content, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
		assert.Contains(t, content, "<root>")
		assert.Contains(t, content, "test_content")
		assert.Contains(t, content, "</root>")
	})

	t.Run("generateDiff", func(t *testing.T) {
		m := NewXMLModule()
		oldContent := "<root><element>old</element></root>"
		newContent := "<root><element>new</element></root>"
		
		diff := m.generateDiff(oldContent, newContent, "/etc/config.xml")
		
		assert.Contains(t, diff, "--- /etc/config.xml")
		assert.Contains(t, diff, "+++ /etc/config.xml")
		assert.Contains(t, diff, "-<root><element>old</element></root>")
		assert.Contains(t, diff, "+<root><element>new</element></root>")
	})

	t.Run("xpath with namespace", func(t *testing.T) {
		processor := &xmlProcessor{
			content: `<?xml version="1.0"?><ns:root xmlns:ns="http://example.com"><ns:element>content</ns:element></ns:root>`,
		}
		
		err := processor.parse()
		assert.NoError(t, err)
		
		namespaces := map[string]interface{}{
			"ns": "http://example.com",
		}
		
		elements := processor.findElements("/ns:root/ns:element", namespaces)
		assert.NotNil(t, elements)
	})

	t.Run("xpath with predicate", func(t *testing.T) {
		processor := &xmlProcessor{
			content: `<?xml version="1.0"?><root><item id="1">first</item><item id="2">second</item></root>`,
		}
		
		err := processor.parse()
		assert.NoError(t, err)
		
		elements := processor.findElements("/root/item[@id='2']", nil)
		assert.NotNil(t, elements)
	})

	t.Run("xpath with index", func(t *testing.T) {
		processor := &xmlProcessor{
			content: `<?xml version="1.0"?><root><item>first</item><item>second</item><item>third</item></root>`,
		}
		
		err := processor.parse()
		assert.NoError(t, err)
		
		elements := processor.findElements("/root/item[2]", nil)
		assert.NotNil(t, elements)
	})
}