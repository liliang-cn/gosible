package modules

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gosinble/gosinble/pkg/types"
)

// LineInFileModule manages lines in text files with regex support
type LineInFileModule struct {
	*BaseModule
}

// NewLineInFileModule creates a new lineinfile module instance
func NewLineInFileModule() *LineInFileModule {
	doc := types.ModuleDoc{
		Name:        "lineinfile",
		Description: "Manage lines in text files",
		Parameters: map[string]types.ParamDoc{
			"path": {
				Description: "Path to the file to modify",
				Required:    true,
				Type:        "string",
			},
			"line": {
				Description: "The line to insert/ensure is present",
				Required:    false,
				Type:        "string",
			},
			"regexp": {
				Description: "Regular expression to find the line to replace",
				Required:    false,
				Type:        "string",
			},
			"state": {
				Description: "Whether the line should be present or absent",
				Required:    false,
				Type:        "string",
				Choices:     []string{"present", "absent"},
				Default:     "present",
			},
			"backup": {
				Description: "Create a backup file including the timestamp information",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"create": {
				Description: "Create the file if it does not exist",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"insertafter": {
				Description: "Insert the line after a line matching the pattern",
				Required:    false,
				Type:        "string",
			},
			"insertbefore": {
				Description: "Insert the line before a line matching the pattern",
				Required:    false,
				Type:        "string",
			},
			"firstmatch": {
				Description: "Only replace/remove the first occurrence of the line",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"validate": {
				Description: "Command to run to validate the file content",
				Required:    false,
				Type:        "string",
			},
		},
		Examples: []string{
			"- name: Ensure a line is present\n  lineinfile:\n    path: /etc/hosts\n    line: '127.0.0.1 localhost'",
			"- name: Replace a line using regex\n  lineinfile:\n    path: /etc/ssh/sshd_config\n    regexp: '^#?PermitRootLogin'\n    line: 'PermitRootLogin no'",
			"- name: Remove a line\n  lineinfile:\n    path: /etc/hosts\n    regexp: '192\\.168\\.1\\.100'\n    state: absent",
			"- name: Insert after a specific line\n  lineinfile:\n    path: /etc/fstab\n    insertafter: '# /etc/fstab'\n    line: '/dev/sdb1 /mnt/backup ext4 defaults 0 2'",
		},
		Returns: map[string]string{
			"backup_file": "Path to backup file if backup was created",
			"changed":     "Whether the file was modified",
			"msg":         "Description of the action taken",
		},
	}

	base := NewBaseModule("lineinfile", doc)

	// Set capabilities - lineinfile module supports both check and diff modes
	capabilities := &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     true,
		AsyncMode:    false,
		Platform:     "any",
		RequiresRoot: false,
	}
	base.SetCapabilities(capabilities)

	return &LineInFileModule{
		BaseModule: base,
	}
}

// LineInFileState represents the state of a file and line operations
type LineInFileState struct {
	Path         string   `json:"path"`
	Exists       bool     `json:"exists"`
	Lines        []string `json:"lines"`
	MatchedLine  int      `json:"matched_line"`  // -1 if not found
	InsertPoint  int      `json:"insert_point"`  // Where to insert new line
	BackupFile   string   `json:"backup_file,omitempty"`
	ModifiedTime string   `json:"modified_time,omitempty"`
}

// Validate validates the module arguments
func (m *LineInFileModule) Validate(args map[string]interface{}) error {
	// Path is required
	path := m.GetStringArg(args, "path", "")
	if path == "" {
		return types.NewValidationError("path", path, "required parameter")
	}

	// Either line or regexp must be provided for present state
	line := m.GetStringArg(args, "line", "")
	regexpStr := m.GetStringArg(args, "regexp", "")
	state := m.GetStringArg(args, "state", "present")

	if state == "present" && line == "" && regexpStr == "" {
		return types.NewValidationError("line/regexp", nil, "either line or regexp must be provided when state is present")
	}

	if state == "absent" && regexpStr == "" && line == "" {
		return types.NewValidationError("regexp/line", nil, "either regexp or line must be provided when state is absent")
	}

	// Validate state
	if state != "" {
		validStates := []string{"present", "absent"}
		valid := false
		for _, validState := range validStates {
			if state == validState {
				valid = true
				break
			}
		}
		if !valid {
			return types.NewValidationError("state", state, fmt.Sprintf("value must be one of: %v", validStates))
		}
	}

	// Validate regexp if provided
	if regexpStr != "" {
		if _, err := regexp.Compile(regexpStr); err != nil {
			return types.NewValidationError("regexp", regexpStr, fmt.Sprintf("invalid regular expression: %v", err))
		}
	}

	// Validate insertafter/insertbefore patterns if provided
	if insertafter := m.GetStringArg(args, "insertafter", ""); insertafter != "" {
		if _, err := regexp.Compile(insertafter); err != nil {
			return types.NewValidationError("insertafter", insertafter, fmt.Sprintf("invalid regular expression: %v", err))
		}
	}

	if insertbefore := m.GetStringArg(args, "insertbefore", ""); insertbefore != "" {
		if _, err := regexp.Compile(insertbefore); err != nil {
			return types.NewValidationError("insertbefore", insertbefore, fmt.Sprintf("invalid regular expression: %v", err))
		}
	}

	// insertafter and insertbefore are mutually exclusive
	insertAfter := m.GetStringArg(args, "insertafter", "")
	insertBefore := m.GetStringArg(args, "insertbefore", "")
	if insertAfter != "" && insertBefore != "" {
		return types.NewValidationError("insertafter/insertbefore", nil, "insertafter and insertbefore are mutually exclusive")
	}

	return nil
}

// Run executes the lineinfile module
func (m *LineInFileModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)

	// Check if we can use advanced mode features
	checkMode := m.CheckMode(args)
	diffMode := m.DiffMode(args)

	// Extract parameters
	filePath := m.GetStringArg(args, "path", "")
	line := m.GetStringArg(args, "line", "")
	regexpStr := m.GetStringArg(args, "regexp", "")
	state := m.GetStringArg(args, "state", "present")
	backup := m.GetBoolArg(args, "backup", false)
	create := m.GetBoolArg(args, "create", false)
	insertAfter := m.GetStringArg(args, "insertafter", "")
	insertBefore := m.GetStringArg(args, "insertbefore", "")
	firstMatch := m.GetBoolArg(args, "firstmatch", false)
	validate := m.GetStringArg(args, "validate", "")

	// Initialize result
	result := m.CreateResult(hostname, true, false, "", make(map[string]interface{}), nil)
	result.Data["path"] = filePath
	result.Data["state"] = state

	// Get current file state
	currentState, err := m.getFileState(ctx, conn, filePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Handle file not existing
	if !currentState.Exists {
		if !create {
			return nil, fmt.Errorf("file %s does not exist", filePath)
		}
		// Create empty file state
		currentState = &LineInFileState{
			Path:   filePath,
			Exists: false,
			Lines:  []string{},
		}
	}

	beforeState := *currentState
	var changes []string
	actuallyChanged := false

	// Prepare regex if provided
	var compiledRegexp *regexp.Regexp
	if regexpStr != "" {
		compiledRegexp, err = regexp.Compile(regexpStr)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %w", err)
		}
	}

	// Process line operations
	finalState := *currentState
	if state == "present" {
		changed, changeMsg, newState, err := m.handlePresentLine(ctx, conn, &finalState, line, compiledRegexp, insertAfter, insertBefore, firstMatch, checkMode)
		if err != nil {
			return nil, fmt.Errorf("failed to handle present line: %w", err)
		}
		if changed {
			changes = append(changes, changeMsg)
			finalState = *newState
			if !checkMode {
				actuallyChanged = true
			}
		}
	} else if state == "absent" {
		changed, changeMsg, newState, err := m.handleAbsentLine(ctx, conn, &finalState, line, compiledRegexp, firstMatch, checkMode)
		if err != nil {
			return nil, fmt.Errorf("failed to handle absent line: %w", err)
		}
		if changed {
			changes = append(changes, changeMsg)
			finalState = *newState
			if !checkMode {
				actuallyChanged = true
			}
		}
	}

	// Create backup if requested and changes were made
	var backupFile string
	if backup && actuallyChanged {
		backupFile, err = m.createBackup(ctx, conn, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
		finalState.BackupFile = backupFile
		result.Data["backup_file"] = backupFile
	}

	// Write file if changes were made and not in check mode
	if actuallyChanged && !checkMode {
		err = m.writeFile(ctx, conn, filePath, finalState.Lines)
		if err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}

		// Validate file if validation command provided
		if validate != "" {
			err = m.validateFile(ctx, conn, filePath, validate)
			if err != nil {
				return nil, fmt.Errorf("file validation failed: %w", err)
			}
		}
	}

	// Set result properties
	result.Changed = actuallyChanged || (checkMode && len(changes) > 0)
	result.Data["after_state"] = finalState
	result.Data["changes"] = changes

	if checkMode {
		result.Simulated = true
		result.Data["check_mode"] = true
	}

	// Generate diff if requested
	if diffMode && (actuallyChanged || (checkMode && len(changes) > 0)) {
		diff := m.generateFileDiff(&beforeState, &finalState, changes)
		result.Diff = diff
	}

	// Set appropriate message
	if len(changes) > 0 {
		if checkMode {
			result.Message = fmt.Sprintf("Would modify file %s: %s", filePath, strings.Join(changes, ", "))
		} else {
			result.Message = fmt.Sprintf("Modified file %s: %s", filePath, strings.Join(changes, ", "))
		}
	} else {
		result.Message = fmt.Sprintf("File %s is already in desired state", filePath)
	}

	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// getFileState reads the current state of the file
func (m *LineInFileModule) getFileState(ctx context.Context, conn types.Connection, filePath string) (*LineInFileState, error) {
	state := &LineInFileState{
		Path:        filePath,
		MatchedLine: -1,
		Lines:       []string{},
	}

	// Try to read the file
	result, err := conn.Execute(ctx, fmt.Sprintf("cat %s", filePath), types.ExecuteOptions{})
	if err != nil || !result.Success {
		// File doesn't exist or can't be read
		state.Exists = false
		return state, os.ErrNotExist
	}

	state.Exists = true
	if result.Message != "" {
		state.Lines = strings.Split(strings.TrimSuffix(result.Message, "\n"), "\n")
		// Handle empty file case
		if len(state.Lines) == 1 && state.Lines[0] == "" {
			state.Lines = []string{}
		}
	}

	return state, nil
}

// handlePresentLine ensures a line is present in the file
func (m *LineInFileModule) handlePresentLine(ctx context.Context, conn types.Connection, state *LineInFileState, line string, compiledRegexp *regexp.Regexp, insertAfter, insertBefore string, firstMatch, checkMode bool) (bool, string, *LineInFileState, error) {
	newState := *state
	
	// Find existing line if regexp is provided
	matchIndex := -1
	if compiledRegexp != nil {
		for i, existingLine := range state.Lines {
			if compiledRegexp.MatchString(existingLine) {
				matchIndex = i
				if firstMatch {
					break
				}
			}
		}
	} else if line != "" {
		// Look for exact line match
		for i, existingLine := range state.Lines {
			if existingLine == line {
				matchIndex = i
				if firstMatch {
					break
				}
			}
		}
	}

	// If line already exists and is correct, no change needed
	if matchIndex >= 0 && line != "" && state.Lines[matchIndex] == line {
		return false, "", &newState, nil
	}

	// Replace existing line
	if matchIndex >= 0 {
		newState.Lines = make([]string, len(state.Lines))
		copy(newState.Lines, state.Lines)
		newState.Lines[matchIndex] = line
		newState.MatchedLine = matchIndex
		return true, fmt.Sprintf("replaced line %d", matchIndex+1), &newState, nil
	}

	// Add new line if not found
	if line != "" {
		insertIndex := len(state.Lines) // Default: append to end
		
		// Find insert position
		if insertAfter != "" {
			afterRegexp, _ := regexp.Compile(insertAfter)
			for i, existingLine := range state.Lines {
				if afterRegexp.MatchString(existingLine) {
					insertIndex = i + 1
					break
				}
			}
		} else if insertBefore != "" {
			beforeRegexp, _ := regexp.Compile(insertBefore)
			for i, existingLine := range state.Lines {
				if beforeRegexp.MatchString(existingLine) {
					insertIndex = i
					break
				}
			}
		}

		// Insert line at determined position
		newState.Lines = make([]string, len(state.Lines)+1)
		copy(newState.Lines[:insertIndex], state.Lines[:insertIndex])
		newState.Lines[insertIndex] = line
		copy(newState.Lines[insertIndex+1:], state.Lines[insertIndex:])
		newState.InsertPoint = insertIndex

		return true, fmt.Sprintf("added line at position %d", insertIndex+1), &newState, nil
	}

	return false, "", &newState, nil
}

// handleAbsentLine ensures a line is absent from the file
func (m *LineInFileModule) handleAbsentLine(ctx context.Context, conn types.Connection, state *LineInFileState, line string, compiledRegexp *regexp.Regexp, firstMatch, checkMode bool) (bool, string, *LineInFileState, error) {
	newState := *state
	var removedLines []int

	// Find lines to remove
	for i := len(state.Lines) - 1; i >= 0; i-- { // Reverse order to maintain indices
		shouldRemove := false
		
		if compiledRegexp != nil && compiledRegexp.MatchString(state.Lines[i]) {
			shouldRemove = true
		} else if line != "" && state.Lines[i] == line {
			shouldRemove = true
		}

		if shouldRemove {
			removedLines = append([]int{i}, removedLines...) // Prepend to maintain order
			if firstMatch {
				break
			}
		}
	}

	if len(removedLines) == 0 {
		return false, "", &newState, nil
	}

	// Remove lines in reverse order to maintain indices
	newState.Lines = make([]string, len(state.Lines))
	copy(newState.Lines, state.Lines)
	
	for i := len(removedLines) - 1; i >= 0; i-- {
		lineIndex := removedLines[i]
		newState.Lines = append(newState.Lines[:lineIndex], newState.Lines[lineIndex+1:]...)
	}

	changeMsg := fmt.Sprintf("removed %d line(s)", len(removedLines))
	return true, changeMsg, &newState, nil
}

// createBackup creates a backup of the original file
func (m *LineInFileModule) createBackup(ctx context.Context, conn types.Connection, filePath string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s.backup", filePath, timestamp)
	
	result, err := conn.Execute(ctx, fmt.Sprintf("cp %s %s", filePath, backupPath), types.ExecuteOptions{})
	if err != nil || !result.Success {
		return "", fmt.Errorf("failed to create backup: %v", err)
	}

	return backupPath, nil
}

// writeFile writes the new content to the file
func (m *LineInFileModule) writeFile(ctx context.Context, conn types.Connection, filePath string, lines []string) error {
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n" // Add final newline
	}

	// Write to temporary file first for atomic operation
	tmpPath := filePath + ".tmp"
	
	// Use echo to write content (handles special characters better than echo)
	escapedContent := strings.ReplaceAll(content, "'", "'\"'\"'")
	cmd := fmt.Sprintf("echo -n '%s' > %s", escapedContent, tmpPath)
	
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return fmt.Errorf("failed to write temporary file: %v", err)
	}

	// Atomic move
	result, err = conn.Execute(ctx, fmt.Sprintf("mv %s %s", tmpPath, filePath), types.ExecuteOptions{})
	if err != nil || !result.Success {
		// Clean up temp file on failure
		conn.Execute(ctx, fmt.Sprintf("rm -f %s", tmpPath), types.ExecuteOptions{})
		return fmt.Errorf("failed to move file: %v", err)
	}

	return nil
}

// validateFile runs validation command on the file
func (m *LineInFileModule) validateFile(ctx context.Context, conn types.Connection, filePath, validateCmd string) error {
	// Replace %s in validation command with file path
	cmd := strings.ReplaceAll(validateCmd, "%s", filePath)
	
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return fmt.Errorf("validation command failed: %s", result.Message)
	}

	return nil
}

// generateFileDiff generates a diff between before and after file states
func (m *LineInFileModule) generateFileDiff(before, after *LineInFileState, changes []string) *types.DiffResult {
	beforeContent := strings.Join(before.Lines, "\n")
	afterContent := strings.Join(after.Lines, "\n")
	
	if len(before.Lines) > 0 {
		beforeContent += "\n"
	}
	if len(after.Lines) > 0 {
		afterContent += "\n"
	}

	return &types.DiffResult{
		Before:      beforeContent,
		After:       afterContent,
		BeforeLines: before.Lines,
		AfterLines:  after.Lines,
		Prepared:    true,
		Diff:        fmt.Sprintf("Changes: %s", strings.Join(changes, ", ")),
	}
}