package modules

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// EnhancedCopyModule implements file copying with detailed progress tracking
type EnhancedCopyModule struct {
	BaseModule
}

// NewEnhancedCopyModule creates a new enhanced copy module instance
func NewEnhancedCopyModule() *EnhancedCopyModule {
	return &EnhancedCopyModule{
		BaseModule: BaseModule{
			name: "enhanced_copy",
		},
	}
}

// ProgressCopyConnection interface for connections that support progress-aware copying
type ProgressCopyConnection interface {
	types.Connection
	CopyWithProgress(ctx context.Context, src io.Reader, dest string, mode int, totalSize int64, progressCallback func(progress types.ProgressInfo)) error
}

// Run executes the enhanced copy module
func (m *EnhancedCopyModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	src := m.GetStringArg(args, "src", "")
	dest := m.GetStringArg(args, "dest", "")
	content := m.GetStringArg(args, "content", "")
	mode := m.GetStringArg(args, "mode", "0644")
	backup := m.GetBoolArg(args, "backup", false)
	showProgress := m.GetBoolArg(args, "show_progress", true)

	if src == "" && content == "" {
		return nil, fmt.Errorf("enhanced_copy: either 'src' or 'content' must be specified")
	}
	if dest == "" {
		return nil, fmt.Errorf("enhanced_copy: 'dest' parameter is required")
	}

	// Parse file mode
	fileMode, err := parseFileMode(mode)
	if err != nil {
		return nil, fmt.Errorf("enhanced_copy: invalid mode %s: %v", mode, err)
	}

	// Check if connection supports progress copying
	if progressConn, ok := conn.(ProgressCopyConnection); ok && showProgress {
		return m.copyWithProgress(ctx, progressConn, src, dest, content, fileMode, backup)
	}

	// Fallback to standard copy
	return m.copyStandard(ctx, conn, src, dest, content, fileMode, backup)
}

// copyWithProgress performs copy operation with progress tracking
func (m *EnhancedCopyModule) copyWithProgress(ctx context.Context, conn ProgressCopyConnection, src, dest, content string, mode os.FileMode, backup bool) (*types.Result, error) {
	var reader io.Reader
	var totalSize int64
	var sourceType string

	startTime := time.Now()

	// Prepare source
	if content != "" {
		reader = strings.NewReader(content)
		totalSize = int64(len(content))
		sourceType = "content"
	} else {
		// For file source, we'd need to get the file size first
		// This is a simplified implementation
		sourceFile, err := os.Open(src)
		if err != nil {
			return nil, fmt.Errorf("enhanced_copy: failed to open source file %s: %v", src, err)
		}
		defer sourceFile.Close()

		stat, err := sourceFile.Stat()
		if err != nil {
			return nil, fmt.Errorf("enhanced_copy: failed to stat source file %s: %v", src, err)
		}

		reader = sourceFile
		totalSize = stat.Size()
		sourceType = "file"
	}

	// Progress tracking variables
	var progressUpdates []types.ProgressInfo
	var lastProgress types.ProgressInfo

	progressCallback := func(progress types.ProgressInfo) {
		progressUpdates = append(progressUpdates, progress)
		lastProgress = progress

		if showProgress := m.GetBoolArg(map[string]interface{}{"show_progress": true}, "show_progress", true); showProgress {
			fmt.Printf("📁 Copy Progress: %.1f%% - %s\n", progress.Percentage, progress.Message)
		}
	}

	// Create backup if requested
	var backupPath string
	if backup {
		backupPath = dest + ".backup." + fmt.Sprintf("%d", time.Now().Unix())
		// Check if destination exists
		if _, err := os.Stat(dest); err == nil {
			backupFile, err := os.Open(dest)
			if err == nil {
				defer backupFile.Close()
				err = conn.Copy(ctx, backupFile, backupPath, int(mode))
				if err != nil {
					return nil, fmt.Errorf("enhanced_copy: failed to create backup: %v", err)
				}
				fmt.Printf("📋 Backup created: %s\n", backupPath)
			}
		}
	}

	// Perform copy with progress
	err := conn.CopyWithProgress(ctx, reader, dest, int(mode), totalSize, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("enhanced_copy: copy operation failed: %v", err)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Calculate transfer statistics
	var avgSpeed float64
	if duration.Seconds() > 0 {
		avgSpeed = float64(totalSize) / duration.Seconds()
	}

	result := &types.Result{
		Success:    true,
		Changed:    true,
		Message:    fmt.Sprintf("File copied successfully from %s to %s", sourceType, dest),
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		ModuleName: "enhanced_copy",
		Data: map[string]interface{}{
			"src":              src,
			"dest":             dest,
			"source_type":      sourceType,
			"file_size":        totalSize,
			"transfer_time":    duration.String(),
			"average_speed":    formatBytesEnhanced(int64(avgSpeed)) + "/s",
			"progress_updates": len(progressUpdates),
			"backup_created":   backup,
			"backup_path":      backupPath,
			"file_mode":        fmt.Sprintf("%o", mode),
		},
	}

	// Add final progress info
	if len(progressUpdates) > 0 {
		result.Data["final_progress"] = lastProgress
		result.Data["transfer_completed"] = lastProgress.Percentage >= 100.0
	}

	return result, nil
}

// copyStandard performs standard copy operation without progress
func (m *EnhancedCopyModule) copyStandard(ctx context.Context, conn types.Connection, src, dest, content string, mode os.FileMode, backup bool) (*types.Result, error) {
	var reader io.Reader

	if content != "" {
		reader = strings.NewReader(content)
	} else {
		sourceFile, err := os.Open(src)
		if err != nil {
			return nil, fmt.Errorf("enhanced_copy: failed to open source file %s: %v", src, err)
		}
		defer sourceFile.Close()
		reader = sourceFile
	}

	// Create backup if requested
	var backupPath string
	if backup {
		backupPath = dest + ".backup." + fmt.Sprintf("%d", time.Now().Unix())
		if _, err := os.Stat(dest); err == nil {
			backupFile, err := os.Open(dest)
			if err == nil {
				defer backupFile.Close()
				err = conn.Copy(ctx, backupFile, backupPath, int(mode))
				if err != nil {
					return nil, fmt.Errorf("enhanced_copy: failed to create backup: %v", err)
				}
			}
		}
	}

	// Perform standard copy
	startTime := time.Now()
	err := conn.Copy(ctx, reader, dest, int(mode))
	if err != nil {
		return nil, fmt.Errorf("enhanced_copy: copy operation failed: %v", err)
	}
	endTime := time.Now()

	sourceType := "file"
	if content != "" {
		sourceType = "content"
	}

	result := &types.Result{
		Success:    true,
		Changed:    true,
		Message:    fmt.Sprintf("File copied successfully from %s to %s", sourceType, dest),
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime),
		ModuleName: "enhanced_copy",
		Data: map[string]interface{}{
			"src":            src,
			"dest":           dest,
			"source_type":    sourceType,
			"backup_created": backup,
			"backup_path":    backupPath,
			"file_mode":      fmt.Sprintf("%o", mode),
			"progress_enabled": false,
		},
	}

	return result, nil
}

// Validate checks if the module arguments are valid
func (m *EnhancedCopyModule) Validate(args map[string]interface{}) error {
	src := m.GetStringArg(args, "src", "")
	content := m.GetStringArg(args, "content", "")
	dest := m.GetStringArg(args, "dest", "")

	// Either src or content must be specified
	if src == "" && content == "" {
		return fmt.Errorf("enhanced_copy: either 'src' or 'content' must be specified")
	}

	// Both src and content cannot be specified
	if src != "" && content != "" {
		return fmt.Errorf("enhanced_copy: cannot specify both 'src' and 'content'")
	}

	// Destination is required
	if dest == "" {
		return fmt.Errorf("enhanced_copy: 'dest' parameter is required")
	}

	// Validate mode if provided
	if modeStr := m.GetStringArg(args, "mode", ""); modeStr != "" {
		if _, err := parseFileMode(modeStr); err != nil {
			return fmt.Errorf("enhanced_copy: invalid mode %s: %v", modeStr, err)
		}
	}

	return nil
}

// Documentation returns the module documentation
func (m *EnhancedCopyModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "enhanced_copy",
		Description: "Copy files with detailed progress tracking and advanced features",
		Parameters: map[string]types.ParamDoc{
			"src": {
				Description: "Source file path (alternative to content)",
				Required:    false,
				Type:        "string",
			},
			"content": {
				Description: "File content as string (alternative to src)",
				Required:    false,
				Type:        "string",
			},
			"dest": {
				Description: "Destination file path",
				Required:    true,
				Type:        "string",
			},
			"mode": {
				Description: "File permissions in octal notation",
				Required:    false,
				Type:        "string",
				Default:     "0644",
			},
			"backup": {
				Description: "Create backup of existing destination file",
				Required:    false,
				Type:        "boolean",
				Default:     "false",
			},
			"show_progress": {
				Description: "Display transfer progress (when supported)",
				Required:    false,
				Type:        "boolean",
				Default:     "true",
			},
		},
		Examples: []string{
			"- name: Copy file with progress\n  enhanced_copy:\n    src: /source/file.txt\n    dest: /dest/file.txt\n    show_progress: true",
			"- name: Copy content with backup\n  enhanced_copy:\n    content: 'Hello World'\n    dest: /tmp/hello.txt\n    backup: true\n    mode: '0755'",
		},
		Returns: map[string]string{
			"file_size":        "Size of transferred file in bytes",
			"transfer_time":    "Time taken for transfer",
			"average_speed":    "Average transfer speed",
			"progress_updates": "Number of progress updates received",
			"backup_created":   "Whether backup was created",
			"final_progress":   "Final progress information",
		},
	}
}

// Helper functions

// parseFileMode parses a file mode string into os.FileMode
func parseFileMode(modeStr string) (os.FileMode, error) {
	// Remove any leading "0" if present for octal notation
	if strings.HasPrefix(modeStr, "0") && len(modeStr) > 1 {
		modeStr = modeStr[1:]
	}

	// Parse as octal
	var mode uint32
	_, err := fmt.Sscanf(modeStr, "%o", &mode)
	if err != nil {
		return 0, err
	}

	return os.FileMode(mode), nil
}

// formatBytes formats byte counts in human-readable format (duplicate for local use)
func formatBytesEnhanced(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}