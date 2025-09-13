package strategy

import (
	"context"
	"fmt"
	"sync"

	"github.com/liliang-cn/gosible/pkg/types"
	"golang.org/x/sync/errgroup"
)

// Strategy defines how tasks are executed across hosts
type Strategy interface {
	// Name returns the strategy name
	Name() string
	// Execute runs tasks with the strategy
	Execute(ctx context.Context, tasks []types.Task, hosts []types.Host, executor TaskExecutor) ([]types.Result, error)
	// SetOptions configures the strategy
	SetOptions(options map[string]interface{}) error
}

// TaskExecutor executes a single task on a host
type TaskExecutor func(ctx context.Context, task types.Task, host types.Host) (*types.Result, error)

// StrategyManager manages execution strategies
type StrategyManager struct {
	strategies map[string]Strategy
	mu         sync.RWMutex
}

// NewStrategyManager creates a new strategy manager
func NewStrategyManager() *StrategyManager {
	sm := &StrategyManager{
		strategies: make(map[string]Strategy),
	}
	
	// Register built-in strategies
	sm.Register(NewLinearStrategy())
	sm.Register(NewFreeStrategy())
	sm.Register(NewDebugStrategy())
	
	return sm
}

// Register adds a strategy
func (sm *StrategyManager) Register(strategy Strategy) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.strategies[strategy.Name()] = strategy
}

// Get returns a strategy by name
func (sm *StrategyManager) Get(name string) (Strategy, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	strategy, exists := sm.strategies[name]
	if !exists {
		return nil, fmt.Errorf("strategy '%s' not found", name)
	}
	
	return strategy, nil
}

// LinearStrategy executes tasks one at a time across all hosts
type LinearStrategy struct {
	forks int
}

// NewLinearStrategy creates a new linear strategy
func NewLinearStrategy() *LinearStrategy {
	return &LinearStrategy{
		forks: 5, // Default forks
	}
}

// Name returns "linear"
func (ls *LinearStrategy) Name() string {
	return "linear"
}

// SetOptions configures the strategy
func (ls *LinearStrategy) SetOptions(options map[string]interface{}) error {
	if forks, ok := options["forks"].(int); ok {
		ls.forks = forks
	}
	return nil
}

// Execute runs tasks in linear order
func (ls *LinearStrategy) Execute(ctx context.Context, tasks []types.Task, hosts []types.Host, executor TaskExecutor) ([]types.Result, error) {
	var allResults []types.Result
	
	// Execute each task across all hosts before moving to next task
	for _, task := range tasks {
		// Use errgroup for parallel execution with fork limit
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(ls.forks)
		
		results := make([]types.Result, len(hosts))
		resultsMu := sync.Mutex{}
		
		for i, host := range hosts {
			i, host := i, host // Capture loop variables
			
			g.Go(func() error {
				result, err := executor(ctx, task, host)
				if err != nil {
					// Store error result
					result = &types.Result{
						Host:    host.Name,
						Success: false,
						Error:   err,
						Message: err.Error(),
					}
				}
				
				resultsMu.Lock()
				results[i] = *result
				resultsMu.Unlock()
				
				return nil // Don't fail the whole batch on single host failure
			})
		}
		
		// Wait for all hosts to complete this task
		if err := g.Wait(); err != nil {
			return allResults, err
		}
		
		// Add results for this task
		allResults = append(allResults, results...)
		
		// Check if any host failed and stop if needed
		for _, result := range results {
			if !result.Success && !task.IgnoreErrors {
				return allResults, fmt.Errorf("task '%s' failed on host '%s'", task.Name, result.Host)
			}
		}
	}
	
	return allResults, nil
}

// FreeStrategy executes tasks as fast as possible without waiting
type FreeStrategy struct {
	forks int
}

// NewFreeStrategy creates a new free strategy
func NewFreeStrategy() *FreeStrategy {
	return &FreeStrategy{
		forks: 5,
	}
}

// Name returns "free"
func (fs *FreeStrategy) Name() string {
	return "free"
}

// SetOptions configures the strategy
func (fs *FreeStrategy) SetOptions(options map[string]interface{}) error {
	if forks, ok := options["forks"].(int); ok {
		fs.forks = forks
	}
	return nil
}

// Execute runs tasks as fast as possible
func (fs *FreeStrategy) Execute(ctx context.Context, tasks []types.Task, hosts []types.Host, executor TaskExecutor) ([]types.Result, error) {
	var allResults []types.Result
	resultsMu := sync.Mutex{}
	
	// Create a pool of workers
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(fs.forks)
	
	// Queue all task-host combinations
	for _, host := range hosts {
		host := host // Capture loop variable
		
		g.Go(func() error {
			// Each host processes all tasks independently
			for _, task := range tasks {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				
				result, err := executor(ctx, task, host)
				if err != nil {
					result = &types.Result{
						Host:    host.Name,
						Success: false,
						Error:   err,
						Message: err.Error(),
					}
				}
				
				resultsMu.Lock()
				allResults = append(allResults, *result)
				resultsMu.Unlock()
				
				// Stop this host's execution on failure
				if !result.Success && !task.IgnoreErrors {
					return fmt.Errorf("task '%s' failed on host '%s'", task.Name, host.Name)
				}
			}
			return nil
		})
	}
	
	// Wait for all to complete
	err := g.Wait()
	return allResults, err
}

// DebugStrategy executes tasks one by one with debugging
type DebugStrategy struct {
	debugger TaskDebugger
}

// TaskDebugger provides debugging interface
type TaskDebugger interface {
	// BeforeTask is called before task execution
	BeforeTask(task types.Task, host types.Host) bool // Return false to skip
	// AfterTask is called after task execution
	AfterTask(task types.Task, host types.Host, result *types.Result)
}

// NewDebugStrategy creates a new debug strategy
func NewDebugStrategy() *DebugStrategy {
	return &DebugStrategy{
		debugger: &InteractiveDebugger{},
	}
}

// Name returns "debug"
func (ds *DebugStrategy) Name() string {
	return "debug"
}

// SetOptions configures the strategy
func (ds *DebugStrategy) SetOptions(options map[string]interface{}) error {
	// Could set custom debugger here
	return nil
}

// Execute runs tasks with debugging
func (ds *DebugStrategy) Execute(ctx context.Context, tasks []types.Task, hosts []types.Host, executor TaskExecutor) ([]types.Result, error) {
	var allResults []types.Result
	
	// Execute one task at a time, one host at a time
	for _, task := range tasks {
		for _, host := range hosts {
			// Check for cancellation
			select {
			case <-ctx.Done():
				return allResults, ctx.Err()
			default:
			}
			
			// Debug before task
			if ds.debugger != nil {
				if !ds.debugger.BeforeTask(task, host) {
					// Skip this task
					allResults = append(allResults, types.Result{
						Host:    host.Name,
						Success: true,
						Message: "Skipped by debugger",
					})
					continue
				}
			}
			
			// Execute task
			result, err := executor(ctx, task, host)
			if err != nil {
				result = &types.Result{
					Host:    host.Name,
					Success: false,
					Error:   err,
					Message: err.Error(),
				}
			}
			
			// Debug after task
			if ds.debugger != nil {
				ds.debugger.AfterTask(task, host, result)
			}
			
			allResults = append(allResults, *result)
			
			// Stop on failure unless ignored
			if !result.Success && !task.IgnoreErrors {
				return allResults, fmt.Errorf("task '%s' failed on host '%s'", task.Name, host.Name)
			}
		}
	}
	
	return allResults, nil
}

// InteractiveDebugger provides interactive debugging
type InteractiveDebugger struct {
	// Could add stdin/stdout for interaction
}

// BeforeTask is called before task execution
func (id *InteractiveDebugger) BeforeTask(task types.Task, host types.Host) bool {
	fmt.Printf("[DEBUG] About to execute task '%s' on host '%s'\n", task.Name, host.Name)
	fmt.Printf("[DEBUG] Task module: %s\n", task.Module)
	fmt.Printf("[DEBUG] Task args: %v\n", task.Args)
	// In a real implementation, could wait for user input
	return true // Continue execution
}

// AfterTask is called after task execution
func (id *InteractiveDebugger) AfterTask(task types.Task, host types.Host, result *types.Result) {
	fmt.Printf("[DEBUG] Task '%s' on host '%s' completed\n", task.Name, host.Name)
	fmt.Printf("[DEBUG] Success: %v, Changed: %v\n", result.Success, result.Changed)
	if result.Message != "" {
		fmt.Printf("[DEBUG] Message: %s\n", result.Message)
	}
	if result.Error != nil {
		fmt.Printf("[DEBUG] Error: %v\n", result.Error)
	}
}

// HostPinnedStrategy keeps tasks on the same host
type HostPinnedStrategy struct {
	forks int
}

// NewHostPinnedStrategy creates a new host-pinned strategy
func NewHostPinnedStrategy() *HostPinnedStrategy {
	return &HostPinnedStrategy{
		forks: 5,
	}
}

// Name returns "host_pinned"
func (hp *HostPinnedStrategy) Name() string {
	return "host_pinned"
}

// SetOptions configures the strategy
func (hp *HostPinnedStrategy) SetOptions(options map[string]interface{}) error {
	if forks, ok := options["forks"].(int); ok {
		hp.forks = forks
	}
	return nil
}

// Execute runs all tasks for a host before moving to the next
func (hp *HostPinnedStrategy) Execute(ctx context.Context, tasks []types.Task, hosts []types.Host, executor TaskExecutor) ([]types.Result, error) {
	var allResults []types.Result
	resultsMu := sync.Mutex{}
	
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(hp.forks)
	
	// Process each host completely before moving to next
	for _, host := range hosts {
		host := host // Capture loop variable
		
		g.Go(func() error {
			var hostResults []types.Result
			
			// Execute all tasks for this host
			for _, task := range tasks {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				
				result, err := executor(ctx, task, host)
				if err != nil {
					result = &types.Result{
						Host:    host.Name,
						Success: false,
						Error:   err,
						Message: err.Error(),
					}
				}
				
				hostResults = append(hostResults, *result)
				
				// Stop this host on failure
				if !result.Success && !task.IgnoreErrors {
					resultsMu.Lock()
					allResults = append(allResults, hostResults...)
					resultsMu.Unlock()
					return fmt.Errorf("task '%s' failed on host '%s'", task.Name, host.Name)
				}
			}
			
			// Add all results for this host
			resultsMu.Lock()
			allResults = append(allResults, hostResults...)
			resultsMu.Unlock()
			
			return nil
		})
	}
	
	err := g.Wait()
	return allResults, err
}