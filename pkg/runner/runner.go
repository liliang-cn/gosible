// Package runner provides the task execution engine with parallel execution capabilities.
package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/gosinble/gosinble/pkg/types"
	"github.com/gosinble/gosinble/pkg/connection"
	"github.com/gosinble/gosinble/pkg/modules"
	"github.com/gosinble/gosinble/pkg/vars"
)

// TaskRunner implements the Runner interface with parallel execution support
type TaskRunner struct {
	maxConcurrency   int
	moduleRegistry   *modules.ModuleRegistry
	connectionMgr    *connection.ConnectionManager
	varManager       *vars.VarManager
	handlerManager   *HandlerManager
	mu               sync.RWMutex
	connections      map[string]types.Connection
	connectionTTL    time.Duration
	tags             []string  // Tags to filter task execution
}

// NewTaskRunner creates a new task runner
func NewTaskRunner() *TaskRunner {
	return &TaskRunner{
		maxConcurrency: 5, // Default to 5 parallel executions
		moduleRegistry: modules.DefaultModuleRegistry,
		connectionMgr:  connection.DefaultConnectionManager,
		varManager:     vars.NewVarManager(),
		handlerManager: NewHandlerManager(),
		connections:    make(map[string]types.Connection),
		connectionTTL:  30 * time.Minute,
		tags:           []string{},
	}
}

// NewTaskRunnerWithDependencies creates a task runner with custom dependencies
func NewTaskRunnerWithDependencies(moduleRegistry *modules.ModuleRegistry, connectionMgr *connection.ConnectionManager, varMgr *vars.VarManager) *TaskRunner {
	return &TaskRunner{
		maxConcurrency: 5,
		moduleRegistry: moduleRegistry,
		connectionMgr:  connectionMgr,
		varManager:     varMgr,
		handlerManager: NewHandlerManager(),
		connections:    make(map[string]types.Connection),
		connectionTTL:  30 * time.Minute,
		tags:           []string{},
	}
}

// SetMaxConcurrency sets the maximum number of concurrent executions
func (r *TaskRunner) SetMaxConcurrency(max int) {
	if max <= 0 {
		max = 1
	}
	r.maxConcurrency = max
}

// SetTags sets the tags for filtering task execution
func (r *TaskRunner) SetTags(tags []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tags = tags
}

// GetHandlerManager returns the handler manager
func (r *TaskRunner) GetHandlerManager() *HandlerManager {
	return r.handlerManager
}

// shouldRunTask checks if a task should run based on tags
func (r *TaskRunner) shouldRunTask(task types.Task) bool {
	// If no tags are set, run all tasks
	if len(r.tags) == 0 {
		return true
	}
	
	// If task has no tags, skip it when tags are specified
	if len(task.Tags) == 0 {
		// Unless 'always' or 'all' tag is specified
		for _, tag := range r.tags {
			if tag == "always" || tag == "all" {
				return true
			}
		}
		return false
	}
	
	// Check if any task tag matches runner tags
	for _, taskTag := range task.Tags {
		// Always run tasks with 'always' tag
		if taskTag == "always" {
			return true
		}
		
		for _, runnerTag := range r.tags {
			if taskTag == runnerTag {
				return true
			}
		}
	}
	
	return false
}

// RegisterModule registers a module for use
func (r *TaskRunner) RegisterModule(module types.Module) error {
	return r.moduleRegistry.RegisterModule(module)
}

// GetModule returns a module by name
func (r *TaskRunner) GetModule(name string) (types.Module, error) {
	return r.moduleRegistry.GetModule(name)
}

// Run executes a single task on specified hosts
func (r *TaskRunner) Run(ctx context.Context, task types.Task, hosts []types.Host, vars map[string]interface{}) ([]types.Result, error) {
	if len(hosts) == 0 {
		return []types.Result{}, nil
	}

	// Check if task should be skipped based on tags
	if !r.shouldRunTask(task) {
		// Skip task due to tags
		results := make([]types.Result, len(hosts))
		for i, host := range hosts {
			results[i] = types.Result{
				Host:       host.Name,
				Success:    true,
				Changed:    false,
				Message:    "Skipped due to tags",
				Data:       map[string]interface{}{"skipped": true},
				TaskName:   task.Name,
				ModuleName: task.Module.String(),
				StartTime:  types.GetCurrentTime(),
				EndTime:    types.GetCurrentTime(),
			}
		}
		return results, nil
	}

	// Merge task vars with provided vars
	mergedVars := make(map[string]interface{})
	for k, v := range vars {
		mergedVars[k] = v
	}
	if task.Vars != nil {
		for k, v := range task.Vars {
			mergedVars[k] = v
		}
	}

	// Evaluate when condition
	if task.When != nil {
		evaluator := NewConditionEvaluator(mergedVars)
		shouldRun, err := evaluator.EvaluateWhen(task.When)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate when condition: %w", err)
		}
		if !shouldRun {
			// Skip task - return success results with skipped flag
			results := make([]types.Result, len(hosts))
			for i, host := range hosts {
				results[i] = types.Result{
					Host:       host.Name,
					Success:    true,
					Changed:    false,
					Message:    "Skipped due to when condition",
					Data:       map[string]interface{}{"skipped": true},
					TaskName:   task.Name,
					ModuleName: task.Module.String(),
					StartTime:  types.GetCurrentTime(),
					EndTime:    types.GetCurrentTime(),
				}
			}
			return results, nil
		}
	}

	// Get the module
	module, err := r.GetModule(task.Module.String())
	if err != nil {
		return nil, fmt.Errorf("module %s not found: %w", task.Module, err)
	}

	// Validate module arguments
	if err := module.Validate(task.Args); err != nil {
		return nil, fmt.Errorf("module validation failed: %w", err)
	}

	// Handle loops
	var results []types.Result
	if task.Loop != nil || task.WithItems != nil {
		results, err = r.executeWithLoop(ctx, task, module, hosts, mergedVars)
	} else {
		// Execute task on all hosts with controlled concurrency
		results, err = r.executeOnHosts(ctx, task, module, hosts, mergedVars)
	}
	
	if err != nil {
		return results, err
	}
	
	// Handle notifications if task changed something
	if task.Notify != nil && len(task.Notify) > 0 {
		// Check if any result shows a change
		hasChanges := false
		for _, result := range results {
			if result.Changed {
				hasChanges = true
				break
			}
		}
		
		if hasChanges && r.handlerManager != nil {
			r.handlerManager.Notify(task.Notify)
		}
	}
	
	return results, nil
}

// executeOnHosts executes a task on multiple hosts with parallel execution
func (r *TaskRunner) executeOnHosts(ctx context.Context, task types.Task, module types.Module, hosts []types.Host, vars map[string]interface{}) ([]types.Result, error) {
	results := make([]types.Result, len(hosts))
	
	// Use errgroup to control concurrency and handle errors
	g, ctx := errgroup.WithContext(ctx)
	
	// Create a semaphore to limit concurrency
	sem := make(chan struct{}, r.maxConcurrency)
	
	for i, host := range hosts {
		i, host := i, host // Capture loop variables
		
		g.Go(func() error {
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()
			
			// Execute task on this host
			result, err := r.executeOnHost(ctx, task, module, host, vars)
			if err != nil {
				// Don't fail the entire operation for individual host errors
				// Store the error in the result
				result = &types.Result{
					Host:       host.Name,
					Success:    false,
					Changed:    false,
					Error:      err,
					Message:    fmt.Sprintf("Task execution failed: %v", err),
					StartTime:  types.GetCurrentTime(),
					EndTime:    types.GetCurrentTime(),
					TaskName:   task.Name,
					ModuleName: task.Module.String(),
					Data:       make(map[string]interface{}),
				}
			}
			
			results[i] = *result
			return nil
		})
	}
	
	// Wait for all tasks to complete
	if err := g.Wait(); err != nil {
		return results, err
	}
	
	return results, nil
}

// executeOnHost executes a task on a single host
func (r *TaskRunner) executeOnHost(ctx context.Context, task types.Task, module types.Module, host types.Host, vars map[string]interface{}) (*types.Result, error) {
	// Get or create connection to host
	conn, err := r.getConnection(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to host %s: %w", host.Name, err)
	}

	// Merge host variables with task variables
	hostVars, err := r.getHostVariables(host, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to get host variables: %w", err)
	}

	// Set environment variables if specified
	if task.Environment != nil {
		for k, v := range task.Environment {
			hostVars["ansible_env_"+k] = v
		}
	}

	// Expand variables in task arguments
	expandedArgs := r.expandTaskArguments(task.Args, hostVars)

	// Create task with expanded arguments and host variables
	expandedTask := task
	expandedTask.Args = expandedArgs

	// Add special variables
	moduleArgs := make(map[string]interface{})
	for k, v := range expandedArgs {
		moduleArgs[k] = v
	}
	
	// Add meta variables for check mode, diff mode, etc.
	if checkMode, exists := hostVars["_check_mode"]; exists {
		moduleArgs["_check_mode"] = checkMode
	}
	if diffMode, exists := hostVars["_diff"]; exists {
		moduleArgs["_diff"] = diffMode
	}
	
	// Add task variables to module args for access
	moduleArgs["_task_vars"] = hostVars

	// Handle retries if specified
	maxRetries := 1
	if task.Retries > 0 {
		maxRetries = task.Retries + 1
	}
	
	var result *types.Result
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 && task.Delay > 0 {
			time.Sleep(time.Duration(task.Delay) * time.Second)
		}
		
		// Execute the module
		result, err = module.Run(ctx, conn, moduleArgs)
		if err != nil && task.IgnoreErrors {
			// Convert error to result with success = false
			result = &types.Result{
				Host:       host.Name,
				Success:    false,
				Changed:    false,
				Error:      err,
				Message:    fmt.Sprintf("Error (ignored): %v", err),
				TaskName:   task.Name,
				ModuleName: task.Module.String(),
				StartTime:  types.GetCurrentTime(),
				EndTime:    types.GetCurrentTime(),
				Data:       make(map[string]interface{}),
			}
			err = nil
		} else if err != nil {
			return result, err
		}
		
		// Set the task name and host in the result
		if result != nil {
			result.TaskName = task.Name
			result.Host = host.Name
		}
		
		// Evaluate changed_when condition
		if task.ChangedWhen != nil {
			evaluator := NewConditionEvaluator(hostVars)
			changed, evalErr := evaluator.EvaluateChangedWhen(task.ChangedWhen, result)
			if evalErr != nil {
				return nil, fmt.Errorf("failed to evaluate changed_when: %w", evalErr)
			}
			result.Changed = changed
		}
		
		// Evaluate failed_when condition
		if task.FailedWhen != nil {
			evaluator := NewConditionEvaluator(hostVars)
			failed, evalErr := evaluator.EvaluateFailedWhen(task.FailedWhen, result)
			if evalErr != nil {
				return nil, fmt.Errorf("failed to evaluate failed_when: %w", evalErr)
			}
			result.Success = !failed
			if failed && !task.IgnoreErrors {
				result.Error = fmt.Errorf("task failed due to failed_when condition")
			}
		}
		
		// Check until condition for retries
		if task.Until != nil && attempt < maxRetries-1 {
			evaluator := NewConditionEvaluator(hostVars)
			// Add result to vars for until evaluation
			hostVars["result"] = result
			success, evalErr := evaluator.EvaluateWhen(task.Until)
			if evalErr != nil {
				return nil, fmt.Errorf("failed to evaluate until condition: %w", evalErr)
			}
			if success {
				break // Condition met, stop retrying
			}
			// Continue to next retry
		} else if !result.Success && attempt < maxRetries-1 {
			// Retry on failure if retries are configured
			continue
		} else {
			break
		}
	}
	
	// Register result if specified
	if task.Register != "" && r.varManager != nil {
		r.varManager.SetVar(task.Register, result)
	}
	
	return result, nil
}

// executeWithLoop executes a task with loop iterations
func (r *TaskRunner) executeWithLoop(ctx context.Context, task types.Task, module types.Module, hosts []types.Host, vars map[string]interface{}) ([]types.Result, error) {
	// Determine loop items
	var loopItems interface{}
	if task.Loop != nil {
		loopItems = task.Loop
	} else if task.WithItems != nil {
		loopItems = task.WithItems
	}
	
	// Evaluate loop items
	evaluator := NewConditionEvaluator(vars)
	items, err := evaluator.EvaluateLoopItems(loopItems)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate loop items: %w", err)
	}
	
	if len(items) == 0 {
		return []types.Result{}, nil
	}
	
	// Collect all results
	var allResults []types.Result
	
	// Get loop control settings
	var loopVar string = "item"
	var indexVar string = ""
	if task.LoopControl != nil {
		if lv, ok := task.LoopControl["loop_var"].(string); ok {
			loopVar = lv
		}
		if iv, ok := task.LoopControl["index_var"].(string); ok {
			indexVar = iv
		}
	}
	
	// Execute task for each item
	for index, item := range items {
		// Create vars with loop item
		loopVars := make(map[string]interface{})
		for k, v := range vars {
			loopVars[k] = v
		}
		loopVars[loopVar] = item
		if indexVar != "" {
			loopVars[indexVar] = index
		}
		
		// Execute task with loop vars
		results, err := r.executeOnHosts(ctx, task, module, hosts, loopVars)
		if err != nil {
			return allResults, err
		}
		
		// Add loop info to results
		for i := range results {
			results[i].Data["ansible_loop"] = map[string]interface{}{
				"index":     index,
				"index0":    index,
				"index1":    index + 1,
				"first":     index == 0,
				"last":      index == len(items)-1,
				"length":    len(items),
				loopVar:     item,
			}
			allResults = append(allResults, results[i])
		}
	}
	
	return allResults, nil
}

// getConnection gets or creates a connection to a host
func (r *TaskRunner) getConnection(ctx context.Context, host types.Host) (types.Connection, error) {
	r.mu.RLock()
	if conn, exists := r.connections[host.Name]; exists && conn.IsConnected() {
		r.mu.RUnlock()
		return conn, nil
	}
	r.mu.RUnlock()

	// Create connection info
	connInfo := types.ConnectionInfo{
		Type:       "ssh", // Default connection type
		Host:       host.Address,
		Port:       host.Port,
		User:       host.User,
		Password:   host.Password,
		Timeout:    30 * time.Second,
		Variables:  host.Variables,
	}

	// Override with localhost for local connections
	if host.Address == "localhost" || host.Address == "127.0.0.1" {
		connInfo.Type = "local"
	}

	// Create connection
	conn, err := r.connectionMgr.GetConnection(ctx, connInfo)
	if err != nil {
		return nil, err
	}

	// Store connection for reuse
	r.mu.Lock()
	r.connections[host.Name] = conn
	r.mu.Unlock()

	return conn, nil
}

// getHostVariables gets all variables for a host
func (r *TaskRunner) getHostVariables(host types.Host, taskVars map[string]interface{}) (map[string]interface{}, error) {
	// Start with task variables
	result := make(map[string]interface{})
	if taskVars != nil {
		result = types.DeepMergeInterfaceMaps(result, taskVars)
	}

	// Add host variables
	if host.Variables != nil {
		result = types.DeepMergeInterfaceMaps(result, host.Variables)
	}

	// Add built-in host variables
	result["inventory_hostname"] = host.Name
	result["inventory_hostname_short"] = host.Name
	result["ansible_host"] = host.Address
	if host.Port > 0 {
		result["ansible_port"] = host.Port
	}
	if host.User != "" {
		result["ansible_user"] = host.User
	}

	return result, nil
}

// expandTaskArguments expands variables in task arguments
func (r *TaskRunner) expandTaskArguments(args map[string]interface{}, vars map[string]interface{}) map[string]interface{} {
	expanded := make(map[string]interface{})
	
	for key, value := range args {
		expanded[key] = r.expandValue(value, vars)
	}
	
	return expanded
}

// expandValue recursively expands variables in a value
func (r *TaskRunner) expandValue(value interface{}, vars map[string]interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return types.ExpandVariables(v, vars)
	case map[string]interface{}:
		expanded := make(map[string]interface{})
		for k, val := range v {
			expanded[k] = r.expandValue(val, vars)
		}
		return expanded
	case []interface{}:
		expanded := make([]interface{}, len(v))
		for i, val := range v {
			expanded[i] = r.expandValue(val, vars)
		}
		return expanded
	default:
		return value
	}
}

// RunPlay executes a play
func (r *TaskRunner) RunPlay(ctx context.Context, play types.Play, inventory types.Inventory, vars map[string]interface{}) ([]types.Result, error) {
	// This would use the playbook executor, but to avoid circular dependencies,
	// we'll implement a simplified version here for now
	return r.executePlay(ctx, play, inventory, vars)
}

// RunPlaybook executes a complete playbook
func (r *TaskRunner) RunPlaybook(ctx context.Context, playbook types.Playbook, inventory types.Inventory, vars map[string]interface{}) ([]types.Result, error) {
	var allResults []types.Result
	
	for _, play := range playbook.Plays {
		results, err := r.RunPlay(ctx, play, inventory, vars)
		if err != nil {
			return allResults, err
		}
		allResults = append(allResults, results...)
	}
	
	return allResults, nil
}

// executePlay is a simplified play execution for the runner
func (r *TaskRunner) executePlay(ctx context.Context, play types.Play, inventory types.Inventory, vars map[string]interface{}) ([]types.Result, error) {
	// Get hosts for the play
	hosts, err := r.getPlayHosts(ctx, play, inventory)
	if err != nil {
		return nil, err
	}
	
	var allResults []types.Result
	
	// Execute tasks
	for _, task := range play.Tasks {
		results, err := r.Run(ctx, task, hosts, vars)
		if err != nil {
			return allResults, err
		}
		allResults = append(allResults, results...)
	}
	
	return allResults, nil
}

// getPlayHosts gets hosts for a play
func (r *TaskRunner) getPlayHosts(ctx context.Context, play types.Play, inventory types.Inventory) ([]types.Host, error) {
	switch hosts := play.Hosts.(type) {
	case string:
		return inventory.GetHosts(hosts)
	case []interface{}:
		var allHosts []types.Host
		for _, h := range hosts {
			if hostStr, ok := h.(string); ok {
				playHosts, err := inventory.GetHosts(hostStr)
				if err != nil {
					return nil, err
				}
				allHosts = append(allHosts, playHosts...)
			}
		}
		return allHosts, nil
	default:
		return nil, fmt.Errorf("invalid hosts format in play")
	}
}

// Close closes all connections
func (r *TaskRunner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for hostName, conn := range r.connections {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
		delete(r.connections, hostName)
	}

	return lastErr
}

// GetConnectionCount returns the number of active connections
func (r *TaskRunner) GetConnectionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.connections)
}

// CleanupStaleConnections removes connections that are no longer needed
func (r *TaskRunner) CleanupStaleConnections() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for hostName, conn := range r.connections {
		if !conn.IsConnected() {
			conn.Close()
			delete(r.connections, hostName)
		}
	}
}

// SetConnectionTTL sets the connection time-to-live
func (r *TaskRunner) SetConnectionTTL(ttl time.Duration) {
	r.connectionTTL = ttl
}

// GetStats returns execution statistics
func (r *TaskRunner) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"max_concurrency":     r.maxConcurrency,
		"active_connections":  len(r.connections),
		"registered_modules":  len(r.moduleRegistry.ListModules()),
		"connection_ttl_mins": int(r.connectionTTL.Minutes()),
	}
}

// ExecuteTask is a helper method for executing a single task
func (r *TaskRunner) ExecuteTask(ctx context.Context, taskName, moduleName string, args map[string]interface{}, hosts []types.Host, vars map[string]interface{}) ([]types.Result, error) {
	task := types.Task{
		Name:   taskName,
		Module: types.ModuleType(moduleName),
		Args:   args,
	}

	return r.Run(ctx, task, hosts, vars)
}

// SetVarManager sets the variable manager
func (r *TaskRunner) SetVarManager(varMgr *vars.VarManager) {
	r.varManager = varMgr
}

// GetVarManager returns the variable manager
func (r *TaskRunner) GetVarManager() *vars.VarManager {
	return r.varManager
}

// ValidateTask validates a task before execution
func (r *TaskRunner) ValidateTask(task types.Task) error {
	// Check if module exists
	module, err := r.GetModule(task.Module.String())
	if err != nil {
		return err
	}

	// Validate module arguments
	return module.Validate(task.Args)
}

// ListModules returns all available modules
func (r *TaskRunner) ListModules() []string {
	return r.moduleRegistry.ListModules()
}

// GetModuleDocumentation returns documentation for a module
func (r *TaskRunner) GetModuleDocumentation(moduleName string) (*types.ModuleDoc, error) {
	return r.moduleRegistry.GetModuleDocumentation(moduleName)
}

// EnableDebugMode enables debug logging (placeholder)
func (r *TaskRunner) EnableDebugMode(enabled bool) {
	// TODO: Implement debug logging when logger is added
}

// SetTimeout sets the default timeout for operations
func (r *TaskRunner) SetTimeout(timeout time.Duration) {
	// TODO: Implement timeout configuration
}

// DefaultTaskRunner provides a default task runner instance
var DefaultTaskRunner = NewTaskRunner()