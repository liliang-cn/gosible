package modules

import (
	"context"
	"fmt"
	"strings"

	"github.com/liliang-cn/gosible/pkg/types"
)

// DebugModule implements the debug module for displaying messages
type DebugModule struct {
	*BaseModule
}

// NewDebugModule creates a new debug module
func NewDebugModule() *DebugModule {
	doc := types.ModuleDoc{
		Name:        "debug",
		Description: "Print statements during execution",
		Parameters: map[string]types.ParamDoc{
			"msg": {
				Description: "The customized message that is printed",
				Required:    false,
				Type:        "string",
			},
			"var": {
				Description: "A variable name to debug",
				Required:    false,
				Type:        "string",
			},
			"verbosity": {
				Description: "A number that controls when the debug is run, if you set to 3 it will only run debug when -vvv or above",
				Required:    false,
				Type:        "int",
				Default:     0,
			},
		},
		Examples: []string{
			`- name: Print a simple message
  debug:
    msg: "System {{ inventory_hostname }} has uuid {{ ansible_product_uuid }}"`,
			`- name: Print a variable
  debug:
    var: result`,
			`- name: Print message with verbosity
  debug:
    msg: "Debug message"
    verbosity: 2`,
		},
		Returns: map[string]string{
			"msg": "The message that was displayed",
		},
	}

	return &DebugModule{
		BaseModule: NewBaseModule("debug", doc),
	}
}

// Validate validates the module arguments
func (m *DebugModule) Validate(args map[string]interface{}) error {
	msg := m.GetStringArg(args, "msg", "")
	varName := m.GetStringArg(args, "var", "")

	// Either msg or var must be provided, but not both
	if msg == "" && varName == "" {
		return types.NewValidationError("msg/var", nil, "either msg or var must be provided")
	}

	if msg != "" && varName != "" {
		return types.NewValidationError("msg/var", nil, "msg and var are mutually exclusive")
	}

	// Validate field types
	fieldTypes := map[string]string{
		"msg":       "string",
		"var":       "string",
		"verbosity": "int",
	}
	return m.ValidateTypes(args, fieldTypes)
}

// Run executes the debug module
func (m *DebugModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	return m.ExecuteWithTiming(ctx, conn, args, func() (*types.Result, error) {
		host := m.GetHostFromConnection(conn)

		// Get parameters
		msg := m.GetStringArg(args, "msg", "")
		varName := m.GetStringArg(args, "var", "")
		verbosity, _ := m.GetIntArg(args, "verbosity", 0)

		// For now, we'll ignore verbosity level checking as we don't have access to global verbosity settings
		_ = verbosity

		var displayMsg string
		var debugData map[string]interface{}

		if msg != "" {
			// Display custom message
			displayMsg = msg
			debugData = map[string]interface{}{
				"msg": msg,
			}
		} else if varName != "" {
			// Display variable - we need to get it from task variables
			// For now, we'll create a placeholder implementation
			varValue := m.getVariableValue(args, varName)
			displayMsg = fmt.Sprintf("%s: %s", varName, m.formatValue(varValue))
			debugData = map[string]interface{}{
				varName: varValue,
				"msg":   displayMsg,
			}
		}

		// Debug module always succeeds and never changes anything
		result := m.CreateSuccessResult(host, false, displayMsg, debugData)
		
		// Add debug-specific metadata
		result.Data["_debug"] = true
		result.Data["_verbosity"] = verbosity

		return result, nil
	})
}

// getVariableValue retrieves a variable value from the task context
func (m *DebugModule) getVariableValue(args map[string]interface{}, varName string) interface{} {
	// Try to get the variable from task vars (this would be passed by the runner)
	if taskVars, exists := args["_task_vars"]; exists {
		if vars, ok := taskVars.(map[string]interface{}); ok {
			if value, exists := vars[varName]; exists {
				return value
			}
		}
	}

	// Try to get from args directly (in case variable is passed as parameter)
	if value, exists := args[varName]; exists {
		return value
	}

	// Variable not found
	return fmt.Sprintf("VARIABLE IS NOT DEFINED: %s", varName)
}

// formatValue formats a value for display
func (m *DebugModule) formatValue(value interface{}) string {
	if value == nil {
		return "<null>"
	}

	switch v := value.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", v)
	case map[string]interface{}:
		return m.formatMap(v, 0)
	case []interface{}:
		return m.formatSlice(v, 0)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatMap formats a map for display with indentation
func (m *DebugModule) formatMap(data map[string]interface{}, indent int) string {
	if len(data) == 0 {
		return "{}"
	}

	var lines []string
	indentStr := strings.Repeat("  ", indent)
	nextIndentStr := strings.Repeat("  ", indent+1)

	lines = append(lines, "{")
	
	for key, value := range data {
		formattedValue := m.formatValueWithIndent(value, indent+1)
		lines = append(lines, fmt.Sprintf("%s\"%s\": %s,", nextIndentStr, key, formattedValue))
	}
	
	// Remove trailing comma from last item
	if len(lines) > 1 {
		lastIdx := len(lines) - 1
		lines[lastIdx] = strings.TrimSuffix(lines[lastIdx], ",")
	}
	
	lines = append(lines, indentStr+"}")
	
	return strings.Join(lines, "\n")
}

// formatSlice formats a slice for display with indentation
func (m *DebugModule) formatSlice(data []interface{}, indent int) string {
	if len(data) == 0 {
		return "[]"
	}

	var lines []string
	indentStr := strings.Repeat("  ", indent)
	nextIndentStr := strings.Repeat("  ", indent+1)

	lines = append(lines, "[")
	
	for _, item := range data {
		formattedValue := m.formatValueWithIndent(item, indent+1)
		lines = append(lines, fmt.Sprintf("%s%s,", nextIndentStr, formattedValue))
	}
	
	// Remove trailing comma from last item
	if len(lines) > 1 {
		lastIdx := len(lines) - 1
		lines[lastIdx] = strings.TrimSuffix(lines[lastIdx], ",")
	}
	
	lines = append(lines, indentStr+"]")
	
	return strings.Join(lines, "\n")
}

// formatValueWithIndent formats a value with proper indentation
func (m *DebugModule) formatValueWithIndent(value interface{}, indent int) string {
	if value == nil {
		return "null"
	}

	switch v := value.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", v)
	case map[string]interface{}:
		return m.formatMap(v, indent)
	case []interface{}:
		return m.formatSlice(v, indent)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%g", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}