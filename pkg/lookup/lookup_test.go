package lookup

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookupManager(t *testing.T) {
	lm := NewLookupManager()
	
	// Check built-in plugins are registered
	plugins := []string{"file", "password", "env", "url", "pipe", "template"}
	for _, name := range plugins {
		plugin, err := lm.Get(name)
		if err != nil {
			t.Errorf("Failed to get plugin '%s': %v", name, err)
		}
		if plugin == nil {
			t.Errorf("Plugin '%s' is nil", name)
		}
	}
	
	// Test non-existent plugin
	_, err := lm.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent plugin")
	}
}

func TestFileLookup(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!\nThis is a test file.\n"
	
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	lookup := NewFileLookup()
	lookup.SetOptions(map[string]interface{}{"basepath": tmpDir})
	
	ctx := context.Background()
	results, err := lookup.Lookup(ctx, []string{"test.txt"}, nil)
	
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	
	if results[0] != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, results[0])
	}
	
	// Test with strip options
	lookup.SetOptions(map[string]interface{}{
		"basepath": tmpDir,
		"lstrip":   true,
		"rstrip":   true,
	})
	
	results, err = lookup.Lookup(ctx, []string{"test.txt"}, nil)
	if err != nil {
		t.Fatalf("Lookup with strip failed: %v", err)
	}
	
	expected := strings.TrimSpace(testContent)
	if results[0] != expected {
		t.Errorf("Expected stripped content '%s', got '%s'", expected, results[0])
	}
}

func TestFileLookup_NotFound(t *testing.T) {
	lookup := NewFileLookup()
	ctx := context.Background()
	
	_, err := lookup.Lookup(ctx, []string{"nonexistent.txt"}, nil)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestPasswordLookup(t *testing.T) {
	tmpDir := t.TempDir()
	lookup := NewPasswordLookup()
	lookup.passwordDir = tmpDir
	
	ctx := context.Background()
	
	// Test new password generation
	results, err := lookup.Lookup(ctx, []string{"testpass"}, nil)
	if err != nil {
		t.Fatalf("Password generation failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	
	password1 := results[0].(string)
	if len(password1) != 20 { // Default length
		t.Errorf("Expected password length 20, got %d", len(password1))
	}
	
	// Test retrieving existing password
	results, err = lookup.Lookup(ctx, []string{"testpass"}, nil)
	if err != nil {
		t.Fatalf("Password retrieval failed: %v", err)
	}
	
	password2 := results[0].(string)
	if password1 != password2 {
		t.Error("Expected same password on second lookup")
	}
	
	// Test with custom length
	results, err = lookup.Lookup(ctx, []string{"testpass2 length=30"}, nil)
	if err != nil {
		t.Fatalf("Password generation with custom length failed: %v", err)
	}
	
	password3 := results[0].(string)
	if len(password3) != 30 {
		t.Errorf("Expected password length 30, got %d", len(password3))
	}
}

func TestEnvLookup(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")
	
	lookup := NewEnvLookup()
	ctx := context.Background()
	
	results, err := lookup.Lookup(ctx, []string{"TEST_VAR", "NONEXISTENT_VAR"}, nil)
	if err != nil {
		t.Fatalf("Env lookup failed: %v", err)
	}
	
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
	
	if results[0] != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", results[0])
	}
	
	if results[1] != "" {
		t.Errorf("Expected empty string for non-existent var, got '%v'", results[1])
	}
	
	// Test with default value
	lookup.SetOptions(map[string]interface{}{"default": "default_value"})
	results, err = lookup.Lookup(ctx, []string{"NONEXISTENT_VAR"}, nil)
	if err != nil {
		t.Fatalf("Env lookup with default failed: %v", err)
	}
	
	if results[0] != "default_value" {
		t.Errorf("Expected default value, got '%v'", results[0])
	}
}

func TestURLLookup(t *testing.T) {
	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/test":
			w.Write([]byte("test content"))
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"key": "value"}`))
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	
	lookup := NewURLLookup()
	ctx := context.Background()
	
	// Test successful fetch
	results, err := lookup.Lookup(ctx, []string{server.URL + "/test"}, nil)
	if err != nil {
		t.Fatalf("URL lookup failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	
	if results[0] != "test content" {
		t.Errorf("Expected 'test content', got '%v'", results[0])
	}
	
	// Test JSON fetch
	results, err = lookup.Lookup(ctx, []string{server.URL + "/json"}, nil)
	if err != nil {
		t.Fatalf("JSON URL lookup failed: %v", err)
	}
	
	if !strings.Contains(results[0].(string), "\"key\": \"value\"") {
		t.Errorf("Expected JSON content, got '%v'", results[0])
	}
	
	// Test error response
	_, err = lookup.Lookup(ctx, []string{server.URL + "/error"}, nil)
	if err == nil {
		t.Error("Expected error for 500 response")
	}
	
	// Test with headers
	lookup.SetOptions(map[string]interface{}{
		"headers": map[string]string{
			"X-Custom-Header": "test",
		},
	})
	
	results, err = lookup.Lookup(ctx, []string{server.URL + "/test"}, nil)
	if err != nil {
		t.Fatalf("URL lookup with headers failed: %v", err)
	}
}

func TestPipeLookup(t *testing.T) {
	lookup := NewPipeLookup()
	ctx := context.Background()
	
	results, err := lookup.Lookup(ctx, []string{"echo hello", "date"}, nil)
	if err != nil {
		t.Fatalf("Pipe lookup failed: %v", err)
	}
	
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
	
	// Check placeholder output (actual implementation would execute commands)
	if !strings.Contains(results[0].(string), "echo hello") {
		t.Errorf("Expected command in output, got '%v'", results[0])
	}
}

func TestTemplateLookup(t *testing.T) {
	// Create temp template file
	tmpDir := t.TempDir()
	templateFile := filepath.Join(tmpDir, "template.j2")
	templateContent := "Hello {{ name }}!"
	
	if err := os.WriteFile(templateFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}
	
	lookup := NewTemplateLookup()
	lookup.SetOptions(map[string]interface{}{"basepath": tmpDir})
	
	ctx := context.Background()
	variables := map[string]interface{}{
		"name": "World",
	}
	
	results, err := lookup.Lookup(ctx, []string{"template.j2"}, variables)
	if err != nil {
		t.Fatalf("Template lookup failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	
	// For now, returns raw content (would need template engine integration)
	if results[0] != templateContent {
		t.Errorf("Expected template content, got '%v'", results[0])
	}
}

func TestConsulLookup(t *testing.T) {
	// Create mock Consul server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/kv/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		
		key := strings.TrimPrefix(r.URL.Path, "/v1/kv/")
		switch key {
		case "test/key":
			w.Write([]byte("test_value"))
		case "missing":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	
	// Parse server URL to get host and port
	parts := strings.Split(strings.TrimPrefix(server.URL, "http://"), ":")
	host := parts[0]
	port := 80
	if len(parts) > 1 {
		port = atoi(parts[1])
	}
	
	lookup := NewConsulLookup()
	lookup.SetOptions(map[string]interface{}{
		"host": host,
		"port": port,
	})
	
	ctx := context.Background()
	
	// Test existing key
	results, err := lookup.Lookup(ctx, []string{"test/key"}, nil)
	if err != nil {
		t.Fatalf("Consul lookup failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	
	if results[0] != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", results[0])
	}
	
	// Test missing key
	results, err = lookup.Lookup(ctx, []string{"missing"}, nil)
	if err != nil {
		t.Fatalf("Consul lookup for missing key failed: %v", err)
	}
	
	if results[0] != nil {
		t.Errorf("Expected nil for missing key, got '%v'", results[0])
	}
}

func TestLinesLookup(t *testing.T) {
	// Create temp file with multiple lines
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lines.txt")
	testContent := "line1\nline2\nline3\n"
	
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	lookup := NewLinesLookup()
	ctx := context.Background()
	
	results, err := lookup.Lookup(ctx, []string{testFile}, nil)
	if err != nil {
		t.Fatalf("Lines lookup failed: %v", err)
	}
	
	// Should return individual lines
	expectedLines := 4 // includes empty line at end
	if len(results) != expectedLines {
		t.Errorf("Expected %d lines, got %d", expectedLines, len(results))
	}
	
	if results[0] != "line1" {
		t.Errorf("Expected 'line1', got '%v'", results[0])
	}
	
	if results[1] != "line2" {
		t.Errorf("Expected 'line2', got '%v'", results[1])
	}
	
	if results[2] != "line3" {
		t.Errorf("Expected 'line3', got '%v'", results[2])
	}
}

func TestDNSLookup(t *testing.T) {
	lookup := NewDNSLookup()
	ctx := context.Background()
	
	// Test with default A record type
	results, err := lookup.Lookup(ctx, []string{"example.com"}, nil)
	if err != nil {
		t.Fatalf("DNS lookup failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	
	// Check placeholder output (actual implementation would do DNS queries)
	if !strings.Contains(results[0].(string), "DNS A record") {
		t.Errorf("Expected DNS record info, got '%v'", results[0])
	}
	
	// Test with custom record type
	lookup.SetOptions(map[string]interface{}{"qtype": "MX"})
	results, err = lookup.Lookup(ctx, []string{"example.com"}, nil)
	if err != nil {
		t.Fatalf("DNS MX lookup failed: %v", err)
	}
	
	if !strings.Contains(results[0].(string), "DNS MX record") {
		t.Errorf("Expected DNS MX record info, got '%v'", results[0])
	}
}

// Helper function
func atoi(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}