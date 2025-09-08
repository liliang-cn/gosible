package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// ArchiveModule handles creation of archive files
type ArchiveModule struct {
	BaseModule
}

// NewArchiveModule creates a new archive module instance
func NewArchiveModule() *ArchiveModule {
	return &ArchiveModule{
		BaseModule: BaseModule{
			name: "archive",
		},
	}
}

// Validate validates the module arguments
func (m *ArchiveModule) Validate(args map[string]interface{}) error {
	// Path is required
	path, hasPath := args["path"]
	if !hasPath {
		return fmt.Errorf("path is required")
	}

	// Validate path is string or list
	switch v := path.(type) {
	case string:
		if v == "" {
			return fmt.Errorf("path cannot be empty")
		}
	case []interface{}:
		if len(v) == 0 {
			return fmt.Errorf("path list cannot be empty")
		}
		for _, p := range v {
			if _, ok := p.(string); !ok {
				return types.NewValidationError("path", path, "path must be a string or list of strings")
			}
		}
	default:
		return types.NewValidationError("path", path, "path must be a string or list of strings")
	}

	// Validate format
	if format, hasFormat := args["format"]; hasFormat {
		validFormats := []string{"bz2", "gz", "tar", "xz", "zip"}
		formatStr, ok := format.(string)
		if !ok {
			return types.NewValidationError("format", format, "format must be a string")
		}
		
		valid := false
		for _, f := range validFormats {
			if formatStr == f {
				valid = true
				break
			}
		}
		if !valid {
			return types.NewValidationError("format", format, fmt.Sprintf("format must be one of: %v", validFormats))
		}
	}

	// Validate dest
	if dest, hasDest := args["dest"]; hasDest {
		if _, ok := dest.(string); !ok && dest != nil {
			return types.NewValidationError("dest", dest, "dest must be a string")
		}
	}

	// Validate remove
	if remove, hasRemove := args["remove"]; hasRemove {
		if _, ok := remove.(bool); !ok {
			return types.NewValidationError("remove", remove, "remove must be a boolean")
		}
	}

	// Validate exclude_path
	if exclude, hasExclude := args["exclude_path"]; hasExclude {
		switch v := exclude.(type) {
		case string:
			// OK
		case []interface{}:
			for _, pattern := range v {
				if _, ok := pattern.(string); !ok {
					return types.NewValidationError("exclude_path", exclude, "exclude_path must be a string or list of strings")
				}
			}
		default:
			return types.NewValidationError("exclude_path", exclude, "exclude_path must be a string or list of strings")
		}
	}

	// Validate force_archive
	if force, hasForce := args["force_archive"]; hasForce {
		if _, ok := force.(bool); !ok {
			return types.NewValidationError("force_archive", force, "force_archive must be a boolean")
		}
	}

	// Validate mode
	if mode, hasMode := args["mode"]; hasMode {
		if _, ok := mode.(string); !ok && mode != nil {
			return types.NewValidationError("mode", mode, "mode must be a string")
		}
	}

	// Validate owner
	if owner, hasOwner := args["owner"]; hasOwner {
		if _, ok := owner.(string); !ok && owner != nil {
			return types.NewValidationError("owner", owner, "owner must be a string")
		}
	}

	// Validate group
	if group, hasGroup := args["group"]; hasGroup {
		if _, ok := group.(string); !ok && group != nil {
			return types.NewValidationError("group", group, "group must be a string")
		}
	}

	return nil
}

// Run executes the archive module
func (m *ArchiveModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Validate arguments
	if err := m.Validate(args); err != nil {
		return &types.Result{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Parse arguments
	paths := m.getPaths(args)
	format := m.GetStringArg(args, "format", "gz")
	dest := m.GetStringArg(args, "dest", "")
	remove := m.GetBoolArg(args, "remove", false)
	forceArchive := m.GetBoolArg(args, "force_archive", false)
	mode := m.GetStringArg(args, "mode", "")
	owner := m.GetStringArg(args, "owner", "")
	group := m.GetStringArg(args, "group", "")
	excludePaths := m.getExcludePaths(args)

	// Check if paths exist
	existingPaths, missingPaths, err := m.checkPaths(ctx, conn, paths)
	if err != nil {
		return &types.Result{
			Success: false,
			Message: fmt.Sprintf("Failed to check paths: %v", err),
		}, err
	}

	if len(missingPaths) > 0 {
		return &types.Result{
			Success: false,
			Message: fmt.Sprintf("Path does not exist: %s", strings.Join(missingPaths, ", ")),
		}, fmt.Errorf("paths not found")
	}

	// Determine destination path
	if dest == "" {
		if len(existingPaths) == 1 {
			// Single file/directory - create archive with same name
			dest = m.getDefaultDestination(existingPaths[0], format)
		} else {
			// Multiple paths - need explicit destination
			return &types.Result{
				Success: false,
				Message: "dest is required when archiving multiple paths",
			}, fmt.Errorf("dest required for multiple paths")
		}
	}

	// Check if we need to create an archive
	needsArchive := forceArchive || len(existingPaths) > 1
	if !needsArchive {
		// Check if single path is a directory
		isDir, err := m.isDirectory(ctx, conn, existingPaths[0])
		if err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to check path type: %v", err),
			}, err
		}
		needsArchive = isDir
	}

	// Check if destination already exists
	destExists, err := m.pathExists(ctx, conn, dest)
	if err != nil {
		return &types.Result{
			Success: false,
			Message: fmt.Sprintf("Failed to check destination: %v", err),
		}, err
	}

	// Check mode
	if m.CheckMode(args) {
		if destExists {
			return &types.Result{
				Success: true,
				Changed: false,
				Message: fmt.Sprintf("Archive %s already exists", dest),
			}, nil
		}
		return &types.Result{
			Success: true,
			Changed: true,
			Message: fmt.Sprintf("Would create archive %s from %s", dest, strings.Join(existingPaths, ", ")),
		}, nil
	}

	// Create archive
	var archiveSize int64
	var fileCount int

	if needsArchive || format != "gz" {
		// Create tar/zip archive
		archiveSize, fileCount, err = m.createArchive(ctx, conn, existingPaths, dest, format, excludePaths)
		if err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to create archive: %v", err),
			}, err
		}
	} else {
		// Compress single file
		archiveSize, err = m.compressSingleFile(ctx, conn, existingPaths[0], dest, format)
		if err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to compress file: %v", err),
			}, err
		}
		fileCount = 1
	}

	// Set permissions on archive if specified
	if mode != "" || owner != "" || group != "" {
		if err := m.setPermissions(ctx, conn, dest, mode, owner, group); err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to set permissions: %v", err),
			}, err
		}
	}

	// Remove source files if requested
	if remove {
		if err := m.removeSourceFiles(ctx, conn, existingPaths); err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to remove source files: %v", err),
			}, err
		}
	}

	result := &types.Result{
		Success: true,
		Changed: true,
		Message: fmt.Sprintf("Created archive %s (%d bytes, %d files)", dest, archiveSize, fileCount),
		Data: map[string]interface{}{
			"dest":           dest,
			"archived_files": fileCount,
			"archive_size":   archiveSize,
			"format":         format,
		},
	}

	return result, nil
}

// getPaths extracts path list from arguments
func (m *ArchiveModule) getPaths(args map[string]interface{}) []string {
	paths := []string{}
	
	if value, exists := args["path"]; exists {
		switch v := value.(type) {
		case string:
			paths = append(paths, v)
		case []interface{}:
			for _, p := range v {
				if str, ok := p.(string); ok {
					paths = append(paths, str)
				}
			}
		}
	}
	
	return paths
}

// getExcludePaths extracts exclude patterns from arguments
func (m *ArchiveModule) getExcludePaths(args map[string]interface{}) []string {
	excludes := []string{}
	
	if value, exists := args["exclude_path"]; exists {
		switch v := value.(type) {
		case string:
			excludes = append(excludes, v)
		case []interface{}:
			for _, p := range v {
				if str, ok := p.(string); ok {
					excludes = append(excludes, str)
				}
			}
		}
	}
	
	return excludes
}

// checkPaths checks which paths exist
func (m *ArchiveModule) checkPaths(ctx context.Context, conn types.Connection, paths []string) ([]string, []string, error) {
	existing := []string{}
	missing := []string{}

	for _, path := range paths {
		exists, err := m.pathExists(ctx, conn, path)
		if err != nil {
			return nil, nil, err
		}
		if exists {
			existing = append(existing, path)
		} else {
			missing = append(missing, path)
		}
	}

	return existing, missing, nil
}

// pathExists checks if a path exists
func (m *ArchiveModule) pathExists(ctx context.Context, conn types.Connection, path string) (bool, error) {
	result, err := conn.Execute(ctx, fmt.Sprintf("test -e %s", path), types.ExecuteOptions{})
	if err != nil {
		return false, nil
	}
	return result.Success, nil
}

// isDirectory checks if a path is a directory
func (m *ArchiveModule) isDirectory(ctx context.Context, conn types.Connection, path string) (bool, error) {
	result, err := conn.Execute(ctx, fmt.Sprintf("test -d %s", path), types.ExecuteOptions{})
	if err != nil {
		return false, nil
	}
	return result.Success, nil
}

// getDefaultDestination generates default destination path
func (m *ArchiveModule) getDefaultDestination(path, format string) string {
	base := filepath.Base(path)
	
	switch format {
	case "tar":
		return base + ".tar"
	case "gz":
		if strings.HasSuffix(base, ".tar") {
			return base + ".gz"
		}
		return base + ".tar.gz"
	case "bz2":
		if strings.HasSuffix(base, ".tar") {
			return base + ".bz2"
		}
		return base + ".tar.bz2"
	case "xz":
		if strings.HasSuffix(base, ".tar") {
			return base + ".xz"
		}
		return base + ".tar.xz"
	case "zip":
		return base + ".zip"
	default:
		return base + "." + format
	}
}

// createArchive creates an archive from multiple paths
func (m *ArchiveModule) createArchive(ctx context.Context, conn types.Connection, paths []string, dest, format string, excludes []string) (int64, int, error) {
	var cmd string

	// Build path list
	pathList := strings.Join(paths, " ")

	// Build exclude options
	excludeOpts := ""
	for _, exclude := range excludes {
		switch format {
		case "tar", "gz", "bz2", "xz":
			excludeOpts += fmt.Sprintf(" --exclude='%s'", exclude)
		case "zip":
			excludeOpts += fmt.Sprintf(" -x '%s'", exclude)
		}
	}

	// Build archive command based on format
	switch format {
	case "tar":
		cmd = fmt.Sprintf("tar -cf %s%s %s", dest, excludeOpts, pathList)
	case "gz":
		cmd = fmt.Sprintf("tar -czf %s%s %s", dest, excludeOpts, pathList)
	case "bz2":
		cmd = fmt.Sprintf("tar -cjf %s%s %s", dest, excludeOpts, pathList)
	case "xz":
		cmd = fmt.Sprintf("tar -cJf %s%s %s", dest, excludeOpts, pathList)
	case "zip":
		cmd = fmt.Sprintf("zip -r %s %s%s", dest, pathList, excludeOpts)
	default:
		return 0, 0, fmt.Errorf("unsupported format: %s", format)
	}

	// Create the archive
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return 0, 0, err
	}
	if !result.Success {
		return 0, 0, fmt.Errorf("archive creation failed")
	}

	// Get archive size
	sizeResult, err := conn.Execute(ctx, fmt.Sprintf("stat -c %%s %s", dest), types.ExecuteOptions{})
	if err != nil {
		return 0, 0, err
	}
	
	var size int64
	if stdout, ok := sizeResult.Data["stdout"].(string); ok {
		fmt.Sscanf(strings.TrimSpace(stdout), "%d", &size)
	}

	// Count files in archive
	var countCmd string
	switch format {
	case "tar":
		countCmd = fmt.Sprintf("tar -tf %s | wc -l", dest)
	case "gz", "bz2", "xz":
		if format == "gz" {
			countCmd = fmt.Sprintf("tar -tzf %s | wc -l", dest)
		} else if format == "bz2" {
			countCmd = fmt.Sprintf("tar -tjf %s | wc -l", dest)
		} else {
			countCmd = fmt.Sprintf("tar -tJf %s | wc -l", dest)
		}
	case "zip":
		countCmd = fmt.Sprintf("unzip -l %s | tail -1 | awk '{print $2}'", dest)
	}

	fileCount := 0
	if countCmd != "" {
		countResult, err := conn.Execute(ctx, countCmd, types.ExecuteOptions{})
		if err == nil && countResult.Success {
			if stdout, ok := countResult.Data["stdout"].(string); ok {
				fmt.Sscanf(strings.TrimSpace(stdout), "%d", &fileCount)
			}
		}
	}

	return size, fileCount, nil
}

// compressSingleFile compresses a single file
func (m *ArchiveModule) compressSingleFile(ctx context.Context, conn types.Connection, source, dest, format string) (int64, error) {
	var cmd string

	switch format {
	case "gz":
		cmd = fmt.Sprintf("gzip -c %s > %s", source, dest)
	case "bz2":
		cmd = fmt.Sprintf("bzip2 -c %s > %s", source, dest)
	case "xz":
		cmd = fmt.Sprintf("xz -c %s > %s", source, dest)
	default:
		return 0, fmt.Errorf("unsupported compression format: %s", format)
	}

	// Compress the file
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return 0, err
	}
	if !result.Success {
		return 0, fmt.Errorf("compression failed")
	}

	// Get compressed file size
	sizeResult, err := conn.Execute(ctx, fmt.Sprintf("stat -c %%s %s", dest), types.ExecuteOptions{})
	if err != nil {
		return 0, err
	}
	
	var size int64
	if stdout, ok := sizeResult.Data["stdout"].(string); ok {
		fmt.Sscanf(strings.TrimSpace(stdout), "%d", &size)
	}

	return size, nil
}

// setPermissions sets permissions on a file
func (m *ArchiveModule) setPermissions(ctx context.Context, conn types.Connection, path, mode, owner, group string) error {
	// Set permissions if specified
	if mode != "" {
		cmd := fmt.Sprintf("chmod %s %s", mode, path)
		if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
			return err
		}
	}

	// Set ownership if specified
	if owner != "" || group != "" {
		ownerGroup := ""
		if owner != "" && group != "" {
			ownerGroup = fmt.Sprintf("%s:%s", owner, group)
		} else if owner != "" {
			ownerGroup = owner
		} else {
			ownerGroup = fmt.Sprintf(":%s", group)
		}
		cmd := fmt.Sprintf("chown %s %s", ownerGroup, path)
		if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// removeSourceFiles removes the source files after archiving
func (m *ArchiveModule) removeSourceFiles(ctx context.Context, conn types.Connection, paths []string) error {
	for _, path := range paths {
		// Check if directory or file
		isDir, _ := m.isDirectory(ctx, conn, path)
		
		var cmd string
		if isDir {
			cmd = fmt.Sprintf("rm -rf %s", path)
		} else {
			cmd = fmt.Sprintf("rm -f %s", path)
		}

		result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil {
			return err
		}
		if !result.Success {
			return fmt.Errorf("failed to remove %s", path)
		}
	}

	return nil
}