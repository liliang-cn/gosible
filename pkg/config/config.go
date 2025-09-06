// Package config provides configuration management functionality for gosinble.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/gosinble/gosinble/pkg/types"
)

// Config implements configuration management
type Config struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewConfig creates a new configuration manager
func NewConfig() *Config {
	config := &Config{
		data: make(map[string]interface{}),
	}
	
	// Load defaults
	config.loadDefaults()
	
	// Load from environment variables
	config.loadFromEnv()
	
	return config
}

// Get retrieves a configuration value
func (c *Config) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[key]
}

// GetString retrieves a string configuration value
func (c *Config) GetString(key string) string {
	if value := c.Get(key); value != nil {
		return types.ConvertToString(value)
	}
	return ""
}

// GetInt retrieves an integer configuration value
func (c *Config) GetInt(key string) int {
	if value := c.Get(key); value != nil {
		if intVal, err := types.ConvertToInt(value); err == nil {
			return intVal
		}
	}
	return 0
}

// GetBool retrieves a boolean configuration value
func (c *Config) GetBool(key string) bool {
	if value := c.Get(key); value != nil {
		return types.ConvertToBool(value)
	}
	return false
}

// GetStringSlice retrieves a string slice configuration value
func (c *Config) GetStringSlice(key string) []string {
	if value := c.Get(key); value != nil {
		switch v := value.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, len(v))
			for i, item := range v {
				result[i] = types.ConvertToString(item)
			}
			return result
		case string:
			// Split comma-separated values
			return strings.Split(v, ",")
		}
	}
	return nil
}

// Set stores a configuration value
func (c *Config) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

// SetString stores a string configuration value
func (c *Config) SetString(key, value string) {
	c.Set(key, value)
}

// SetInt stores an integer configuration value
func (c *Config) SetInt(key string, value int) {
	c.Set(key, value)
}

// SetBool stores a boolean configuration value
func (c *Config) SetBool(key string, value bool) {
	c.Set(key, value)
}

// Load loads configuration from file
func (c *Config) Load(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	var configData map[string]interface{}
	if err := yaml.Unmarshal(data, &configData); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Merge loaded configuration with existing configuration
	for key, value := range configData {
		c.data[key] = value
	}

	return nil
}

// Save saves configuration to file
func (c *Config) Save(filePath string) error {
	c.mu.RLock()
	data := make(map[string]interface{})
	for k, v := range c.data {
		data[k] = v
	}
	c.mu.RUnlock()

	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	if dir := filepath.Dir(filePath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return os.WriteFile(filePath, yamlData, 0644)
}

// GetDefaults returns default configuration values
func (c *Config) GetDefaults() map[string]interface{} {
	defaults := make(map[string]interface{})
	
	defaults["timeout"] = 30
	defaults["forks"] = 5
	defaults["gather_facts"] = true
	defaults["host_key_checking"] = true
	defaults["retry_files_enabled"] = false
	defaults["log_path"] = ""
	defaults["private_key_file"] = ""
	defaults["remote_user"] = ""
	defaults["become"] = false
	defaults["become_method"] = "sudo"
	defaults["become_user"] = "root"
	defaults["become_ask_pass"] = false
	defaults["ask_pass"] = false
	defaults["transport"] = "ssh"
	defaults["remote_port"] = 22
	defaults["module_lang"] = "C"
	defaults["gathering"] = "smart"
	defaults["fact_caching"] = false
	defaults["fact_caching_connection"] = ""
	defaults["fact_caching_timeout"] = 86400
	defaults["stdout_callback"] = "default"
	defaults["callback_whitelist"] = []string{}
	defaults["task_includes_static"] = false
	defaults["handler_includes_static"] = false
	defaults["sudo_flags"] = "-H -S -n"
	defaults["display_skipped_hosts"] = true
	defaults["display_ok_hosts"] = true
	defaults["error_on_undefined_vars"] = false
	defaults["system_warnings"] = true
	defaults["deprecation_warnings"] = true
	defaults["command_warnings"] = false
	defaults["default_gathering"] = "smart"
	defaults["jinja2_extensions"] = []string{}
	defaults["ansible_managed"] = "Ansible managed"
	defaults["interpretter_python"] = "auto_legacy_silent"
	defaults["inventory_enabled"] = []string{"host_list", "script", "auto", "yaml", "ini", "toml"}
	defaults["vars_enabled"] = []string{"host_group_vars"}
	defaults["diff_always"] = false
	defaults["diff_context"] = 3
	defaults["show_custom_stats"] = false
	
	return defaults
}

// loadDefaults loads default configuration values
func (c *Config) loadDefaults() {
	defaults := c.GetDefaults()
	for key, value := range defaults {
		c.data[key] = value
	}
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() {
	envVars := map[string]string{
		"GOSINBLE_TIMEOUT":               "timeout",
		"GOSINBLE_FORKS":                 "forks",
		"GOSINBLE_GATHER_FACTS":          "gather_facts",
		"GOSINBLE_HOST_KEY_CHECKING":     "host_key_checking",
		"GOSINBLE_RETRY_FILES_ENABLED":   "retry_files_enabled",
		"GOSINBLE_LOG_PATH":              "log_path",
		"GOSINBLE_PRIVATE_KEY_FILE":      "private_key_file",
		"GOSINBLE_REMOTE_USER":           "remote_user",
		"GOSINBLE_BECOME":                "become",
		"GOSINBLE_BECOME_METHOD":         "become_method",
		"GOSINBLE_BECOME_USER":           "become_user",
		"GOSINBLE_BECOME_ASK_PASS":       "become_ask_pass",
		"GOSINBLE_ASK_PASS":              "ask_pass",
		"GOSINBLE_TRANSPORT":             "transport",
		"GOSINBLE_REMOTE_PORT":           "remote_port",
		"GOSINBLE_MODULE_LANG":           "module_lang",
		"GOSINBLE_GATHERING":             "gathering",
		"GOSINBLE_FACT_CACHING":          "fact_caching",
		"GOSINBLE_STDOUT_CALLBACK":       "stdout_callback",
		"GOSINBLE_DISPLAY_SKIPPED_HOSTS": "display_skipped_hosts",
		"GOSINBLE_DISPLAY_OK_HOSTS":      "display_ok_hosts",
		"GOSINBLE_ERROR_ON_UNDEFINED_VARS": "error_on_undefined_vars",
		"GOSINBLE_SYSTEM_WARNINGS":       "system_warnings",
		"GOSINBLE_DEPRECATION_WARNINGS":  "deprecation_warnings",
		"GOSINBLE_COMMAND_WARNINGS":      "command_warnings",
	}

	for envVar, configKey := range envVars {
		if value := os.Getenv(envVar); value != "" {
			c.setEnvValue(configKey, value)
		}
	}
}

// setEnvValue sets a configuration value from an environment variable
func (c *Config) setEnvValue(key, value string) {
	// Try to convert to appropriate type based on existing value
	if existing := c.data[key]; existing != nil {
		switch existing.(type) {
		case bool:
			if boolVal, err := strconv.ParseBool(value); err == nil {
				c.data[key] = boolVal
				return
			}
		case int:
			if intVal, err := strconv.Atoi(value); err == nil {
				c.data[key] = intVal
				return
			}
		case []string:
			c.data[key] = strings.Split(value, ",")
			return
		}
	}
	
	// Default to string
	c.data[key] = value
}

// GetAll returns all configuration values
func (c *Config) GetAll() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string]interface{})
	for k, v := range c.data {
		result[k] = v
	}
	return result
}

// Clear clears all configuration values
func (c *Config) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]interface{})
}

// Reset resets configuration to defaults
func (c *Config) Reset() {
	c.Clear()
	c.loadDefaults()
	c.loadFromEnv()
}

// Has checks if a configuration key exists
func (c *Config) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.data[key]
	return exists
}

// Delete removes a configuration key
func (c *Config) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// GetConfigPaths returns possible configuration file paths
func GetConfigPaths() []string {
	var paths []string
	
	// Current directory
	paths = append(paths, "./gosinble.yaml")
	paths = append(paths, "./gosinble.yml")
	paths = append(paths, "./.gosinble.yaml")
	paths = append(paths, "./.gosinble.yml")
	
	// Home directory
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".gosinble.yaml"))
		paths = append(paths, filepath.Join(home, ".gosinble.yml"))
		paths = append(paths, filepath.Join(home, ".config", "gosinble", "config.yaml"))
		paths = append(paths, filepath.Join(home, ".config", "gosinble", "config.yml"))
	}
	
	// System paths
	paths = append(paths, "/etc/gosinble/config.yaml")
	paths = append(paths, "/etc/gosinble/config.yml")
	
	return paths
}

// LoadFromDefaultPaths attempts to load configuration from default paths
func (c *Config) LoadFromDefaultPaths() error {
	paths := GetConfigPaths()
	
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			// File exists, try to load it
			if err := c.Load(path); err != nil {
				// Log error but continue trying other paths
				continue
			}
			return nil
		}
	}
	
	// No configuration file found, use defaults
	return nil
}

// Validate validates the current configuration
func (c *Config) Validate() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Validate timeout
	if timeout := c.GetInt("timeout"); timeout <= 0 {
		return types.NewValidationError("timeout", timeout, "timeout must be positive")
	}
	
	// Validate forks
	if forks := c.GetInt("forks"); forks <= 0 {
		return types.NewValidationError("forks", forks, "forks must be positive")
	}
	
	// Validate transport
	transport := c.GetString("transport")
	validTransports := []string{"ssh", "local", "paramiko_ssh"}
	valid := false
	for _, validTransport := range validTransports {
		if transport == validTransport {
			valid = true
			break
		}
	}
	if !valid {
		return types.NewValidationError("transport", transport, "invalid transport type")
	}
	
	// Validate become_method
	becomeMethod := c.GetString("become_method")
	validMethods := []string{"sudo", "su", "pbrun", "pfexec", "runas"}
	valid = false
	for _, validMethod := range validMethods {
		if becomeMethod == validMethod {
			valid = true
			break
		}
	}
	if !valid {
		return types.NewValidationError("become_method", becomeMethod, "invalid become method")
	}
	
	return nil
}

// DefaultConfig provides a default configuration instance
var DefaultConfig = NewConfig()