package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/liliang-cn/gosible/pkg/types"
)

// UnarchiveModule handles extraction of archive files
type UnarchiveModule struct {
	BaseModule
}

// NewUnarchiveModule creates a new unarchive module instance
func NewUnarchiveModule() *UnarchiveModule {
	return &UnarchiveModule{
		BaseModule: BaseModule{
			name: "unarchive",
		},
	}
}

// Validate validates the module arguments
func (m *UnarchiveModule) Validate(args map[string]interface{}) error {
	// Validate required fields
	src, hasSrc := args["src"].(string)
	if !hasSrc || src == "" {
		return fmt.Errorf("src is required")
	}

	dest, hasDest := args["dest"].(string)
	if !hasDest || dest == "" {
		return fmt.Errorf("dest is required")
	}

	// Validate remote_src
	if remoteSrc, hasRemoteSrc := args["remote_src"]; hasRemoteSrc {
		if _, ok := remoteSrc.(bool); !ok {
			return types.NewValidationError("remote_src", remoteSrc, "remote_src must be a boolean")
		}
	}

	// Validate creates
	if creates, hasCreates := args["creates"]; hasCreates {
		if _, ok := creates.(string); !ok {
			return types.NewValidationError("creates", creates, "creates must be a string")
		}
	}

	// Validate list_files
	if listFiles, hasListFiles := args["list_files"]; hasListFiles {
		if _, ok := listFiles.(bool); !ok {
			return types.NewValidationError("list_files", listFiles, "list_files must be a boolean")
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

	// Validate exclude patterns
	if exclude, hasExclude := args["exclude"]; hasExclude {
		switch v := exclude.(type) {
		case string:
			// Single exclude pattern - OK
		case []interface{}:
			// List of exclude patterns - validate each is a string
			for _, pattern := range v {
				if _, ok := pattern.(string); !ok {
					return types.NewValidationError("exclude", exclude, "exclude must be a string or list of strings")
				}
			}
		default:
			return types.NewValidationError("exclude", exclude, "exclude must be a string or list of strings")
		}
	}

	// Validate include patterns
	if include, hasInclude := args["include"]; hasInclude {
		switch v := include.(type) {
		case string:
			// Single include pattern - OK
		case []interface{}:
			// List of include patterns - validate each is a string
			for _, pattern := range v {
				if _, ok := pattern.(string); !ok {
					return types.NewValidationError("include", include, "include must be a string or list of strings")
				}
			}
		default:
			return types.NewValidationError("include", include, "include must be a string or list of strings")
		}
	}

	// Validate keep_newer
	if keepNewer, hasKeepNewer := args["keep_newer"]; hasKeepNewer {
		if _, ok := keepNewer.(bool); !ok {
			return types.NewValidationError("keep_newer", keepNewer, "keep_newer must be a boolean")
		}
	}

	// Validate validate_certs
	if validateCerts, hasValidateCerts := args["validate_certs"]; hasValidateCerts {
		if _, ok := validateCerts.(bool); !ok {
			return types.NewValidationError("validate_certs", validateCerts, "validate_certs must be a boolean")
		}
	}

	return nil
}

// Run executes the unarchive module
func (m *UnarchiveModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Validate arguments
	if err := m.Validate(args); err != nil {
		return &types.Result{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Parse arguments
	src := args["src"].(string)
	dest := args["dest"].(string)
	remoteSrc := m.GetBoolArg(args, "remote_src", false)
	creates := m.GetStringArg(args, "creates", "")
	listFiles := m.GetBoolArg(args, "list_files", false)
	mode := m.GetStringArg(args, "mode", "")
	owner := m.GetStringArg(args, "owner", "")
	group := m.GetStringArg(args, "group", "")
	keepNewer := m.GetBoolArg(args, "keep_newer", false)
	validateCerts := m.GetBoolArg(args, "validate_certs", true)

	// Get exclude and include patterns
	excludePatterns := m.getPatterns(args, "exclude")
	includePatterns := m.getPatterns(args, "include")

	// Check if destination exists
	destExists, err := m.pathExists(ctx, conn, dest)
	if err != nil {
		return &types.Result{
			Success: false,
			Message: fmt.Sprintf("Failed to check destination: %v", err),
		}, err
	}

	if !destExists {
		// Create destination directory
		if m.CheckMode(args) {
			return &types.Result{
				Success: true,
				Changed: true,
				Message: fmt.Sprintf("Would create destination directory %s", dest),
			}, nil
		}

		if err := m.createDirectory(ctx, conn, dest, mode, owner, group); err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to create destination: %v", err),
			}, err
		}
	}

	// Check if creates path exists
	if creates != "" {
		createsExists, err := m.pathExists(ctx, conn, creates)
		if err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to check creates path: %v", err),
			}, err
		}

		if createsExists {
			return &types.Result{
				Success: true,
				Changed: false,
				Message: fmt.Sprintf("Creates path %s already exists, skipping extraction", creates),
			}, nil
		}
	}

	// Download remote archive if needed
	archivePath := src
	if !remoteSrc && strings.HasPrefix(src, "http") {
		if m.CheckMode(args) {
			return &types.Result{
				Success: true,
				Changed: true,
				Message: fmt.Sprintf("Would download and extract %s to %s", src, dest),
			}, nil
		}

		// Download archive
		tempPath, err := m.downloadArchive(ctx, conn, src, validateCerts)
		if err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to download archive: %v", err),
			}, err
		}
		archivePath = tempPath
		defer m.cleanupTempFile(ctx, conn, tempPath)
	}

	// Detect archive type
	archiveType := m.detectArchiveType(archivePath)
	if archiveType == "" {
		return &types.Result{
			Success: false,
			Message: fmt.Sprintf("Unsupported archive format: %s", archivePath),
		}, fmt.Errorf("unsupported archive format")
	}

	// Check mode
	if m.CheckMode(args) {
		return &types.Result{
			Success: true,
			Changed: true,
			Message: fmt.Sprintf("Would extract %s to %s", archivePath, dest),
		}, nil
	}

	// Extract archive
	extractedFiles, err := m.extractArchive(ctx, conn, archivePath, dest, archiveType, 
		excludePatterns, includePatterns, keepNewer, listFiles)
	if err != nil {
		return &types.Result{
			Success: false,
			Message: fmt.Sprintf("Failed to extract archive: %v", err),
		}, err
	}

	// Set permissions on extracted files if specified
	if mode != "" || owner != "" || group != "" {
		if err := m.setPermissions(ctx, conn, dest, mode, owner, group); err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to set permissions: %v", err),
			}, err
		}
	}

	result := &types.Result{
		Success: true,
		Changed: len(extractedFiles) > 0,
		Message: fmt.Sprintf("Extracted %d files from %s to %s", len(extractedFiles), archivePath, dest),
		Data:    make(map[string]interface{}),
	}

	if listFiles {
		result.Data["files"] = extractedFiles
	}

	return result, nil
}

// getPatterns extracts patterns from arguments
func (m *UnarchiveModule) getPatterns(args map[string]interface{}, key string) []string {
	patterns := []string{}
	
	if value, exists := args[key]; exists {
		switch v := value.(type) {
		case string:
			patterns = append(patterns, v)
		case []interface{}:
			for _, pattern := range v {
				if str, ok := pattern.(string); ok {
					patterns = append(patterns, str)
				}
			}
		}
	}
	
	return patterns
}

// pathExists checks if a path exists
func (m *UnarchiveModule) pathExists(ctx context.Context, conn types.Connection, path string) (bool, error) {
	result, err := conn.Execute(ctx, fmt.Sprintf("test -e %s", path), types.ExecuteOptions{})
	if err != nil {
		return false, nil
	}
	return result.Success, nil
}

// createDirectory creates a directory with specified permissions
func (m *UnarchiveModule) createDirectory(ctx context.Context, conn types.Connection, path, mode, owner, group string) error {
	cmd := fmt.Sprintf("mkdir -p %s", path)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("failed to create directory")
	}

	// Set permissions if specified
	if mode != "" {
		cmd = fmt.Sprintf("chmod %s %s", mode, path)
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
		cmd = fmt.Sprintf("chown %s %s", ownerGroup, path)
		if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// downloadArchive downloads a remote archive
func (m *UnarchiveModule) downloadArchive(ctx context.Context, conn types.Connection, url string, validateCerts bool) (string, error) {
	tempFile := fmt.Sprintf("/tmp/ansible_unarchive_%d", m.GetCurrentTimestamp())
	
	cmd := "wget"
	if !validateCerts {
		cmd += " --no-check-certificate"
	}
	cmd += fmt.Sprintf(" -O %s %s", tempFile, url)

	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return "", err
	}
	if !result.Success {
		return "", fmt.Errorf("failed to download archive")
	}

	return tempFile, nil
}

// cleanupTempFile removes a temporary file
func (m *UnarchiveModule) cleanupTempFile(ctx context.Context, conn types.Connection, path string) {
	conn.Execute(ctx, fmt.Sprintf("rm -f %s", path), types.ExecuteOptions{})
}

// detectArchiveType detects the type of archive based on extension
func (m *UnarchiveModule) detectArchiveType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	// Check for tar variants
	if strings.Contains(base, ".tar.") {
		if strings.HasSuffix(base, ".tar.gz") || strings.HasSuffix(base, ".tgz") {
			return "tar.gz"
		} else if strings.HasSuffix(base, ".tar.bz2") || strings.HasSuffix(base, ".tbz2") {
			return "tar.bz2"
		} else if strings.HasSuffix(base, ".tar.xz") || strings.HasSuffix(base, ".txz") {
			return "tar.xz"
		} else if strings.HasSuffix(base, ".tar.Z") {
			return "tar.Z"
		}
		return "tar"
	}

	// Check single extensions
	switch ext {
	case ".zip":
		return "zip"
	case ".gz":
		return "gz"
	case ".bz2":
		return "bz2"
	case ".xz":
		return "xz"
	case ".tar":
		return "tar"
	case ".tgz":
		return "tar.gz"
	case ".tbz2", ".tbz":
		return "tar.bz2"
	case ".txz":
		return "tar.xz"
	case ".7z":
		return "7z"
	case ".rar":
		return "rar"
	}

	return ""
}

// extractArchive extracts an archive
func (m *UnarchiveModule) extractArchive(ctx context.Context, conn types.Connection, archivePath, dest, archiveType string,
	excludePatterns, includePatterns []string, keepNewer, listFiles bool) ([]string, error) {
	
	var cmd string
	var listCmd string

	switch archiveType {
	case "tar":
		cmd = fmt.Sprintf("tar -xf %s -C %s", archivePath, dest)
		listCmd = fmt.Sprintf("tar -tf %s", archivePath)
		if keepNewer {
			cmd = fmt.Sprintf("tar -xf %s -C %s --keep-newer-files", archivePath, dest)
		}
	case "tar.gz", "tgz":
		cmd = fmt.Sprintf("tar -xzf %s -C %s", archivePath, dest)
		listCmd = fmt.Sprintf("tar -tzf %s", archivePath)
		if keepNewer {
			cmd = fmt.Sprintf("tar -xzf %s -C %s --keep-newer-files", archivePath, dest)
		}
	case "tar.bz2", "tbz2":
		cmd = fmt.Sprintf("tar -xjf %s -C %s", archivePath, dest)
		listCmd = fmt.Sprintf("tar -tjf %s", archivePath)
		if keepNewer {
			cmd = fmt.Sprintf("tar -xjf %s -C %s --keep-newer-files", archivePath, dest)
		}
	case "tar.xz", "txz":
		cmd = fmt.Sprintf("tar -xJf %s -C %s", archivePath, dest)
		listCmd = fmt.Sprintf("tar -tJf %s", archivePath)
		if keepNewer {
			cmd = fmt.Sprintf("tar -xJf %s -C %s --keep-newer-files", archivePath, dest)
		}
	case "zip":
		cmd = fmt.Sprintf("unzip -o %s -d %s", archivePath, dest)
		listCmd = fmt.Sprintf("unzip -l %s", archivePath)
		if keepNewer {
			cmd = fmt.Sprintf("unzip -u %s -d %s", archivePath, dest)
		}
	case "gz":
		// Single file gzip
		outputFile := filepath.Join(dest, strings.TrimSuffix(filepath.Base(archivePath), ".gz"))
		cmd = fmt.Sprintf("gunzip -c %s > %s", archivePath, outputFile)
		listCmd = fmt.Sprintf("gunzip -l %s", archivePath)
	case "bz2":
		// Single file bzip2
		outputFile := filepath.Join(dest, strings.TrimSuffix(filepath.Base(archivePath), ".bz2"))
		cmd = fmt.Sprintf("bunzip2 -c %s > %s", archivePath, outputFile)
		listCmd = ""
	case "xz":
		// Single file xz
		outputFile := filepath.Join(dest, strings.TrimSuffix(filepath.Base(archivePath), ".xz"))
		cmd = fmt.Sprintf("xz -dc %s > %s", archivePath, outputFile)
		listCmd = ""
	case "7z":
		cmd = fmt.Sprintf("7z x -o%s %s -y", dest, archivePath)
		listCmd = fmt.Sprintf("7z l %s", archivePath)
	case "rar":
		cmd = fmt.Sprintf("unrar x -o+ %s %s", archivePath, dest)
		listCmd = fmt.Sprintf("unrar l %s", archivePath)
	default:
		return nil, fmt.Errorf("unsupported archive type: %s", archiveType)
	}

	// Add exclude patterns
	for _, pattern := range excludePatterns {
		switch archiveType {
		case "tar", "tar.gz", "tar.bz2", "tar.xz":
			cmd += fmt.Sprintf(" --exclude='%s'", pattern)
		case "zip":
			cmd += fmt.Sprintf(" -x '%s'", pattern)
		}
	}

	// Get list of files if requested
	var extractedFiles []string
	if listFiles && listCmd != "" {
		result, err := conn.Execute(ctx, listCmd, types.ExecuteOptions{})
		if err == nil && result.Success {
			if stdout, ok := result.Data["stdout"].(string); ok {
				extractedFiles = m.parseFileList(stdout, archiveType)
			}
		}
	}

	// Extract the archive
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("extraction failed")
	}

	return extractedFiles, nil
}

// parseFileList parses the file list from archive listing output
func (m *UnarchiveModule) parseFileList(output, archiveType string) []string {
	files := []string{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch archiveType {
		case "tar", "tar.gz", "tar.bz2", "tar.xz":
			// tar lists files directly
			files = append(files, line)
		case "zip":
			// unzip -l format: skip header and parse file names
			fields := strings.Fields(line)
			if len(fields) >= 4 && fields[0] != "Length" {
				// File name is the last field
				files = append(files, fields[len(fields)-1])
			}
		case "7z":
			// 7z l format: parse file names from listing
			if strings.Contains(line, "....A") {
				fields := strings.Fields(line)
				if len(fields) >= 6 {
					files = append(files, fields[len(fields)-1])
				}
			}
		}
	}

	return files
}

// setPermissions sets permissions on extracted files
func (m *UnarchiveModule) setPermissions(ctx context.Context, conn types.Connection, path, mode, owner, group string) error {
	// Set permissions recursively if specified
	if mode != "" {
		cmd := fmt.Sprintf("find %s -type f -exec chmod %s {} +", path, mode)
		if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
			return err
		}
		cmd = fmt.Sprintf("find %s -type d -exec chmod %s {} +", path, mode)
		if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
			return err
		}
	}

	// Set ownership recursively if specified
	if owner != "" || group != "" {
		ownerGroup := ""
		if owner != "" && group != "" {
			ownerGroup = fmt.Sprintf("%s:%s", owner, group)
		} else if owner != "" {
			ownerGroup = owner
		} else {
			ownerGroup = fmt.Sprintf(":%s", group)
		}
		cmd := fmt.Sprintf("chown -R %s %s", ownerGroup, path)
		if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// GetCurrentTimestamp returns current timestamp
func (m *UnarchiveModule) GetCurrentTimestamp() int64 {
	return types.GetCurrentTime().Unix()
}