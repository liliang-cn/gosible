package playbook

import (
	"context"
	"fmt"
	"strings"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// Executor handles playbook execution
type Executor struct {
	runner    types.Runner
	inventory types.Inventory
	varMgr    types.VarManager
	events    []types.EventCallback
}

// NewExecutor creates a new playbook executor
func NewExecutor(runner types.Runner, inventory types.Inventory, varMgr types.VarManager) *Executor {
	return &Executor{
		runner:    runner,
		inventory: inventory,
		varMgr:    varMgr,
		events:    make([]types.EventCallback, 0),
	}
}

// AddEventCallback adds an event callback
func (e *Executor) AddEventCallback(callback types.EventCallback) {
	e.events = append(e.events, callback)
}

// emitEvent emits an event to all callbacks
func (e *Executor) emitEvent(event types.Event) {
	for _, callback := range e.events {
		callback(event)
	}
}

// Execute executes a complete playbook
func (e *Executor) Execute(ctx context.Context, playbook *types.Playbook, extraVars map[string]interface{}) ([]types.Result, error) {
	var allResults []types.Result

	// Merge playbook vars with extra vars
	playbookVars := make(map[string]interface{})
	if playbook.Vars != nil {
		playbookVars = types.DeepMergeInterfaceMaps(playbookVars, playbook.Vars)
	}
	if extraVars != nil {
		playbookVars = types.DeepMergeInterfaceMaps(playbookVars, extraVars)
	}

	// Execute each play in the playbook
	for i, play := range playbook.Plays {
		e.emitEvent(types.Event{
			Type:      types.EventPlayStart,
			Timestamp: types.GetCurrentTime(),
			Play:      play.Name,
			Data: map[string]interface{}{
				"play_index": i,
				"play_name":  play.Name,
			},
		})

		results, err := e.ExecutePlay(ctx, &play, playbookVars)
		if err != nil {
			e.emitEvent(types.Event{
				Type:      types.EventError,
				Timestamp: types.GetCurrentTime(),
				Play:      play.Name,
				Error:     err,
			})
			return allResults, types.NewPlaybookError("playbook", play.Name, "", "play execution failed", err)
		}

		allResults = append(allResults, results...)

		e.emitEvent(types.Event{
			Type:      types.EventPlayComplete,
			Timestamp: types.GetCurrentTime(),
			Play:      play.Name,
			Data: map[string]interface{}{
				"results_count": len(results),
			},
		})

		// Check if we should stop on failure
		if e.shouldStopOnFailure(results) {
			break
		}
	}

	return allResults, nil
}

// ExecutePlay executes a single play
func (e *Executor) ExecutePlay(ctx context.Context, play *types.Play, vars map[string]interface{}) ([]types.Result, error) {
	// Get hosts for this play
	hosts, err := e.getPlayHosts(play)
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts for play %s: %w", play.Name, err)
	}

	if len(hosts) == 0 {
		return []types.Result{}, nil
	}

	// Merge play vars with provided vars
	playVars := e.mergePlayVars(play, vars)

	var allResults []types.Result

	// Execute pre_tasks
	if len(play.PreTasks) > 0 {
		results, err := e.executeTasks(ctx, play.PreTasks, hosts, playVars, play.Name, "pre_tasks")
		if err != nil {
			return allResults, err
		}
		allResults = append(allResults, results...)
	}

	// Gather facts if needed
	if e.shouldGatherFacts(playVars) {
		factResults, err := e.gatherFacts(ctx, hosts)
		if err != nil {
			return allResults, fmt.Errorf("failed to gather facts: %w", err)
		}
		allResults = append(allResults, factResults...)
	}

	// Execute main tasks
	if len(play.Tasks) > 0 {
		results, err := e.executeTasks(ctx, play.Tasks, hosts, playVars, play.Name, "tasks")
		if err != nil {
			return allResults, err
		}
		allResults = append(allResults, results...)
	}

	// Execute post_tasks
	if len(play.PostTasks) > 0 {
		results, err := e.executeTasks(ctx, play.PostTasks, hosts, playVars, play.Name, "post_tasks")
		if err != nil {
			return allResults, err
		}
		allResults = append(allResults, results...)
	}

	// Execute handlers (triggered tasks)
	if len(play.Handlers) > 0 {
		handlerResults, err := e.executeHandlers(ctx, play.Handlers, hosts, playVars, play.Name)
		if err != nil {
			return allResults, err
		}
		allResults = append(allResults, handlerResults...)
	}

	return allResults, nil
}

// executeTasks executes a list of tasks
func (e *Executor) executeTasks(ctx context.Context, tasks []types.Task, hosts []types.Host, vars map[string]interface{}, playName, taskType string) ([]types.Result, error) {
	var allResults []types.Result

	for i, task := range tasks {
		// Skip tasks that don't match tags or conditions
		if e.shouldSkipTask(&task, vars) {
			continue
		}

		// Emit task start event
		e.emitEvent(types.Event{
			Type:      types.EventTaskStart,
			Timestamp: types.GetCurrentTime(),
			Host:      "", // Will be set per host
			Task:      task.Name,
			Play:      playName,
			Data: map[string]interface{}{
				"task_index": i,
				"task_type":  taskType,
			},
		})

		// Merge task vars
		taskVars := e.mergeTaskVars(&task, vars)

		// Execute task
		results, err := e.executeTask(ctx, &task, hosts, taskVars)
		if err != nil {
			e.emitEvent(types.Event{
				Type:      types.EventTaskFailed,
				Timestamp: types.GetCurrentTime(),
				Task:      task.Name,
				Play:      playName,
				Error:     err,
			})

			if !task.IgnoreErrors {
				return allResults, err
			}
		}

		allResults = append(allResults, results...)

		// Emit task complete event
		e.emitEvent(types.Event{
			Type:      types.EventTaskComplete,
			Timestamp: types.GetCurrentTime(),
			Task:      task.Name,
			Play:      playName,
			Data: map[string]interface{}{
				"results_count": len(results),
			},
		})

		// Check for failures
		if e.shouldStopOnTaskFailure(results, &task) {
			return allResults, fmt.Errorf("task '%s' failed on one or more hosts", task.Name)
		}
	}

	return allResults, nil
}

// executeTask executes a single task on multiple hosts
func (e *Executor) executeTask(ctx context.Context, task *types.Task, hosts []types.Host, vars map[string]interface{}) ([]types.Result, error) {
	// Handle loop execution
	if task.Loop != nil {
		return e.executeTaskWithLoop(ctx, task, hosts, vars)
	}

	// Handle delegation
	if task.Delegate != "" {
		return e.executeTaskWithDelegation(ctx, task, hosts, vars)
	}

	// Handle run_once
	if task.RunOnce {
		return e.executeTaskRunOnce(ctx, task, hosts, vars)
	}

	// Normal execution
	return e.runner.Run(ctx, *task, hosts, vars)
}

// executeTaskWithLoop executes a task with a loop
func (e *Executor) executeTaskWithLoop(ctx context.Context, task *types.Task, hosts []types.Host, vars map[string]interface{}) ([]types.Result, error) {
	loopItems, err := e.resolveLoopItems(task.Loop, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve loop items: %w", err)
	}

	var allResults []types.Result

	for i, item := range loopItems {
		// Create task copy with loop variables
		loopTask := *task
		loopVars := make(map[string]interface{})
		for k, v := range vars {
			loopVars[k] = v
		}
		loopVars["item"] = item
		loopVars["item_index"] = i

		results, err := e.runner.Run(ctx, loopTask, hosts, loopVars)
		if err != nil {
			if !task.IgnoreErrors {
				return allResults, err
			}
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// executeTaskWithDelegation executes a task with delegation to another host
func (e *Executor) executeTaskWithDelegation(ctx context.Context, task *types.Task, hosts []types.Host, vars map[string]interface{}) ([]types.Result, error) {
	// Find delegate host
	delegateHost, err := e.inventory.GetHost(task.Delegate)
	if err != nil {
		return nil, fmt.Errorf("delegate host %s not found: %w", task.Delegate, err)
	}

	// Execute on delegate host
	return e.runner.Run(ctx, *task, []types.Host{*delegateHost}, vars)
}

// executeTaskRunOnce executes a task only once (on first host)
func (e *Executor) executeTaskRunOnce(ctx context.Context, task *types.Task, hosts []types.Host, vars map[string]interface{}) ([]types.Result, error) {
	if len(hosts) == 0 {
		return []types.Result{}, nil
	}

	// Execute only on first host
	return e.runner.Run(ctx, *task, hosts[:1], vars)
}

// executeHandlers executes handler tasks
func (e *Executor) executeHandlers(ctx context.Context, handlers []types.Task, hosts []types.Host, vars map[string]interface{}, playName string) ([]types.Result, error) {
	// In a full implementation, this would track which handlers were notified
	// For now, we'll skip handler execution
	return []types.Result{}, nil
}

// getPlayHosts resolves the hosts for a play
func (e *Executor) getPlayHosts(play *types.Play) ([]types.Host, error) {
	parser := NewParser()
	patterns := parser.ParseInventoryPattern(play.Hosts)

	var allHosts []types.Host
	for _, pattern := range patterns {
		hosts, err := e.inventory.GetHosts(pattern)
		if err != nil {
			return nil, err
		}
		allHosts = append(allHosts, hosts...)
	}

	// Remove duplicates
	return e.removeDuplicateHosts(allHosts), nil
}

// removeDuplicateHosts removes duplicate hosts from a slice
func (e *Executor) removeDuplicateHosts(hosts []types.Host) []types.Host {
	seen := make(map[string]bool)
	result := make([]types.Host, 0, len(hosts))

	for _, host := range hosts {
		if !seen[host.Name] {
			seen[host.Name] = true
			result = append(result, host)
		}
	}

	return result
}

// mergePlayVars merges play variables with global variables
func (e *Executor) mergePlayVars(play *types.Play, globalVars map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Start with global vars
	if globalVars != nil {
		result = types.DeepMergeInterfaceMaps(result, globalVars)
	}

	// Add play vars
	if play.Vars != nil {
		result = types.DeepMergeInterfaceMaps(result, play.Vars)
	}

	return result
}

// mergeTaskVars merges task variables with play/global variables
func (e *Executor) mergeTaskVars(task *types.Task, playVars map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Start with play vars
	if playVars != nil {
		result = types.DeepMergeInterfaceMaps(result, playVars)
	}

	// Add task vars
	if task.Vars != nil {
		result = types.DeepMergeInterfaceMaps(result, task.Vars)
	}

	return result
}

// shouldSkipTask determines if a task should be skipped
func (e *Executor) shouldSkipTask(task *types.Task, vars map[string]interface{}) bool {
	// Check when condition
	if task.When != nil {
		// Convert to string for simple evaluation
		whenStr := ""
		switch v := task.When.(type) {
		case string:
			whenStr = v
		case bool:
			if !v {
				return true
			}
			return false
		default:
			// For complex conditions, skip for now
			return false
		}
		
		if whenStr != "" {
			// Simple condition evaluation - in a real implementation, this would be more complex
			return !e.evaluateCondition(whenStr, vars)
		}
	}

	// Check tags (simplified - real Ansible has complex tag logic)
	// For now, assume all tasks run
	return false
}

// evaluateCondition evaluates a when condition
func (e *Executor) evaluateCondition(condition string, vars map[string]interface{}) bool {
	// Simplified condition evaluation
	// Real Ansible uses Jinja2 expressions
	condition = types.ExpandVariables(condition, vars)

	// Basic true/false evaluation
	switch condition {
	case "true", "True", "yes", "Yes", "1":
		return true
	case "false", "False", "no", "No", "0":
		return false
	default:
		// For complex conditions, assume true for now
		return true
	}
}

// resolveLoopItems resolves loop items from various sources
func (e *Executor) resolveLoopItems(loop interface{}, vars map[string]interface{}) ([]interface{}, error) {
	switch l := loop.(type) {
	case []interface{}:
		return l, nil
	case string:
		// Could be a variable reference
		expanded := types.ExpandVariables(l, vars)
		if expanded != l {
			// Variable was expanded, try to resolve it
			if value, exists := vars[strings.Trim(expanded, "{}")]; exists {
				if slice, ok := value.([]interface{}); ok {
					return slice, nil
				}
			}
		}
		// Treat as single item
		return []interface{}{expanded}, nil
	default:
		return []interface{}{loop}, nil
	}
}

// shouldGatherFacts determines if facts should be gathered
func (e *Executor) shouldGatherFacts(vars map[string]interface{}) bool {
	if gatherFacts, exists := vars["gather_facts"]; exists {
		return types.ConvertToBool(gatherFacts)
	}
	return true // Default to gathering facts
}

// gatherFacts gathers facts from hosts
func (e *Executor) gatherFacts(ctx context.Context, hosts []types.Host) ([]types.Result, error) {
	setupTask := types.Task{
		Name:   "Gathering Facts",
		Module: "setup",
		Args:   make(map[string]interface{}),
	}

	return e.runner.Run(ctx, setupTask, hosts, make(map[string]interface{}))
}

// shouldStopOnFailure determines if execution should stop on failure
func (e *Executor) shouldStopOnFailure(results []types.Result) bool {
	for _, result := range results {
		if !result.Success && result.Error != nil {
			// In a real implementation, this would check any_errors_fatal and other settings
			return false // Continue execution for now
		}
	}
	return false
}

// shouldStopOnTaskFailure determines if execution should stop on task failure
func (e *Executor) shouldStopOnTaskFailure(results []types.Result, task *types.Task) bool {
	if task.IgnoreErrors {
		return false
	}

	for _, result := range results {
		if !result.Success {
			return true
		}
	}
	return false
}