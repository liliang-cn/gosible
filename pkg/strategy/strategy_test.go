package strategy

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// MockExecutor tracks task executions
type MockExecutor struct {
	executions []Execution
	mu         sync.Mutex
	delay      time.Duration
	failOn     map[string]bool
}

type Execution struct {
	Task      string
	Host      string
	Timestamp time.Time
}

func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		executions: []Execution{},
		failOn:     make(map[string]bool),
	}
}

func (m *MockExecutor) Execute(ctx context.Context, task types.Task, host types.Host) (*types.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Record execution
	m.executions = append(m.executions, Execution{
		Task:      task.Name,
		Host:      host.Name,
		Timestamp: time.Now(),
	})
	
	// Simulate work
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	
	// Check if should fail
	key := fmt.Sprintf("%s-%s", task.Name, host.Name)
	if m.failOn[key] {
		return &types.Result{
			Host:    host.Name,
			Success: false,
			Error:   fmt.Errorf("mock failure"),
			Message: "Task failed",
		}, nil
	}
	
	return &types.Result{
		Host:    host.Name,
		Success: true,
		Changed: true,
		Message: "Task completed",
	}, nil
}

func (m *MockExecutor) GetExecutions() []Execution {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Execution, len(m.executions))
	copy(result, m.executions)
	return result
}

func TestStrategyManager(t *testing.T) {
	sm := NewStrategyManager()
	
	// Check built-in strategies are registered
	strategies := []string{"linear", "free", "debug"}
	for _, name := range strategies {
		strategy, err := sm.Get(name)
		if err != nil {
			t.Errorf("Failed to get strategy '%s': %v", name, err)
		}
		if strategy == nil {
			t.Errorf("Strategy '%s' is nil", name)
		}
	}
	
	// Test non-existent strategy
	_, err := sm.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent strategy")
	}
}

func TestLinearStrategy_Basic(t *testing.T) {
	strategy := NewLinearStrategy()
	executor := NewMockExecutor()
	
	tasks := []types.Task{
		{Name: "task1"},
		{Name: "task2"},
	}
	
	hosts := []types.Host{
		{Name: "host1"},
		{Name: "host2"},
	}
	
	ctx := context.Background()
	results, err := strategy.Execute(ctx, tasks, hosts, executor.Execute)
	
	if err != nil {
		t.Fatalf("Strategy execution failed: %v", err)
	}
	
	// Should have 4 results (2 tasks x 2 hosts)
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}
	
	// Check execution order: task1 on all hosts, then task2 on all hosts
	executions := executor.GetExecutions()
	if len(executions) != 4 {
		t.Fatalf("Expected 4 executions, got %d", len(executions))
	}
	
	// First two should be task1
	for i := 0; i < 2; i++ {
		if executions[i].Task != "task1" {
			t.Errorf("Expected execution %d to be task1, got %s", i, executions[i].Task)
		}
	}
	
	// Last two should be task2
	for i := 2; i < 4; i++ {
		if executions[i].Task != "task2" {
			t.Errorf("Expected execution %d to be task2, got %s", i, executions[i].Task)
		}
	}
}

func TestLinearStrategy_Forks(t *testing.T) {
	strategy := NewLinearStrategy()
	strategy.SetOptions(map[string]interface{}{"forks": 1})
	
	executor := NewMockExecutor()
	executor.delay = 10 * time.Millisecond
	
	tasks := []types.Task{{Name: "task1"}}
	hosts := []types.Host{
		{Name: "host1"},
		{Name: "host2"},
		{Name: "host3"},
	}
	
	start := time.Now()
	ctx := context.Background()
	_, err := strategy.Execute(ctx, tasks, hosts, executor.Execute)
	duration := time.Since(start)
	
	if err != nil {
		t.Fatalf("Strategy execution failed: %v", err)
	}
	
	// With forks=1, should execute sequentially
	// Expect at least 30ms (3 hosts * 10ms)
	if duration < 30*time.Millisecond {
		t.Errorf("Expected sequential execution to take at least 30ms, took %v", duration)
	}
}

func TestLinearStrategy_Failure(t *testing.T) {
	strategy := NewLinearStrategy()
	executor := NewMockExecutor()
	executor.failOn["task1-host2"] = true
	
	tasks := []types.Task{
		{Name: "task1", IgnoreErrors: false},
		{Name: "task2"},
	}
	
	hosts := []types.Host{
		{Name: "host1"},
		{Name: "host2"},
	}
	
	ctx := context.Background()
	results, err := strategy.Execute(ctx, tasks, hosts, executor.Execute)
	
	// Should fail when task1 fails on host2
	if err == nil {
		t.Error("Expected error when task fails")
	}
	
	// Should have 2 results (task1 on both hosts)
	if len(results) != 2 {
		t.Errorf("Expected 2 results before failure, got %d", len(results))
	}
	
	// task2 should not have been executed
	executions := executor.GetExecutions()
	for _, exec := range executions {
		if exec.Task == "task2" {
			t.Error("task2 should not have been executed after task1 failure")
		}
	}
}

func TestLinearStrategy_IgnoreErrors(t *testing.T) {
	strategy := NewLinearStrategy()
	executor := NewMockExecutor()
	executor.failOn["task1-host2"] = true
	
	tasks := []types.Task{
		{Name: "task1", IgnoreErrors: true},
		{Name: "task2"},
	}
	
	hosts := []types.Host{
		{Name: "host1"},
		{Name: "host2"},
	}
	
	ctx := context.Background()
	results, err := strategy.Execute(ctx, tasks, hosts, executor.Execute)
	
	// Should not fail when IgnoreErrors is true
	if err != nil {
		t.Errorf("Expected no error with IgnoreErrors=true, got: %v", err)
	}
	
	// Should have 4 results (all tasks on all hosts)
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}
}

func TestFreeStrategy_Basic(t *testing.T) {
	strategy := NewFreeStrategy()
	executor := NewMockExecutor()
	
	tasks := []types.Task{
		{Name: "task1"},
		{Name: "task2"},
		{Name: "task3"},
	}
	
	hosts := []types.Host{
		{Name: "host1"},
		{Name: "host2"},
	}
	
	ctx := context.Background()
	results, err := strategy.Execute(ctx, tasks, hosts, executor.Execute)
	
	if err != nil {
		t.Fatalf("Strategy execution failed: %v", err)
	}
	
	// Should have 6 results (3 tasks x 2 hosts)
	if len(results) != 6 {
		t.Errorf("Expected 6 results, got %d", len(results))
	}
	
	// Each host should complete all its tasks independently
	executions := executor.GetExecutions()
	
	// Group executions by host
	hostExecs := make(map[string][]string)
	for _, exec := range executions {
		hostExecs[exec.Host] = append(hostExecs[exec.Host], exec.Task)
	}
	
	// Each host should have executed all tasks in order
	for host, tasks := range hostExecs {
		if len(tasks) != 3 {
			t.Errorf("Host %s executed %d tasks, expected 3", host, len(tasks))
		}
		
		// Tasks should be in order for each host
		expectedOrder := []string{"task1", "task2", "task3"}
		for i, task := range tasks {
			if task != expectedOrder[i] {
				t.Errorf("Host %s: expected task %s at position %d, got %s", 
					host, expectedOrder[i], i, task)
			}
		}
	}
}

func TestFreeStrategy_HostFailure(t *testing.T) {
	strategy := NewFreeStrategy()
	executor := NewMockExecutor()
	executor.failOn["task2-host1"] = true
	
	tasks := []types.Task{
		{Name: "task1"},
		{Name: "task2", IgnoreErrors: false},
		{Name: "task3"},
	}
	
	hosts := []types.Host{
		{Name: "host1"},
		{Name: "host2"},
	}
	
	ctx := context.Background()
	results, _ := strategy.Execute(ctx, tasks, hosts, executor.Execute)
	
	// Count results by host
	host1Results := 0
	host2Results := 0
	for _, result := range results {
		if result.Host == "host1" {
			host1Results++
		} else if result.Host == "host2" {
			host2Results++
		}
	}
	
	// host1 should stop at task2 (2 results)
	if host1Results != 2 {
		t.Errorf("Expected 2 results for host1 (stopped at failure), got %d", host1Results)
	}
	
	// host2 should complete all tasks (3 results)
	if host2Results != 3 {
		t.Errorf("Expected 3 results for host2 (completed all), got %d", host2Results)
	}
}

func TestDebugStrategy_Basic(t *testing.T) {
	strategy := NewDebugStrategy()
	executor := NewMockExecutor()
	
	tasks := []types.Task{
		{Name: "task1"},
		{Name: "task2"},
	}
	
	hosts := []types.Host{
		{Name: "host1"},
		{Name: "host2"},
	}
	
	ctx := context.Background()
	results, err := strategy.Execute(ctx, tasks, hosts, executor.Execute)
	
	if err != nil {
		t.Fatalf("Strategy execution failed: %v", err)
	}
	
	// Should execute one by one: 2 tasks x 2 hosts = 4 results
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}
	
	// Check sequential execution
	executions := executor.GetExecutions()
	expectedOrder := []Execution{
		{Task: "task1", Host: "host1"},
		{Task: "task1", Host: "host2"},
		{Task: "task2", Host: "host1"},
		{Task: "task2", Host: "host2"},
	}
	
	for i, exec := range executions {
		if exec.Task != expectedOrder[i].Task || exec.Host != expectedOrder[i].Host {
			t.Errorf("Execution %d: expected %s on %s, got %s on %s",
				i, expectedOrder[i].Task, expectedOrder[i].Host, exec.Task, exec.Host)
		}
	}
}

func TestHostPinnedStrategy_Basic(t *testing.T) {
	strategy := NewHostPinnedStrategy()
	executor := NewMockExecutor()
	executor.delay = 5 * time.Millisecond
	
	tasks := []types.Task{
		{Name: "task1"},
		{Name: "task2"},
		{Name: "task3"},
	}
	
	hosts := []types.Host{
		{Name: "host1"},
		{Name: "host2"},
	}
	
	ctx := context.Background()
	results, err := strategy.Execute(ctx, tasks, hosts, executor.Execute)
	
	if err != nil {
		t.Fatalf("Strategy execution failed: %v", err)
	}
	
	// Should have 6 results
	if len(results) != 6 {
		t.Errorf("Expected 6 results, got %d", len(results))
	}
	
	// Verify each host completes all tasks before moving to next
	executions := executor.GetExecutions()
	
	// With parallel execution, we can't guarantee order between hosts,
	// but we can verify each host completes its tasks in order
	hostTasks := make(map[string][]string)
	for _, exec := range executions {
		hostTasks[exec.Host] = append(hostTasks[exec.Host], exec.Task)
	}
	
	for host, tasks := range hostTasks {
		if len(tasks) != 3 {
			t.Errorf("Host %s should have 3 tasks, got %d", host, len(tasks))
		}
		
		// Check task order for this host
		for i, task := range tasks {
			expected := fmt.Sprintf("task%d", i+1)
			if task != expected {
				t.Errorf("Host %s: expected %s at position %d, got %s",
					host, expected, i, task)
			}
		}
	}
}

func TestStrategy_ContextCancellation(t *testing.T) {
	strategy := NewLinearStrategy()
	
	var executionCount int32
	slowExecutor := func(ctx context.Context, task types.Task, host types.Host) (*types.Result, error) {
		atomic.AddInt32(&executionCount, 1)
		
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return &types.Result{
				Host:    host.Name,
				Success: true,
			}, nil
		}
	}
	
	tasks := []types.Task{
		{Name: "task1"},
		{Name: "task2"},
	}
	
	hosts := []types.Host{
		{Name: "host1"},
		{Name: "host2"},
		{Name: "host3"},
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	
	_, err := strategy.Execute(ctx, tasks, hosts, slowExecutor)
	
	// Should return context error
	if err == nil {
		t.Error("Expected context cancellation error")
	}
	
	// Not all tasks should have been executed
	count := atomic.LoadInt32(&executionCount)
	if count >= 6 {
		t.Errorf("Expected fewer than 6 executions due to cancellation, got %d", count)
	}
}

func TestStrategy_SetOptions(t *testing.T) {
	tests := []struct {
		name     string
		strategy Strategy
		options  map[string]interface{}
	}{
		{
			name:     "LinearStrategy forks",
			strategy: NewLinearStrategy(),
			options:  map[string]interface{}{"forks": 10},
		},
		{
			name:     "FreeStrategy forks",
			strategy: NewFreeStrategy(),
			options:  map[string]interface{}{"forks": 20},
		},
		{
			name:     "HostPinnedStrategy forks",
			strategy: NewHostPinnedStrategy(),
			options:  map[string]interface{}{"forks": 15},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.strategy.SetOptions(tt.options)
			if err != nil {
				t.Errorf("Failed to set options: %v", err)
			}
		})
	}
}