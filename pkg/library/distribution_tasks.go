package library

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"github.com/liliang-cn/gosible/pkg/types"
)

// DistributionTasks handles distribution of external files and binaries to cluster nodes
type DistributionTasks struct {
	// Local paths to distribute
	sources map[string]SourceInfo
	// Remote destinations
	destinations map[string]DestInfo
}

// SourceInfo contains information about a source file or directory
type SourceInfo struct {
	Path     string
	Type     string // "file", "directory", "binary", "archive"
	Checksum string // SHA256 checksum for verification
	Size     int64
	Mode     string
}

// DestInfo contains information about destination
type DestInfo struct {
	Path      string
	Owner     string
	Group     string
	Mode      string
	Backup    bool
	Validate  string // Command to validate after deployment
}

// NewDistributionTasks creates a new DistributionTasks instance
func NewDistributionTasks() *DistributionTasks {
	return &DistributionTasks{
		sources:      make(map[string]SourceInfo),
		destinations: make(map[string]DestInfo),
	}
}

// AddSource registers a local file or directory to distribute
func (dt *DistributionTasks) AddSource(name, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat source %s: %w", path, err)
	}
	
	source := SourceInfo{
		Path: path,
		Size: info.Size(),
		Mode: fmt.Sprintf("%04o", info.Mode().Perm()),
	}
	
	if info.IsDir() {
		source.Type = "directory"
	} else {
		source.Type = "file"
		// Calculate checksum for files
		if checksum, err := dt.calculateChecksum(path); err == nil {
			source.Checksum = checksum
		}
		
		// Detect binary files
		if dt.isBinary(path) {
			source.Type = "binary"
		}
		
		// Detect archives
		if dt.isArchive(path) {
			source.Type = "archive"
		}
	}
	
	dt.sources[name] = source
	return nil
}

// AddDestination configures where and how to deploy a source
func (dt *DistributionTasks) AddDestination(sourceName, destPath, owner, group, mode string) {
	dt.destinations[sourceName] = DestInfo{
		Path:   destPath,
		Owner:  owner,
		Group:  group,
		Mode:   mode,
		Backup: true,
	}
}

// DistributeFile creates tasks to distribute a single file
func (dt *DistributionTasks) DistributeFile(name string) []types.Task {
	source, exists := dt.sources[name]
	if !exists {
		return []types.Task{{
			Name:   "Source not found",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Source '%s' not registered", name),
			},
		}}
	}
	
	dest := dt.destinations[name]
	if dest.Path == "" {
		dest.Path = filepath.Join("/tmp", filepath.Base(source.Path))
	}
	
	tasks := []types.Task{}
	
	// For large files, use synchronize module for efficiency
	if source.Size > 10*1024*1024 { // > 10MB
		tasks = append(tasks, dt.distributeLargeFile(source, dest)...)
	} else {
		tasks = append(tasks, dt.distributeSmallFile(source, dest)...)
	}
	
	// Verify checksum if available
	if source.Checksum != "" {
		tasks = append(tasks, dt.verifyChecksum(dest.Path, source.Checksum)...)
	}
	
	// Run validation if specified
	if dest.Validate != "" {
		tasks = append(tasks, types.Task{
			Name:   "Validate distributed file",
			Module: "command",
			Args: map[string]interface{}{
				"cmd": dest.Validate + " " + dest.Path,
			},
		})
	}
	
	return tasks
}

// distributeSmallFile handles small file distribution via copy module
func (dt *DistributionTasks) distributeSmallFile(source SourceInfo, dest DestInfo) []types.Task {
	return []types.Task{
		{
			Name:   fmt.Sprintf("Distribute file %s", filepath.Base(source.Path)),
			Module: "copy",
			Args: map[string]interface{}{
				"src":    source.Path,
				"dest":   dest.Path,
				"owner":  dest.Owner,
				"group":  dest.Group,
				"mode":   dest.Mode,
				"backup": dest.Backup,
			},
		},
	}
}

// distributeLargeFile handles large file distribution via synchronize
func (dt *DistributionTasks) distributeLargeFile(source SourceInfo, dest DestInfo) []types.Task {
	tasks := []types.Task{
		{
			Name:   fmt.Sprintf("Synchronize large file %s", filepath.Base(source.Path)),
			Module: "synchronize",
			Args: map[string]interface{}{
				"src":      source.Path,
				"dest":     dest.Path,
				"compress": true,
				"checksum": true,
			},
		},
	}
	
	// Set permissions after sync
	if dest.Owner != "" || dest.Group != "" || dest.Mode != "" {
		tasks = append(tasks, types.Task{
			Name:   "Set file permissions",
			Module: "file",
			Args: map[string]interface{}{
				"path":  dest.Path,
				"owner": dest.Owner,
				"group": dest.Group,
				"mode":  dest.Mode,
			},
		})
	}
	
	return tasks
}

// DistributeBinary creates tasks to distribute and install a binary
func (dt *DistributionTasks) DistributeBinary(name, destPath string, makeExecutable bool) []types.Task {
	source, exists := dt.sources[name]
	if !exists {
		return []types.Task{{
			Name:   "Binary not found",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Binary '%s' not registered", name),
			},
		}}
	}
	
	if destPath == "" {
		destPath = filepath.Join("/usr/local/bin", filepath.Base(source.Path))
	}
	
	tasks := []types.Task{
		{
			Name:   fmt.Sprintf("Distribute binary %s", filepath.Base(source.Path)),
			Module: "copy",
			Args: map[string]interface{}{
				"src":  source.Path,
				"dest": destPath,
				"mode": "0755",
				"owner": "root",
				"group": "root",
			},
		},
	}
	
	// Verify binary works
	tasks = append(tasks, types.Task{
		Name:   "Verify binary is executable",
		Module: "command",
		Args: map[string]interface{}{
			"cmd": destPath + " --version",
		},
		IgnoreErrors: true, // Some binaries might not have --version
	})
	
	// Create symlink if needed
	if makeExecutable {
		binaryName := filepath.Base(destPath)
		tasks = append(tasks, types.Task{
			Name:   fmt.Sprintf("Create symlink for %s", binaryName),
			Module: "file",
			Args: map[string]interface{}{
				"src":   destPath,
				"dest":  filepath.Join("/usr/bin", binaryName),
				"state": "link",
			},
		})
	}
	
	return tasks
}

// DistributeDirectory creates tasks to distribute an entire directory
func (dt *DistributionTasks) DistributeDirectory(name, destPath string, excludePatterns []string) []types.Task {
	source, exists := dt.sources[name]
	if !exists || source.Type != "directory" {
		return []types.Task{{
			Name:   "Directory not found",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Directory '%s' not registered", name),
			},
		}}
	}
	
	if destPath == "" {
		destPath = filepath.Join("/opt", filepath.Base(source.Path))
	}
	
	dest := dt.destinations[name]
	
	syncArgs := map[string]interface{}{
		"src":      source.Path + "/",
		"dest":     destPath,
		"recursive": true,
		"compress": true,
		"delete":   false, // Don't delete extra files by default
	}
	
	// Add exclude patterns
	if len(excludePatterns) > 0 {
		syncArgs["rsync_opts"] = excludePatterns
	}
	
	tasks := []types.Task{
		{
			Name:   fmt.Sprintf("Ensure destination directory %s exists", destPath),
			Module: "file",
			Args: map[string]interface{}{
				"path":  destPath,
				"state": "directory",
				"owner": dest.Owner,
				"group": dest.Group,
				"mode":  "0755",
			},
		},
		{
			Name:   fmt.Sprintf("Synchronize directory %s", filepath.Base(source.Path)),
			Module: "synchronize",
			Args:   syncArgs,
		},
	}
	
	// Set ownership recursively if specified
	if dest.Owner != "" || dest.Group != "" {
		tasks = append(tasks, types.Task{
			Name:   "Set directory ownership",
			Module: "file",
			Args: map[string]interface{}{
				"path":    destPath,
				"owner":   dest.Owner,
				"group":   dest.Group,
				"recurse": true,
			},
		})
	}
	
	return tasks
}

// DistributeArchive creates tasks to distribute and extract an archive
func (dt *DistributionTasks) DistributeArchive(name, destPath string, stripComponents int) []types.Task {
	source, exists := dt.sources[name]
	if !exists {
		return []types.Task{{
			Name:   "Archive not found",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Archive '%s' not registered", name),
			},
		}}
	}
	
	if destPath == "" {
		destPath = "/opt"
	}
	
	tempPath := filepath.Join("/tmp", filepath.Base(source.Path))
	
	tasks := []types.Task{
		{
			Name:   fmt.Sprintf("Copy archive %s to remote", filepath.Base(source.Path)),
			Module: "copy",
			Args: map[string]interface{}{
				"src":  source.Path,
				"dest": tempPath,
			},
		},
		{
			Name:   fmt.Sprintf("Extract archive to %s", destPath),
			Module: "unarchive",
			Args: map[string]interface{}{
				"src":        tempPath,
				"dest":       destPath,
				"remote_src": true,
				"extra_opts": dt.getExtractOptions(source.Path, stripComponents),
			},
		},
		{
			Name:   "Clean up archive file",
			Module: "file",
			Args: map[string]interface{}{
				"path":  tempPath,
				"state": "absent",
			},
		},
	}
	
	// Verify extraction
	if source.Checksum != "" {
		tasks = append(tasks, types.Task{
			Name:   "Verify extraction completed",
			Module: "stat",
			Args: map[string]interface{}{
				"path": destPath,
			},
			Register: "extraction_result",
		})
	}
	
	return tasks
}

// DistributeWithFallback distributes using multiple methods with fallback
func (dt *DistributionTasks) DistributeWithFallback(name string, methods []string) []types.Task {
	source, exists := dt.sources[name]
	if !exists {
		return []types.Task{{
			Name:   "Source not found",
			Module: "fail",
			Args: map[string]interface{}{
				"msg": fmt.Sprintf("Source '%s' not registered", name),
			},
		}}
	}
	
	dest := dt.destinations[name]
	tasks := []types.Task{}
	
	for i, method := range methods {
		var methodTasks []types.Task
		
		switch method {
		case "rsync":
			methodTasks = dt.distributeViaRsync(source, dest)
		case "scp":
			methodTasks = dt.distributeViaSCP(source, dest)
		case "http":
			methodTasks = dt.distributeViaHTTP(source, dest)
		case "s3":
			methodTasks = dt.distributeViaS3(source, dest)
		default:
			continue
		}
		
		// Add conditional execution (skip if previous method succeeded)
		for j := range methodTasks {
			if i > 0 {
				methodTasks[j].When = fmt.Sprintf("distribution_method_%d is failed", i-1)
			}
			if j == 0 {
				methodTasks[j].Register = fmt.Sprintf("distribution_method_%d", i)
			}
		}
		
		tasks = append(tasks, methodTasks...)
	}
	
	return tasks
}

// distributeViaRsync uses rsync for distribution
func (dt *DistributionTasks) distributeViaRsync(source SourceInfo, dest DestInfo) []types.Task {
	return []types.Task{
		{
			Name:   "Distribute via rsync",
			Module: "synchronize",
			Args: map[string]interface{}{
				"src":      source.Path,
				"dest":     dest.Path,
				"compress": true,
				"checksum": true,
			},
		},
	}
}

// distributeViaSCP uses SCP for distribution
func (dt *DistributionTasks) distributeViaSCP(source SourceInfo, dest DestInfo) []types.Task {
	return []types.Task{
		{
			Name:   "Distribute via SCP",
			Module: "copy",
			Args: map[string]interface{}{
				"src":  source.Path,
				"dest": dest.Path,
			},
		},
	}
}

// distributeViaHTTP downloads from HTTP server
func (dt *DistributionTasks) distributeViaHTTP(source SourceInfo, dest DestInfo) []types.Task {
	// Assumes source.Path contains HTTP URL
	return []types.Task{
		{
			Name:   "Download via HTTP",
			Module: "get_url",
			Args: map[string]interface{}{
				"url":      source.Path,
				"dest":     dest.Path,
				"checksum": "sha256:" + source.Checksum,
			},
		},
	}
}

// distributeViaS3 downloads from S3
func (dt *DistributionTasks) distributeViaS3(source SourceInfo, dest DestInfo) []types.Task {
	// Assumes source.Path contains S3 URL
	return []types.Task{
		{
			Name:   "Download from S3",
			Module: "aws_s3",
			Args: map[string]interface{}{
				"bucket": dt.extractS3Bucket(source.Path),
				"object": dt.extractS3Object(source.Path),
				"dest":   dest.Path,
				"mode":   "get",
			},
		},
	}
}

// ParallelDistribute creates tasks to distribute multiple files in parallel
func (dt *DistributionTasks) ParallelDistribute(names []string, maxParallel int) []types.Task {
	if maxParallel <= 0 {
		maxParallel = 5
	}
	
	tasks := []types.Task{}
	
	// Group files for parallel distribution
	for i := 0; i < len(names); i += maxParallel {
		end := i + maxParallel
		if end > len(names) {
			end = len(names)
		}
		
		batch := names[i:end]
		
		// Create async tasks for this batch
		for _, name := range batch {
			source := dt.sources[name]
			dest := dt.destinations[name]
			
			tasks = append(tasks, types.Task{
				Name:   fmt.Sprintf("Distribute %s (parallel)", name),
				Module: "copy",
				Args: map[string]interface{}{
					"src":   source.Path,
					"dest":  dest.Path,
					"owner": dest.Owner,
					"group": dest.Group,
					"mode":  dest.Mode,
				},
				Async: 30, // 30 second timeout
				Poll:  0,  // Don't wait
			})
		}
		
		// Wait for batch to complete
		tasks = append(tasks, types.Task{
			Name:   fmt.Sprintf("Wait for batch %d distribution", i/maxParallel+1),
			Module: "async_status",
			Args: map[string]interface{}{
				"jid": "{{ item.ansible_job_id }}",
			},
			Loop:       "{{ async_results.results }}",
			Register:   fmt.Sprintf("batch_%d_result", i/maxParallel+1),
			Until:      "item.finished",
			Retries:    30,
			Delay:      1,
		})
	}
	
	return tasks
}

// verifyChecksum creates tasks to verify file checksum
func (dt *DistributionTasks) verifyChecksum(path, expectedChecksum string) []types.Task {
	return []types.Task{
		{
			Name:   "Calculate file checksum",
			Module: "stat",
			Args: map[string]interface{}{
				"path":     path,
				"checksum_algorithm": "sha256",
			},
			Register: "file_stat",
		},
		{
			Name:   "Verify checksum matches",
			Module: "assert",
			Args: map[string]interface{}{
				"that": []string{
					fmt.Sprintf("file_stat.stat.checksum == '%s'", expectedChecksum),
				},
				"fail_msg": fmt.Sprintf("Checksum mismatch for %s", path),
			},
		},
	}
}

// calculateChecksum calculates SHA256 checksum of a file
func (dt *DistributionTasks) calculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// isBinary checks if a file is likely a binary
func (dt *DistributionTasks) isBinary(path string) bool {
	// Check by extension
	binaryExts := []string{".exe", ".bin", ".so", ".dll", ".dylib", ".app"}
	for _, ext := range binaryExts {
		if filepath.Ext(path) == ext {
			return true
		}
	}
	
	// Check if file has no extension and is executable
	info, err := os.Stat(path)
	if err == nil && filepath.Ext(path) == "" && info.Mode()&0111 != 0 {
		return true
	}
	
	return false
}

// isArchive checks if a file is an archive
func (dt *DistributionTasks) isArchive(path string) bool {
	archiveExts := []string{".tar", ".gz", ".tgz", ".zip", ".bz2", ".xz", ".tar.gz", ".tar.bz2", ".tar.xz"}
	basename := filepath.Base(path)
	for _, ext := range archiveExts {
		if filepath.Ext(path) == ext {
			return true
		}
		// Check for compound extensions like .tar.gz
		if len(basename) >= len(ext) && basename[len(basename)-len(ext):] == ext {
			return true
		}
	}
	return false
}

// getExtractOptions returns extraction options based on archive type
func (dt *DistributionTasks) getExtractOptions(path string, stripComponents int) []string {
	opts := []string{}
	
	if stripComponents > 0 {
		opts = append(opts, fmt.Sprintf("--strip-components=%d", stripComponents))
	}
	
	return opts
}

// extractS3Bucket extracts bucket name from S3 URL
func (dt *DistributionTasks) extractS3Bucket(s3url string) string {
	// Simple extraction, assumes s3://bucket/object format
	if len(s3url) > 5 && s3url[:5] == "s3://" {
		parts := filepath.SplitList(s3url[5:])
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}

// extractS3Object extracts object path from S3 URL
func (dt *DistributionTasks) extractS3Object(s3url string) string {
	// Simple extraction, assumes s3://bucket/object format
	if len(s3url) > 5 && s3url[:5] == "s3://" {
		idx := len("s3://")
		remaining := s3url[idx:]
		if pos := len(remaining); pos > 0 {
			parts := filepath.SplitList(remaining)
			if len(parts) > 1 {
				return filepath.Join(parts[1:]...)
			}
		}
	}
	return ""
}

// CleanupDistributed creates tasks to clean up distributed files
func (dt *DistributionTasks) CleanupDistributed(names []string) []types.Task {
	tasks := []types.Task{}
	
	for _, name := range names {
		dest := dt.destinations[name]
		if dest.Path != "" {
			tasks = append(tasks, types.Task{
				Name:   fmt.Sprintf("Remove distributed file %s", name),
				Module: "file",
				Args: map[string]interface{}{
					"path":  dest.Path,
					"state": "absent",
				},
			})
		}
	}
	
	return tasks
}