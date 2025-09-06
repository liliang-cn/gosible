package types

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MatchPattern checks if a string matches a pattern (supports wildcards and regex)
func MatchPattern(pattern, text string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	
	// Check for exact match first
	if pattern == text {
		return true
	}
	
	// Convert shell-style wildcards to regex
	if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") {
		regexPattern := regexp.QuoteMeta(pattern)
		regexPattern = strings.ReplaceAll(regexPattern, "\\*", ".*")
		regexPattern = strings.ReplaceAll(regexPattern, "\\?", ".")
		regexPattern = "^" + regexPattern + "$"
		
		matched, err := regexp.MatchString(regexPattern, text)
		return err == nil && matched
	}
	
	// Try as regex
	matched, err := regexp.MatchString(pattern, text)
	return err == nil && matched
}

// ParseHostPattern parses host patterns and returns individual hosts and groups
func ParseHostPattern(pattern string) (hosts []string, groups []string) {
	if pattern == "" {
		return
	}
	
	// Split by comma or semicolon
	parts := regexp.MustCompile(`[,;]`).Split(pattern, -1)
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// For now, treat everything as potentially both a host pattern and a group pattern
		// The inventory will check both and return matches from either
		// This is simpler and more flexible than trying to guess the intent
		hosts = append(hosts, part)
		groups = append(groups, part)
	}
	
	return
}

// ConvertToString converts various types to string
func ConvertToString(value interface{}) string {
	if value == nil {
		return ""
	}
	
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ConvertToBool converts various types to bool
func ConvertToBool(value interface{}) bool {
	if value == nil {
		return false
	}
	
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "yes", "on", "1", "y", "t":
			return true
		default:
			return false
		}
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(v).Int() != 0
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Uint() != 0
	case float32, float64:
		return reflect.ValueOf(v).Float() != 0.0
	default:
		return false
	}
}

// ConvertToInt converts various types to int
func ConvertToInt(value interface{}) (int, error) {
	if value == nil {
		return 0, nil
	}
	
	switch v := value.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(strings.TrimSpace(v))
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// MergeStringMaps merges multiple string maps, with later maps taking precedence
func MergeStringMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	
	return result
}

// MergeInterfaceMaps merges multiple interface maps, with later maps taking precedence
func MergeInterfaceMaps(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	
	return result
}

// DeepMergeInterfaceMaps recursively merges interface maps
func DeepMergeInterfaceMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy base map
	for k, v := range base {
		result[k] = v
	}
	
	// Merge override map
	for k, v := range override {
		if existing, exists := result[k]; exists {
			// If both values are maps, merge them recursively
			if existingMap, ok := existing.(map[string]interface{}); ok {
				if overrideMap, ok := v.(map[string]interface{}); ok {
					result[k] = DeepMergeInterfaceMaps(existingMap, overrideMap)
					continue
				}
			}
		}
		result[k] = v
	}
	
	return result
}

// ExpandVariables expands variables in a string using Jinja2-style {{VAR}} syntax
func ExpandVariables(text string, vars map[string]interface{}) string {
	// Regular expression to match {{VAR}} patterns (Jinja2-style)
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	
	return re.ReplaceAllStringFunc(text, func(match string) string {
		// Extract variable name from {{variable}}
		varName := strings.TrimSpace(match[2 : len(match)-2])
		
		if value, exists := vars[varName]; exists {
			return ConvertToString(value)
		}
		
		// Return original match if variable not found
		return match
	})
}

// ValidateRequiredFields checks if required fields are present in a map
func ValidateRequiredFields(args map[string]interface{}, required []string) error {
	for _, field := range required {
		if _, exists := args[field]; !exists {
			return NewValidationError(field, nil, "required field is missing")
		}
	}
	return nil
}

// ValidateFieldTypes checks if fields have the correct types
func ValidateFieldTypes(args map[string]interface{}, types map[string]string) error {
	for field, expectedType := range types {
		value, exists := args[field]
		if !exists {
			continue // Skip validation for non-existent fields
		}
		
		actualType := reflect.TypeOf(value).Kind().String()
		if expectedType == "string" && actualType != "string" {
			return NewValidationError(field, value, fmt.Sprintf("expected string, got %s", actualType))
		}
		if expectedType == "int" && (actualType != "int" && actualType != "int64" && actualType != "float64") {
			return NewValidationError(field, value, fmt.Sprintf("expected int, got %s", actualType))
		}
		if expectedType == "bool" && actualType != "bool" {
			return NewValidationError(field, value, fmt.Sprintf("expected bool, got %s", actualType))
		}
		if expectedType == "slice" && actualType != "slice" {
			return NewValidationError(field, value, fmt.Sprintf("expected slice, got %s", actualType))
		}
		if expectedType == "map" && actualType != "map" {
			return NewValidationError(field, value, fmt.Sprintf("expected map, got %s", actualType))
		}
	}
	return nil
}

// SanitizePath sanitizes file paths for security
func SanitizePath(path string) string {
	// Remove any path traversal attempts
	path = strings.ReplaceAll(path, "../", "")
	path = strings.ReplaceAll(path, "..\\", "")
	
	// Normalize path separators
	path = strings.ReplaceAll(path, "\\", "/")
	
	// Remove duplicate slashes
	re := regexp.MustCompile(`/+`)
	path = re.ReplaceAllString(path, "/")
	
	return strings.TrimSpace(path)
}

// StringSliceContains checks if a string slice contains a value
func StringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// UniqueStrings removes duplicates from a string slice
func UniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))
	
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// GetCurrentTime returns the current time
func GetCurrentTime() time.Time {
	return time.Now()
}