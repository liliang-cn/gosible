package modules

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gosinble/gosinble/pkg/types"
)

// ReplaceModule performs find and replace operations on files
type ReplaceModule struct {
	BaseModule
}

// ReplaceState represents the current state of a file for replacement operations
type ReplaceState struct {
	Path       string   `json:"path"`
	Exists     bool     `json:"exists"`
	Content    string   `json:"content"`
	Lines      []string `json:"lines"`
	BackupFile string   `json:"backup_file,omitempty"`
}

// NewReplaceModule creates a new replace module instance
func NewReplaceModule() *ReplaceModule {
	return &ReplaceModule{
		BaseModule: BaseModule{},
	}
}

// Name returns the module name
func (m *ReplaceModule) Name() string {
	return "replace"
}

// Capabilities returns the module capabilities
func (m *ReplaceModule) Capabilities() *types.ModuleCapability {
	return &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     true,
		AsyncMode:    false,
		Platform:     "all",
		RequiresRoot: false,
	}
}

// Validate validates the module arguments
func (m *ReplaceModule) Validate(args map[string]interface{}) error {
	// Required parameters
	path := m.GetStringArg(args, "path", "")
	if path == "" {
		return types.NewValidationError("path", nil, "path parameter is required")
	}
	
	regexpStr := m.GetStringArg(args, "regexp", "")
	if regexpStr == "" {
		return types.NewValidationError("regexp", nil, "regexp parameter is required")
	}
	
	// Validate regexp syntax
	if _, err := regexp.Compile(regexpStr); err != nil {
		return types.NewValidationError("regexp", regexpStr, fmt.Sprintf("invalid regular expression: %v", err))
	}
	
	// Replace parameter is optional (can be empty string to remove matches)
	
	// Optional parameters validation
	encoding := m.GetStringArg(args, "encoding", "")
	if encoding != "" {
		validEncodings := []string{"utf-8", "latin1", "ascii"}
		valid := false
		for _, validEncoding := range validEncodings {
			if encoding == validEncoding {
				valid = true
				break
			}
		}
		if !valid {
			return types.NewValidationError("encoding", encoding, fmt.Sprintf("encoding must be one of: %v", validEncodings))
		}
	}
	
	return nil
}

// Run executes the replace module
func (m *ReplaceModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)
	diffMode := m.DiffMode(args)
	
	// Parse arguments
	path := m.GetStringArg(args, "path", "")
	regexStr := m.GetStringArg(args, "regexp", "")
	replace := m.GetStringArg(args, "replace", "")
	backup := m.GetBoolArg(args, "backup", false)
	validate := m.GetStringArg(args, "validate", "")
	// encoding := m.GetStringArg(args, "encoding", "utf-8") // Currently unused
	
	// Compile regexp
	compiledRegex, err := regexp.Compile(regexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression: %w", err)
	}
	
	// Get current file state
	currentState, err := m.getFileState(ctx, conn, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	if !currentState.Exists {
		return nil, fmt.Errorf("file %s does not exist", path)
	}
	
	// Store original state for diff mode
	beforeState := *currentState
	
	// Perform replacement
	originalContent := currentState.Content
	newContent := compiledRegex.ReplaceAllString(originalContent, replace)
	
	// Check if content changed
	contentChanged := originalContent != newContent
	var changes []string
	actuallyChanged := false
	
	// Initialize result
	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"path":         path,
		"before_state": beforeState,
		"regexp":       regexStr,
		"replace":      replace,
	})
	
	if contentChanged {
		if checkMode {
			changes = append(changes, fmt.Sprintf("would replace %d occurrence(s) in %s", m.countMatches(compiledRegex, originalContent), path))
			currentState.Content = newContent
			currentState.Lines = strings.Split(newContent, "\n")
		} else {
			// Create backup if requested
			if backup {
				backupFile, err := m.createBackup(ctx, conn, path)
				if err != nil {
					return nil, fmt.Errorf("failed to create backup: %w", err)
				}
				currentState.BackupFile = backupFile
				result.Data["backup_file"] = backupFile
			}
			
			// Write new content
			if err := m.writeFile(ctx, conn, path, newContent); err != nil {
				return nil, fmt.Errorf("failed to write file: %w", err)
			}
			
			// Validate file if validation command provided
			if validate != "" {
				if err := m.validateFile(ctx, conn, path, validate); err != nil {
					return nil, fmt.Errorf("file validation failed: %w", err)
				}
			}
			
			changes = append(changes, fmt.Sprintf("replaced %d occurrence(s) in %s", m.countMatches(compiledRegex, originalContent), path))
			actuallyChanged = true
			currentState.Content = newContent
			currentState.Lines = strings.Split(newContent, "\n")
		}
	}
	
	// Set result properties
	result.Changed = actuallyChanged || (checkMode && len(changes) > 0)
	result.Data["after_state"] = *currentState
	result.Data["changes"] = changes
	result.Data["occurrences"] = m.countMatches(compiledRegex, originalContent)
	
	if checkMode {
		result.Simulated = true
		result.Data["check_mode"] = true
	}
	
	// Generate diff if requested
	if diffMode && (actuallyChanged || (checkMode && len(changes) > 0)) {
		diff := m.GenerateDiff(originalContent, newContent)
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
		result.Message = fmt.Sprintf("No changes needed in %s", path)
	}
	
	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	return result, nil
}

// getFileState reads the current state of a file
func (m *ReplaceModule) getFileState(ctx context.Context, conn types.Connection, path string) (*ReplaceState, error) {
	state := &ReplaceState{
		Path:   path,
		Exists: false,
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
	
	return state, nil
}

// countMatches counts how many times the regex matches in the content
func (m *ReplaceModule) countMatches(regex *regexp.Regexp, content string) int {
	matches := regex.FindAllString(content, -1)
	return len(matches)
}

// createBackup creates a backup of the original file
func (m *ReplaceModule) createBackup(ctx context.Context, conn types.Connection, path string) (string, error) {
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
func (m *ReplaceModule) writeFile(ctx context.Context, conn types.Connection, path, content string) error {
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
func (m *ReplaceModule) validateFile(ctx context.Context, conn types.Connection, path, validateCmd string) error {
	// Replace %s in validation command with the file path
	cmd := strings.ReplaceAll(validateCmd, "%s", path)
	
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return fmt.Errorf("validation command failed: %s", result.Message)
	}
	
	return nil
}

// shellEscape escapes a string for shell usage
func (m *ReplaceModule) shellEscape(s string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", "'\"'\"'"))
}