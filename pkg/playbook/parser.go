// Package playbook provides playbook parsing and execution functionality.
package playbook

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/liliang-cn/gosible/pkg/types"
)

// Parser handles parsing of YAML playbook files
type Parser struct{}

// NewParser creates a new playbook parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile parses a playbook from a YAML file
func (p *Parser) ParseFile(filepath string) (*types.Playbook, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, types.NewPlaybookError(filepath, "", "", "failed to read playbook file", err)
	}

	return p.Parse(data, filepath)
}

// Parse parses a playbook from YAML data
func (p *Parser) Parse(data []byte, source string) (*types.Playbook, error) {
	// First try to parse as array of plays (standard format)
	var plays []types.Play
	if err := yaml.Unmarshal(data, &plays); err == nil && len(plays) > 0 {
		return &types.Playbook{
			Plays: plays,
		}, nil
	}

	// Try to parse as single play
	var singlePlay types.Play
	if err := yaml.Unmarshal(data, &singlePlay); err == nil && singlePlay.Name != "" {
		return &types.Playbook{
			Plays: []types.Play{singlePlay},
		}, nil
	}

	// Try to parse as playbook with metadata
	var playbookData struct {
		Vars  map[string]interface{} `yaml:"vars,omitempty"`
		Plays []types.Play          `yaml:"plays,omitempty"`
	}
	
	if err := yaml.Unmarshal(data, &playbookData); err != nil {
		return nil, types.NewPlaybookError(source, "", "", "failed to parse YAML", err)
	}

	playbook := &types.Playbook{
		Plays: playbookData.Plays,
		Vars:  playbookData.Vars,
	}

	// Validate the parsed playbook
	if err := p.validatePlaybook(playbook); err != nil {
		return nil, types.NewPlaybookError(source, "", "", "playbook validation failed", err)
	}

	// Post-process the playbook
	if err := p.processPlaybook(playbook); err != nil {
		return nil, types.NewPlaybookError(source, "", "", "playbook processing failed", err)
	}

	return playbook, nil
}

// validatePlaybook validates a parsed playbook
func (p *Parser) validatePlaybook(playbook *types.Playbook) error {
	if len(playbook.Plays) == 0 {
		return fmt.Errorf("playbook must contain at least one play")
	}

	for i, play := range playbook.Plays {
		if err := p.validatePlay(&play, i); err != nil {
			return err
		}
	}

	return nil
}

// validatePlay validates a single play
func (p *Parser) validatePlay(play *types.Play, index int) error {
	if play.Name == "" {
		return fmt.Errorf("play %d must have a name", index)
	}

	if play.Hosts == nil {
		return fmt.Errorf("play '%s' must specify hosts", play.Name)
	}

	// Validate hosts format
	switch hosts := play.Hosts.(type) {
	case string:
		if strings.TrimSpace(hosts) == "" {
			return fmt.Errorf("play '%s' hosts cannot be empty", play.Name)
		}
	case []interface{}:
		if len(hosts) == 0 {
			return fmt.Errorf("play '%s' hosts cannot be empty", play.Name)
		}
	default:
		return fmt.Errorf("play '%s' hosts must be string or array", play.Name)
	}

	// Validate tasks
	for i, task := range play.Tasks {
		if err := p.validateTask(&task, i, play.Name); err != nil {
			return err
		}
	}

	// Validate pre_tasks
	for i, task := range play.PreTasks {
		if err := p.validateTask(&task, i, play.Name); err != nil {
			return err
		}
	}

	// Validate post_tasks
	for i, task := range play.PostTasks {
		if err := p.validateTask(&task, i, play.Name); err != nil {
			return err
		}
	}

	// Validate handlers
	for i, handler := range play.Handlers {
		if err := p.validateTask(&handler, i, play.Name); err != nil {
			return err
		}
	}

	return nil
}

// validateTask validates a single task
func (p *Parser) validateTask(task *types.Task, index int, playName string) error {
	if task.Name == "" {
		return fmt.Errorf("task %d in play '%s' must have a name", index, playName)
	}

	if task.Module == "" {
		return fmt.Errorf("task '%s' in play '%s' must specify a module", task.Name, playName)
	}

	// Validate loop syntax
	if task.Loop != nil {
		switch loop := task.Loop.(type) {
		case string, []interface{}:
			// Valid loop formats
		default:
			return fmt.Errorf("task '%s' loop must be string or array, got %T", task.Name, loop)
		}
	}

	// Validate conditional syntax
	if task.When != nil {
		// Basic validation - in a full implementation, you'd parse the condition
		if whenStr, ok := task.When.(string); ok {
			if strings.TrimSpace(whenStr) == "" {
				return fmt.Errorf("task '%s' when condition cannot be empty", task.Name)
			}
		}
	}

	return nil
}

// processPlaybook performs post-processing on a parsed playbook
func (p *Parser) processPlaybook(playbook *types.Playbook) error {
	for i := range playbook.Plays {
		if err := p.processPlay(&playbook.Plays[i]); err != nil {
			return err
		}
	}
	return nil
}

// processPlay performs post-processing on a single play
func (p *Parser) processPlay(play *types.Play) error {
	// Normalize hosts to consistent format
	play.Hosts = p.normalizeHosts(play.Hosts)

	// Set default serial value
	if play.Serial == 0 {
		play.Serial = 1
	}

	// Set default strategy
	if play.Strategy == "" {
		play.Strategy = "linear"
	}

	// Process tasks
	for i := range play.Tasks {
		p.processTask(&play.Tasks[i])
	}

	for i := range play.PreTasks {
		p.processTask(&play.PreTasks[i])
	}

	for i := range play.PostTasks {
		p.processTask(&play.PostTasks[i])
	}

	for i := range play.Handlers {
		p.processTask(&play.Handlers[i])
	}

	return nil
}

// processTask performs post-processing on a single task
func (p *Parser) processTask(task *types.Task) {
	// Initialize maps if they're nil
	if task.Args == nil {
		task.Args = make(map[string]interface{})
	}
	if task.Vars == nil {
		task.Vars = make(map[string]interface{})
	}

	// Handle different module argument formats
	p.normalizeTaskArgs(task)

	// Set default tags
	if task.Tags == nil {
		task.Tags = make([]string, 0)
	}
}

// normalizeHosts normalizes the hosts field to a consistent format
func (p *Parser) normalizeHosts(hosts interface{}) interface{} {
	switch h := hosts.(type) {
	case string:
		return strings.TrimSpace(h)
	case []interface{}:
		// Convert to string slice
		result := make([]string, len(h))
		for i, item := range h {
			result[i] = fmt.Sprintf("%v", item)
		}
		return result
	default:
		// Return as-is for now
		return hosts
	}
}

// normalizeTaskArgs normalizes task arguments from various YAML formats
func (p *Parser) normalizeTaskArgs(task *types.Task) {
	// Handle module arguments specified at task level
	// This is a simplified version - real Ansible has complex argument parsing
	
	// If args is empty but we have other fields, they might be module args
	if len(task.Args) == 0 {
		// Common patterns: copy specific fields to args
		switch task.Module {
		case "command", "shell":
			// These modules often have 'cmd' as the main argument
			if cmd := p.findArgInTask(task, "cmd"); cmd != nil {
				task.Args["cmd"] = cmd
			}
		case "copy":
			if src := p.findArgInTask(task, "src"); src != nil {
				task.Args["src"] = src
			}
			if dest := p.findArgInTask(task, "dest"); dest != nil {
				task.Args["dest"] = dest
			}
		case "template":
			if src := p.findArgInTask(task, "src"); src != nil {
				task.Args["src"] = src
			}
			if dest := p.findArgInTask(task, "dest"); dest != nil {
				task.Args["dest"] = dest
			}
		}
	}
}

// findArgInTask looks for an argument in the task's raw YAML data
// This is a placeholder for more complex argument extraction
func (p *Parser) findArgInTask(task *types.Task, argName string) interface{} {
	// In a real implementation, this would examine the raw YAML node
	// For now, return nil since we don't have access to the raw data
	return nil
}

// ParseInventoryPattern parses host patterns from playbook
func (p *Parser) ParseInventoryPattern(pattern interface{}) []string {
	switch p := pattern.(type) {
	case string:
		// Split comma-separated patterns
		patterns := strings.Split(p, ",")
		result := make([]string, len(patterns))
		for i, pat := range patterns {
			result[i] = strings.TrimSpace(pat)
		}
		return result
	case []interface{}:
		result := make([]string, len(p))
		for i, item := range p {
			result[i] = fmt.Sprintf("%v", item)
		}
		return result
	case []string:
		return p
	default:
		return []string{}
	}
}

// ExtractTaskModule extracts module name from task
func (p *Parser) ExtractTaskModule(task map[string]interface{}) (string, map[string]interface{}, error) {
	// Look for known module names in the task
	knownModules := []string{
		"command", "shell", "copy", "template", "file", "service", "user", "group",
		"yum", "apt", "package", "systemd", "cron", "mount", "lineinfile",
		"debug", "set_fact", "include_tasks", "import_tasks", "include_vars",
		"pause", "wait_for", "uri", "get_url", "unarchive", "synchronize",
	}

	var moduleName string
	var moduleArgs map[string]interface{}

	for _, module := range knownModules {
		if args, exists := task[module]; exists {
			moduleName = module
			switch a := args.(type) {
			case map[string]interface{}:
				moduleArgs = a
			case string:
				// For modules like command/shell that can take a string
				moduleArgs = map[string]interface{}{"cmd": a}
			default:
				moduleArgs = map[string]interface{}{"args": a}
			}
			break
		}
	}

	if moduleName == "" {
		return "", nil, fmt.Errorf("no recognized module found in task")
	}

	return moduleName, moduleArgs, nil
}

// LoadIncludeFile loads an included file (tasks, vars, etc.)
func (p *Parser) LoadIncludeFile(filepath string) (interface{}, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read include file %s: %w", filepath, err)
	}

	var result interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse include file %s: %w", filepath, err)
	}

	return result, nil
}

// ValidatePlaybookStructure performs structural validation of a playbook
func (p *Parser) ValidatePlaybookStructure(data []byte) error {
	// Try to parse as generic YAML first
	var yamlData interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	// Check if it's an array (list of plays) or object (single play or playbook)
	switch yamlData.(type) {
	case []interface{}:
		// Array format - should be list of plays
		return p.validatePlaysArray(yamlData)
	case map[string]interface{}:
		// Object format - could be single play or full playbook
		return p.validatePlaybookObject(yamlData)
	default:
		return fmt.Errorf("playbook must be YAML object or array")
	}
}

func (p *Parser) validatePlaysArray(data interface{}) error {
	plays, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("expected array of plays")
	}

	if len(plays) == 0 {
		return fmt.Errorf("playbook must contain at least one play")
	}

	for i, play := range plays {
		playMap, ok := play.(map[string]interface{})
		if !ok {
			return fmt.Errorf("play %d must be an object", i)
		}

		if err := p.validatePlayObject(playMap, i); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) validatePlaybookObject(data interface{}) error {
	obj, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected playbook object")
	}

	// Check if this is a full playbook (with 'plays' key) or a single play
	if plays, hasPlays := obj["plays"]; hasPlays {
		// Full playbook format
		return p.validatePlaysArray(plays)
	} else if _, hasHosts := obj["hosts"]; hasHosts {
		// Single play format
		return p.validatePlayObject(obj, 0)
	} else {
		return fmt.Errorf("playbook must contain either 'plays' array or be a single play with 'hosts'")
	}
}

func (p *Parser) validatePlayObject(playObj map[string]interface{}, index int) error {
	// Check required fields
	if _, hasName := playObj["name"]; !hasName {
		return fmt.Errorf("play %d must have a 'name'", index)
	}

	if _, hasHosts := playObj["hosts"]; !hasHosts {
		return fmt.Errorf("play %d must have 'hosts'", index)
	}

	return nil
}