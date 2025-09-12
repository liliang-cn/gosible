package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// BlockInFileModule manages text blocks in files
type BlockInFileModule struct {
	BaseModule
}

// BlockInFileState represents the current state of a file for block operations
type BlockInFileState struct {
	Path         string   `json:"path"`
	Exists       bool     `json:"exists"`
	Content      string   `json:"content"`
	Lines        []string `json:"lines"`
	BlockExists  bool     `json:"block_exists"`
	BlockStart   int      `json:"block_start"`   // Line number where block starts (-1 if not found)
	BlockEnd     int      `json:"block_end"`     // Line number where block ends (-1 if not found)
	BackupFile   string   `json:"backup_file,omitempty"`
}

// NewBlockInFileModule creates a new blockinfile module instance
func NewBlockInFileModule() *BlockInFileModule {
	return &BlockInFileModule{
		BaseModule: BaseModule{},
	}
}

// Name returns the module name
func (m *BlockInFileModule) Name() string {
	return "blockinfile"
}

// Capabilities returns the module capabilities
func (m *BlockInFileModule) Capabilities() *types.ModuleCapability {
	return &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     true,
		AsyncMode:    false,
		Platform:     "all",
		RequiresRoot: false,
	}
}

// Validate validates the module arguments
func (m *BlockInFileModule) Validate(args map[string]interface{}) error {
	// Required parameters
	path := m.GetStringArg(args, "path", "")
	if path == "" {
		return types.NewValidationError("path", nil, "path parameter is required")
	}
	
	// State validation
	state := m.GetStringArg(args, "state", "present")
	validStates := []string{"present", "absent"}
	if !m.isValidChoice(state, validStates) {
		return types.NewValidationError("state", state, fmt.Sprintf("state must be one of: %v", validStates))
	}
	
	// If state is present, block is required
	block := m.GetStringArg(args, "block", "")
	if state == "present" && block == "" {
		return types.NewValidationError("block", nil, "block parameter is required when state is present")
	}
	
	// Marker validation
	marker := m.GetStringArg(args, "marker", "# {mark} ANSIBLE MANAGED BLOCK")
	if !strings.Contains(marker, "{mark}") {
		return types.NewValidationError("marker", marker, "marker must contain {mark} placeholder")
	}
	
	// Insert location validation
	insertAfter := m.GetStringArg(args, "insertafter", "")
	insertBefore := m.GetStringArg(args, "insertbefore", "")
	
	if insertAfter != "" && insertBefore != "" {
		return types.NewValidationError("insertafter/insertbefore", nil, "insertafter and insertbefore are mutually exclusive")
	}
	
	return nil
}

// Run executes the blockinfile module
func (m *BlockInFileModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)
	diffMode := m.DiffMode(args)
	
	// Parse arguments
	path := m.GetStringArg(args, "path", "")
	block := m.GetStringArg(args, "block", "")
	state := m.GetStringArg(args, "state", "present")
	marker := m.GetStringArg(args, "marker", "# {mark} ANSIBLE MANAGED BLOCK")
	backup := m.GetBoolArg(args, "backup", false)
	create := m.GetBoolArg(args, "create", false)
	insertAfter := m.GetStringArg(args, "insertafter", "")
	insertBefore := m.GetStringArg(args, "insertbefore", "")
	validate := m.GetStringArg(args, "validate", "")
	
	// Generate begin and end markers
	beginMarker := strings.ReplaceAll(marker, "{mark}", "BEGIN")
	endMarker := strings.ReplaceAll(marker, "{mark}", "END")
	
	// Get current file state
	currentState, err := m.getFileState(ctx, conn, path, beginMarker, endMarker)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	if !currentState.Exists {
		if !create {
			return nil, fmt.Errorf("file %s does not exist", path)
		}
		// Initialize empty file state
		currentState.Exists = false
		currentState.Content = ""
		currentState.Lines = []string{}
	}
	
	// Store original state for diff mode
	beforeState := *currentState
	
	// Track changes
	var changes []string
	actuallyChanged := false
	
	// Initialize result
	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"path":         path,
		"before_state": beforeState,
		"block":        block,
		"state":        state,
		"marker":       marker,
	})
	
	// Process based on state
	if state == "present" {
		// Ensure block is present
		changed, changeMsg, newState, err := m.handlePresentBlock(ctx, conn, currentState, block, beginMarker, endMarker, insertAfter, insertBefore, checkMode)
		if err != nil {
			return nil, fmt.Errorf("failed to handle present block: %w", err)
		}
		
		if changed {
			changes = append(changes, changeMsg)
			currentState = newState
			if !checkMode {
				actuallyChanged = true
			}
		}
	} else if state == "absent" {
		// Remove block
		changed, changeMsg, newState, err := m.handleAbsentBlock(ctx, conn, currentState, beginMarker, endMarker, checkMode)
		if err != nil {
			return nil, fmt.Errorf("failed to handle absent block: %w", err)
		}
		
		if changed {
			changes = append(changes, changeMsg)
			currentState = newState
			if !checkMode {
				actuallyChanged = true
			}
		}
	}
	
	// Create backup if requested and changes were made
	if backup && actuallyChanged {
		backupFile, err := m.createBackup(ctx, conn, path)
		if err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
		currentState.BackupFile = backupFile
		result.Data["backup_file"] = backupFile
	}
	
	// Write file if changes were made and not in check mode
	if actuallyChanged && !checkMode {
		newContent := strings.Join(currentState.Lines, "\n")
		if len(currentState.Lines) > 0 {
			newContent += "\n"
		}
		
		err = m.writeFile(ctx, conn, path, newContent)
		if err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
		
		// Validate file if validation command provided
		if validate != "" {
			err = m.validateFile(ctx, conn, path, validate)
			if err != nil {
				return nil, fmt.Errorf("file validation failed: %w", err)
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
		beforeContent := beforeState.Content
		afterContent := strings.Join(currentState.Lines, "\n")
		if len(currentState.Lines) > 0 {
			afterContent += "\n"
		}
		diff := m.GenerateDiff(beforeContent, afterContent)
		result.Diff = diff
	}
	
	// Set appropriate message
	if len(changes) > 0 {
		if checkMode {
			result.Message = fmt.Sprintf("Would make changes: %s", strings.Join(changes, ", "))
		} else {
			result.Message = fmt.Sprintf("Made changes: %s", strings.Join(changes, ", "))
		}
	} else {
		result.Message = "Block is already in desired state"
	}
	
	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	return result, nil
}

// getFileState reads the current state of a file and analyzes block markers
func (m *BlockInFileModule) getFileState(ctx context.Context, conn types.Connection, path, beginMarker, endMarker string) (*BlockInFileState, error) {
	state := &BlockInFileState{
		Path:        path,
		Exists:      false,
		BlockExists: false,
		BlockStart:  -1,
		BlockEnd:    -1,
	}
	
	// Try to read the file
	result, err := conn.Execute(ctx, fmt.Sprintf("cat %s", path), types.ExecuteOptions{})
	if err != nil || !result.Success {
		// File doesn't exist or can't be read
		return state, nil
	}
	
	state.Exists = true
	state.Content = result.Message
	if state.Content != "" {
		state.Lines = strings.Split(strings.TrimSuffix(state.Content, "\n"), "\n")
	}
	
	// Find block markers
	for i, line := range state.Lines {
		if strings.Contains(line, beginMarker) {
			state.BlockStart = i
		}
		if strings.Contains(line, endMarker) && state.BlockStart != -1 {
			state.BlockEnd = i
			state.BlockExists = true
			break
		}
	}
	
	return state, nil
}

// handlePresentBlock ensures the block is present in the file
func (m *BlockInFileModule) handlePresentBlock(ctx context.Context, conn types.Connection, state *BlockInFileState, block, beginMarker, endMarker, insertAfter, insertBefore string, checkMode bool) (bool, string, *BlockInFileState, error) {
	newState := *state
	
	// Prepare block lines
	blockLines := []string{beginMarker}
	if block != "" {
		blockLines = append(blockLines, strings.Split(block, "\n")...)
	}
	blockLines = append(blockLines, endMarker)
	
	if state.BlockExists {
		// Block exists, check if content needs update
		existingBlockLines := state.Lines[state.BlockStart : state.BlockEnd+1]
		if m.blockContentEqual(existingBlockLines, blockLines) {
			return false, "", &newState, nil
		}
		
		// Update existing block
		newLines := make([]string, 0, len(state.Lines))
		newLines = append(newLines, state.Lines[:state.BlockStart]...)
		newLines = append(newLines, blockLines...)
		newLines = append(newLines, state.Lines[state.BlockEnd+1:]...)
		
		newState.Lines = newLines
		newState.BlockStart = state.BlockStart
		newState.BlockEnd = state.BlockStart + len(blockLines) - 1
		
		return true, "updated block in file", &newState, nil
	} else {
		// Block doesn't exist, need to insert it
		insertPos := m.findInsertPosition(state.Lines, insertAfter, insertBefore)
		
		newLines := make([]string, 0, len(state.Lines)+len(blockLines))
		newLines = append(newLines, state.Lines[:insertPos]...)
		newLines = append(newLines, blockLines...)
		newLines = append(newLines, state.Lines[insertPos:]...)
		
		newState.Lines = newLines
		newState.Exists = true
		newState.BlockExists = true
		newState.BlockStart = insertPos
		newState.BlockEnd = insertPos + len(blockLines) - 1
		
		return true, "inserted block into file", &newState, nil
	}
}

// handleAbsentBlock ensures the block is not present in the file
func (m *BlockInFileModule) handleAbsentBlock(ctx context.Context, conn types.Connection, state *BlockInFileState, beginMarker, endMarker string, checkMode bool) (bool, string, *BlockInFileState, error) {
	newState := *state
	
	if !state.BlockExists {
		return false, "", &newState, nil
	}
	
	// Remove block
	newLines := make([]string, 0, len(state.Lines))
	newLines = append(newLines, state.Lines[:state.BlockStart]...)
	newLines = append(newLines, state.Lines[state.BlockEnd+1:]...)
	
	newState.Lines = newLines
	newState.BlockExists = false
	newState.BlockStart = -1
	newState.BlockEnd = -1
	
	return true, "removed block from file", &newState, nil
}

// blockContentEqual compares two block content slices
func (m *BlockInFileModule) blockContentEqual(existing, new []string) bool {
	if len(existing) != len(new) {
		return false
	}
	
	for i, line := range existing {
		if line != new[i] {
			return false
		}
	}
	
	return true
}

// findInsertPosition finds where to insert the block
func (m *BlockInFileModule) findInsertPosition(lines []string, insertAfter, insertBefore string) int {
	if insertAfter != "" {
		for i, line := range lines {
			if strings.Contains(line, insertAfter) {
				return i + 1
			}
		}
	}
	
	if insertBefore != "" {
		for i, line := range lines {
			if strings.Contains(line, insertBefore) {
				return i
			}
		}
	}
	
	// Default to end of file
	return len(lines)
}

// createBackup creates a backup of the original file
func (m *BlockInFileModule) createBackup(ctx context.Context, conn types.Connection, path string) (string, error) {
	backupPath := fmt.Sprintf("%s.backup.%d", path, time.Now().Unix())
	
	// Copy file to backup location
	cmd := fmt.Sprintf("cp %s %s", path, backupPath)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return "", fmt.Errorf("failed to create backup: %v", err)
	}
	
	return backupPath, nil
}

// writeFile writes content to a file
func (m *BlockInFileModule) writeFile(ctx context.Context, conn types.Connection, path, content string) error {
	// Use a temporary file and atomic move for safety
	tempFile := fmt.Sprintf("%s.tmp.%d", path, time.Now().Unix())
	
	// Write content to temporary file
	writeCmd := fmt.Sprintf("echo -n %s > %s", m.shellEscape(content), tempFile)
	result, err := conn.Execute(ctx, writeCmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return fmt.Errorf("failed to write temporary file: %v", err)
	}
	
	// Move temporary file to final location
	moveCmd := fmt.Sprintf("mv %s %s", tempFile, path)
	result, err = conn.Execute(ctx, moveCmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		// Clean up temp file
		conn.Execute(ctx, fmt.Sprintf("rm -f %s", tempFile), types.ExecuteOptions{})
		return fmt.Errorf("failed to move file to final location: %v", err)
	}
	
	return nil
}

// validateFile runs a validation command on the file
func (m *BlockInFileModule) validateFile(ctx context.Context, conn types.Connection, path, validateCmd string) error {
	// Replace %s in validation command with the file path
	cmd := strings.ReplaceAll(validateCmd, "%s", path)
	
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return fmt.Errorf("validation command failed: %s", result.Message)
	}
	
	return nil
}

// shellEscape escapes a string for shell usage
func (m *BlockInFileModule) shellEscape(s string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", "'\"'\"'"))
}

// isValidChoice checks if a value is in a list of valid choices
func (m *BlockInFileModule) isValidChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}