package lookup

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LookupPlugin interface for all lookup plugins
type LookupPlugin interface {
	// Name returns the plugin name
	Name() string
	// Lookup performs the lookup operation
	Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error)
	// SetOptions sets plugin-specific options
	SetOptions(options map[string]interface{}) error
}

// LookupManager manages lookup plugins
type LookupManager struct {
	plugins map[string]LookupPlugin
	mu      sync.RWMutex
}

// NewLookupManager creates a new lookup manager
func NewLookupManager() *LookupManager {
	lm := &LookupManager{
		plugins: make(map[string]LookupPlugin),
	}
	
	// Register built-in plugins
	lm.Register(NewFileLookup())
	lm.Register(NewPasswordLookup())
	lm.Register(NewEnvLookup())
	lm.Register(NewURLLookup())
	lm.Register(NewPipeLookup())
	lm.Register(NewTemplateLookup())
	
	return lm
}

// Register adds a lookup plugin
func (lm *LookupManager) Register(plugin LookupPlugin) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.plugins[plugin.Name()] = plugin
}

// Get returns a lookup plugin by name
func (lm *LookupManager) Get(name string) (LookupPlugin, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	
	plugin, exists := lm.plugins[name]
	if !exists {
		return nil, fmt.Errorf("lookup plugin '%s' not found", name)
	}
	
	return plugin, nil
}

// Lookup performs a lookup using the specified plugin
func (lm *LookupManager) Lookup(ctx context.Context, pluginName string, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	plugin, err := lm.Get(pluginName)
	if err != nil {
		return nil, err
	}
	
	return plugin.Lookup(ctx, terms, variables)
}

// FileLookup reads file contents
type FileLookup struct {
	basePath string
	lstrip   bool
	rstrip   bool
}

// NewFileLookup creates a new file lookup plugin
func NewFileLookup() *FileLookup {
	return &FileLookup{
		basePath: ".",
	}
}

// Name returns "file"
func (fl *FileLookup) Name() string {
	return "file"
}

// SetOptions sets file lookup options
func (fl *FileLookup) SetOptions(options map[string]interface{}) error {
	if basePath, ok := options["basepath"].(string); ok {
		fl.basePath = basePath
	}
	if lstrip, ok := options["lstrip"].(bool); ok {
		fl.lstrip = lstrip
	}
	if rstrip, ok := options["rstrip"].(bool); ok {
		fl.rstrip = rstrip
	}
	return nil
}

// Lookup reads files and returns their contents
func (fl *FileLookup) Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))
	
	for _, term := range terms {
		filePath := term
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(fl.basePath, filePath)
		}
		
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file '%s': %w", filePath, err)
		}
		
		contentStr := string(content)
		if fl.lstrip {
			contentStr = strings.TrimLeft(contentStr, " \t\n\r")
		}
		if fl.rstrip {
			contentStr = strings.TrimRight(contentStr, " \t\n\r")
		}
		
		results = append(results, contentStr)
	}
	
	return results, nil
}

// PasswordLookup generates or retrieves passwords
type PasswordLookup struct {
	length      int
	encrypt     string
	passwordDir string
	chars       string
}

// NewPasswordLookup creates a new password lookup plugin
func NewPasswordLookup() *PasswordLookup {
	return &PasswordLookup{
		length:      20,
		passwordDir: os.ExpandEnv("$HOME/.ansible/passwords"),
		chars:       "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
	}
}

// Name returns "password"
func (pl *PasswordLookup) Name() string {
	return "password"
}

// SetOptions sets password lookup options
func (pl *PasswordLookup) SetOptions(options map[string]interface{}) error {
	if length, ok := options["length"].(int); ok {
		pl.length = length
	}
	if encrypt, ok := options["encrypt"].(string); ok {
		pl.encrypt = encrypt
	}
	if chars, ok := options["chars"].(string); ok {
		pl.chars = chars
	}
	return nil
}

// Lookup generates or retrieves passwords
func (pl *PasswordLookup) Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))
	
	// Ensure password directory exists
	if err := os.MkdirAll(pl.passwordDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create password directory: %w", err)
	}
	
	for _, term := range terms {
		parts := strings.Split(term, " ")
		passwordFile := filepath.Join(pl.passwordDir, parts[0])
		
		// Parse options from term
		length := pl.length
		for _, part := range parts[1:] {
			if strings.HasPrefix(part, "length=") {
				fmt.Sscanf(part, "length=%d", &length)
			}
		}
		
		// Check if password file exists
		if content, err := os.ReadFile(passwordFile); err == nil {
			results = append(results, strings.TrimSpace(string(content)))
			continue
		}
		
		// Generate new password
		password := pl.generatePassword(length)
		
		// Save password to file
		if err := os.WriteFile(passwordFile, []byte(password), 0600); err != nil {
			return nil, fmt.Errorf("failed to save password: %w", err)
		}
		
		results = append(results, password)
	}
	
	return results, nil
}

// generatePassword generates a random password
func (pl *PasswordLookup) generatePassword(length int) string {
	b := make([]byte, length)
	charLen := len(pl.chars)
	
	for i := range b {
		randByte := make([]byte, 1)
		rand.Read(randByte)
		b[i] = pl.chars[int(randByte[0])%charLen]
	}
	
	return string(b)
}

// EnvLookup reads environment variables
type EnvLookup struct {
	defaultValue string
}

// NewEnvLookup creates a new environment variable lookup plugin
func NewEnvLookup() *EnvLookup {
	return &EnvLookup{}
}

// Name returns "env"
func (el *EnvLookup) Name() string {
	return "env"
}

// SetOptions sets env lookup options
func (el *EnvLookup) SetOptions(options map[string]interface{}) error {
	if defaultValue, ok := options["default"].(string); ok {
		el.defaultValue = defaultValue
	}
	return nil
}

// Lookup reads environment variables
func (el *EnvLookup) Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))
	
	for _, term := range terms {
		value := os.Getenv(term)
		if value == "" && el.defaultValue != "" {
			value = el.defaultValue
		}
		results = append(results, value)
	}
	
	return results, nil
}

// URLLookup fetches content from URLs
type URLLookup struct {
	timeout       time.Duration
	validateCerts bool
	headers       map[string]string
	client        *http.Client
}

// NewURLLookup creates a new URL lookup plugin
func NewURLLookup() *URLLookup {
	return &URLLookup{
		timeout:       30 * time.Second,
		validateCerts: true,
		headers:       make(map[string]string),
	}
}

// Name returns "url"
func (ul *URLLookup) Name() string {
	return "url"
}

// SetOptions sets URL lookup options
func (ul *URLLookup) SetOptions(options map[string]interface{}) error {
	if timeout, ok := options["timeout"].(int); ok {
		ul.timeout = time.Duration(timeout) * time.Second
	}
	if validateCerts, ok := options["validate_certs"].(bool); ok {
		ul.validateCerts = validateCerts
	}
	if headers, ok := options["headers"].(map[string]string); ok {
		ul.headers = headers
	}
	
	// Create HTTP client
	ul.client = &http.Client{
		Timeout: ul.timeout,
	}
	
	return nil
}

// Lookup fetches content from URLs
func (ul *URLLookup) Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))
	
	if ul.client == nil {
		ul.client = &http.Client{
			Timeout: ul.timeout,
		}
	}
	
	for _, url := range terms {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request for '%s': %w", url, err)
		}
		
		// Add headers
		for key, value := range ul.headers {
			req.Header.Set(key, value)
		}
		
		resp, err := ul.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch URL '%s': %w", url, err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("URL '%s' returned status %d", url, resp.StatusCode)
		}
		
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response from '%s': %w", url, err)
		}
		
		results = append(results, string(content))
	}
	
	return results, nil
}

// PipeLookup executes commands and returns their output
type PipeLookup struct {
	executable string
}

// NewPipeLookup creates a new pipe lookup plugin
func NewPipeLookup() *PipeLookup {
	return &PipeLookup{
		executable: "/bin/sh",
	}
}

// Name returns "pipe"
func (pl *PipeLookup) Name() string {
	return "pipe"
}

// SetOptions sets pipe lookup options
func (pl *PipeLookup) SetOptions(options map[string]interface{}) error {
	if executable, ok := options["executable"].(string); ok {
		pl.executable = executable
	}
	return nil
}

// Lookup executes commands and returns their output
func (pl *PipeLookup) Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))
	
	for _, command := range terms {
		// Execute command using connection
		// This would integrate with the connection plugin system
		// For now, return a placeholder
		results = append(results, fmt.Sprintf("Output of: %s", command))
	}
	
	return results, nil
}

// TemplateLookup renders templates
type TemplateLookup struct {
	basePath string
	convert  bool
}

// NewTemplateLookup creates a new template lookup plugin
func NewTemplateLookup() *TemplateLookup {
	return &TemplateLookup{
		basePath: ".",
		convert:  false,
	}
}

// Name returns "template"
func (tl *TemplateLookup) Name() string {
	return "template"
}

// SetOptions sets template lookup options
func (tl *TemplateLookup) SetOptions(options map[string]interface{}) error {
	if basePath, ok := options["basepath"].(string); ok {
		tl.basePath = basePath
	}
	if convert, ok := options["convert_data"].(bool); ok {
		tl.convert = convert
	}
	return nil
}

// Lookup renders templates and returns the result
func (tl *TemplateLookup) Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))
	
	for _, term := range terms {
		templatePath := term
		if !filepath.IsAbs(templatePath) {
			templatePath = filepath.Join(tl.basePath, templatePath)
		}
		
		// Read template file
		content, err := os.ReadFile(templatePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read template '%s': %w", templatePath, err)
		}
		
		// TODO: Integrate with template engine
		// For now, return raw content
		results = append(results, string(content))
	}
	
	return results, nil
}

// ConsulLookup queries Consul KV store
type ConsulLookup struct {
	host   string
	port   int
	scheme string
	token  string
	client *http.Client
}

// NewConsulLookup creates a new Consul lookup plugin
func NewConsulLookup() *ConsulLookup {
	return &ConsulLookup{
		host:   "localhost",
		port:   8500,
		scheme: "http",
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name returns "consul_kv"
func (cl *ConsulLookup) Name() string {
	return "consul_kv"
}

// SetOptions sets Consul lookup options
func (cl *ConsulLookup) SetOptions(options map[string]interface{}) error {
	if host, ok := options["host"].(string); ok {
		cl.host = host
	}
	if port, ok := options["port"].(int); ok {
		cl.port = port
	}
	if scheme, ok := options["scheme"].(string); ok {
		cl.scheme = scheme
	}
	if token, ok := options["token"].(string); ok {
		cl.token = token
	}
	return nil
}

// Lookup queries Consul KV store
func (cl *ConsulLookup) Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))
	
	for _, key := range terms {
		url := fmt.Sprintf("%s://%s:%d/v1/kv/%s?raw", cl.scheme, cl.host, cl.port, key)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request for key '%s': %w", key, err)
		}
		
		if cl.token != "" {
			req.Header.Set("X-Consul-Token", cl.token)
		}
		
		resp, err := cl.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to query Consul for key '%s': %w", key, err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusNotFound {
			results = append(results, nil)
			continue
		}
		
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Consul returned status %d for key '%s'", resp.StatusCode, key)
		}
		
		value, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read Consul response for key '%s': %w", key, err)
		}
		
		results = append(results, string(value))
	}
	
	return results, nil
}

// EtcdLookup queries etcd KV store
type EtcdLookup struct {
	host     string
	port     int
	version  string
	username string
	password string
	client   *http.Client
}

// NewEtcdLookup creates a new etcd lookup plugin
func NewEtcdLookup() *EtcdLookup {
	return &EtcdLookup{
		host:    "localhost",
		port:    2379,
		version: "v3",
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Name returns "etcd"
func (el *EtcdLookup) Name() string {
	return "etcd"
}

// SetOptions sets etcd lookup options
func (el *EtcdLookup) SetOptions(options map[string]interface{}) error {
	if host, ok := options["host"].(string); ok {
		el.host = host
	}
	if port, ok := options["port"].(int); ok {
		el.port = port
	}
	if version, ok := options["version"].(string); ok {
		el.version = version
	}
	if username, ok := options["username"].(string); ok {
		el.username = username
	}
	if password, ok := options["password"].(string); ok {
		el.password = password
	}
	return nil
}

// Lookup queries etcd KV store
func (el *EtcdLookup) Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))
	
	for _, key := range terms {
		// Encode key to base64 for etcd v3 API
		encodedKey := base64.StdEncoding.EncodeToString([]byte(key))
		
		url := fmt.Sprintf("http://%s:%d/v3/kv/range", el.host, el.port)
		body := fmt.Sprintf(`{"key": "%s"}`, encodedKey)
		
		req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request for key '%s': %w", key, err)
		}
		
		req.Header.Set("Content-Type", "application/json")
		
		if el.username != "" && el.password != "" {
			req.SetBasicAuth(el.username, el.password)
		}
		
		resp, err := el.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to query etcd for key '%s': %w", key, err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("etcd returned status %d for key '%s'", resp.StatusCode, key)
		}
		
		// Parse response and extract value
		// This is simplified - real implementation would parse JSON response
		value, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read etcd response for key '%s': %w", key, err)
		}
		
		results = append(results, string(value))
	}
	
	return results, nil
}

// LinesLookup reads lines from files
type LinesLookup struct {
	encoding string
}

// NewLinesLookup creates a new lines lookup plugin
func NewLinesLookup() *LinesLookup {
	return &LinesLookup{
		encoding: "utf-8",
	}
}

// Name returns "lines"
func (ll *LinesLookup) Name() string {
	return "lines"
}

// SetOptions sets lines lookup options
func (ll *LinesLookup) SetOptions(options map[string]interface{}) error {
	if encoding, ok := options["encoding"].(string); ok {
		ll.encoding = encoding
	}
	return nil
}

// Lookup reads files and returns lines as array
func (ll *LinesLookup) Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0)
	
	for _, filePath := range terms {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file '%s': %w", filePath, err)
		}
		
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			results = append(results, line)
		}
	}
	
	return results, nil
}

// DNSLookup performs DNS queries
type DNSLookup struct {
	qtype string
}

// NewDNSLookup creates a new DNS lookup plugin
func NewDNSLookup() *DNSLookup {
	return &DNSLookup{
		qtype: "A",
	}
}

// Name returns "dig"
func (dl *DNSLookup) Name() string {
	return "dig"
}

// SetOptions sets DNS lookup options
func (dl *DNSLookup) SetOptions(options map[string]interface{}) error {
	if qtype, ok := options["qtype"].(string); ok {
		dl.qtype = qtype
	}
	return nil
}

// Lookup performs DNS queries
func (dl *DNSLookup) Lookup(ctx context.Context, terms []string, variables map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))
	
	for _, domain := range terms {
		// This would use net.Resolver for actual DNS queries
		// For now, return placeholder
		results = append(results, fmt.Sprintf("DNS %s record for %s", dl.qtype, domain))
	}
	
	return results, nil
}