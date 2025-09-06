package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine()
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}

	// Check that built-in functions are registered
	functions := engine.ListFunctions()
	if len(functions) == 0 {
		t.Error("engine should have built-in functions")
	}

	// Check for some expected functions
	expectedFunctions := []string{"upper", "lower", "trim", "replace", "default", "empty"}
	for _, expected := range expectedFunctions {
		found := false
		for _, fn := range functions {
			if fn == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected function %s not found", expected)
		}
	}
}

func TestEngineRenderBasic(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name     string
		template string
		vars     map[string]interface{}
		expected string
		wantErr  bool
	}{
		{
			name:     "simple variable substitution",
			template: "Hello {{.name}}!",
			vars:     map[string]interface{}{"name": "World"},
			expected: "Hello World!",
			wantErr:  false,
		},
		{
			name:     "multiple variables",
			template: "{{.greeting}} {{.name}}, you are {{.age}} years old",
			vars:     map[string]interface{}{"greeting": "Hi", "name": "Alice", "age": 30},
			expected: "Hi Alice, you are 30 years old",
			wantErr:  false,
		},
		{
			name:     "nested variables",
			template: "Server: {{.server.host}}:{{.server.port}}",
			vars: map[string]interface{}{
				"server": map[string]interface{}{
					"host": "localhost",
					"port": 8080,
				},
			},
			expected: "Server: localhost:8080",
			wantErr:  false,
		},
		{
			name:     "with built-in functions",
			template: "{{.name | upper}} - {{.message | lower}}",
			vars:     map[string]interface{}{"name": "john", "message": "HELLO WORLD"},
			expected: "JOHN - hello world",
			wantErr:  false,
		},
		{
			name:     "invalid template",
			template: "{{.name",
			vars:     map[string]interface{}{"name": "test"},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Render(tt.template, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("Render() result = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestEngineBuiltinFunctions(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name     string
		template string
		vars     map[string]interface{}
		expected string
	}{
		{
			name:     "upper function",
			template: "{{.text | upper}}",
			vars:     map[string]interface{}{"text": "hello world"},
			expected: "HELLO WORLD",
		},
		{
			name:     "lower function",
			template: "{{.text | lower}}",
			vars:     map[string]interface{}{"text": "HELLO WORLD"},
			expected: "hello world",
		},
		{
			name:     "default function",
			template: "{{.missing | default \"fallback\"}}",
			vars:     map[string]interface{}{},
			expected: "fallback",
		},
		{
			name:     "quote function",
			template: "{{.text | quote}}",
			vars:     map[string]interface{}{"text": "hello world"},
			expected: `"hello world"`,
		},
		{
			name:     "replace function",
			template: "{{replace \"world\" \"Go\" .text}}",
			vars:     map[string]interface{}{"text": "hello world"},
			expected: "hello Go",
		},
		{
			name:     "length function",
			template: "Length: {{.items | length}}",
			vars:     map[string]interface{}{"items": []interface{}{"a", "b", "c"}},
			expected: "Length: 3",
		},
		{
			name:     "join function",
			template: "{{join \", \" .items}}",
			vars:     map[string]interface{}{"items": []string{"apple", "banana", "cherry"}},
			expected: "apple, banana, cherry",
		},
		{
			name:     "empty function",
			template: "{{if empty .value}}empty{{else}}not empty{{end}}",
			vars:     map[string]interface{}{"value": ""},
			expected: "empty",
		},
		{
			name:     "ternary function", 
			template: "{{ternary .condition \"yes\" \"no\"}}",
			vars:     map[string]interface{}{"condition": true},
			expected: "yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Render(tt.template, tt.vars)
			if err != nil {
				t.Errorf("Render() failed: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Render() result = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestEngineRenderFile(t *testing.T) {
	engine := NewEngine()

	// Create a temporary template file
	tempDir := t.TempDir()
	templateFile := filepath.Join(tempDir, "test.tmpl")
	templateContent := "Hello {{.name}}!\nYour age is {{.age}}."

	if err := os.WriteFile(templateFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}

	vars := map[string]interface{}{
		"name": "Alice",
		"age":  25,
	}

	result, err := engine.RenderFile(templateFile, vars)
	if err != nil {
		t.Fatalf("RenderFile() failed: %v", err)
	}

	expected := "Hello Alice!\nYour age is 25."
	if result != expected {
		t.Errorf("RenderFile() result = %q, expected %q", result, expected)
	}

	// Test with non-existent file
	_, err = engine.RenderFile("/non/existent/file.tmpl", vars)
	if err == nil {
		t.Error("RenderFile() should fail with non-existent file")
	}
}

func TestEngineAddFunction(t *testing.T) {
	engine := NewEngine()

	// Add a custom function
	customFunc := func(s string) string {
		return strings.Repeat(s, 2)
	}

	err := engine.AddFunction("double", customFunc)
	if err != nil {
		t.Fatalf("AddFunction() failed: %v", err)
	}

	// Test the custom function
	template := "{{double .text}}"
	vars := map[string]interface{}{"text": "hello"}
	result, err := engine.Render(template, vars)
	if err != nil {
		t.Fatalf("Render() with custom function failed: %v", err)
	}

	expected := "hellohello"
	if result != expected {
		t.Errorf("Custom function result = %q, expected %q", result, expected)
	}

	// Test error cases
	err = engine.AddFunction("", customFunc)
	if err == nil {
		t.Error("AddFunction() should fail with empty name")
	}

	err = engine.AddFunction("test", nil)
	if err == nil {
		t.Error("AddFunction() should fail with nil function")
	}
}

func TestEngineValidateTemplate(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{
			name:     "valid template",
			template: "Hello {{.name}}!",
			wantErr:  false,
		},
		{
			name:     "template with functions",
			template: "{{.name | upper}}",
			wantErr:  false,
		},
		{
			name:     "invalid template - unclosed",
			template: "{{.name",
			wantErr:  true,
		},
		{
			name:     "invalid template - unknown function",
			template: "{{.name | unknownFunction}}",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.ValidateTemplate(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEngineRenderWithDefaults(t *testing.T) {
	engine := NewEngine()

	template := "Hello {{.name}}, your role is {{.role}}"
	vars := map[string]interface{}{
		"name": "Alice",
	}
	defaults := map[string]interface{}{
		"name": "Unknown",
		"role": "user",
	}

	result, err := engine.RenderWithDefaults(template, vars, defaults)
	if err != nil {
		t.Fatalf("RenderWithDefaults() failed: %v", err)
	}

	expected := "Hello Alice, your role is user"
	if result != expected {
		t.Errorf("RenderWithDefaults() result = %q, expected %q", result, expected)
	}
}

func TestEngineClone(t *testing.T) {
	engine := NewEngine()

	// Add a custom function
	customFunc := func(s string) string {
		return "custom-" + s
	}
	engine.AddFunction("customFunc", customFunc)

	// Clone the engine
	cloned := engine.Clone()

	// Test that the clone has the same functions
	originalFunctions := engine.ListFunctions()
	clonedFunctions := cloned.ListFunctions()

	if len(originalFunctions) != len(clonedFunctions) {
		t.Errorf("Clone should have same number of functions. Original: %d, Cloned: %d",
			len(originalFunctions), len(clonedFunctions))
	}

	// Test that custom function works in clone
	template := "{{customFunc .text}}"
	vars := map[string]interface{}{"text": "test"}

	result, err := cloned.Render(template, vars)
	if err != nil {
		t.Fatalf("Cloned engine render failed: %v", err)
	}

	expected := "custom-test"
	if result != expected {
		t.Errorf("Cloned engine result = %q, expected %q", result, expected)
	}
}

func TestEngineFunctionHelpers(t *testing.T) {
	engine := NewEngine()

	// Test toString function
	if result := engine.toString(123); result != "123" {
		t.Errorf("toString(123) = %q, expected \"123\"", result)
	}

	// Test toBool function
	if result := engine.toBool("true"); !result {
		t.Error("toBool(\"true\") should return true")
	}
	if result := engine.toBool("false"); result {
		t.Error("toBool(\"false\") should return false")
	}

	// Test length function
	if result := engine.length("hello"); result != 5 {
		t.Errorf("length(\"hello\") = %d, expected 5", result)
	}
	if result := engine.length([]interface{}{"a", "b", "c"}); result != 3 {
		t.Errorf("length(slice) = %d, expected 3", result)
	}

	// Test first/last functions
	slice := []interface{}{"first", "middle", "last"}
	if result := engine.first(slice); result != "first" {
		t.Errorf("first(slice) = %v, expected \"first\"", result)
	}
	if result := engine.last(slice); result != "last" {
		t.Errorf("last(slice) = %v, expected \"last\"", result)
	}

	// Test isEmpty function
	if !engine.isEmpty("") {
		t.Error("isEmpty(\"\") should return true")
	}
	if !engine.isEmpty([]interface{}{}) {
		t.Error("isEmpty(empty slice) should return true")
	}
	if engine.isEmpty("hello") {
		t.Error("isEmpty(\"hello\") should return false")
	}

	// Test reverse function
	if result := engine.reverse("hello"); result != "olleh" {
		t.Errorf("reverse(\"hello\") = %q, expected \"olleh\"", result)
	}

	// Test defaultValue function
	if result := engine.defaultValue("default", ""); result != "default" {
		t.Errorf("defaultValue with empty value should return default")
	}
	if result := engine.defaultValue("default", "value"); result != "value" {
		t.Errorf("defaultValue with non-empty value should return value")
	}
}

func TestEngineComplexTemplate(t *testing.T) {
	engine := NewEngine()

	template := `
{{- if .users }}
Users:
{{- range .users }}
  - Name: {{.name | title}}
    Email: {{.email | lower}}
    Active: {{ternary .active "Yes" "No"}}
{{- end }}
{{- else }}
No users found.
{{- end }}

Environment: {{env "HOME" | default "/tmp"}}
`

	vars := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"name":   "john doe",
				"email":  "JOHN@EXAMPLE.COM",
				"active": true,
			},
			map[string]interface{}{
				"name":   "jane smith",
				"email":  "JANE@EXAMPLE.COM",
				"active": false,
			},
		},
	}

	result, err := engine.Render(template, vars)
	if err != nil {
		t.Fatalf("Complex template render failed: %v", err)
	}

	// Check that basic structure is present
	if !strings.Contains(result, "Users:") {
		t.Error("Result should contain 'Users:'")
	}
	if !strings.Contains(result, "John Doe") {
		t.Error("Result should contain title-cased names")
	}
	if !strings.Contains(result, "john@example.com") {
		t.Error("Result should contain lowercased emails")
	}
	if !strings.Contains(result, "Active: Yes") {
		t.Error("Result should contain ternary output")
	}
}

// Benchmark tests
func BenchmarkEngineRender(b *testing.B) {
	engine := NewEngine()
	template := "Hello {{.name}}, you are {{.age}} years old and live in {{.city}}."
	vars := map[string]interface{}{
		"name": "Alice",
		"age":  30,
		"city": "New York",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.Render(template, vars)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEngineRenderWithFunctions(b *testing.B) {
	engine := NewEngine()
	template := "{{.name | upper | quote}} - {{.message | lower | trim}}"
	vars := map[string]interface{}{
		"name":    "john",
		"message": "  HELLO WORLD  ",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.Render(template, vars)
		if err != nil {
			b.Fatal(err)
		}
	}
}