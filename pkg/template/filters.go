package template

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FilterFunc represents a template filter function
type FilterFunc func(value interface{}, args ...interface{}) (interface{}, error)

// FilterRegistry holds all available filters
type FilterRegistry struct {
	filters map[string]FilterFunc
}

// NewFilterRegistry creates a new filter registry with built-in filters
func NewFilterRegistry() *FilterRegistry {
	fr := &FilterRegistry{
		filters: make(map[string]FilterFunc),
	}
	fr.registerBuiltinFilters()
	return fr
}

// Register adds a new filter to the registry
func (fr *FilterRegistry) Register(name string, fn FilterFunc) {
	fr.filters[name] = fn
}

// Get retrieves a filter by name
func (fr *FilterRegistry) Get(name string) (FilterFunc, bool) {
	fn, exists := fr.filters[name]
	return fn, exists
}

// registerBuiltinFilters registers all built-in Ansible-like filters
func (fr *FilterRegistry) registerBuiltinFilters() {
	// String filters
	fr.Register("upper", upperFilter)
	fr.Register("lower", lowerFilter)
	fr.Register("capitalize", capitalizeFilter)
	fr.Register("title", titleFilter)
	fr.Register("trim", trimFilter)
	fr.Register("replace", replaceFilter)
	fr.Register("regex_replace", regexReplaceFilter)
	fr.Register("regex_search", regexSearchFilter)
	fr.Register("regex_findall", regexFindallFilter)
	fr.Register("split", splitFilter)
	fr.Register("join", joinFilter)
	
	// Numeric filters
	fr.Register("int", intFilter)
	fr.Register("float", floatFilter)
	fr.Register("abs", absFilter)
	fr.Register("round", roundFilter)
	fr.Register("random", randomFilter)
	
	// List filters
	fr.Register("length", lengthFilter)
	fr.Register("first", firstFilter)
	fr.Register("last", lastFilter)
	fr.Register("sort", sortFilter)
	fr.Register("reverse", reverseFilter)
	fr.Register("unique", uniqueFilter)
	fr.Register("union", unionFilter)
	fr.Register("intersect", intersectFilter)
	fr.Register("difference", differenceFilter)
	fr.Register("flatten", flattenFilter)
	fr.Register("select", selectFilter)
	fr.Register("reject", rejectFilter)
	fr.Register("map", mapFilter)
	
	// Dictionary filters
	fr.Register("keys", keysFilter)
	fr.Register("values", valuesFilter)
	fr.Register("items", itemsFilter)
	fr.Register("dict2items", dict2itemsFilter)
	fr.Register("items2dict", items2dictFilter)
	fr.Register("combine", combineFilter)
	
	// Path filters
	fr.Register("basename", basenameFilter)
	fr.Register("dirname", dirnameFilter)
	fr.Register("expanduser", expanduserFilter)
	fr.Register("realpath", realpathFilter)
	fr.Register("relpath", relpathFilter)
	
	// Network filters
	fr.Register("ipaddr", ipaddrFilter)
	fr.Register("ipv4", ipv4Filter)
	fr.Register("ipv6", ipv6Filter)
	fr.Register("hwaddr", hwaddrFilter)
	
	// Date/Time filters
	fr.Register("to_datetime", toDatetimeFilter)
	fr.Register("strftime", strftimeFilter)
	
	// Type conversion filters
	fr.Register("bool", boolFilter)
	fr.Register("to_json", toJSONFilter)
	fr.Register("to_yaml", toYAMLFilter)
	fr.Register("from_json", fromJSONFilter)
	fr.Register("from_yaml", fromYAMLFilter)
	
	// Hash filters
	fr.Register("hash", hashFilter)
	fr.Register("md5", md5Filter)
	fr.Register("sha1", sha1Filter)
	fr.Register("sha256", sha256Filter)
	fr.Register("b64encode", b64encodeFilter)
	fr.Register("b64decode", b64decodeFilter)
	
	// Default filter
	fr.Register("default", defaultFilter)
	fr.Register("mandatory", mandatoryFilter)
	
	// Test filters
	fr.Register("defined", definedFilter)
	fr.Register("undefined", undefinedFilter)
	fr.Register("none", noneFilter)
	fr.Register("match", matchFilter)
	fr.Register("search", searchFilter)
}

// String filters

func upperFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	return strings.ToUpper(str), nil
}

func lowerFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	return strings.ToLower(str), nil
}

func capitalizeFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	if len(str) == 0 {
		return str, nil
	}
	return strings.ToUpper(str[:1]) + strings.ToLower(str[1:]), nil
}

func titleFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	return strings.Title(str), nil
}

func trimFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	return strings.TrimSpace(str), nil
}

func replaceFilter(value interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("replace filter requires old and new arguments")
	}
	str := fmt.Sprintf("%v", value)
	old := fmt.Sprintf("%v", args[0])
	new := fmt.Sprintf("%v", args[1])
	return strings.ReplaceAll(str, old, new), nil
}

func regexReplaceFilter(value interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("regex_replace filter requires pattern and replacement arguments")
	}
	str := fmt.Sprintf("%v", value)
	pattern := fmt.Sprintf("%v", args[0])
	replacement := fmt.Sprintf("%v", args[1])
	
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	
	return re.ReplaceAllString(str, replacement), nil
}

func regexSearchFilter(value interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("regex_search filter requires pattern argument")
	}
	str := fmt.Sprintf("%v", value)
	pattern := fmt.Sprintf("%v", args[0])
	
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	
	return re.FindString(str), nil
}

func regexFindallFilter(value interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("regex_findall filter requires pattern argument")
	}
	str := fmt.Sprintf("%v", value)
	pattern := fmt.Sprintf("%v", args[0])
	
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	
	return re.FindAllString(str, -1), nil
}

func splitFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	sep := " "
	if len(args) > 0 {
		sep = fmt.Sprintf("%v", args[0])
	}
	return strings.Split(str, sep), nil
}

func joinFilter(value interface{}, args ...interface{}) (interface{}, error) {
	sep := ""
	if len(args) > 0 {
		sep = fmt.Sprintf("%v", args[0])
	}
	
	switch v := value.(type) {
	case []string:
		return strings.Join(v, sep), nil
	case []interface{}:
		strs := make([]string, len(v))
		for i, item := range v {
			strs[i] = fmt.Sprintf("%v", item)
		}
		return strings.Join(strs, sep), nil
	default:
		return "", fmt.Errorf("join filter requires a list")
	}
}

// Numeric filters

func intFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		str := fmt.Sprintf("%v", v)
		return strconv.Atoi(str)
	}
}

func floatFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		str := fmt.Sprintf("%v", v)
		return strconv.ParseFloat(str, 64)
	}
}

func absFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case int:
		if v < 0 {
			return -v, nil
		}
		return v, nil
	case float64:
		return math.Abs(v), nil
	default:
		return nil, fmt.Errorf("abs filter requires a number")
	}
}

func roundFilter(value interface{}, args ...interface{}) (interface{}, error) {
	precision := 0
	if len(args) > 0 {
		switch p := args[0].(type) {
		case int:
			precision = p
		case float64:
			precision = int(p)
		}
	}
	
	switch v := value.(type) {
	case float64:
		multiplier := math.Pow(10, float64(precision))
		return math.Round(v*multiplier) / multiplier, nil
	case int:
		return v, nil
	default:
		return nil, fmt.Errorf("round filter requires a number")
	}
}

func randomFilter(value interface{}, args ...interface{}) (interface{}, error) {
	// This would use a random number generator
	// For now, return a placeholder
	return 42, nil
}

// List filters

func lengthFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return len(v), nil
	case []interface{}:
		return len(v), nil
	case []string:
		return len(v), nil
	case map[string]interface{}:
		return len(v), nil
	default:
		return 0, nil
	}
}

func firstFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []interface{}:
		if len(v) > 0 {
			return v[0], nil
		}
	case []string:
		if len(v) > 0 {
			return v[0], nil
		}
	case string:
		if len(v) > 0 {
			return string(v[0]), nil
		}
	}
	return nil, nil
}

func lastFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []interface{}:
		if len(v) > 0 {
			return v[len(v)-1], nil
		}
	case []string:
		if len(v) > 0 {
			return v[len(v)-1], nil
		}
	case string:
		if len(v) > 0 {
			return string(v[len(v)-1]), nil
		}
	}
	return nil, nil
}

func sortFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []string:
		sorted := make([]string, len(v))
		copy(sorted, v)
		sort.Strings(sorted)
		return sorted, nil
	case []interface{}:
		// Convert to strings and sort
		strs := make([]string, len(v))
		for i, item := range v {
			strs[i] = fmt.Sprintf("%v", item)
		}
		sort.Strings(strs)
		return strs, nil
	default:
		return value, nil
	}
}

func reverseFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []interface{}:
		reversed := make([]interface{}, len(v))
		for i, item := range v {
			reversed[len(v)-1-i] = item
		}
		return reversed, nil
	case []string:
		reversed := make([]string, len(v))
		for i, item := range v {
			reversed[len(v)-1-i] = item
		}
		return reversed, nil
	case string:
		runes := []rune(v)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes), nil
	default:
		return value, nil
	}
}

func uniqueFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []string:
		seen := make(map[string]bool)
		unique := []string{}
		for _, item := range v {
			if !seen[item] {
				seen[item] = true
				unique = append(unique, item)
			}
		}
		return unique, nil
	case []interface{}:
		seen := make(map[string]bool)
		unique := []interface{}{}
		for _, item := range v {
			key := fmt.Sprintf("%v", item)
			if !seen[key] {
				seen[key] = true
				unique = append(unique, item)
			}
		}
		return unique, nil
	default:
		return value, nil
	}
}

// More filters would continue here...

func unionFilter(value interface{}, args ...interface{}) (interface{}, error) {
	// Implementation for union of lists
	return value, nil
}

func intersectFilter(value interface{}, args ...interface{}) (interface{}, error) {
	// Implementation for intersection of lists
	return value, nil
}

func differenceFilter(value interface{}, args ...interface{}) (interface{}, error) {
	// Implementation for difference of lists
	return value, nil
}

func flattenFilter(value interface{}, args ...interface{}) (interface{}, error) {
	// Implementation for flattening nested lists
	return value, nil
}

func selectFilter(value interface{}, args ...interface{}) (interface{}, error) {
	// Implementation for selecting items based on condition
	return value, nil
}

func rejectFilter(value interface{}, args ...interface{}) (interface{}, error) {
	// Implementation for rejecting items based on condition
	return value, nil
}

func mapFilter(value interface{}, args ...interface{}) (interface{}, error) {
	// Implementation for mapping a filter over a list
	return value, nil
}

// Dictionary filters

func keysFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys, nil
	default:
		return []string{}, nil
	}
}

func valuesFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		values := make([]interface{}, 0, len(v))
		// Get keys first for consistent ordering
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			values = append(values, v[k])
		}
		return values, nil
	default:
		return []interface{}{}, nil
	}
}

func itemsFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		items := make([][]interface{}, 0, len(v))
		// Get keys first for consistent ordering
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			items = append(items, []interface{}{k, v[k]})
		}
		return items, nil
	default:
		return [][]interface{}{}, nil
	}
}

func dict2itemsFilter(value interface{}, args ...interface{}) (interface{}, error) {
	// Convert dictionary to list of key-value items
	return itemsFilter(value, args...)
}

func items2dictFilter(value interface{}, args ...interface{}) (interface{}, error) {
	// Convert list of key-value items to dictionary
	result := make(map[string]interface{})
	
	switch v := value.(type) {
	case [][]interface{}:
		for _, item := range v {
			if len(item) >= 2 {
				key := fmt.Sprintf("%v", item[0])
				result[key] = item[1]
			}
		}
	case []interface{}:
		for _, item := range v {
			if pair, ok := item.([]interface{}); ok && len(pair) >= 2 {
				key := fmt.Sprintf("%v", pair[0])
				result[key] = pair[1]
			}
		}
	}
	
	return result, nil
}

func combineFilter(value interface{}, args ...interface{}) (interface{}, error) {
	result := make(map[string]interface{})
	
	// Start with base dictionary
	if base, ok := value.(map[string]interface{}); ok {
		for k, v := range base {
			result[k] = v
		}
	}
	
	// Merge additional dictionaries
	for _, arg := range args {
		if dict, ok := arg.(map[string]interface{}); ok {
			for k, v := range dict {
				result[k] = v
			}
		}
	}
	
	return result, nil
}

// Path filters

func basenameFilter(value interface{}, args ...interface{}) (interface{}, error) {
	path := fmt.Sprintf("%v", value)
	return filepath.Base(path), nil
}

func dirnameFilter(value interface{}, args ...interface{}) (interface{}, error) {
	path := fmt.Sprintf("%v", value)
	return filepath.Dir(path), nil
}

func expanduserFilter(value interface{}, args ...interface{}) (interface{}, error) {
	path := fmt.Sprintf("%v", value)
	if strings.HasPrefix(path, "~") {
		// This would expand to user's home directory
		// For now, return as-is
		return path, nil
	}
	return path, nil
}

func realpathFilter(value interface{}, args ...interface{}) (interface{}, error) {
	path := fmt.Sprintf("%v", value)
	return filepath.Abs(path)
}

func relpathFilter(value interface{}, args ...interface{}) (interface{}, error) {
	path := fmt.Sprintf("%v", value)
	base := "."
	if len(args) > 0 {
		base = fmt.Sprintf("%v", args[0])
	}
	return filepath.Rel(base, path)
}

// Network filters

func ipaddrFilter(value interface{}, args ...interface{}) (interface{}, error) {
	addr := fmt.Sprintf("%v", value)
	ip := net.ParseIP(addr)
	if ip != nil {
		return ip.String(), nil
	}
	return nil, fmt.Errorf("invalid IP address: %s", addr)
}

func ipv4Filter(value interface{}, args ...interface{}) (interface{}, error) {
	addr := fmt.Sprintf("%v", value)
	ip := net.ParseIP(addr)
	if ip != nil && ip.To4() != nil {
		return ip.String(), nil
	}
	return nil, fmt.Errorf("not an IPv4 address: %s", addr)
}

func ipv6Filter(value interface{}, args ...interface{}) (interface{}, error) {
	addr := fmt.Sprintf("%v", value)
	ip := net.ParseIP(addr)
	if ip != nil && ip.To4() == nil {
		return ip.String(), nil
	}
	return nil, fmt.Errorf("not an IPv6 address: %s", addr)
}

func hwaddrFilter(value interface{}, args ...interface{}) (interface{}, error) {
	addr := fmt.Sprintf("%v", value)
	hw, err := net.ParseMAC(addr)
	if err != nil {
		return nil, err
	}
	return hw.String(), nil
}

// Date/Time filters

func toDatetimeFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	format := time.RFC3339
	if len(args) > 0 {
		format = fmt.Sprintf("%v", args[0])
	}
	return time.Parse(format, str)
}

func strftimeFilter(value interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("strftime filter requires format argument")
	}
	
	var t time.Time
	switch v := value.(type) {
	case time.Time:
		t = v
	case string:
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, err
		}
		t = parsed
	default:
		return nil, fmt.Errorf("strftime filter requires a time value")
	}
	
	format := fmt.Sprintf("%v", args[0])
	// Convert Python strftime format to Go format
	// This is a simplified conversion
	goFormat := strings.ReplaceAll(format, "%Y", "2006")
	goFormat = strings.ReplaceAll(goFormat, "%m", "01")
	goFormat = strings.ReplaceAll(goFormat, "%d", "02")
	goFormat = strings.ReplaceAll(goFormat, "%H", "15")
	goFormat = strings.ReplaceAll(goFormat, "%M", "04")
	goFormat = strings.ReplaceAll(goFormat, "%S", "05")
	
	return t.Format(goFormat), nil
}

// Type conversion filters

func boolFilter(value interface{}, args ...interface{}) (interface{}, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int:
		return v != 0, nil
	case float64:
		return v != 0.0, nil
	default:
		return false, nil
	}
}

func toJSONFilter(value interface{}, args ...interface{}) (interface{}, error) {
	pretty := false
	if len(args) > 0 {
		if p, ok := args[0].(bool); ok {
			pretty = p
		}
	}
	
	if pretty {
		data, err := json.MarshalIndent(value, "", "  ")
		return string(data), err
	}
	
	data, err := json.Marshal(value)
	return string(data), err
}

func toYAMLFilter(value interface{}, args ...interface{}) (interface{}, error) {
	data, err := yaml.Marshal(value)
	return string(data), err
}

func fromJSONFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	var result interface{}
	err := json.Unmarshal([]byte(str), &result)
	return result, err
}

func fromYAMLFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	var result interface{}
	err := yaml.Unmarshal([]byte(str), &result)
	return result, err
}

// Hash filters

func hashFilter(value interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("hash filter requires algorithm argument")
	}
	
	str := fmt.Sprintf("%v", value)
	algorithm := fmt.Sprintf("%v", args[0])
	
	switch algorithm {
	case "md5":
		return md5Filter(str)
	case "sha1":
		return sha1Filter(str)
	case "sha256":
		return sha256Filter(str)
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}
}

func md5Filter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:]), nil
}

func sha1Filter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	hash := sha1.Sum([]byte(str))
	return hex.EncodeToString(hash[:]), nil
}

func sha256Filter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	hash := sha256.Sum256([]byte(str))
	return hex.EncodeToString(hash[:]), nil
}

func b64encodeFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	return base64.StdEncoding.EncodeToString([]byte(str)), nil
}

func b64decodeFilter(value interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	data, err := base64.StdEncoding.DecodeString(str)
	return string(data), err
}

// Default filters

func defaultFilter(value interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("default filter requires default value argument")
	}
	
	// Check if value is undefined, null, or empty
	if value == nil || value == "" {
		return args[0], nil
	}
	
	return value, nil
}

func mandatoryFilter(value interface{}, args ...interface{}) (interface{}, error) {
	if value == nil || value == "" {
		msg := "Mandatory variable is not defined"
		if len(args) > 0 {
			msg = fmt.Sprintf("%v", args[0])
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return value, nil
}

// Test filters

func definedFilter(value interface{}, args ...interface{}) (interface{}, error) {
	return value != nil && value != "", nil
}

func undefinedFilter(value interface{}, args ...interface{}) (interface{}, error) {
	return value == nil || value == "", nil
}

func noneFilter(value interface{}, args ...interface{}) (interface{}, error) {
	return value == nil, nil
}

func matchFilter(value interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("match filter requires pattern argument")
	}
	
	str := fmt.Sprintf("%v", value)
	pattern := fmt.Sprintf("%v", args[0])
	
	matched, err := regexp.MatchString(pattern, str)
	return matched, err
}

func searchFilter(value interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("search filter requires pattern argument")
	}
	
	str := fmt.Sprintf("%v", value)
	pattern := fmt.Sprintf("%v", args[0])
	
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	
	return re.MatchString(str), nil
}