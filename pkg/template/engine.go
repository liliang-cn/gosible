// Package template provides template rendering functionality for gosible.
package template

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/liliang-cn/gosiblepkg/types"
)

// Engine implements the TemplateEngine interface
type Engine struct {
	mu        sync.RWMutex
	functions map[string]interface{}
}

// NewEngine creates a new template engine
func NewEngine() *Engine {
	engine := &Engine{
		functions: make(map[string]interface{}),
	}

	// Register built-in functions
	engine.registerBuiltinFunctions()

	return engine
}

// Render processes a template string with the given variables
func (e *Engine) Render(templateStr string, vars map[string]interface{}) (string, error) {
	e.mu.RLock()
	functions := make(map[string]interface{})
	for k, v := range e.functions {
		functions[k] = v
	}
	e.mu.RUnlock()

	// Create template with functions
	tmpl, err := template.New("template").
		Delims("{{", "}}").
		Funcs(functions).
		Parse(templateStr)
	if err != nil {
		return "", types.NewTemplateError("inline", 0, 0, "failed to parse template", err)
	}

	// Execute template
	var result strings.Builder
	if err := tmpl.Execute(&result, vars); err != nil {
		return "", types.NewTemplateError("inline", 0, 0, "failed to execute template", err)
	}

	return result.String(), nil
}

// RenderFile processes a template file with the given variables
func (e *Engine) RenderFile(filepath string, vars map[string]interface{}) (string, error) {
	// Read template file
	content, err := os.ReadFile(filepath)
	if err != nil {
		return "", types.NewTemplateError(filepath, 0, 0, "failed to read template file", err)
	}

	// Render the template content
	result, err := e.Render(string(content), vars)
	if err != nil {
		// Update error with file information
		if templateErr, ok := err.(*types.TemplateError); ok {
			templateErr.Template = filepath
			return "", templateErr
		}
		return "", types.NewTemplateError(filepath, 0, 0, "failed to render template", err)
	}

	return result, nil
}

// AddFunction adds a custom function to the template engine
func (e *Engine) AddFunction(name string, fn interface{}) error {
	if name == "" {
		return types.NewValidationError("name", name, "function name cannot be empty")
	}
	if fn == nil {
		return types.NewValidationError("fn", fn, "function cannot be nil")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.functions[name] = fn
	return nil
}

// registerBuiltinFunctions registers built-in template functions
func (e *Engine) registerBuiltinFunctions() {
	// String manipulation functions
	e.functions["upper"] = strings.ToUpper
	e.functions["lower"] = strings.ToLower
	e.functions["title"] = strings.Title
	e.functions["trim"] = strings.TrimSpace
	e.functions["replace"] = e.replaceString
	e.functions["split"] = strings.Split
	e.functions["join"] = e.joinStrings
	e.functions["contains"] = strings.Contains
	e.functions["hasPrefix"] = strings.HasPrefix
	e.functions["hasSuffix"] = strings.HasSuffix

	// Type conversion functions
	e.functions["toString"] = e.toString
	e.functions["toInt"] = e.toInt
	e.functions["toBool"] = e.toBool

	// Collection functions
	e.functions["length"] = e.length
	e.functions["first"] = e.first
	e.functions["last"] = e.last
	e.functions["reverse"] = e.reverse

	// Logic functions
	e.functions["default"] = e.defaultValue
	e.functions["empty"] = e.isEmpty
	e.functions["notEmpty"] = e.isNotEmpty

	// Path functions
	e.functions["basename"] = filepath.Base
	e.functions["dirname"] = filepath.Dir
	e.functions["joinPath"] = filepath.Join
	e.functions["cleanPath"] = filepath.Clean

	// Formatting functions
	e.functions["quote"] = e.quote
	e.functions["indent"] = e.indent
	e.functions["nindent"] = e.nindent

	// Conditional functions
	e.functions["ternary"] = e.ternary

	// Regular expression functions
	e.functions["regexMatch"] = e.regexMatch
	e.functions["regexReplace"] = e.regexReplace

	// URL functions
	e.functions["urlParse"] = e.urlParse

	// Environment functions
	e.functions["env"] = os.Getenv

	// List functions
	e.functions["list"] = e.list
	e.functions["dict"] = e.dict
}

// Built-in template function implementations

func (e *Engine) toString(v interface{}) string {
	return types.ConvertToString(v)
}

func (e *Engine) toInt(v interface{}) (int, error) {
	return types.ConvertToInt(v)
}

func (e *Engine) toBool(v interface{}) bool {
	return types.ConvertToBool(v)
}

func (e *Engine) length(v interface{}) int {
	if v == nil {
		return 0
	}

	switch val := v.(type) {
	case string:
		return len(val)
	case []interface{}:
		return len(val)
	case map[string]interface{}:
		return len(val)
	default:
		return 0
	}
}

func (e *Engine) first(v interface{}) interface{} {
	switch val := v.(type) {
	case []interface{}:
		if len(val) > 0 {
			return val[0]
		}
	case string:
		if len(val) > 0 {
			return string(val[0])
		}
	}
	return nil
}

func (e *Engine) last(v interface{}) interface{} {
	switch val := v.(type) {
	case []interface{}:
		if len(val) > 0 {
			return val[len(val)-1]
		}
	case string:
		if len(val) > 0 {
			return string(val[len(val)-1])
		}
	}
	return nil
}

func (e *Engine) reverse(v interface{}) interface{} {
	switch val := v.(type) {
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[len(val)-1-i] = item
		}
		return result
	case string:
		runes := []rune(val)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes)
	}
	return v
}

func (e *Engine) defaultValue(defaultVal, value interface{}) interface{} {
	if value == nil || value == "" {
		return defaultVal
	}
	return value
}

func (e *Engine) isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case string:
		return val == ""
	case []interface{}:
		return len(val) == 0
	case map[string]interface{}:
		return len(val) == 0
	case bool:
		return !val
	case int, int64, float64:
		return val == 0
	default:
		return false
	}
}

func (e *Engine) isNotEmpty(v interface{}) bool {
	return !e.isEmpty(v)
}

func (e *Engine) quote(s string) string {
	return fmt.Sprintf("%q", s)
}

func (e *Engine) indent(spaces int, text string) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}

func (e *Engine) nindent(spaces int, text string) string {
	return "\n" + e.indent(spaces, text)
}

func (e *Engine) ternary(condition bool, trueVal, falseVal interface{}) interface{} {
	if condition {
		return trueVal
	}
	return falseVal
}

func (e *Engine) regexMatch(pattern, text string) bool {
	matched, err := regexp.MatchString(pattern, text)
	return err == nil && matched
}

func (e *Engine) regexReplace(pattern, replacement, text string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return text // Return original text if regex is invalid
	}
	return re.ReplaceAllString(text, replacement)
}

func (e *Engine) urlParse(urlStr string) map[string]interface{} {
	// Simple URL parsing - in production, use net/url
	result := make(map[string]interface{})

	// Extract protocol
	if idx := strings.Index(urlStr, "://"); idx > 0 {
		result["scheme"] = urlStr[:idx]
		urlStr = urlStr[idx+3:]
	}

	// Extract host and path
	if idx := strings.Index(urlStr, "/"); idx > 0 {
		result["host"] = urlStr[:idx]
		result["path"] = urlStr[idx:]
	} else {
		result["host"] = urlStr
		result["path"] = "/"
	}

	return result
}

func (e *Engine) list(items ...interface{}) []interface{} {
	return items
}

func (e *Engine) dict(items ...interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Items should come in key-value pairs
	for i := 0; i < len(items)-1; i += 2 {
		if key, ok := items[i].(string); ok {
			result[key] = items[i+1]
		}
	}

	return result
}

func (e *Engine) replaceString(old, new, s string) string {
	return strings.ReplaceAll(s, old, new)
}

func (e *Engine) joinStrings(sep string, items []string) string {
	return strings.Join(items, sep)
}

// RenderWithDefaults renders a template with default variables
func (e *Engine) RenderWithDefaults(templateStr string, vars, defaults map[string]interface{}) (string, error) {
	// Merge variables with defaults (vars take precedence)
	mergedVars := types.DeepMergeInterfaceMaps(defaults, vars)
	return e.Render(templateStr, mergedVars)
}

// RenderFileWithDefaults renders a template file with default variables
func (e *Engine) RenderFileWithDefaults(filepath string, vars, defaults map[string]interface{}) (string, error) {
	// Merge variables with defaults (vars take precedence)
	mergedVars := types.DeepMergeInterfaceMaps(defaults, vars)
	return e.RenderFile(filepath, mergedVars)
}

// ValidateTemplate validates that a template string is syntactically correct
func (e *Engine) ValidateTemplate(templateStr string) error {
	e.mu.RLock()
	functions := make(map[string]interface{})
	for k, v := range e.functions {
		functions[k] = v
	}
	e.mu.RUnlock()

	_, err := template.New("validation").
		Delims("{{", "}}").
		Funcs(functions).
		Parse(templateStr)

	if err != nil {
		return types.NewTemplateError("validation", 0, 0, "template validation failed", err)
	}

	return nil
}

// ValidateTemplateFile validates that a template file is syntactically correct
func (e *Engine) ValidateTemplateFile(filepath string) error {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return types.NewTemplateError(filepath, 0, 0, "failed to read template file", err)
	}

	if err := e.ValidateTemplate(string(content)); err != nil {
		if templateErr, ok := err.(*types.TemplateError); ok {
			templateErr.Template = filepath
		}
		return err
	}

	return nil
}

// ListFunctions returns a list of all available template functions
func (e *Engine) ListFunctions() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var functions []string
	for name := range e.functions {
		functions = append(functions, name)
	}

	return functions
}

// Clone creates a copy of the template engine with all functions
func (e *Engine) Clone() *Engine {
	e.mu.RLock()
	defer e.mu.RUnlock()

	clone := &Engine{
		functions: make(map[string]interface{}),
	}

	for k, v := range e.functions {
		clone.functions[k] = v
	}

	return clone
}

// DefaultTemplateEngine provides a default template engine instance
var DefaultTemplateEngine = NewEngine()
