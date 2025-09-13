package filter

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FilterPlugin interface for all filter plugins
type FilterPlugin interface {
	// Name returns the filter name
	Name() string
	// Filter applies the filter to the input
	Filter(input interface{}, args ...interface{}) (interface{}, error)
}

// FilterManager manages filter plugins
type FilterManager struct {
	filters map[string]FilterPlugin
}

// NewFilterManager creates a new filter manager
func NewFilterManager() *FilterManager {
	fm := &FilterManager{
		filters: make(map[string]FilterPlugin),
	}
	
	// Register built-in filters
	fm.RegisterBuiltinFilters()
	
	return fm
}

// RegisterBuiltinFilters registers all built-in filters
func (fm *FilterManager) RegisterBuiltinFilters() {
	// String filters
	fm.Register(&UpperFilter{})
	fm.Register(&LowerFilter{})
	fm.Register(&TitleFilter{})
	fm.Register(&CapitalizeFilter{})
	fm.Register(&ReplaceFilter{})
	fm.Register(&RegexReplaceFilter{})
	fm.Register(&RegexSearchFilter{})
	fm.Register(&RegexFindallFilter{})
	fm.Register(&SplitFilter{})
	fm.Register(&JoinFilter{})
	fm.Register(&TrimFilter{})
	fm.Register(&Base64EncodeFilter{})
	fm.Register(&Base64DecodeFilter{})
	
	// Hash filters
	fm.Register(&HashFilter{})
	fm.Register(&MD5Filter{})
	fm.Register(&SHA1Filter{})
	fm.Register(&SHA256Filter{})
	fm.Register(&SHA512Filter{})
	
	// List filters
	fm.Register(&UniqueFilter{})
	fm.Register(&SortFilter{})
	fm.Register(&ReverseFilter{})
	fm.Register(&FlattenFilter{})
	fm.Register(&MinFilter{})
	fm.Register(&MaxFilter{})
	fm.Register(&FirstFilter{})
	fm.Register(&LastFilter{})
	fm.Register(&LengthFilter{})
	fm.Register(&SelectFilter{})
	fm.Register(&RejectFilter{})
	fm.Register(&SelectAttrFilter{})
	fm.Register(&MapFilter{})
	
	// Dict filters
	fm.Register(&CombineFilter{})
	fm.Register(&DictToItemsFilter{})
	fm.Register(&ItemsToDictFilter{})
	fm.Register(&JSONQueryFilter{})
	
	// Type conversion filters
	fm.Register(&IntFilter{})
	fm.Register(&FloatFilter{})
	fm.Register(&BoolFilter{})
	fm.Register(&StringFilter{})
	fm.Register(&ListFilter{})
	fm.Register(&DictFilter{})
	
	// Date filters
	fm.Register(&DateFormatFilter{})
	fm.Register(&ToDatetimeFilter{})
	
	// IP filters
	fm.Register(&IPAddrFilter{})
	fm.Register(&IPWrapFilter{})
	fm.Register(&IPv4Filter{})
	fm.Register(&IPv6Filter{})
	
	// Path filters
	fm.Register(&BasenameFilter{})
	fm.Register(&DirnameFilter{})
	fm.Register(&ExpanduserFilter{})
	fm.Register(&RealpathFilter{})
	
	// JSON/YAML filters
	fm.Register(&ToJSONFilter{})
	fm.Register(&FromJSONFilter{})
	fm.Register(&ToYAMLFilter{})
	fm.Register(&FromYAMLFilter{})
}

// Register adds a filter plugin
func (fm *FilterManager) Register(filter FilterPlugin) {
	fm.filters[filter.Name()] = filter
}

// Get returns a filter by name
func (fm *FilterManager) Get(name string) (FilterPlugin, error) {
	filter, exists := fm.filters[name]
	if !exists {
		return nil, fmt.Errorf("filter '%s' not found", name)
	}
	return filter, nil
}

// Apply applies a filter to input
func (fm *FilterManager) Apply(name string, input interface{}, args ...interface{}) (interface{}, error) {
	filter, err := fm.Get(name)
	if err != nil {
		return nil, err
	}
	return filter.Filter(input, args...)
}

// String Filters

// UpperFilter converts string to uppercase
type UpperFilter struct{}

func (f *UpperFilter) Name() string { return "upper" }
func (f *UpperFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("upper filter requires string input")
	}
	return strings.ToUpper(str), nil
}

// LowerFilter converts string to lowercase
type LowerFilter struct{}

func (f *LowerFilter) Name() string { return "lower" }
func (f *LowerFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("lower filter requires string input")
	}
	return strings.ToLower(str), nil
}

// TitleFilter converts string to title case
type TitleFilter struct{}

func (f *TitleFilter) Name() string { return "title" }
func (f *TitleFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("title filter requires string input")
	}
	return strings.Title(str), nil
}

// CapitalizeFilter capitalizes first letter
type CapitalizeFilter struct{}

func (f *CapitalizeFilter) Name() string { return "capitalize" }
func (f *CapitalizeFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("capitalize filter requires string input")
	}
	if len(str) == 0 {
		return str, nil
	}
	return strings.ToUpper(string(str[0])) + str[1:], nil
}

// ReplaceFilter replaces text in string
type ReplaceFilter struct{}

func (f *ReplaceFilter) Name() string { return "replace" }
func (f *ReplaceFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("replace filter requires string input")
	}
	if len(args) < 2 {
		return nil, fmt.Errorf("replace filter requires old and new arguments")
	}
	old, _ := args[0].(string)
	new, _ := args[1].(string)
	return strings.ReplaceAll(str, old, new), nil
}

// RegexReplaceFilter replaces text using regex
type RegexReplaceFilter struct{}

func (f *RegexReplaceFilter) Name() string { return "regex_replace" }
func (f *RegexReplaceFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("regex_replace filter requires string input")
	}
	if len(args) < 2 {
		return nil, fmt.Errorf("regex_replace filter requires pattern and replacement")
	}
	
	pattern, _ := args[0].(string)
	replacement, _ := args[1].(string)
	
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	return re.ReplaceAllString(str, replacement), nil
}

// RegexSearchFilter searches for regex pattern
type RegexSearchFilter struct{}

func (f *RegexSearchFilter) Name() string { return "regex_search" }
func (f *RegexSearchFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("regex_search filter requires string input")
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("regex_search filter requires pattern")
	}
	
	pattern, _ := args[0].(string)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	return re.FindString(str), nil
}

// RegexFindallFilter finds all regex matches
type RegexFindallFilter struct{}

func (f *RegexFindallFilter) Name() string { return "regex_findall" }
func (f *RegexFindallFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("regex_findall filter requires string input")
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("regex_findall filter requires pattern")
	}
	
	pattern, _ := args[0].(string)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	return re.FindAllString(str, -1), nil
}

// SplitFilter splits string into list
type SplitFilter struct{}

func (f *SplitFilter) Name() string { return "split" }
func (f *SplitFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("split filter requires string input")
	}
	
	sep := " "
	if len(args) > 0 {
		sep, _ = args[0].(string)
	}
	
	return strings.Split(str, sep), nil
}

// JoinFilter joins list into string
type JoinFilter struct{}

func (f *JoinFilter) Name() string { return "join" }
func (f *JoinFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	sep := ""
	if len(args) > 0 {
		sep, _ = args[0].(string)
	}
	
	switch v := input.(type) {
	case []string:
		return strings.Join(v, sep), nil
	case []interface{}:
		strs := make([]string, len(v))
		for i, item := range v {
			strs[i] = fmt.Sprintf("%v", item)
		}
		return strings.Join(strs, sep), nil
	default:
		return nil, fmt.Errorf("join filter requires list input")
	}
}

// TrimFilter trims whitespace
type TrimFilter struct{}

func (f *TrimFilter) Name() string { return "trim" }
func (f *TrimFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("trim filter requires string input")
	}
	return strings.TrimSpace(str), nil
}

// Base64EncodeFilter encodes to base64
type Base64EncodeFilter struct{}

func (f *Base64EncodeFilter) Name() string { return "b64encode" }
func (f *Base64EncodeFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("b64encode filter requires string input")
	}
	return base64.StdEncoding.EncodeToString([]byte(str)), nil
}

// Base64DecodeFilter decodes from base64
type Base64DecodeFilter struct{}

func (f *Base64DecodeFilter) Name() string { return "b64decode" }
func (f *Base64DecodeFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("b64decode filter requires string input")
	}
	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	return string(decoded), nil
}

// Hash Filters

// HashFilter generic hash filter
type HashFilter struct{}

func (f *HashFilter) Name() string { return "hash" }
func (f *HashFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("hash filter requires string input")
	}
	
	hashType := "sha256"
	if len(args) > 0 {
		hashType, _ = args[0].(string)
	}
	
	var h hash.Hash
	switch hashType {
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
		return nil, fmt.Errorf("unknown hash type: %s", hashType)
	}
	
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MD5Filter computes MD5 hash
type MD5Filter struct{}

func (f *MD5Filter) Name() string { return "md5" }
func (f *MD5Filter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("md5 filter requires string input")
	}
	h := md5.Sum([]byte(str))
	return hex.EncodeToString(h[:]), nil
}

// SHA1Filter computes SHA1 hash
type SHA1Filter struct{}

func (f *SHA1Filter) Name() string { return "sha1" }
func (f *SHA1Filter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("sha1 filter requires string input")
	}
	h := sha1.Sum([]byte(str))
	return hex.EncodeToString(h[:]), nil
}

// SHA256Filter computes SHA256 hash
type SHA256Filter struct{}

func (f *SHA256Filter) Name() string { return "sha256" }
func (f *SHA256Filter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("sha256 filter requires string input")
	}
	h := sha256.Sum256([]byte(str))
	return hex.EncodeToString(h[:]), nil
}

// SHA512Filter computes SHA512 hash
type SHA512Filter struct{}

func (f *SHA512Filter) Name() string { return "sha512" }
func (f *SHA512Filter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("sha512 filter requires string input")
	}
	h := sha512.Sum512([]byte(str))
	return hex.EncodeToString(h[:]), nil
}

// List Filters

// UniqueFilter removes duplicates from list
type UniqueFilter struct{}

func (f *UniqueFilter) Name() string { return "unique" }
func (f *UniqueFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case []string:
		seen := make(map[string]bool)
		result := []string{}
		for _, item := range v {
			if !seen[item] {
				seen[item] = true
				result = append(result, item)
			}
		}
		return result, nil
	case []interface{}:
		seen := make(map[string]bool)
		result := []interface{}{}
		for _, item := range v {
			key := fmt.Sprintf("%v", item)
			if !seen[key] {
				seen[key] = true
				result = append(result, item)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unique filter requires list input")
	}
}

// SortFilter sorts a list
type SortFilter struct{}

func (f *SortFilter) Name() string { return "sort" }
func (f *SortFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case []string:
		result := make([]string, len(v))
		copy(result, v)
		sort.Strings(result)
		return result, nil
	case []int:
		result := make([]int, len(v))
		copy(result, v)
		sort.Ints(result)
		return result, nil
	case []interface{}:
		// Convert to strings and sort
		strs := make([]string, len(v))
		for i, item := range v {
			strs[i] = fmt.Sprintf("%v", item)
		}
		sort.Strings(strs)
		result := make([]interface{}, len(strs))
		for i, s := range strs {
			result[i] = s
		}
		return result, nil
	default:
		return nil, fmt.Errorf("sort filter requires list input")
	}
}

// ReverseFilter reverses a list
type ReverseFilter struct{}

func (f *ReverseFilter) Name() string { return "reverse" }
func (f *ReverseFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case []string:
		result := make([]string, len(v))
		for i, item := range v {
			result[len(v)-1-i] = item
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[len(v)-1-i] = item
		}
		return result, nil
	default:
		return nil, fmt.Errorf("reverse filter requires list input")
	}
}

// FlattenFilter flattens nested lists
type FlattenFilter struct{}

func (f *FlattenFilter) Name() string { return "flatten" }
func (f *FlattenFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	depth := -1
	if len(args) > 0 {
		if d, ok := args[0].(int); ok {
			depth = d
		}
	}
	
	result := []interface{}{}
	f.flattenRecursive(input, &result, depth)
	return result, nil
}

func (f *FlattenFilter) flattenRecursive(input interface{}, result *[]interface{}, depth int) {
	if depth == 0 {
		*result = append(*result, input)
		return
	}
	
	switch v := input.(type) {
	case []interface{}:
		for _, item := range v {
			f.flattenRecursive(item, result, depth-1)
		}
	case []string:
		for _, item := range v {
			*result = append(*result, item)
		}
	default:
		*result = append(*result, input)
	}
}

// MinFilter finds minimum value
type MinFilter struct{}

func (f *MinFilter) Name() string { return "min" }
func (f *MinFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case []int:
		if len(v) == 0 {
			return nil, fmt.Errorf("min filter requires non-empty list")
		}
		min := v[0]
		for _, n := range v[1:] {
			if n < min {
				min = n
			}
		}
		return min, nil
	case []float64:
		if len(v) == 0 {
			return nil, fmt.Errorf("min filter requires non-empty list")
		}
		min := v[0]
		for _, n := range v[1:] {
			if n < min {
				min = n
			}
		}
		return min, nil
	default:
		return nil, fmt.Errorf("min filter requires numeric list")
	}
}

// MaxFilter finds maximum value
type MaxFilter struct{}

func (f *MaxFilter) Name() string { return "max" }
func (f *MaxFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case []int:
		if len(v) == 0 {
			return nil, fmt.Errorf("max filter requires non-empty list")
		}
		max := v[0]
		for _, n := range v[1:] {
			if n > max {
				max = n
			}
		}
		return max, nil
	case []float64:
		if len(v) == 0 {
			return nil, fmt.Errorf("max filter requires non-empty list")
		}
		max := v[0]
		for _, n := range v[1:] {
			if n > max {
				max = n
			}
		}
		return max, nil
	default:
		return nil, fmt.Errorf("max filter requires numeric list")
	}
}

// FirstFilter gets first element
type FirstFilter struct{}

func (f *FirstFilter) Name() string { return "first" }
func (f *FirstFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case []interface{}:
		if len(v) == 0 {
			return nil, nil
		}
		return v[0], nil
	case []string:
		if len(v) == 0 {
			return nil, nil
		}
		return v[0], nil
	case string:
		if len(v) == 0 {
			return "", nil
		}
		return string(v[0]), nil
	default:
		return nil, fmt.Errorf("first filter requires list or string input")
	}
}

// LastFilter gets last element
type LastFilter struct{}

func (f *LastFilter) Name() string { return "last" }
func (f *LastFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case []interface{}:
		if len(v) == 0 {
			return nil, nil
		}
		return v[len(v)-1], nil
	case []string:
		if len(v) == 0 {
			return nil, nil
		}
		return v[len(v)-1], nil
	case string:
		if len(v) == 0 {
			return "", nil
		}
		return string(v[len(v)-1]), nil
	default:
		return nil, fmt.Errorf("last filter requires list or string input")
	}
}

// LengthFilter gets length of list or string
type LengthFilter struct{}

func (f *LengthFilter) Name() string { return "length" }
func (f *LengthFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case []interface{}:
		return len(v), nil
	case []string:
		return len(v), nil
	case string:
		return len(v), nil
	case map[string]interface{}:
		return len(v), nil
	default:
		return 0, nil
	}
}

// SelectFilter selects items based on attribute
type SelectFilter struct{}

func (f *SelectFilter) Name() string { return "select" }
func (f *SelectFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("select filter requires attribute name")
	}
	
	// This is simplified - real implementation would evaluate expressions
	return input, nil
}

// RejectFilter rejects items based on attribute
type RejectFilter struct{}

func (f *RejectFilter) Name() string { return "reject" }
func (f *RejectFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("reject filter requires attribute name")
	}
	
	// This is simplified - real implementation would evaluate expressions
	return input, nil
}

// SelectAttrFilter selects attribute from items
type SelectAttrFilter struct{}

func (f *SelectAttrFilter) Name() string { return "selectattr" }
func (f *SelectAttrFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("selectattr filter requires attribute name")
	}
	
	attrName, _ := args[0].(string)
	
	switch v := input.(type) {
	case []map[string]interface{}:
		result := []interface{}{}
		for _, item := range v {
			if val, ok := item[attrName]; ok {
				result = append(result, val)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("selectattr filter requires list of dicts")
	}
}

// MapFilter applies attribute to all items
type MapFilter struct{}

func (f *MapFilter) Name() string { return "map" }
func (f *MapFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("map filter requires attribute name")
	}
	
	attrName, _ := args[0].(string)
	
	switch v := input.(type) {
	case []map[string]interface{}:
		result := []interface{}{}
		for _, item := range v {
			if val, ok := item[attrName]; ok {
				result = append(result, val)
			} else {
				result = append(result, nil)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("map filter requires list of dicts")
	}
}

// Dict Filters

// CombineFilter combines multiple dictionaries
type CombineFilter struct{}

func (f *CombineFilter) Name() string { return "combine" }
func (f *CombineFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	result := make(map[string]interface{})
	
	// Add input dict
	if inputDict, ok := input.(map[string]interface{}); ok {
		for k, v := range inputDict {
			result[k] = v
		}
	}
	
	// Add all argument dicts
	for _, arg := range args {
		if dict, ok := arg.(map[string]interface{}); ok {
			for k, v := range dict {
				result[k] = v
			}
		}
	}
	
	return result, nil
}

// DictToItemsFilter converts dict to list of key-value pairs
type DictToItemsFilter struct{}

func (f *DictToItemsFilter) Name() string { return "dict2items" }
func (f *DictToItemsFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	dict, ok := input.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("dict2items filter requires dict input")
	}
	
	result := []map[string]interface{}{}
	for k, v := range dict {
		result = append(result, map[string]interface{}{
			"key":   k,
			"value": v,
		})
	}
	
	return result, nil
}

// ItemsToDictFilter converts list of key-value pairs to dict
type ItemsToDictFilter struct{}

func (f *ItemsToDictFilter) Name() string { return "items2dict" }
func (f *ItemsToDictFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	items, ok := input.([]map[string]interface{})
	if !ok {
		// Try converting from []interface{}
		if itemsInterface, ok := input.([]interface{}); ok {
			items = []map[string]interface{}{}
			for _, item := range itemsInterface {
				if m, ok := item.(map[string]interface{}); ok {
					items = append(items, m)
				}
			}
		} else {
			return nil, fmt.Errorf("items2dict filter requires list of dicts")
		}
	}
	
	result := make(map[string]interface{})
	for _, item := range items {
		if key, ok := item["key"].(string); ok {
			result[key] = item["value"]
		}
	}
	
	return result, nil
}

// JSONQueryFilter queries JSON data using JMESPath-like syntax
type JSONQueryFilter struct{}

func (f *JSONQueryFilter) Name() string { return "json_query" }
func (f *JSONQueryFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("json_query filter requires query string")
	}
	
	query, _ := args[0].(string)
	
	// This is a simplified implementation
	// Real implementation would use a JMESPath library
	
	// Handle simple dot notation queries
	parts := strings.Split(query, ".")
	current := input
	
	for _, part := range parts {
		// Handle array index notation
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			// Extract field and index
			field := strings.Split(part, "[")[0]
			indexStr := strings.TrimSuffix(strings.Split(part, "[")[1], "]")
			
			// Navigate to field
			if field != "" {
				if dict, ok := current.(map[string]interface{}); ok {
					current = dict[field]
				} else {
					return nil, nil
				}
			}
			
			// Apply index
			if arr, ok := current.([]interface{}); ok {
				if index, err := strconv.Atoi(indexStr); err == nil && index < len(arr) {
					current = arr[index]
				} else {
					return nil, nil
				}
			} else {
				return nil, nil
			}
		} else {
			// Regular field navigation
			if dict, ok := current.(map[string]interface{}); ok {
				current = dict[part]
			} else {
				return nil, nil
			}
		}
	}
	
	return current, nil
}

// Type Conversion Filters

// IntFilter converts to integer
type IntFilter struct{}

func (f *IntFilter) Name() string { return "int" }
func (f *IntFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return nil, fmt.Errorf("cannot convert %T to int", input)
	}
}

// FloatFilter converts to float
type FloatFilter struct{}

func (f *FloatFilter) Name() string { return "float" }
func (f *FloatFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return nil, fmt.Errorf("cannot convert %T to float", input)
	}
}

// BoolFilter converts to boolean
type BoolFilter struct{}

func (f *BoolFilter) Name() string { return "bool" }
func (f *BoolFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int:
		return v != 0, nil
	case float64:
		return v != 0.0, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to bool", input)
	}
}

// StringFilter converts to string
type StringFilter struct{}

func (f *StringFilter) Name() string { return "string" }
func (f *StringFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	return fmt.Sprintf("%v", input), nil
}

// ListFilter converts to list
type ListFilter struct{}

func (f *ListFilter) Name() string { return "list" }
func (f *ListFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case []interface{}:
		return v, nil
	case []string:
		result := make([]interface{}, len(v))
		for i, s := range v {
			result[i] = s
		}
		return result, nil
	case string:
		// Split string into list
		return strings.Fields(v), nil
	default:
		// Wrap single item in list
		return []interface{}{input}, nil
	}
}

// DictFilter converts to dictionary
type DictFilter struct{}

func (f *DictFilter) Name() string { return "dict" }
func (f *DictFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case map[string]interface{}:
		return v, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to dict", input)
	}
}

// Date Filters

// DateFormatFilter formats date/time
type DateFormatFilter struct{}

func (f *DateFormatFilter) Name() string { return "strftime" }
func (f *DateFormatFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	format := "2006-01-02 15:04:05"
	if len(args) > 0 {
		if fmt, ok := args[0].(string); ok {
			// Convert Python strftime format to Go format
			format = convertStrftimeToGoFormat(fmt)
		}
	}
	
	var t time.Time
	switch v := input.(type) {
	case time.Time:
		t = v
	case int64:
		t = time.Unix(v, 0)
	case string:
		var err error
		t, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse time: %w", err)
		}
	default:
		return nil, fmt.Errorf("strftime filter requires time input")
	}
	
	return t.Format(format), nil
}

// ToDatetimeFilter converts to datetime
type ToDatetimeFilter struct{}

func (f *ToDatetimeFilter) Name() string { return "to_datetime" }
func (f *ToDatetimeFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	format := time.RFC3339
	if len(args) > 0 {
		if fmt, ok := args[0].(string); ok {
			format = convertStrftimeToGoFormat(fmt)
		}
	}
	
	switch v := input.(type) {
	case string:
		return time.Parse(format, v)
	case int64:
		return time.Unix(v, 0), nil
	default:
		return nil, fmt.Errorf("to_datetime filter requires string or int input")
	}
}

// IP Filters

// IPAddrFilter formats IP address
type IPAddrFilter struct{}

func (f *IPAddrFilter) Name() string { return "ipaddr" }
func (f *IPAddrFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("ipaddr filter requires string input")
	}
	
	// Parse IP or CIDR
	if strings.Contains(str, "/") {
		_, ipnet, err := net.ParseCIDR(str)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR: %w", err)
		}
		return ipnet.String(), nil
	} else {
		ip := net.ParseIP(str)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address")
		}
		return ip.String(), nil
	}
}

// IPWrapFilter wraps IPv6 addresses in brackets
type IPWrapFilter struct{}

func (f *IPWrapFilter) Name() string { return "ipwrap" }
func (f *IPWrapFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("ipwrap filter requires string input")
	}
	
	ip := net.ParseIP(str)
	if ip == nil {
		return str, nil
	}
	
	if ip.To4() == nil {
		// IPv6
		return "[" + str + "]", nil
	}
	
	return str, nil
}

// IPv4Filter checks if IP is IPv4
type IPv4Filter struct{}

func (f *IPv4Filter) Name() string { return "ipv4" }
func (f *IPv4Filter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return false, nil
	}
	
	ip := net.ParseIP(str)
	if ip == nil {
		return false, nil
	}
	
	return ip.To4() != nil, nil
}

// IPv6Filter checks if IP is IPv6
type IPv6Filter struct{}

func (f *IPv6Filter) Name() string { return "ipv6" }
func (f *IPv6Filter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return false, nil
	}
	
	ip := net.ParseIP(str)
	if ip == nil {
		return false, nil
	}
	
	return ip.To4() == nil && ip.To16() != nil, nil
}

// Path Filters

// BasenameFilter gets base name of path
type BasenameFilter struct{}

func (f *BasenameFilter) Name() string { return "basename" }
func (f *BasenameFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("basename filter requires string input")
	}
	return strings.TrimSuffix(strings.TrimPrefix(str, strings.TrimSuffix(str, "/")+"/"), "/"), nil
}

// DirnameFilter gets directory name of path
type DirnameFilter struct{}

func (f *DirnameFilter) Name() string { return "dirname" }
func (f *DirnameFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("dirname filter requires string input")
	}
	
	lastSlash := strings.LastIndex(str, "/")
	if lastSlash == -1 {
		return ".", nil
	}
	return str[:lastSlash], nil
}

// ExpanduserFilter expands ~ in path
type ExpanduserFilter struct{}

func (f *ExpanduserFilter) Name() string { return "expanduser" }
func (f *ExpanduserFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("expanduser filter requires string input")
	}
	
	if strings.HasPrefix(str, "~/") {
		home := os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("USERPROFILE") // Windows
		}
		return home + str[1:], nil
	}
	
	return str, nil
}

// RealpathFilter gets real path
type RealpathFilter struct{}

func (f *RealpathFilter) Name() string { return "realpath" }
func (f *RealpathFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("realpath filter requires string input")
	}
	
	// This would use filepath.Abs in real implementation
	return str, nil
}

// JSON/YAML Filters

// ToJSONFilter converts to JSON
type ToJSONFilter struct{}

func (f *ToJSONFilter) Name() string { return "to_json" }
func (f *ToJSONFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	pretty := false
	if len(args) > 0 {
		pretty, _ = args[0].(bool)
	}
	
	if pretty {
		data, err := json.MarshalIndent(input, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
		}
		return string(data), nil
	}
	
	data, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	return string(data), nil
}

// FromJSONFilter parses JSON
type FromJSONFilter struct{}

func (f *FromJSONFilter) Name() string { return "from_json" }
func (f *FromJSONFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	str, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("from_json filter requires string input")
	}
	
	var result interface{}
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	return result, nil
}

// ToYAMLFilter converts to YAML
type ToYAMLFilter struct{}

func (f *ToYAMLFilter) Name() string { return "to_yaml" }
func (f *ToYAMLFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	// This would use a YAML library in real implementation
	return fmt.Sprintf("%v", input), nil
}

// FromYAMLFilter parses YAML
type FromYAMLFilter struct{}

func (f *FromYAMLFilter) Name() string { return "from_yaml" }
func (f *FromYAMLFilter) Filter(input interface{}, args ...interface{}) (interface{}, error) {
	// This would use a YAML library in real implementation
	return input, nil
}

// Helper function to convert Python strftime format to Go format
func convertStrftimeToGoFormat(format string) string {
	replacements := map[string]string{
		"%Y": "2006",
		"%m": "01",
		"%d": "02",
		"%H": "15",
		"%M": "04",
		"%S": "05",
		"%y": "06",
		"%b": "Jan",
		"%B": "January",
		"%a": "Mon",
		"%A": "Monday",
	}
	
	result := format
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}
	
	return result
}

// ChainFilters applies multiple filters in sequence
func ChainFilters(fm *FilterManager, input interface{}, filters ...string) (interface{}, error) {
	current := input
	
	for _, filterExpr := range filters {
		parts := strings.SplitN(filterExpr, ":", 2)
		filterName := parts[0]
		
		var args []interface{}
		if len(parts) > 1 {
			// Parse arguments (simplified)
			argStrs := strings.Split(parts[1], ",")
			for _, arg := range argStrs {
				args = append(args, strings.TrimSpace(arg))
			}
		}
		
		result, err := fm.Apply(filterName, current, args...)
		if err != nil {
			return nil, fmt.Errorf("filter '%s' failed: %w", filterName, err)
		}
		
		current = result
	}
	
	return current, nil
}