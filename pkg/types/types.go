// Package types provides core types and interfaces for the gosible library.
// These types are part of the public API and can be used by library consumers.
package types

import (
	"context"
	"io"
	"time"

	"gopkg.in/yaml.v3"
)

// ModuleType represents a type-safe module identifier
type ModuleType string

// Module type constants for type-safe task definitions
const (
	// Core system modules
	TypeFile    ModuleType = "file"
	TypeService ModuleType = "service"
	TypePackage ModuleType = "package"
	TypeUser    ModuleType = "user"
	TypeGroup   ModuleType = "group"

	// Content modules
	TypeCopy     ModuleType = "copy"
	TypeTemplate ModuleType = "template"

	// Execution modules
	TypeCommand ModuleType = "command"
	TypeShell   ModuleType = "shell"

	// Utility modules
	TypePing  ModuleType = "ping"
	TypeSetup ModuleType = "setup"
	TypeDebug ModuleType = "debug"
)

// String returns the string representation of the module type
func (m ModuleType) String() string {
	return string(m)
}

// IsValid checks if the module type is valid
func (m ModuleType) IsValid() bool {
	switch m {
	case TypeFile, TypeService, TypePackage, TypeUser, TypeGroup,
		TypeCopy, TypeTemplate, TypeCommand, TypeShell,
		TypePing, TypeSetup, TypeDebug:
		return true
	default:
		return false
	}
}

// AllModuleTypes returns all valid module types
func AllModuleTypes() []ModuleType {
	return []ModuleType{
		TypeFile, TypeService, TypePackage, TypeUser, TypeGroup,
		TypeCopy, TypeTemplate, TypeCommand, TypeShell,
		TypePing, TypeSetup, TypeDebug,
	}
}

// State constants for common module states
const (
	StatePresent    = "present"
	StateAbsent     = "absent"
	StateStarted    = "started"
	StateStopped    = "stopped"
	StateRestarted  = "restarted"
	StateReloaded   = "reloaded"
	StateLatest     = "latest"
	StateFile       = "file"
	StateDirectory  = "directory"
	StateLink       = "link"
	StateTouch      = "touch"
)

// DiffResult represents the before/after state for diff mode
type DiffResult struct {
	Before      string   `json:"before"`
	After       string   `json:"after"`
	BeforeLines []string `json:"before_lines,omitempty"`
	AfterLines  []string `json:"after_lines,omitempty"`
	Prepared    bool     `json:"prepared"`
	Diff        string   `json:"diff,omitempty"` // Unified diff format
}

// Result represents the outcome of a task execution
type Result struct {
	Success    bool                   `json:"success"`
	Changed    bool                   `json:"changed"`
	Message    string                 `json:"message,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	Duration   time.Duration          `json:"duration"`
	Error      error                  `json:"error,omitempty"`
	Host       string                 `json:"host"`
	TaskName   string                 `json:"task_name"`
	ModuleName string                 `json:"module_name"`
	Diff       *DiffResult            `json:"diff,omitempty"`     // Diff output for diff mode
	Simulated  bool                   `json:"simulated,omitempty"` // True when in check mode
}

// Host represents a target host in the inventory
type Host struct {
	Name      string                 `yaml:"name" json:"name"`
	Address   string                 `yaml:"address" json:"address"`
	Port      int                    `yaml:"port" json:"port"`
	User      string                 `yaml:"user" json:"user"`
	Password  string                 `yaml:"password,omitempty" json:"password,omitempty"`
	Variables map[string]interface{} `yaml:"vars,omitempty" json:"vars,omitempty"`
	Groups    []string               `yaml:"groups,omitempty" json:"groups,omitempty"`
}

// Group represents a collection of hosts
type Group struct {
	Name      string                 `yaml:"name" json:"name"`
	Hosts     []string               `yaml:"hosts,omitempty" json:"hosts,omitempty"`
	Children  []string               `yaml:"children,omitempty" json:"children,omitempty"`
	Variables map[string]interface{} `yaml:"vars,omitempty" json:"vars,omitempty"`
}

// Task represents a single automation task
type Task struct {
	Name         string                 `yaml:"name" json:"name"`
	Module       ModuleType             `yaml:"module" json:"module"`
	Args         map[string]interface{} `yaml:"args,omitempty" json:"args,omitempty"`
	When         interface{}            `yaml:"when,omitempty" json:"when,omitempty"`
	Loop         interface{}            `yaml:"loop,omitempty" json:"loop,omitempty"`
	Vars         map[string]interface{} `yaml:"vars,omitempty" json:"vars,omitempty"`
	Tags         []string               `yaml:"tags,omitempty" json:"tags,omitempty"`
	IgnoreErrors bool                   `yaml:"ignore_errors,omitempty" json:"ignore_errors,omitempty"`
	RunOnce      bool                   `yaml:"run_once,omitempty" json:"run_once,omitempty"`
	Delegate     string                 `yaml:"delegate_to,omitempty" json:"delegate_to,omitempty"`
	
	// Advanced conditional execution
	FailedWhen   interface{}            `yaml:"failed_when,omitempty" json:"failed_when,omitempty"`
	ChangedWhen  interface{}            `yaml:"changed_when,omitempty" json:"changed_when,omitempty"`
	Until        interface{}            `yaml:"until,omitempty" json:"until,omitempty"`
	Retries      int                    `yaml:"retries,omitempty" json:"retries,omitempty"`
	Delay        int                    `yaml:"delay,omitempty" json:"delay,omitempty"`
	
	// Loop control
	WithItems    interface{}            `yaml:"with_items,omitempty" json:"with_items,omitempty"`
	LoopControl  map[string]interface{} `yaml:"loop_control,omitempty" json:"loop_control,omitempty"`
	
	// Handler support
	Notify       []string               `yaml:"notify,omitempty" json:"notify,omitempty"`
	Listen       string                 `yaml:"listen,omitempty" json:"listen,omitempty"`
	
	// Task metadata
	Register     string                 `yaml:"register,omitempty" json:"register,omitempty"`
	Environment  map[string]string      `yaml:"environment,omitempty" json:"environment,omitempty"`
	Async        int                    `yaml:"async,omitempty" json:"async,omitempty"`
	Poll         int                    `yaml:"poll,omitempty" json:"poll,omitempty"`
	
	// Execution modes
	CheckMode    bool                   `yaml:"check_mode,omitempty" json:"check_mode,omitempty"`
	DiffMode     bool                   `yaml:"diff,omitempty" json:"diff,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshalling for Ansible-style task syntax
func (t *Task) UnmarshalYAML(value *yaml.Node) error {
	// First, try to unmarshal common task fields
	type TaskAlias Task // Avoid recursion
	var alias TaskAlias
	
	// Create a map to capture all fields
	var rawTask map[string]interface{}
	if err := value.Decode(&rawTask); err != nil {
		return err
	}
	
	// Extract known task fields
	if name, ok := rawTask["name"].(string); ok {
		alias.Name = name
		delete(rawTask, "name")
	}
	if module, ok := rawTask["module"].(string); ok {
		alias.Module = ModuleType(module)
		delete(rawTask, "module")
	}
	if args, ok := rawTask["args"].(map[string]interface{}); ok {
		alias.Args = args
		delete(rawTask, "args")
	}
	if when, ok := rawTask["when"]; ok {
		alias.When = when
		delete(rawTask, "when")
	}
	if loop, ok := rawTask["loop"]; ok {
		alias.Loop = loop
		delete(rawTask, "loop")
	}
	if vars, ok := rawTask["vars"].(map[string]interface{}); ok {
		alias.Vars = vars
		delete(rawTask, "vars")
	}
	if tags, ok := rawTask["tags"].([]interface{}); ok {
		alias.Tags = make([]string, len(tags))
		for i, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				alias.Tags[i] = tagStr
			}
		}
		delete(rawTask, "tags")
	}
	if ignoreErrors, ok := rawTask["ignore_errors"].(bool); ok {
		alias.IgnoreErrors = ignoreErrors
		delete(rawTask, "ignore_errors")
	}
	if runOnce, ok := rawTask["run_once"].(bool); ok {
		alias.RunOnce = runOnce
		delete(rawTask, "run_once")
	}
	if delegate, ok := rawTask["delegate_to"].(string); ok {
		alias.Delegate = delegate
		delete(rawTask, "delegate_to")
	}
	
	// Parse new advanced fields
	if failedWhen, ok := rawTask["failed_when"]; ok {
		alias.FailedWhen = failedWhen
		delete(rawTask, "failed_when")
	}
	if changedWhen, ok := rawTask["changed_when"]; ok {
		alias.ChangedWhen = changedWhen
		delete(rawTask, "changed_when")
	}
	if until, ok := rawTask["until"]; ok {
		alias.Until = until
		delete(rawTask, "until")
	}
	if retries, ok := rawTask["retries"].(int); ok {
		alias.Retries = retries
		delete(rawTask, "retries")
	}
	if delay, ok := rawTask["delay"].(int); ok {
		alias.Delay = delay
		delete(rawTask, "delay")
	}
	if withItems, ok := rawTask["with_items"]; ok {
		alias.WithItems = withItems
		delete(rawTask, "with_items")
	}
	if loopControl, ok := rawTask["loop_control"].(map[string]interface{}); ok {
		alias.LoopControl = loopControl
		delete(rawTask, "loop_control")
	}
	if notify, ok := rawTask["notify"]; ok {
		switch v := notify.(type) {
		case string:
			alias.Notify = []string{v}
		case []interface{}:
			alias.Notify = make([]string, len(v))
			for i, n := range v {
				if nStr, ok := n.(string); ok {
					alias.Notify[i] = nStr
				}
			}
		}
		delete(rawTask, "notify")
	}
	if listen, ok := rawTask["listen"].(string); ok {
		alias.Listen = listen
		delete(rawTask, "listen")
	}
	if register, ok := rawTask["register"].(string); ok {
		alias.Register = register
		delete(rawTask, "register")
	}
	if environment, ok := rawTask["environment"].(map[string]interface{}); ok {
		alias.Environment = make(map[string]string)
		for k, v := range environment {
			if vStr, ok := v.(string); ok {
				alias.Environment[k] = vStr
			}
		}
		delete(rawTask, "environment")
	}
	if async, ok := rawTask["async"].(int); ok {
		alias.Async = async
		delete(rawTask, "async")
	}
	if poll, ok := rawTask["poll"].(int); ok {
		alias.Poll = poll
		delete(rawTask, "poll")
	}
	
	// If module is not set, look for Ansible-style module syntax (e.g., "command: {...}")
	if alias.Module == "" {
		// Known module names that might be used as keys
		knownModules := []string{
			"command", "shell", "copy", "template", "file", "service", 
			"package", "user", "group", "debug", "setup", "lineinfile",
			"replace", "blockinfile", "fetch", "synchronize", "unarchive",
			"git", "apt", "yum", "pip", "systemd", "cron", "mount",
		}
		
		for _, moduleName := range knownModules {
			if moduleArgs, exists := rawTask[moduleName]; exists {
				alias.Module = ModuleType(moduleName)
				if moduleArgs != nil {
					if argsMap, ok := moduleArgs.(map[string]interface{}); ok {
						alias.Args = argsMap
					} else {
						// Handle case where module has no args (e.g., "setup:")
						alias.Args = make(map[string]interface{})
					}
				} else {
					alias.Args = make(map[string]interface{})
				}
				break
			}
		}
	}
	
	*t = Task(alias)
	return nil
}

// Play represents a collection of tasks to execute on hosts
type Play struct {
	Name      string                 `yaml:"name" json:"name"`
	Hosts     interface{}            `yaml:"hosts" json:"hosts"` // string or []string
	Vars      map[string]interface{} `yaml:"vars,omitempty" json:"vars,omitempty"`
	Tasks     []Task                 `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	PreTasks  []Task                 `yaml:"pre_tasks,omitempty" json:"pre_tasks,omitempty"`
	PostTasks []Task                 `yaml:"post_tasks,omitempty" json:"post_tasks,omitempty"`
	Handlers  []Task                 `yaml:"handlers,omitempty" json:"handlers,omitempty"`
	Tags      []string               `yaml:"tags,omitempty" json:"tags,omitempty"`
	Serial    int                    `yaml:"serial,omitempty" json:"serial,omitempty"`
	Strategy  string                 `yaml:"strategy,omitempty" json:"strategy,omitempty"`
}

// Playbook represents a collection of plays
type Playbook struct {
	Plays []Play `yaml:"plays" json:"plays"`
	Vars  map[string]interface{} `yaml:"vars,omitempty" json:"vars,omitempty"`
}

// ConnectionInfo contains connection details for a host
type ConnectionInfo struct {
	Type       string        `yaml:"type" json:"type"`
	Host       string        `yaml:"host" json:"host"`
	Port       int           `yaml:"port" json:"port"`
	User       string        `yaml:"user" json:"user"`
	Password   string        `yaml:"password,omitempty" json:"password,omitempty"`
	PrivateKey string        `yaml:"private_key,omitempty" json:"private_key,omitempty"`
	Timeout    time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Variables  map[string]interface{} `yaml:"vars,omitempty" json:"vars,omitempty"`
	
	// Windows/WinRM specific fields
	UseSSL     bool          `yaml:"use_ssl,omitempty" json:"use_ssl,omitempty"`
	SkipVerify bool          `yaml:"skip_verify,omitempty" json:"skip_verify,omitempty"`
}

// IsWindows returns true if this connection is for a Windows host
func (c ConnectionInfo) IsWindows() bool {
	return c.Type == "winrm"
}

// ExecuteOptions contains options for command execution
type ExecuteOptions struct {
	WorkingDir string
	Env        map[string]string
	Timeout    time.Duration
	User       string
	Sudo       bool
	Shell      string // For WinRM: "powershell" or "cmd"
	
	// Execution modes
	CheckMode    bool `json:"check_mode"`    // Don't make actual changes
	DiffMode     bool `json:"diff_mode"`     // Show what would change
	ForceHandler bool `json:"force_handler"` // Force handler execution
	
	// Streaming options
	StreamOutput     bool                              // Enable real-time streaming
	OutputCallback   func(line string, isStderr bool) // Real-time line callback
	ProgressCallback func(progress ProgressInfo)       // Progress updates
	
	// Module-specific options
	ModuleOptions map[string]interface{} `json:"module_options,omitempty"` // Module-specific overrides
}

// StepInfo contains detailed information about a specific step
type StepInfo struct {
	ID          string                 `json:"id"`          // Unique step identifier
	Name        string                 `json:"name"`        // Human-readable step name
	Description string                 `json:"description"` // Detailed step description
	Status      StepStatus             `json:"status"`      // Current step status
	StartTime   time.Time              `json:"start_time"`  // When step started
	EndTime     time.Time              `json:"end_time"`    // When step completed
	Duration    time.Duration          `json:"duration"`    // Step execution time
	Metadata    map[string]interface{} `json:"metadata"`    // Additional step data
}

// StepStatus represents the status of a step
type StepStatus string

const (
	StepPending    StepStatus = "pending"     // Not started yet
	StepRunning    StepStatus = "running"     // Currently executing
	StepCompleted  StepStatus = "completed"   // Successfully completed
	StepFailed     StepStatus = "failed"      // Failed with error
	StepSkipped    StepStatus = "skipped"     // Skipped due to conditions
	StepCancelled  StepStatus = "cancelled"   // Cancelled by user/system
)

// ProgressInfo contains progress information for long-running operations
type ProgressInfo struct {
	Stage       string    `json:"stage"`       // "connecting", "executing", "transferring"
	Percentage  float64   `json:"percentage"`  // 0-100
	Message     string    `json:"message"`     // Current operation description
	BytesTotal  int64     `json:"bytes_total"` // For file transfers
	BytesDone   int64     `json:"bytes_done"`  // For file transfers
	Timestamp   time.Time `json:"timestamp"`
	
	// NEW: Step tracking
	CurrentStep     *StepInfo   `json:"current_step,omitempty"`     // Currently executing step
	CompletedSteps  []StepInfo  `json:"completed_steps,omitempty"`  // Steps that have finished
	TotalSteps      int         `json:"total_steps,omitempty"`      // Total number of steps
	StepNumber      int         `json:"step_number,omitempty"`      // Current step number (1-based)
}

// StreamEventType represents the type of streaming event
type StreamEventType string

const (
	StreamStdout     StreamEventType = "stdout"      // Standard output line
	StreamStderr     StreamEventType = "stderr"      // Standard error line
	StreamProgress   StreamEventType = "progress"    // Progress update
	StreamDone       StreamEventType = "done"        // Command completed
	StreamError      StreamEventType = "error"       // Error occurred
	StreamStepStart  StreamEventType = "step_start"  // Step started
	StreamStepUpdate StreamEventType = "step_update" // Step progress update
	StreamStepEnd    StreamEventType = "step_end"    // Step completed
)

// StreamEvent represents a single event in the execution stream
type StreamEvent struct {
	Type      StreamEventType `json:"type"`
	Data      string         `json:"data,omitempty"`      // Output line or error message
	Progress  *ProgressInfo  `json:"progress,omitempty"`  // Progress information
	Step      *StepInfo      `json:"step,omitempty"`      // Step information (for step events)
	Result    *Result        `json:"result,omitempty"`    // Final result (only for "done" events)
	Error     error          `json:"error,omitempty"`     // Error (only for "error" events)
	Timestamp time.Time      `json:"timestamp"`
}

// Connection interface defines methods for connecting to and executing commands on hosts
type Connection interface {
	// Connect establishes a connection to the target host
	Connect(ctx context.Context, info ConnectionInfo) error
	
	// Execute runs a command on the target host
	Execute(ctx context.Context, command string, options ExecuteOptions) (*Result, error)
	
	// Copy transfers a file to the target host
	Copy(ctx context.Context, src io.Reader, dest string, mode int) error
	
	// Fetch retrieves a file from the target host
	Fetch(ctx context.Context, src string) (io.Reader, error)
	
	// Close terminates the connection
	Close() error
	
	// IsConnected returns true if the connection is active
	IsConnected() bool
}

// StreamingConnection extends Connection with streaming execution capabilities
type StreamingConnection interface {
	Connection
	
	// ExecuteStream runs a command with real-time output streaming
	ExecuteStream(ctx context.Context, command string, options ExecuteOptions) (<-chan StreamEvent, error)
}

// Module interface defines the contract for all automation modules
type Module interface {
	// Name returns the module name
	Name() string
	
	// Run executes the module with the given arguments
	Run(ctx context.Context, conn Connection, args map[string]interface{}) (*Result, error)
	
	// Validate checks if the module arguments are valid
	Validate(args map[string]interface{}) error
	
	// Documentation returns module documentation
	Documentation() ModuleDoc
}

// ModuleDoc contains module documentation
type ModuleDoc struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]ParamDoc `json:"parameters"`
	Examples    []string          `json:"examples"`
	Returns     map[string]string `json:"returns"`
}

// ParamDoc documents a module parameter
type ParamDoc struct {
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Type        string      `json:"type"`
	Choices     []string    `json:"choices,omitempty"`
}

// Inventory interface defines methods for managing hosts and groups
type Inventory interface {
	// GetHosts returns all hosts matching the pattern
	GetHosts(pattern string) ([]Host, error)
	
	// GetHost returns a specific host by name
	GetHost(name string) (*Host, error)
	
	// GetGroup returns a specific group by name
	GetGroup(name string) (*Group, error)
	
	// GetGroups returns all groups
	GetGroups() ([]Group, error)
	
	// AddHost adds a host to the inventory
	AddHost(host Host) error
	
	// AddGroup adds a group to the inventory
	AddGroup(group Group) error
	
	// GetHostVars returns variables for a specific host
	GetHostVars(hostname string) (map[string]interface{}, error)
	
	// GetGroupVars returns variables for a specific group
	GetGroupVars(groupname string) (map[string]interface{}, error)
}

// Runner interface defines the task execution engine
type Runner interface {
	// Run executes a single task on specified hosts
	Run(ctx context.Context, task Task, hosts []Host, vars map[string]interface{}) ([]Result, error)
	
	// RunPlay executes a play
	RunPlay(ctx context.Context, play Play, inventory Inventory, vars map[string]interface{}) ([]Result, error)
	
	// RunPlaybook executes a complete playbook
	RunPlaybook(ctx context.Context, playbook Playbook, inventory Inventory, vars map[string]interface{}) ([]Result, error)
	
	// SetMaxConcurrency sets the maximum number of concurrent executions
	SetMaxConcurrency(max int)
	
	// RegisterModule registers a module for use
	RegisterModule(module Module) error
	
	// GetModule returns a module by name
	GetModule(name string) (Module, error)
}

// TemplateEngine interface defines template rendering capabilities
type TemplateEngine interface {
	// Render processes a template string with the given variables
	Render(template string, vars map[string]interface{}) (string, error)
	
	// RenderFile processes a template file with the given variables
	RenderFile(filepath string, vars map[string]interface{}) (string, error)
	
	// AddFunction adds a custom function to the template engine
	AddFunction(name string, fn interface{}) error
}

// VarManager interface manages variables and facts
type VarManager interface {
	// SetVar sets a variable
	SetVar(key string, value interface{})
	
	// GetVar gets a variable
	GetVar(key string) (interface{}, bool)
	
	// SetVars sets multiple variables
	SetVars(vars map[string]interface{})
	
	// GetVars returns all variables
	GetVars() map[string]interface{}
	
	// GatherFacts collects system facts from a host
	GatherFacts(ctx context.Context, conn Connection) (map[string]interface{}, error)
	
	// MergeVars merges variables with proper precedence
	MergeVars(base, override map[string]interface{}) map[string]interface{}
}

// Config interface manages library configuration
type Config interface {
	// Get retrieves a configuration value
	Get(key string) interface{}
	
	// Set stores a configuration value
	Set(key string, value interface{})
	
	// Load loads configuration from file
	Load(filepath string) error
	
	// Save saves configuration to file
	Save(filepath string) error
	
	// GetDefaults returns default configuration values
	GetDefaults() map[string]interface{}
}

// EventType represents different types of events
type EventType string

const (
	EventTaskStart    EventType = "task_start"
	EventTaskComplete EventType = "task_complete" 
	EventTaskFailed   EventType = "task_failed"
	EventPlayStart    EventType = "play_start"
	EventPlayComplete EventType = "play_complete"
	EventError        EventType = "error"
)

// Event represents an execution event
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Host      string                 `json:"host,omitempty"`
	Task      string                 `json:"task,omitempty"`
	Play      string                 `json:"play,omitempty"`
	Result    *Result                `json:"result,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     error                  `json:"error,omitempty"`
}

// EventCallback is called when events occur
type EventCallback func(event Event)

// Logger interface for structured logging
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}