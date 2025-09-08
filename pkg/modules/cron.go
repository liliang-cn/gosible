package modules

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// CronModule manages cron jobs
type CronModule struct {
	BaseModule
}

// CronEntry represents a single cron job entry
type CronEntry struct {
	Minute     string `json:"minute"`
	Hour       string `json:"hour"`
	Day        string `json:"day"`
	Month      string `json:"month"`
	Weekday    string `json:"weekday"`
	Command    string `json:"command"`
	Comment    string `json:"comment"`
	User       string `json:"user"`
	Disabled   bool   `json:"disabled"`
	Raw        string `json:"raw"`
}

// CronState represents the current state of cron jobs
type CronState struct {
	User    string       `json:"user"`
	Jobs    []*CronEntry `json:"jobs"`
	RawCron string       `json:"raw_cron"`
}

// NewCronModule creates a new cron module instance
func NewCronModule() *CronModule {
	return &CronModule{
		BaseModule: BaseModule{},
	}
}

// Name returns the module name
func (m *CronModule) Name() string {
	return "cron"
}

// Capabilities returns the module capabilities
func (m *CronModule) Capabilities() *types.ModuleCapability {
	return &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     true,
		AsyncMode:    false,
		Platform:     "linux",
		RequiresRoot: false,
	}
}

// Validate validates the module arguments
func (m *CronModule) Validate(args map[string]interface{}) error {
	job := m.GetStringArg(args, "job", "")
	name := m.GetStringArg(args, "name", "")
	state := m.GetStringArg(args, "state", "present")
	
	// Either job or name is required
	if job == "" && name == "" {
		return types.NewValidationError("job/name", nil, "either job or name parameter is required")
	}
	
	// Validate state
	validStates := []string{"present", "absent"}
	if !m.isValidChoice(state, validStates) {
		return types.NewValidationError("state", state, fmt.Sprintf("value must be one of: %v", validStates))
	}
	
	// Validate cron time format if individual parameters are provided
	minute := m.GetStringArg(args, "minute", "*")
	hour := m.GetStringArg(args, "hour", "*")
	day := m.GetStringArg(args, "day", "*")
	month := m.GetStringArg(args, "month", "*")
	weekday := m.GetStringArg(args, "weekday", "*")
	
	// Always validate time parameters if they are explicitly provided
	if _, exists := args["minute"]; exists {
		if err := m.validateCronTime(minute, "minute", 0, 59); err != nil {
			return err
		}
	}
	if _, exists := args["hour"]; exists {
		if err := m.validateCronTime(hour, "hour", 0, 23); err != nil {
			return err
		}
	}
	if _, exists := args["day"]; exists {
		if err := m.validateCronTime(day, "day", 1, 31); err != nil {
			return err
		}
	}
	if _, exists := args["month"]; exists {
		if err := m.validateCronTime(month, "month", 1, 12); err != nil {
			return err
		}
	}
	if _, exists := args["weekday"]; exists {
		if err := m.validateCronTime(weekday, "weekday", 0, 7); err != nil {
			return err
		}
	}
	
	// Validate user if provided
	user := m.GetStringArg(args, "user", "")
	if user != "" {
		// Basic validation - user should be alphanumeric with some special chars
		if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(user) {
			return types.NewValidationError("user", user, "invalid username format")
		}
	}
	
	return nil
}

// validateCronTime validates a cron time field
func (m *CronModule) validateCronTime(value, field string, min, max int) error {
	if value == "*" || value == "" {
		return nil
	}
	
	// Handle ranges (e.g., "1-5")
	if strings.Contains(value, "-") {
		parts := strings.Split(value, "-")
		if len(parts) != 2 {
			return types.NewValidationError(field, value, "invalid range format")
		}
		start, err1 := strconv.Atoi(parts[0])
		end, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return types.NewValidationError(field, value, "range values must be numeric")
		}
		if start < min || start > max || end < min || end > max {
			return types.NewValidationError(field, value, fmt.Sprintf("range values must be between %d and %d", min, max))
		}
		return nil
	}
	
	// Handle step values (e.g., "*/5")
	if strings.Contains(value, "/") {
		parts := strings.Split(value, "/")
		if len(parts) != 2 {
			return types.NewValidationError(field, value, "invalid step format")
		}
		if parts[0] != "*" {
			base, err := strconv.Atoi(parts[0])
			if err != nil {
				return types.NewValidationError(field, value, "step base must be numeric or *")
			}
			if base < min || base > max {
				return types.NewValidationError(field, value, fmt.Sprintf("step base must be between %d and %d", min, max))
			}
		}
		step, err := strconv.Atoi(parts[1])
		if err != nil || step <= 0 {
			return types.NewValidationError(field, value, "step value must be a positive number")
		}
		return nil
	}
	
	// Handle comma-separated values (e.g., "1,3,5")
	if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			if err := m.validateCronTime(strings.TrimSpace(part), field, min, max); err != nil {
				return err
			}
		}
		return nil
	}
	
	// Single numeric value
	num, err := strconv.Atoi(value)
	if err != nil {
		return types.NewValidationError(field, value, "value must be numeric, *, or a valid cron expression")
	}
	if num < min || num > max {
		return types.NewValidationError(field, value, fmt.Sprintf("value must be between %d and %d", min, max))
	}
	
	return nil
}

// Run executes the cron module
func (m *CronModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)
	diffMode := m.DiffMode(args)
	
	// Parse arguments
	job := m.GetStringArg(args, "job", "")
	name := m.GetStringArg(args, "name", "")
	state := m.GetStringArg(args, "state", "present")
	user := m.GetStringArg(args, "user", "")
	backup := m.GetBoolArg(args, "backup", false)
	disabled := m.GetBoolArg(args, "disabled", false)
	
	// Cron time parameters
	minute := m.GetStringArg(args, "minute", "*")
	hour := m.GetStringArg(args, "hour", "*")
	day := m.GetStringArg(args, "day", "*")
	month := m.GetStringArg(args, "month", "*")
	weekday := m.GetStringArg(args, "weekday", "*")
	
	// Get current cron state
	currentState, err := m.getCronState(ctx, conn, user)
	if err != nil {
		return nil, fmt.Errorf("failed to get cron state: %w", err)
	}
	
	// Store original state for diff mode
	beforeState := *currentState
	
	// Find existing job
	var existingJob *CronEntry
	if name != "" {
		existingJob = m.findJobByName(currentState.Jobs, name)
	} else if job != "" {
		existingJob = m.findJobByCommand(currentState.Jobs, job)
	}
	
	// Track changes
	var changes []string
	actuallyChanged := false
	
	// Initialize result
	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"user":         user,
		"before_state": beforeState,
	})
	
	// Process based on state
	if state == "present" {
		// Build the cron job entry
		var cronEntry *CronEntry
		if job != "" && len(strings.Fields(job)) >= 6 {
			// Parse full cron line format (minute hour day month weekday command...)
			cronEntry, err = m.parseCronJob(job, name)
			if err != nil {
				return nil, fmt.Errorf("failed to parse cron job: %w", err)
			}
		} else {
			// Build from parameters (job is just the command)
			command := job
			if command == "" {
				return nil, fmt.Errorf("command is required when individual time parameters are used")
			}
			
			cronEntry = &CronEntry{
				Minute:   minute,
				Hour:     hour,
				Day:      day,
				Month:    month,
				Weekday:  weekday,
				Command:  command,
				Comment:  name,
				User:     user,
				Disabled: disabled,
			}
		}
		
		// Check if job needs to be added or updated
		if existingJob == nil {
			// Add new job
			if checkMode {
				changes = append(changes, fmt.Sprintf("would add cron job: %s", m.formatCronEntry(cronEntry)))
				currentState.Jobs = append(currentState.Jobs, cronEntry)
			} else {
				// Add job to state first
				currentState.Jobs = append(currentState.Jobs, cronEntry)
				if err := m.addCronJob(ctx, conn, currentState, cronEntry, backup); err != nil {
					return nil, fmt.Errorf("failed to add cron job: %w", err)
				}
				changes = append(changes, fmt.Sprintf("added cron job: %s", m.formatCronEntry(cronEntry)))
				actuallyChanged = true
			}
		} else {
			// Check if existing job needs update
			if m.cronJobChanged(existingJob, cronEntry) {
				if checkMode {
					changes = append(changes, fmt.Sprintf("would update cron job: %s", m.formatCronEntry(cronEntry)))
					m.updateJobInPlace(currentState.Jobs, existingJob, cronEntry)
				} else {
					// Update job in state first
					m.updateJobInPlace(currentState.Jobs, existingJob, cronEntry)
					if err := m.updateCronJob(ctx, conn, currentState, existingJob, cronEntry, backup); err != nil {
						return nil, fmt.Errorf("failed to update cron job: %w", err)
					}
					changes = append(changes, fmt.Sprintf("updated cron job: %s", m.formatCronEntry(cronEntry)))
					actuallyChanged = true
				}
			}
		}
	} else if state == "absent" {
		// Remove job
		if existingJob != nil {
			if checkMode {
				changes = append(changes, fmt.Sprintf("would remove cron job: %s", m.formatCronEntry(existingJob)))
				currentState.Jobs = m.removeJobFromList(currentState.Jobs, existingJob)
			} else {
				// Remove job from state first
				currentState.Jobs = m.removeJobFromList(currentState.Jobs, existingJob)
				if err := m.removeCronJob(ctx, conn, currentState, existingJob, backup); err != nil {
					return nil, fmt.Errorf("failed to remove cron job: %w", err)
				}
				changes = append(changes, fmt.Sprintf("removed cron job: %s", m.formatCronEntry(existingJob)))
				actuallyChanged = true
			}
		}
	}
	
	// Set result properties
	result.Changed = actuallyChanged || (checkMode && len(changes) > 0)
	result.Data["after_state"] = *currentState
	result.Data["changes"] = changes
	
	if checkMode {
		result.Simulated = true
		result.Data["check_mode"] = true
	}
	
	// Generate diff if requested
	if diffMode && (actuallyChanged || (checkMode && len(changes) > 0)) {
		diff := m.generateCronDiff(&beforeState, currentState, changes)
		result.Diff = diff
	}
	
	// Set appropriate message
	if len(changes) > 0 {
		if checkMode {
			result.Message = fmt.Sprintf("Would make changes to cron: %s", strings.Join(changes, ", "))
		} else {
			result.Message = fmt.Sprintf("Changed cron: %s", strings.Join(changes, ", "))
		}
	} else {
		result.Message = "Cron is already in desired state"
	}
	
	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	return result, nil
}

// getCronState retrieves the current cron state for a user
func (m *CronModule) getCronState(ctx context.Context, conn types.Connection, user string) (*CronState, error) {
	state := &CronState{
		User: user,
		Jobs: []*CronEntry{},
	}
	
	// Build crontab command
	var cmd string
	if user != "" {
		cmd = fmt.Sprintf("crontab -u %s -l", user)
	} else {
		cmd = "crontab -l"
	}
	
	// Execute command
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		// crontab returns 1 if no crontab exists, which is normal
		if result != nil {
			message := result.Message
			if stderr, exists := result.Data["stderr"]; exists && stderr != nil {
				if stderrStr, ok := stderr.(string); ok {
					message += " " + stderrStr
				}
			}
			if strings.Contains(message, "no crontab") {
				return state, nil
			}
		}
		// For other errors (like permission denied), return error
		if result != nil {
			return nil, fmt.Errorf("failed to read crontab: %s", result.Message)
		}
		return nil, fmt.Errorf("failed to read crontab: %w", err)
	}
	
	state.RawCron = result.Message
	
	// Parse cron entries
	lines := strings.Split(state.RawCron, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		entry, err := m.parseCronLine(line)
		if err != nil {
			continue // Skip malformed lines
		}
		if entry != nil {
			state.Jobs = append(state.Jobs, entry)
		}
	}
	
	return state, nil
}

// parseCronLine parses a single cron line
func (m *CronModule) parseCronLine(line string) (*CronEntry, error) {
	// Skip comments that are not job comments
	if strings.HasPrefix(line, "#") && !strings.Contains(line, "Ansible:") {
		return nil, nil
	}
	
	entry := &CronEntry{Raw: line}
	
	// Handle disabled jobs (commented out)
	if strings.HasPrefix(line, "#") {
		entry.Disabled = true
		line = strings.TrimPrefix(line, "#")
		line = strings.TrimSpace(line)
	}
	
	// Extract comment if present
	if strings.Contains(line, "#") {
		parts := strings.SplitN(line, "#", 2)
		if len(parts) == 2 {
			line = strings.TrimSpace(parts[0])
			comment := strings.TrimSpace(parts[1])
			if strings.HasPrefix(comment, "Ansible:") {
				entry.Comment = strings.TrimSpace(strings.TrimPrefix(comment, "Ansible:"))
			} else {
				entry.Comment = comment
			}
		}
	}
	
	// Parse cron fields
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return nil, fmt.Errorf("invalid cron line format")
	}
	
	entry.Minute = fields[0]
	entry.Hour = fields[1]
	entry.Day = fields[2]
	entry.Month = fields[3]
	entry.Weekday = fields[4]
	entry.Command = strings.Join(fields[5:], " ")
	
	return entry, nil
}

// parseCronJob parses a job parameter in cron format
func (m *CronModule) parseCronJob(job, name string) (*CronEntry, error) {
	fields := strings.Fields(job)
	if len(fields) < 6 {
		return nil, fmt.Errorf("invalid cron job format, expected: minute hour day month weekday command")
	}
	
	return &CronEntry{
		Minute:  fields[0],
		Hour:    fields[1],
		Day:     fields[2],
		Month:   fields[3],
		Weekday: fields[4],
		Command: strings.Join(fields[5:], " "),
		Comment: name,
		Raw:     job,
	}, nil
}

// findJobByName finds a cron job by its comment/name
func (m *CronModule) findJobByName(jobs []*CronEntry, name string) *CronEntry {
	for _, job := range jobs {
		if job.Comment == name {
			return job
		}
	}
	return nil
}

// findJobByCommand finds a cron job by its command
func (m *CronModule) findJobByCommand(jobs []*CronEntry, command string) *CronEntry {
	for _, job := range jobs {
		if job.Command == command {
			return job
		}
	}
	return nil
}

// cronJobChanged checks if a cron job has changed
func (m *CronModule) cronJobChanged(existing, new *CronEntry) bool {
	return existing.Minute != new.Minute ||
		existing.Hour != new.Hour ||
		existing.Day != new.Day ||
		existing.Month != new.Month ||
		existing.Weekday != new.Weekday ||
		existing.Command != new.Command ||
		existing.Comment != new.Comment ||
		existing.Disabled != new.Disabled
}

// updateJobInPlace updates a job in the jobs list
func (m *CronModule) updateJobInPlace(jobs []*CronEntry, existing, new *CronEntry) {
	for i, job := range jobs {
		if job == existing {
			jobs[i] = new
			break
		}
	}
}

// removeJobFromList removes a job from the jobs list
func (m *CronModule) removeJobFromList(jobs []*CronEntry, toRemove *CronEntry) []*CronEntry {
	result := make([]*CronEntry, 0, len(jobs))
	for _, job := range jobs {
		if job != toRemove {
			result = append(result, job)
		}
	}
	return result
}

// addCronJob adds a new cron job
func (m *CronModule) addCronJob(ctx context.Context, conn types.Connection, state *CronState, entry *CronEntry, backup bool) error {
	return m.writeCronTab(ctx, conn, state, backup)
}

// updateCronJob updates an existing cron job
func (m *CronModule) updateCronJob(ctx context.Context, conn types.Connection, state *CronState, existing, new *CronEntry, backup bool) error {
	return m.writeCronTab(ctx, conn, state, backup)
}

// removeCronJob removes a cron job
func (m *CronModule) removeCronJob(ctx context.Context, conn types.Connection, state *CronState, job *CronEntry, backup bool) error {
	return m.writeCronTab(ctx, conn, state, backup)
}

// writeCronTab writes the crontab
func (m *CronModule) writeCronTab(ctx context.Context, conn types.Connection, state *CronState, backup bool) error {
	// Create backup if requested
	if backup {
		if err := m.createCronBackup(ctx, conn, state.User); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}
	
	// Generate new crontab content
	content := m.generateCronContent(state.Jobs)
	
	// Write to temporary file and install
	tempFile := fmt.Sprintf("/tmp/crontab_%d", time.Now().Unix())
	
	// Write content to temp file
	writeCmd := fmt.Sprintf("echo %s > %s", m.shellEscape(content), tempFile)
	result, err := conn.Execute(ctx, writeCmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return fmt.Errorf("failed to write temporary crontab: %v", err)
	}
	
	// Install the crontab
	var installCmd string
	if state.User != "" {
		installCmd = fmt.Sprintf("crontab -u %s %s", state.User, tempFile)
	} else {
		installCmd = fmt.Sprintf("crontab %s", tempFile)
	}
	
	result, err = conn.Execute(ctx, installCmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		// Clean up temp file
		conn.Execute(ctx, fmt.Sprintf("rm -f %s", tempFile), types.ExecuteOptions{})
		return fmt.Errorf("failed to install crontab: %v", err)
	}
	
	// Clean up temp file
	conn.Execute(ctx, fmt.Sprintf("rm -f %s", tempFile), types.ExecuteOptions{})
	
	return nil
}

// generateCronContent generates crontab content from jobs
func (m *CronModule) generateCronContent(jobs []*CronEntry) string {
	var lines []string
	
	for _, job := range jobs {
		line := m.formatCronEntry(job)
		if job.Disabled {
			line = "#" + line
		}
		lines = append(lines, line)
	}
	
	return strings.Join(lines, "\n") + "\n"
}

// formatCronEntry formats a cron entry as a cron line
func (m *CronModule) formatCronEntry(entry *CronEntry) string {
	cronLine := fmt.Sprintf("%s %s %s %s %s %s",
		entry.Minute, entry.Hour, entry.Day, entry.Month, entry.Weekday, entry.Command)
	
	if entry.Comment != "" {
		cronLine += fmt.Sprintf(" # Ansible: %s", entry.Comment)
	}
	
	return cronLine
}

// createCronBackup creates a backup of the current crontab
func (m *CronModule) createCronBackup(ctx context.Context, conn types.Connection, user string) error {
	backupFile := fmt.Sprintf("/tmp/crontab_backup_%d", time.Now().Unix())
	
	var cmd string
	if user != "" {
		cmd = fmt.Sprintf("crontab -u %s -l > %s 2>/dev/null || true", user, backupFile)
	} else {
		cmd = fmt.Sprintf("crontab -l > %s 2>/dev/null || true", backupFile)
	}
	
	_, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err
}

// shellEscape escapes a string for shell usage
func (m *CronModule) shellEscape(s string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", "'\"'\"'"))
}

// generateCronDiff generates a diff for cron changes
func (m *CronModule) generateCronDiff(before, after *CronState, changes []string) *types.DiffResult {
	beforeContent := m.generateCronContent(before.Jobs)
	afterContent := m.generateCronContent(after.Jobs)
	
	return m.GenerateDiff(beforeContent, afterContent)
}

// isValidChoice checks if a value is in a list of valid choices
func (m *CronModule) isValidChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}