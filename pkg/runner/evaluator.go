package runner

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	
	"github.com/liliang-cn/gosinble/pkg/types"
)

// ConditionEvaluator evaluates conditional expressions for tasks
type ConditionEvaluator struct {
	vars map[string]interface{}
}

// NewConditionEvaluator creates a new condition evaluator
func NewConditionEvaluator(vars map[string]interface{}) *ConditionEvaluator {
	return &ConditionEvaluator{
		vars: vars,
	}
}

// EvaluateWhen evaluates a when condition
func (e *ConditionEvaluator) EvaluateWhen(condition interface{}) (bool, error) {
	if condition == nil {
		return true, nil
	}
	
	switch v := condition.(type) {
	case bool:
		return v, nil
	case string:
		return e.evaluateStringCondition(v)
	case []interface{}:
		// All conditions in the list must be true (AND logic)
		for _, cond := range v {
			result, err := e.EvaluateWhen(cond)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported condition type: %T", condition)
	}
}

// EvaluateFailedWhen evaluates a failed_when condition
func (e *ConditionEvaluator) EvaluateFailedWhen(condition interface{}, result *types.Result) (bool, error) {
	if condition == nil {
		// Default: task fails if result.Success is false
		return !result.Success, nil
	}
	
	// Add result data to variables for evaluation
	evalVars := make(map[string]interface{})
	for k, v := range e.vars {
		evalVars[k] = v
	}
	evalVars["result"] = result
	evalVars["rc"] = result.Data["exit_code"]
	evalVars["stdout"] = result.Data["stdout"]
	evalVars["stderr"] = result.Data["stderr"]
	
	evaluator := NewConditionEvaluator(evalVars)
	return evaluator.EvaluateWhen(condition)
}

// EvaluateChangedWhen evaluates a changed_when condition
func (e *ConditionEvaluator) EvaluateChangedWhen(condition interface{}, result *types.Result) (bool, error) {
	if condition == nil {
		// Default: use module's reported changed status
		return result.Changed, nil
	}
	
	// Special case: false means never changed
	if condition == false {
		return false, nil
	}
	
	// Add result data to variables for evaluation
	evalVars := make(map[string]interface{})
	for k, v := range e.vars {
		evalVars[k] = v
	}
	evalVars["result"] = result
	evalVars["rc"] = result.Data["exit_code"]
	evalVars["stdout"] = result.Data["stdout"]
	evalVars["stderr"] = result.Data["stderr"]
	
	evaluator := NewConditionEvaluator(evalVars)
	return evaluator.EvaluateWhen(condition)
}

// evaluateStringCondition evaluates a string condition expression
func (e *ConditionEvaluator) evaluateStringCondition(condition string) (bool, error) {
	condition = strings.TrimSpace(condition)
	
	// Handle boolean literals
	if condition == "true" || condition == "True" || condition == "yes" {
		return true, nil
	}
	if condition == "false" || condition == "False" || condition == "no" {
		return false, nil
	}
	
	// Handle 'not' operator
	if strings.HasPrefix(condition, "not ") {
		subCondition := strings.TrimPrefix(condition, "not ")
		result, err := e.evaluateStringCondition(subCondition)
		return !result, err
	}
	
	// Handle 'and' operator
	if strings.Contains(condition, " and ") {
		parts := strings.Split(condition, " and ")
		for _, part := range parts {
			result, err := e.evaluateStringCondition(strings.TrimSpace(part))
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	}
	
	// Handle 'or' operator
	if strings.Contains(condition, " or ") {
		parts := strings.Split(condition, " or ")
		for _, part := range parts {
			result, err := e.evaluateStringCondition(strings.TrimSpace(part))
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	}
	
	// Handle comparison operators (order matters - check longer operators first)
	for _, op := range []string{" is defined", " is undefined", " not in ", " in ", "==", "!=", ">=", "<=", ">", "<"} {
		if strings.Contains(condition, op) {
			return e.evaluateComparison(condition, op)
		}
	}
	
	// Handle simple variable evaluation
	value := e.resolveVariable(condition)
	return e.toBool(value), nil
}

// evaluateComparison evaluates a comparison expression
func (e *ConditionEvaluator) evaluateComparison(condition, op string) (bool, error) {
	var left, right string
	
	switch op {
	case " is defined":
		varName := strings.TrimSpace(strings.TrimSuffix(condition, op))
		_, exists := e.getVariable(varName)
		return exists, nil
	case " is undefined":
		varName := strings.TrimSpace(strings.TrimSuffix(condition, op))
		_, exists := e.getVariable(varName)
		return !exists, nil
	case " in ":
		parts := strings.SplitN(condition, op, 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid 'in' expression: %s", condition)
		}
		needle := e.resolveVariable(strings.TrimSpace(parts[0]))
		haystack := e.resolveVariable(strings.TrimSpace(parts[1]))
		return e.contains(haystack, needle), nil
	case " not in ":
		parts := strings.SplitN(condition, op, 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid 'not in' expression: %s", condition)
		}
		needle := e.resolveVariable(strings.TrimSpace(parts[0]))
		haystack := e.resolveVariable(strings.TrimSpace(parts[1]))
		return !e.contains(haystack, needle), nil
	default:
		parts := strings.SplitN(condition, op, 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid comparison expression: %s", condition)
		}
		left = strings.TrimSpace(parts[0])
		right = strings.TrimSpace(parts[1])
	}
	
	leftVal := e.resolveVariable(left)
	rightVal := e.resolveVariable(right)
	
	switch op {
	case "==":
		return e.equals(leftVal, rightVal), nil
	case "!=":
		return !e.equals(leftVal, rightVal), nil
	case ">":
		return e.compare(leftVal, rightVal) > 0, nil
	case "<":
		return e.compare(leftVal, rightVal) < 0, nil
	case ">=":
		return e.compare(leftVal, rightVal) >= 0, nil
	case "<=":
		return e.compare(leftVal, rightVal) <= 0, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", op)
	}
}

// resolveVariable resolves a variable reference to its value
func (e *ConditionEvaluator) resolveVariable(expr string) interface{} {
	expr = strings.TrimSpace(expr)
	
	// Handle string literals
	if (strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'")) ||
	   (strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"")) {
		return expr[1 : len(expr)-1]
	}
	
	// Handle numeric literals
	if num, err := strconv.ParseInt(expr, 10, 64); err == nil {
		return num
	}
	if num, err := strconv.ParseFloat(expr, 64); err == nil {
		return num
	}
	
	// Handle variable references
	value, _ := e.getVariable(expr)
	return value
}

// getVariable gets a variable value, supporting dot notation
func (e *ConditionEvaluator) getVariable(name string) (interface{}, bool) {
	// Support dot notation for nested access
	parts := strings.Split(name, ".")
	current := e.vars
	
	for i, part := range parts {
		// Handle array index notation like items[0]
		if strings.Contains(part, "[") && strings.HasSuffix(part, "]") {
			arrayPart := part[:strings.Index(part, "[")]
			indexStr := part[strings.Index(part, "[")+1 : len(part)-1]
			
			val, exists := current[arrayPart]
			if !exists {
				return nil, false
			}
			
			// Convert to array/slice
			switch v := val.(type) {
			case []interface{}:
				index, err := strconv.Atoi(indexStr)
				if err != nil || index < 0 || index >= len(v) {
					return nil, false
				}
				if i == len(parts)-1 {
					return v[index], true
				}
				// Continue traversing
				if m, ok := v[index].(map[string]interface{}); ok {
					current = m
					continue
				}
				return nil, false
			default:
				return nil, false
			}
		}
		
		val, exists := current[part]
		if !exists {
			return nil, false
		}
		
		if i == len(parts)-1 {
			return val, true
		}
		
		// Continue traversing for nested objects
		if m, ok := val.(map[string]interface{}); ok {
			current = m
		} else {
			return nil, false
		}
	}
	
	return nil, false
}

// toBool converts a value to boolean
func (e *ConditionEvaluator) toBool(value interface{}) bool {
	if value == nil {
		return false
	}
	
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != "" && v != "false" && v != "False" && v != "no" && v != "0"
	case int, int64:
		return v != 0
	case float64:
		return v != 0.0
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	default:
		return false
	}
}

// equals checks if two values are equal
func (e *ConditionEvaluator) equals(a, b interface{}) bool {
	// Handle nil cases
	if a == nil || b == nil {
		return a == b
	}
	
	// Try direct comparison
	if reflect.DeepEqual(a, b) {
		return true
	}
	
	// Convert to strings for comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return aStr == bStr
}

// compare compares two values numerically
func (e *ConditionEvaluator) compare(a, b interface{}) int {
	aNum := e.toNumber(a)
	bNum := e.toNumber(b)
	
	if aNum < bNum {
		return -1
	} else if aNum > bNum {
		return 1
	}
	return 0
}

// toNumber converts a value to float64
func (e *ConditionEvaluator) toNumber(value interface{}) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case string:
		if num, err := strconv.ParseFloat(v, 64); err == nil {
			return num
		}
	}
	return 0
}

// contains checks if haystack contains needle
func (e *ConditionEvaluator) contains(haystack, needle interface{}) bool {
	switch h := haystack.(type) {
	case string:
		needleStr := fmt.Sprintf("%v", needle)
		return strings.Contains(h, needleStr)
	case []interface{}:
		for _, item := range h {
			if e.equals(item, needle) {
				return true
			}
		}
	case map[string]interface{}:
		needleStr := fmt.Sprintf("%v", needle)
		_, exists := h[needleStr]
		return exists
	}
	return false
}

// EvaluateLoopItems expands loop items for iteration
func (e *ConditionEvaluator) EvaluateLoopItems(loop interface{}) ([]interface{}, error) {
	if loop == nil {
		return nil, nil
	}
	
	switch v := loop.(type) {
	case []interface{}:
		return v, nil
	case string:
		// Resolve variable reference
		resolved := e.resolveVariable(v)
		if items, ok := resolved.([]interface{}); ok {
			return items, nil
		}
		// Handle range expressions like "1-5"
		if regexp.MustCompile(`^\d+-\d+$`).MatchString(v) {
			parts := strings.Split(v, "-")
			start, _ := strconv.Atoi(parts[0])
			end, _ := strconv.Atoi(parts[1])
			var items []interface{}
			for i := start; i <= end; i++ {
				items = append(items, i)
			}
			return items, nil
		}
		return []interface{}{resolved}, nil
	default:
		return []interface{}{v}, nil
	}
}