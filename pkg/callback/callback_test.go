package callback

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

func TestCallbackManager_Register(t *testing.T) {
	cm := NewCallbackManager()
	callback := NewDefaultCallback()
	
	cm.Register(callback)
	
	if len(cm.plugins) != 1 {
		t.Errorf("Expected 1 plugin registered, got %d", len(cm.plugins))
	}
}

func TestCallbackManager_Events(t *testing.T) {
	cm := NewCallbackManager()
	
	// Track events
	events := []string{}
	
	// Create a custom callback to track events
	trackCallback := &EventTracker{events: &events}
	cm.Register(trackCallback)
	
	// Fire events
	play := &types.Play{Name: "Test Play"}
	task := &types.Task{Name: "Test Task"}
	hosts := []types.Host{{Name: "host1"}}
	result := &types.Result{
		Host:    "host1",
		Success: true,
		Changed: true,
		Message: "Task completed",
	}
	
	cm.OnPlayStart(play)
	cm.OnTaskStart(task, hosts)
	cm.OnTaskResult(task, result)
	cm.OnPlayEnd(play, []types.Result{*result})
	cm.OnRunnerEnd()
	
	// Check events were fired in order
	expectedEvents := []string{
		"play_start",
		"task_start",
		"task_result",
		"play_end",
		"runner_end",
	}
	
	if len(events) != len(expectedEvents) {
		t.Fatalf("Expected %d events, got %d", len(expectedEvents), len(events))
	}
	
	for i, expected := range expectedEvents {
		if events[i] != expected {
			t.Errorf("Event %d: expected '%s', got '%s'", i, expected, events[i])
		}
	}
}

func TestCallbackManager_Stats(t *testing.T) {
	cm := NewCallbackManager()
	
	task := &types.Task{Name: "Test Task"}
	hosts := []types.Host{{Name: "host1"}, {Name: "host2"}}
	
	// Start a task
	cm.OnTaskStart(task, hosts)
	
	// Report results
	result1 := &types.Result{
		Host:     "host1",
		Success:  true,
		Changed:  true,
		Duration: 100 * time.Millisecond,
	}
	cm.OnTaskResult(task, result1)
	
	result2 := &types.Result{
		Host:     "host2",
		Success:  false,
		Changed:  false,
		Duration: 50 * time.Millisecond,
	}
	cm.OnTaskResult(task, result2)
	
	// Check stats
	if cm.stats.TotalTasks != 1 {
		t.Errorf("Expected 1 total task, got %d", cm.stats.TotalTasks)
	}
	
	if cm.stats.SuccessTasks != 1 {
		t.Errorf("Expected 1 success task, got %d", cm.stats.SuccessTasks)
	}
	
	if cm.stats.FailedTasks != 1 {
		t.Errorf("Expected 1 failed task, got %d", cm.stats.FailedTasks)
	}
	
	if cm.stats.ChangedTasks != 1 {
		t.Errorf("Expected 1 changed task, got %d", cm.stats.ChangedTasks)
	}
	
	// Check host stats
	host1Stats := cm.stats.HostStats["host1"]
	if host1Stats == nil {
		t.Fatal("Expected stats for host1")
	}
	
	if host1Stats.Ok != 1 {
		t.Errorf("Expected 1 ok for host1, got %d", host1Stats.Ok)
	}
	
	if host1Stats.Changed != 1 {
		t.Errorf("Expected 1 changed for host1, got %d", host1Stats.Changed)
	}
	
	host2Stats := cm.stats.HostStats["host2"]
	if host2Stats == nil {
		t.Fatal("Expected stats for host2")
	}
	
	if host2Stats.Failed != 1 {
		t.Errorf("Expected 1 failed for host2, got %d", host2Stats.Failed)
	}
}

func TestDefaultCallback_Output(t *testing.T) {
	var buf bytes.Buffer
	callback := NewDefaultCallback()
	callback.SetOutput(&buf)
	
	// Test play start
	play := &types.Play{Name: "Deploy Application"}
	callback.OnPlayStart(play)
	
	output := buf.String()
	if !strings.Contains(output, "PLAY [Deploy Application]") {
		t.Errorf("Expected play start output, got: %s", output)
	}
	
	// Test task start
	buf.Reset()
	task := &types.Task{Name: "Install packages"}
	hosts := []types.Host{{Name: "server1"}}
	callback.OnTaskStart(task, hosts)
	
	output = buf.String()
	if !strings.Contains(output, "TASK [Install packages]") {
		t.Errorf("Expected task start output, got: %s", output)
	}
	
	// Test task result
	buf.Reset()
	result := &types.Result{
		Host:    "server1",
		Success: true,
		Changed: true,
		Message: "Package installed",
	}
	callback.OnTaskResult(task, result)
	
	output = buf.String()
	if !strings.Contains(output, "changed:") {
		t.Errorf("Expected 'changed' status, got: %s", output)
	}
	if !strings.Contains(output, "server1") {
		t.Errorf("Expected host name in output, got: %s", output)
	}
	
	// Test runner end with recap
	buf.Reset()
	stats := &RunStats{
		HostStats: map[string]*HostStats{
			"server1": {
				Host:    "server1",
				Ok:      5,
				Changed: 2,
				Failed:  0,
				Skipped: 1,
			},
		},
	}
	callback.OnRunnerEnd(stats)
	
	output = buf.String()
	if !strings.Contains(output, "PLAY RECAP") {
		t.Errorf("Expected play recap, got: %s", output)
	}
	if !strings.Contains(output, "ok=5") {
		t.Errorf("Expected ok count in recap, got: %s", output)
	}
}

func TestJSONCallback_Output(t *testing.T) {
	var buf bytes.Buffer
	callback := NewJSONCallback()
	callback.SetOutput(&buf)
	
	// Fire events
	play := &types.Play{Name: "Test Play"}
	callback.OnPlayStart(play)
	
	task := &types.Task{Name: "Test Task"}
	hosts := []types.Host{{Name: "host1"}}
	callback.OnTaskStart(task, hosts)
	
	result := &types.Result{
		Host:    "host1",
		Success: true,
		Changed: false,
		Message: "OK",
	}
	callback.OnTaskResult(task, result)
	
	callback.OnPlayEnd(play, []types.Result{*result})
	
	stats := &RunStats{
		TotalTasks:   1,
		SuccessTasks: 1,
		HostStats: map[string]*HostStats{
			"host1": {Ok: 1},
		},
	}
	callback.OnRunnerEnd(stats)
	
	// Parse JSON output
	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}
	
	// Check events array
	events, ok := output["events"].([]interface{})
	if !ok {
		t.Fatal("Expected 'events' array in JSON output")
	}
	
	if len(events) != 4 {
		t.Errorf("Expected 4 events, got %d", len(events))
	}
	
	// Check first event
	firstEvent, ok := events[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected first event to be a map")
	}
	
	if firstEvent["event"] != "play_start" {
		t.Errorf("Expected first event to be 'play_start', got %v", firstEvent["event"])
	}
	
	if firstEvent["play"] != "Test Play" {
		t.Errorf("Expected play name 'Test Play', got %v", firstEvent["play"])
	}
	
	// Check stats
	statsOutput, ok := output["stats"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'stats' in JSON output")
	}
	
	if totalTasks, ok := statsOutput["TotalTasks"].(float64); !ok || int(totalTasks) != 1 {
		t.Errorf("Expected TotalTasks to be 1, got %v", statsOutput["TotalTasks"])
	}
}

func TestProfileTasksCallback(t *testing.T) {
	var buf bytes.Buffer
	callback := NewProfileTasksCallback()
	callback.SetOutput(&buf)
	
	// Simulate tasks with different durations
	task1 := &types.Task{Name: "Slow Task"}
	task2 := &types.Task{Name: "Fast Task"}
	hosts := []types.Host{{Name: "host1"}}
	
	// Start and end tasks
	callback.OnTaskStart(task1, hosts)
	time.Sleep(10 * time.Millisecond)
	result1 := &types.Result{Host: "host1", Success: true}
	callback.OnTaskResult(task1, result1)
	
	callback.OnTaskStart(task2, hosts)
	time.Sleep(5 * time.Millisecond)
	result2 := &types.Result{Host: "host1", Success: true}
	callback.OnTaskResult(task2, result2)
	
	// Generate profile report
	stats := &RunStats{}
	callback.OnRunnerEnd(stats)
	
	output := buf.String()
	if !strings.Contains(output, "Task Profiling") {
		t.Errorf("Expected profiling header, got: %s", output)
	}
	
	// Check that tasks are listed
	if !strings.Contains(output, "Slow Task") {
		t.Errorf("Expected 'Slow Task' in output, got: %s", output)
	}
	
	if !strings.Contains(output, "Fast Task") {
		t.Errorf("Expected 'Fast Task' in output, got: %s", output)
	}
}

// EventTracker is a test callback that tracks events
type EventTracker struct {
	events *[]string
}

func (et *EventTracker) Name() string { return "tracker" }
func (et *EventTracker) Initialize(config map[string]interface{}) error { return nil }
func (et *EventTracker) SetOutput(writer io.Writer) {}

func (et *EventTracker) OnPlayStart(play *types.Play) {
	*et.events = append(*et.events, "play_start")
}

func (et *EventTracker) OnTaskStart(task *types.Task, hosts []types.Host) {
	*et.events = append(*et.events, "task_start")
}

func (et *EventTracker) OnTaskResult(task *types.Task, result *types.Result) {
	*et.events = append(*et.events, "task_result")
}

func (et *EventTracker) OnPlayEnd(play *types.Play, results []types.Result) {
	*et.events = append(*et.events, "play_end")
}

func (et *EventTracker) OnRunnerEnd(stats *RunStats) {
	*et.events = append(*et.events, "runner_end")
}