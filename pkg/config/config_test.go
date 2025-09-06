package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()
	if config == nil {
		t.Fatal("NewConfig returned nil")
	}

	// Check that defaults are loaded
	if config.GetInt("timeout") == 0 {
		t.Error("default timeout should be set")
	}
	
	if config.GetInt("forks") == 0 {
		t.Error("default forks should be set")
	}
	
	if !config.GetBool("gather_facts") {
		t.Error("default gather_facts should be true")
	}
}

func TestConfigGetSet(t *testing.T) {
	config := NewConfig()

	// Test string values
	config.SetString("test_string", "hello world")
	if value := config.GetString("test_string"); value != "hello world" {
		t.Errorf("expected 'hello world', got %s", value)
	}

	// Test int values
	config.SetInt("test_int", 42)
	if value := config.GetInt("test_int"); value != 42 {
		t.Errorf("expected 42, got %d", value)
	}

	// Test bool values
	config.SetBool("test_bool", true)
	if value := config.GetBool("test_bool"); !value {
		t.Error("expected true")
	}

	// Test generic set/get
	config.Set("test_generic", "generic_value")
	if value := config.Get("test_generic"); value != "generic_value" {
		t.Errorf("expected 'generic_value', got %v", value)
	}
}

func TestConfigGetStringSlice(t *testing.T) {
	config := NewConfig()

	// Test with string slice
	config.Set("test_slice", []string{"a", "b", "c"})
	slice := config.GetStringSlice("test_slice")
	if len(slice) != 3 || slice[0] != "a" || slice[1] != "b" || slice[2] != "c" {
		t.Errorf("unexpected string slice: %v", slice)
	}

	// Test with interface slice
	config.Set("test_interface_slice", []interface{}{"x", "y", "z"})
	interfaceSlice := config.GetStringSlice("test_interface_slice")
	if len(interfaceSlice) != 3 || interfaceSlice[0] != "x" {
		t.Errorf("unexpected interface slice conversion: %v", interfaceSlice)
	}

	// Test with comma-separated string
	config.Set("test_csv", "item1,item2,item3")
	csvSlice := config.GetStringSlice("test_csv")
	if len(csvSlice) != 3 || csvSlice[0] != "item1" {
		t.Errorf("unexpected CSV conversion: %v", csvSlice)
	}

	// Test with nonexistent key
	nonexistent := config.GetStringSlice("nonexistent")
	if nonexistent != nil {
		t.Errorf("expected nil for nonexistent key, got %v", nonexistent)
	}
}

func TestConfigLoadSave(t *testing.T) {
	config := NewConfig()
	
	// Set some test values
	config.SetString("test_key", "test_value")
	config.SetInt("test_number", 123)
	config.SetBool("test_flag", true)

	// Create temporary file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test_config.yaml")

	// Save configuration
	err := config.Save(configFile)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Create new config and load from file
	newConfig := NewConfig()
	newConfig.Clear() // Clear defaults
	
	err = newConfig.Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check loaded values
	if newConfig.GetString("test_key") != "test_value" {
		t.Errorf("loaded string value mismatch")
	}
	
	if newConfig.GetInt("test_number") != 123 {
		t.Errorf("loaded int value mismatch")
	}
	
	if !newConfig.GetBool("test_flag") {
		t.Errorf("loaded bool value mismatch")
	}
}

func TestConfigEnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("GOSINBLE_TIMEOUT", "60")
	os.Setenv("GOSINBLE_FORKS", "10")
	os.Setenv("GOSINBLE_GATHER_FACTS", "false")
	defer func() {
		os.Unsetenv("GOSINBLE_TIMEOUT")
		os.Unsetenv("GOSINBLE_FORKS")
		os.Unsetenv("GOSINBLE_GATHER_FACTS")
	}()

	config := NewConfig()

	// Check environment variable values override defaults
	if config.GetInt("timeout") != 60 {
		t.Errorf("expected timeout 60 from env, got %d", config.GetInt("timeout"))
	}
	
	if config.GetInt("forks") != 10 {
		t.Errorf("expected forks 10 from env, got %d", config.GetInt("forks"))
	}
	
	if config.GetBool("gather_facts") {
		t.Error("expected gather_facts false from env, got true")
	}
}

func TestConfigHasDelete(t *testing.T) {
	config := NewConfig()

	// Test Has with existing key
	if !config.Has("timeout") {
		t.Error("Has should return true for default key")
	}

	// Test Has with non-existing key
	if config.Has("nonexistent") {
		t.Error("Has should return false for nonexistent key")
	}

	// Test Delete
	config.Set("temp_key", "temp_value")
	if !config.Has("temp_key") {
		t.Error("temp_key should exist after setting")
	}

	config.Delete("temp_key")
	if config.Has("temp_key") {
		t.Error("temp_key should not exist after deletion")
	}
}

func TestConfigGetAll(t *testing.T) {
	config := NewConfig()
	config.SetString("custom_key", "custom_value")

	all := config.GetAll()
	if len(all) == 0 {
		t.Error("GetAll should return configuration values")
	}

	if all["custom_key"] != "custom_value" {
		t.Error("GetAll should include custom values")
	}

	// Check that defaults are included
	if all["timeout"] == nil {
		t.Error("GetAll should include default values")
	}
}

func TestConfigClearReset(t *testing.T) {
	config := NewConfig()
	config.SetString("custom_key", "custom_value")

	// Test Clear
	config.Clear()
	if config.Has("custom_key") {
		t.Error("Clear should remove all keys")
	}
	if config.Has("timeout") {
		t.Error("Clear should remove default keys")
	}

	// Test Reset
	config.Reset()
	if !config.Has("timeout") {
		t.Error("Reset should restore default keys")
	}
}

func TestConfigValidate(t *testing.T) {
	config := NewConfig()

	// Test valid configuration
	err := config.Validate()
	if err != nil {
		t.Errorf("valid configuration should not error: %v", err)
	}

	// Test invalid timeout
	config.SetInt("timeout", -1)
	err = config.Validate()
	if err == nil {
		t.Error("negative timeout should cause validation error")
	}

	// Reset and test invalid forks
	config.Reset()
	config.SetInt("forks", 0)
	err = config.Validate()
	if err == nil {
		t.Error("zero forks should cause validation error")
	}

	// Reset and test invalid transport
	config.Reset()
	config.SetString("transport", "invalid_transport")
	err = config.Validate()
	if err == nil {
		t.Error("invalid transport should cause validation error")
	}

	// Reset and test invalid become_method
	config.Reset()
	config.SetString("become_method", "invalid_method")
	err = config.Validate()
	if err == nil {
		t.Error("invalid become_method should cause validation error")
	}
}

func TestConfigDefaults(t *testing.T) {
	config := NewConfig()
	defaults := config.GetDefaults()

	if len(defaults) == 0 {
		t.Error("GetDefaults should return default values")
	}

	// Check some key defaults
	expectedDefaults := map[string]interface{}{
		"timeout":       30,
		"forks":         5,
		"gather_facts":  true,
		"transport":     "ssh",
		"remote_port":   22,
		"become_method": "sudo",
	}

	for key, expectedValue := range expectedDefaults {
		if defaults[key] != expectedValue {
			t.Errorf("default %s expected %v, got %v", key, expectedValue, defaults[key])
		}
	}
}

func TestGetConfigPaths(t *testing.T) {
	paths := GetConfigPaths()
	if len(paths) == 0 {
		t.Error("GetConfigPaths should return at least one path")
	}

	// Check that current directory paths are included
	foundCurrentDir := false
	for _, path := range paths {
		if path == "./gosinble.yaml" || path == "./gosinble.yml" {
			foundCurrentDir = true
			break
		}
	}
	if !foundCurrentDir {
		t.Error("GetConfigPaths should include current directory paths")
	}
}

func TestConfigLoadFromDefaultPaths(t *testing.T) {
	config := NewConfig()

	// This should not error even if no config files exist
	err := config.LoadFromDefaultPaths()
	if err != nil {
		t.Errorf("LoadFromDefaultPaths should not error when no files exist: %v", err)
	}

	// Create a config file in current directory
	configContent := `
timeout: 120
forks: 8
gather_facts: false
custom_setting: test_value
`
	
	tempFile, err := os.CreateTemp(".", "gosinble-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err := tempFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	tempFile.Close()

	// Load configuration
	testConfig := NewConfig()
	err = testConfig.Load(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config file: %v", err)
	}

	// Check loaded values
	if testConfig.GetInt("timeout") != 120 {
		t.Errorf("expected timeout 120, got %d", testConfig.GetInt("timeout"))
	}
	
	if testConfig.GetInt("forks") != 8 {
		t.Errorf("expected forks 8, got %d", testConfig.GetInt("forks"))
	}
	
	if testConfig.GetBool("gather_facts") {
		t.Error("expected gather_facts false")
	}
	
	if testConfig.GetString("custom_setting") != "test_value" {
		t.Errorf("expected custom_setting 'test_value', got %s", testConfig.GetString("custom_setting"))
	}
}

func TestConfigConcurrency(t *testing.T) {
	config := NewConfig()

	// Test concurrent read/write access
	done := make(chan bool, 10)

	// Start multiple goroutines setting values
	for i := 0; i < 5; i++ {
		go func(id int) {
			key := "test_key_" + string(rune('0'+id))
			value := "test_value_" + string(rune('0'+id))
			config.SetString(key, value)
			done <- true
		}(i)
	}

	// Start multiple goroutines reading values
	for i := 0; i < 5; i++ {
		go func() {
			_ = config.GetAll()
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify values were set correctly
	for i := 0; i < 5; i++ {
		key := "test_key_" + string(rune('0'+i))
		expectedValue := "test_value_" + string(rune('0'+i))
		if config.GetString(key) != expectedValue {
			t.Errorf("concurrent write failed for %s", key)
		}
	}
}

// Benchmark tests
func BenchmarkConfigGet(b *testing.B) {
	config := NewConfig()
	config.SetString("benchmark_key", "benchmark_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Get("benchmark_key")
	}
}

func BenchmarkConfigSet(b *testing.B) {
	config := NewConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Set("benchmark_key", "benchmark_value")
	}
}

func BenchmarkConfigGetAll(b *testing.B) {
	config := NewConfig()

	// Set up some values
	for i := 0; i < 100; i++ {
		key := "key_" + string(rune('0'+(i%10)))
		config.SetString(key, "value")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.GetAll()
	}
}