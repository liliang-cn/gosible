package callback

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// CallbackPlugin interface for all callback plugins
type CallbackPlugin interface {
	// Name returns the plugin name
	Name() string
	// Initialize sets up the plugin
	Initialize(config map[string]interface{}) error
	// OnPlayStart is called when a play starts
	OnPlayStart(play *types.Play)
	// OnTaskStart is called when a task starts
	OnTaskStart(task *types.Task, hosts []types.Host)
	// OnTaskResult is called when a task completes
	OnTaskResult(task *types.Task, result *types.Result)
	// OnPlayEnd is called when a play ends
	OnPlayEnd(play *types.Play, results []types.Result)
	// OnRunnerEnd is called when the entire run ends
	OnRunnerEnd(stats *RunStats)
	// SetOutput sets the output writer
	SetOutput(writer io.Writer)
}

// RunStats contains statistics for a run
type RunStats struct {
	StartTime    time.Time
	EndTime      time.Time
	TotalTasks   int
	SuccessTasks int
	FailedTasks  int
	SkippedTasks int
	ChangedTasks int
	HostStats    map[string]*HostStats
}

// HostStats contains statistics for a single host
type HostStats struct {
	Host         string
	Ok           int
	Changed      int
	Unreachable  int
	Failed       int
	Skipped      int
	TotalTime    time.Duration
}

// CallbackManager manages callback plugins
type CallbackManager struct {
	plugins []CallbackPlugin
	mu      sync.RWMutex
	stats   *RunStats
}

// NewCallbackManager creates a new callback manager
func NewCallbackManager() *CallbackManager {
	return &CallbackManager{
		plugins: []CallbackPlugin{},
		stats: &RunStats{
			HostStats: make(map[string]*HostStats),
		},
	}
}

// Register adds a callback plugin
func (cm *CallbackManager) Register(plugin CallbackPlugin) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.plugins = append(cm.plugins, plugin)
}

// OnPlayStart notifies all plugins of play start
func (cm *CallbackManager) OnPlayStart(play *types.Play) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	for _, plugin := range cm.plugins {
		plugin.OnPlayStart(play)
	}
}

// OnTaskStart notifies all plugins of task start
func (cm *CallbackManager) OnTaskStart(task *types.Task, hosts []types.Host) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	cm.stats.TotalTasks++
	
	for _, plugin := range cm.plugins {
		plugin.OnTaskStart(task, hosts)
	}
}

// OnTaskResult notifies all plugins of task result
func (cm *CallbackManager) OnTaskResult(task *types.Task, result *types.Result) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// Update stats
	if result.Success {
		cm.stats.SuccessTasks++
		if result.Changed {
			cm.stats.ChangedTasks++
		}
	} else {
		cm.stats.FailedTasks++
	}
	
	// Update host stats
	if _, exists := cm.stats.HostStats[result.Host]; !exists {
		cm.stats.HostStats[result.Host] = &HostStats{Host: result.Host}
	}
	
	hostStat := cm.stats.HostStats[result.Host]
	if result.Success {
		hostStat.Ok++
		if result.Changed {
			hostStat.Changed++
		}
	} else {
		hostStat.Failed++
	}
	hostStat.TotalTime += result.Duration
	
	for _, plugin := range cm.plugins {
		plugin.OnTaskResult(task, result)
	}
}

// OnPlayEnd notifies all plugins of play end
func (cm *CallbackManager) OnPlayEnd(play *types.Play, results []types.Result) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	for _, plugin := range cm.plugins {
		plugin.OnPlayEnd(play, results)
	}
}

// OnRunnerEnd notifies all plugins of runner end
func (cm *CallbackManager) OnRunnerEnd() {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	cm.stats.EndTime = time.Now()
	
	for _, plugin := range cm.plugins {
		plugin.OnRunnerEnd(cm.stats)
	}
}

// DefaultCallback is the default stdout callback
type DefaultCallback struct {
	output io.Writer
	config map[string]interface{}
}

// NewDefaultCallback creates a new default callback
func NewDefaultCallback() *DefaultCallback {
	return &DefaultCallback{
		output: os.Stdout,
		config: make(map[string]interface{}),
	}
}

// Name returns "default"
func (dc *DefaultCallback) Name() string {
	return "default"
}

// Initialize sets up the plugin
func (dc *DefaultCallback) Initialize(config map[string]interface{}) error {
	dc.config = config
	return nil
}

// SetOutput sets the output writer
func (dc *DefaultCallback) SetOutput(writer io.Writer) {
	dc.output = writer
}

// OnPlayStart handles play start
func (dc *DefaultCallback) OnPlayStart(play *types.Play) {
	fmt.Fprintf(dc.output, "\nPLAY [%s] %s\n", play.Name, strings.Repeat("*", 70-len(play.Name)))
}

// OnTaskStart handles task start
func (dc *DefaultCallback) OnTaskStart(task *types.Task, hosts []types.Host) {
	fmt.Fprintf(dc.output, "\nTASK [%s] %s\n", task.Name, strings.Repeat("*", 70-len(task.Name)))
}

// OnTaskResult handles task results
func (dc *DefaultCallback) OnTaskResult(task *types.Task, result *types.Result) {
	status := "ok"
	if !result.Success {
		status = "failed"
	} else if result.Changed {
		status = "changed"
	}
	
	fmt.Fprintf(dc.output, "%s: [%s] => %s\n", status, result.Host, result.Message)
}

// OnPlayEnd handles play end
func (dc *DefaultCallback) OnPlayEnd(play *types.Play, results []types.Result) {
	// Play recap can be shown here
}

// OnRunnerEnd handles runner end
func (dc *DefaultCallback) OnRunnerEnd(stats *RunStats) {
	fmt.Fprintf(dc.output, "\nPLAY RECAP %s\n", strings.Repeat("*", 70))
	
	for host, hostStats := range stats.HostStats {
		fmt.Fprintf(dc.output, "%s : ok=%d changed=%d unreachable=%d failed=%d skipped=%d\n",
			host, hostStats.Ok, hostStats.Changed, hostStats.Unreachable, 
			hostStats.Failed, hostStats.Skipped)
	}
}

// JSONCallback outputs in JSON format
type JSONCallback struct {
	output  io.Writer
	results []interface{}
	mu      sync.Mutex
}

// NewJSONCallback creates a new JSON callback
func NewJSONCallback() *JSONCallback {
	return &JSONCallback{
		output:  os.Stdout,
		results: []interface{}{},
	}
}

// Name returns "json"
func (jc *JSONCallback) Name() string {
	return "json"
}

// Initialize sets up the plugin
func (jc *JSONCallback) Initialize(config map[string]interface{}) error {
	return nil
}

// SetOutput sets the output writer
func (jc *JSONCallback) SetOutput(writer io.Writer) {
	jc.output = writer
}

// OnPlayStart handles play start
func (jc *JSONCallback) OnPlayStart(play *types.Play) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	
	jc.results = append(jc.results, map[string]interface{}{
		"event": "play_start",
		"play":  play.Name,
		"time":  time.Now().Unix(),
	})
}

// OnTaskStart handles task start
func (jc *JSONCallback) OnTaskStart(task *types.Task, hosts []types.Host) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	
	hostNames := make([]string, len(hosts))
	for i, h := range hosts {
		hostNames[i] = h.Name
	}
	
	jc.results = append(jc.results, map[string]interface{}{
		"event": "task_start",
		"task":  task.Name,
		"hosts": hostNames,
		"time":  time.Now().Unix(),
	})
}

// OnTaskResult handles task results
func (jc *JSONCallback) OnTaskResult(task *types.Task, result *types.Result) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	
	jc.results = append(jc.results, map[string]interface{}{
		"event":   "task_result",
		"task":    task.Name,
		"host":    result.Host,
		"success": result.Success,
		"changed": result.Changed,
		"message": result.Message,
		"time":    time.Now().Unix(),
	})
}

// OnPlayEnd handles play end
func (jc *JSONCallback) OnPlayEnd(play *types.Play, results []types.Result) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	
	jc.results = append(jc.results, map[string]interface{}{
		"event": "play_end",
		"play":  play.Name,
		"time":  time.Now().Unix(),
	})
}

// OnRunnerEnd handles runner end and outputs JSON
func (jc *JSONCallback) OnRunnerEnd(stats *RunStats) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	
	output := map[string]interface{}{
		"events": jc.results,
		"stats":  stats,
	}
	
	encoder := json.NewEncoder(jc.output)
	encoder.SetIndent("", "  ")
	encoder.Encode(output)
}

// ProfileTasksCallback tracks task execution time
type ProfileTasksCallback struct {
	output    io.Writer
	taskTimes map[string]time.Duration
	taskStarts map[string]time.Time
	mu        sync.Mutex
}

// NewProfileTasksCallback creates a new profile tasks callback
func NewProfileTasksCallback() *ProfileTasksCallback {
	return &ProfileTasksCallback{
		output:     os.Stdout,
		taskTimes:  make(map[string]time.Duration),
		taskStarts: make(map[string]time.Time),
	}
}

// Name returns "profile_tasks"
func (pc *ProfileTasksCallback) Name() string {
	return "profile_tasks"
}

// Initialize sets up the plugin
func (pc *ProfileTasksCallback) Initialize(config map[string]interface{}) error {
	return nil
}

// SetOutput sets the output writer
func (pc *ProfileTasksCallback) SetOutput(writer io.Writer) {
	pc.output = writer
}

// OnPlayStart handles play start
func (pc *ProfileTasksCallback) OnPlayStart(play *types.Play) {
	// Not needed for profiling
}

// OnTaskStart records task start time
func (pc *ProfileTasksCallback) OnTaskStart(task *types.Task, hosts []types.Host) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.taskStarts[task.Name] = time.Now()
}

// OnTaskResult calculates task duration
func (pc *ProfileTasksCallback) OnTaskResult(task *types.Task, result *types.Result) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	if startTime, exists := pc.taskStarts[task.Name]; exists {
		duration := time.Since(startTime)
		if existing, ok := pc.taskTimes[task.Name]; ok {
			pc.taskTimes[task.Name] = existing + duration
		} else {
			pc.taskTimes[task.Name] = duration
		}
	}
}

// OnPlayEnd handles play end
func (pc *ProfileTasksCallback) OnPlayEnd(play *types.Play, results []types.Result) {
	// Not needed for profiling
}

// OnRunnerEnd displays timing results
func (pc *ProfileTasksCallback) OnRunnerEnd(stats *RunStats) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	fmt.Fprintf(pc.output, "\nTask Profiling %s\n", strings.Repeat("=", 60))
	
	// Sort tasks by duration
	type taskTime struct {
		name     string
		duration time.Duration
	}
	
	var sorted []taskTime
	for name, duration := range pc.taskTimes {
		sorted = append(sorted, taskTime{name, duration})
	}
	
	// Sort by duration (longest first)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].duration > sorted[j].duration
	})
	
	// Display top tasks
	for i, tt := range sorted {
		if i >= 20 { // Show top 20
			break
		}
		fmt.Fprintf(pc.output, "%-50s : %v\n", tt.name, tt.duration)
	}
}