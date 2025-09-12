package modules

import (
	"context"
	"os"
	"strings"
	"testing"
	
	"github.com/liliang-cn/gosible/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTemplateModule_Validate(t *testing.T) {
	module := NewTemplateModule()
	
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing src",
			args:    map[string]interface{}{"dest": "/tmp/test"},
			wantErr: true,
			errMsg:  "required field is missing",
		},
		{
			name:    "missing dest",
			args:    map[string]interface{}{"src": "test.tmpl"},
			wantErr: true,
			errMsg:  "required field is missing",
		},
		{
			name: "valid args",
			args: map[string]interface{}{
				"src":  "test.tmpl",
				"dest": "/tmp/test",
			},
			wantErr: false,
		},
		{
			name: "with backup",
			args: map[string]interface{}{
				"src":    "test.tmpl",
				"dest":   "/tmp/test",
				"backup": true,
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTemplateModule_RenderTemplate(t *testing.T) {
	module := NewTemplateModule()
	
	tests := []struct {
		name     string
		template string
		vars     map[string]interface{}
		expected string
	}{
		{
			name:     "simple variable",
			template: "Hello {{.name}}!",
			vars:     map[string]interface{}{"name": "World"},
			expected: "Hello World!",
		},
		{
			name:     "multiple variables",
			template: "{{.greeting}} {{.name}}, port: {{.port}}",
			vars: map[string]interface{}{
				"greeting": "Hello",
				"name":     "Server",
				"port":     8080,
			},
			expected: "Hello Server, port: 8080",
		},
		{
			name:     "conditional",
			template: "Debug: {{if .debug}}enabled{{else}}disabled{{end}}",
			vars:     map[string]interface{}{"debug": true},
			expected: "Debug: enabled",
		},
		{
			name:     "range loop",
			template: "Items:{{range .items}} {{.}}{{end}}",
			vars:     map[string]interface{}{"items": []string{"a", "b", "c"}},
			expected: "Items: a b c",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := module.renderTemplate(tt.template, tt.vars)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplateModule_Run_NewFile(t *testing.T) {
	module := NewTemplateModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	// Create a temporary template file
	tmpFile, err := os.CreateTemp("", "test*.tmpl")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	
	templateContent := "Hello {{.name}}!"
	_, err = tmpFile.Write([]byte(templateContent))
	assert.NoError(t, err)
	tmpFile.Close()
	
	args := map[string]interface{}{
		"src":  tmpFile.Name(),
		"dest": "/tmp/test.conf",
		"vars": map[string]interface{}{
			"name": "World",
		},
	}
	
	expectedContent := "Hello World!"
	
	// Mock: Check if destination exists
	mockConn.On("Execute", ctx, "test -f /tmp/test.conf && echo EXISTS || echo NOTEXISTS", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "NOTEXISTS",
	}, nil)
	
	// Mock: Copy file
	mockConn.On("Copy", ctx, mock.MatchedBy(func(reader interface{}) bool {
		// Check that the reader contains the expected content
		if r, ok := reader.(*strings.Reader); ok {
			buf := make([]byte, len(expectedContent))
			n, _ := r.Read(buf)
			r.Reset(expectedContent) // Reset for actual use
			return string(buf[:n]) == expectedContent
		}
		return false
	}), "/tmp/test.conf", 0644).Return(nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "Template rendered and copied successfully", result.Message)
	
	mockConn.AssertExpectations(t)
}

func TestTemplateModule_Run_ExistingFileSameContent(t *testing.T) {
	module := NewTemplateModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	// Create a temporary template file
	tmpFile, err := os.CreateTemp("", "test*.tmpl")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	
	templateContent := "Hello {{.name}}!"
	_, err = tmpFile.Write([]byte(templateContent))
	assert.NoError(t, err)
	tmpFile.Close()
	
	args := map[string]interface{}{
		"src":  tmpFile.Name(),
		"dest": "/tmp/test.conf",
		"vars": map[string]interface{}{
			"name": "World",
		},
	}
	
	expectedContent := "Hello World!"
	
	// Mock: Check if destination exists
	mockConn.On("Execute", ctx, "test -f /tmp/test.conf && echo EXISTS || echo NOTEXISTS", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "EXISTS",
	}, nil)
	
	// Mock: Get current content
	mockConn.On("Execute", ctx, "cat /tmp/test.conf", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: expectedContent,
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.False(t, result.Changed)
	assert.Equal(t, "File already exists with same content", result.Message)
	
	mockConn.AssertExpectations(t)
}

func TestTemplateModule_CalculateChecksum(t *testing.T) {
	module := NewTemplateModule()
	
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "empty string",
			content:  "",
			expected: "00000000",
		},
		{
			name:     "simple string",
			content:  "Hello",
			expected: "000001f4", 
		},
		{
			name:     "same content same checksum",
			content:  "test",
			expected: "000001c0",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := module.calculateChecksum(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
	
	// Test that same content produces same checksum
	checksum1 := module.calculateChecksum("test content")
	checksum2 := module.calculateChecksum("test content")
	assert.Equal(t, checksum1, checksum2)
	
	// Test that different content produces different checksum
	checksum3 := module.calculateChecksum("different content")
	assert.NotEqual(t, checksum1, checksum3)
}