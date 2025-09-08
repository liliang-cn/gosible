package modules

import (
	"context"
	"fmt"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// BaseModule provides common functionality for all modules
type BaseModule struct {
	name string
	doc  types.ModuleDoc
	capabilities *types.ModuleCapability
}

// NewBaseModule creates a new base module
func NewBaseModule(name string, doc types.ModuleDoc) *BaseModule {
	return &BaseModule{
		name: name,
		doc:  doc,
		capabilities: types.DefaultCapabilities(),
	}
}

// Name returns the module name
func (m *BaseModule) Name() string {
	return m.name
}

// Documentation returns module documentation
func (m *BaseModule) Documentation() types.ModuleDoc {
	return m.doc
}

// ValidateRequired validates that required parameters are present
func (m *BaseModule) ValidateRequired(args map[string]interface{}, required []string) error {
	return types.ValidateRequiredFields(args, required)
}

// ValidateTypes validates parameter types
func (m *BaseModule) ValidateTypes(args map[string]interface{}, fieldTypes map[string]string) error {
	return types.ValidateFieldTypes(args, fieldTypes)
}

// GetStringArg gets a string argument with optional default
func (m *BaseModule) GetStringArg(args map[string]interface{}, key string, defaultValue string) string {
	if value, exists := args[key]; exists {
		return types.ConvertToString(value)
	}
	return defaultValue
}

// GetBoolArg gets a boolean argument with optional default
func (m *BaseModule) GetBoolArg(args map[string]interface{}, key string, defaultValue bool) bool {
	if value, exists := args[key]; exists {
		return types.ConvertToBool(value)
	}
	return defaultValue
}

// GetIntArg gets an integer argument with optional default
func (m *BaseModule) GetIntArg(args map[string]interface{}, key string, defaultValue int) (int, error) {
	if value, exists := args[key]; exists {
		return types.ConvertToInt(value)
	}
	return defaultValue, nil
}

// GetMapArg gets a map argument
func (m *BaseModule) GetMapArg(args map[string]interface{}, key string) map[string]interface{} {
	if value, exists := args[key]; exists {
		if mapValue, ok := value.(map[string]interface{}); ok {
			return mapValue
		}
	}
	return nil
}

// GetSliceArg gets a slice argument
func (m *BaseModule) GetSliceArg(args map[string]interface{}, key string) []interface{} {
	if value, exists := args[key]; exists {
		if sliceValue, ok := value.([]interface{}); ok {
			return sliceValue
		}
		// Handle single value as slice
		return []interface{}{value}
	}
	return nil
}

// CreateResult creates a standardized module result
func (m *BaseModule) CreateResult(host string, success bool, changed bool, message string, data map[string]interface{}, err error) *types.Result {
	now := time.Now()
	result := &types.Result{
		Host:       host,
		Success:    success,
		Changed:    changed,
		Message:    message,
		Data:       data,
		Error:      err,
		StartTime:  now,
		EndTime:    now,
		Duration:   0,
		ModuleName: m.name,
	}

	if data == nil {
		result.Data = make(map[string]interface{})
	}

	return result
}

// CreateSuccessResult creates a successful result
func (m *BaseModule) CreateSuccessResult(host string, changed bool, message string, data map[string]interface{}) *types.Result {
	return m.CreateResult(host, true, changed, message, data, nil)
}

// CreateFailureResult creates a failed result
func (m *BaseModule) CreateFailureResult(host string, message string, err error, data map[string]interface{}) *types.Result {
	return m.CreateResult(host, false, false, message, data, err)
}

// CreateErrorResult creates an error result with module error
func (m *BaseModule) CreateErrorResult(host string, message string, err error) *types.Result {
	moduleErr := types.NewModuleError(m.name, host, message, err)
	return m.CreateResult(host, false, false, message, nil, moduleErr)
}

// GenerateDiff generates a diff between before and after states
func (m *BaseModule) GenerateDiff(before, after string) *types.DiffResult {
	if before == after {
		return nil
	}
	
	return &types.DiffResult{
		Before:   before,
		After:    after,
		Prepared: true,
	}
}

// SetCapabilities sets the module capabilities
func (m *BaseModule) SetCapabilities(caps *types.ModuleCapability) {
	m.capabilities = caps
}

// Capabilities returns the module capabilities
func (m *BaseModule) Capabilities() *types.ModuleCapability {
	return m.capabilities
}

// RunWithModes wraps Run to handle check/diff modes
func (m *BaseModule) RunWithModes(ctx context.Context, module types.Module, conn types.Connection, args map[string]interface{}, opts types.ExecuteOptions) (*types.Result, error) {
	// Inject mode flags into args
	if opts.CheckMode {
		args["_check_mode"] = true
	}
	if opts.DiffMode {
		args["_diff"] = true
	}
	
	// Validate module supports requested modes
	if opts.CheckMode && !m.capabilities.CheckMode {
		return nil, fmt.Errorf("module %s does not support check mode", m.name)
	}
	if opts.DiffMode && !m.capabilities.DiffMode {
		return nil, fmt.Errorf("module %s does not support diff mode", m.name)
	}
	
	return module.Run(ctx, conn, args)
}

// ExecuteWithTiming wraps execution with timing information
func (m *BaseModule) ExecuteWithTiming(ctx context.Context, conn types.Connection, args map[string]interface{}, executeFunc func() (*types.Result, error)) (*types.Result, error) {
	startTime := time.Now()

	result, err := executeFunc()
	if err != nil {
		return result, err
	}

	endTime := time.Now()
	if result != nil {
		result.StartTime = startTime
		result.EndTime = endTime
		result.Duration = endTime.Sub(startTime)
	}

	return result, nil
}

// CheckMode determines if the module is running in check mode
func (m *BaseModule) CheckMode(args map[string]interface{}) bool {
	if value, exists := args["_check_mode"]; exists {
		// Only accept actual boolean values for check mode
		if boolValue, ok := value.(bool); ok {
			return boolValue
		}
	}
	return false
}

// DiffMode determines if the module should show diffs
func (m *BaseModule) DiffMode(args map[string]interface{}) bool {
	return m.GetBoolArg(args, "_diff", false)
}

// ExpandPath expands variables in a file path
func (m *BaseModule) ExpandPath(path string, vars map[string]interface{}) string {
	if vars == nil {
		return path
	}
	return types.ExpandVariables(path, vars)
}

// ValidateChoices validates that a parameter value is within allowed choices
func (m *BaseModule) ValidateChoices(args map[string]interface{}, param string, choices []string) error {
	if value, exists := args[param]; exists {
		strValue := types.ConvertToString(value)
		for _, choice := range choices {
			if strValue == choice {
				return nil
			}
		}
		return types.NewValidationError(param, value, fmt.Sprintf("value must be one of: %v", choices))
	}
	return nil
}

// ValidatePath validates and sanitizes a file path
func (m *BaseModule) ValidatePath(path string) (string, error) {
	if path == "" {
		return "", types.NewValidationError("path", path, "path cannot be empty")
	}

	sanitized := types.SanitizePath(path)
	if sanitized == "" {
		return "", types.NewValidationError("path", path, "invalid path")
	}

	return sanitized, nil
}

// GetHostFromConnection extracts host information from connection
func (m *BaseModule) GetHostFromConnection(conn types.Connection) string {
	// Try to get hostname from connection if it implements additional methods
	if hostProvider, ok := conn.(interface{ GetHostname() (string, error) }); ok {
		if hostname, err := hostProvider.GetHostname(); err == nil {
			return hostname
		}
	}

	// Fallback to a default value
	return "unknown"
}

// HandleTimeout handles command timeouts
func (m *BaseModule) HandleTimeout(ctx context.Context, timeout time.Duration, operation func(context.Context) (*types.Result, error)) (*types.Result, error) {
	if timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return operation(timeoutCtx)
	}
	return operation(ctx)
}

// LogDebug logs debug information (placeholder for future logging integration)
func (m *BaseModule) LogDebug(message string, args ...interface{}) {
	// TODO: Integrate with logging system when implemented
	_ = fmt.Sprintf(message, args...)
}

// LogInfo logs informational messages
func (m *BaseModule) LogInfo(message string, args ...interface{}) {
	// TODO: Integrate with logging system when implemented
	_ = fmt.Sprintf(message, args...)
}

// LogWarn logs warning messages
func (m *BaseModule) LogWarn(message string, args ...interface{}) {
	// TODO: Integrate with logging system when implemented
	_ = fmt.Sprintf(message, args...)
}

// LogError logs error messages
func (m *BaseModule) LogError(message string, args ...interface{}) {
	// TODO: Integrate with logging system when implemented
	_ = fmt.Sprintf(message, args...)
}

// ParseStateString parses state strings (present, absent, latest, etc.)
func (m *BaseModule) ParseStateString(state string) string {
	switch state {
	case "present", "installed", "enabled", "started", "running":
		return "present"
	case "absent", "removed", "uninstalled", "disabled", "stopped":
		return "absent"
	case "latest", "updated":
		return "latest"
	case "restarted", "reloaded":
		return state
	default:
		return "present" // default state
	}
}

// IsTruthy checks if a value is truthy (useful for conditions)
func (m *BaseModule) IsTruthy(value interface{}) bool {
	if value == nil {
		return false
	}
	return types.ConvertToBool(value)
}

// CreateCheckModeResult creates a result for check mode operations
func (m *BaseModule) CreateCheckModeResult(host string, changed bool, message string, data map[string]interface{}) *types.Result {
	result := m.CreateSuccessResult(host, changed, message, data)
	if result.Data == nil {
		result.Data = make(map[string]interface{})
	}
	result.Data["check_mode"] = true
	if changed {
		result.Data["would_change"] = true
	}
	result.Simulated = true  // Mark as simulated for check mode
	return result
}

// Retry executes an operation with retries
func (m *BaseModule) Retry(ctx context.Context, maxRetries int, backoff time.Duration, operation func() (*types.Result, error)) (*types.Result, error) {
	var lastResult *types.Result
	var lastError error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
				// Continue with retry
			case <-ctx.Done():
				return lastResult, ctx.Err()
			}
		}

		result, err := operation()
		if err == nil && result != nil && result.Success {
			return result, nil
		}

		lastResult = result
		lastError = err
		m.LogDebug("Module retry attempt %d/%d failed", attempt+1, maxRetries+1)
	}

	return lastResult, lastError
}