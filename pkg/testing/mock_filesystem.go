package testing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockFile represents a file in the mock filesystem
type MockFile struct {
	Content     []byte
	Mode        os.FileMode
	ModTime     time.Time
	IsDir       bool
	Exists      bool
	Owner       string
	Group       string
	ReadOnly    bool
	AccessError error // Error to return when accessing this file
}

// MockFileSystem provides an in-memory filesystem for testing
type MockFileSystem struct {
	t         *testing.T
	mu        sync.RWMutex
	files     map[string]*MockFile
	operations []FileOperation
	readOnlyPaths []string
	simulateErrors bool
}

// FileOperation records filesystem operations for testing
type FileOperation struct {
	Operation string // "read", "write", "create", "delete", "chmod", "chown"
	Path      string
	Content   []byte
	Mode      os.FileMode
	Success   bool
	Error     error
	Timestamp time.Time
}

// NewMockFileSystem creates a new mock filesystem
func NewMockFileSystem(t *testing.T) *MockFileSystem {
	return &MockFileSystem{
		t:         t,
		files:     make(map[string]*MockFile),
		operations: make([]FileOperation, 0),
		readOnlyPaths: make([]string, 0),
		simulateErrors: false,
	}
}

// CreateFile creates a file in the mock filesystem
func (m *MockFileSystem) CreateFile(path string, content []byte, mode os.FileMode) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.files[path] = &MockFile{
		Content: content,
		Mode:    mode,
		ModTime: time.Now(),
		IsDir:   false,
		Exists:  true,
		Owner:   "root",
		Group:   "root",
	}
	
	return m
}

// CreateDirectory creates a directory in the mock filesystem
func (m *MockFileSystem) CreateDirectory(path string, mode os.FileMode) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.files[path] = &MockFile{
		Mode:    mode,
		ModTime: time.Now(),
		IsDir:   true,
		Exists:  true,
		Owner:   "root",
		Group:   "root",
	}
	
	return m
}

// SetFileOwner sets the owner and group of a file
func (m *MockFileSystem) SetFileOwner(path, owner, group string) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if file, exists := m.files[path]; exists {
		file.Owner = owner
		file.Group = group
	}
	
	return m
}

// SetFileReadOnly marks a file as read-only
func (m *MockFileSystem) SetFileReadOnly(path string, readOnly bool) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if file, exists := m.files[path]; exists {
		file.ReadOnly = readOnly
	}
	
	return m
}

// SetFileError sets an error to be returned when accessing a file
func (m *MockFileSystem) SetFileError(path string, err error) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if file, exists := m.files[path]; exists {
		file.AccessError = err
	} else {
		// Create a file that doesn't exist but has an error
		m.files[path] = &MockFile{
			Exists:      false,
			AccessError: err,
		}
	}
	
	return m
}

// AddReadOnlyPath adds a path pattern that should be treated as read-only
func (m *MockFileSystem) AddReadOnlyPath(pathPattern string) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.readOnlyPaths = append(m.readOnlyPaths, pathPattern)
	return m
}

// EnableErrorSimulation enables simulation of various filesystem errors
func (m *MockFileSystem) EnableErrorSimulation(enable bool) *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.simulateErrors = enable
	return m
}

// FileExists checks if a file exists in the mock filesystem
func (m *MockFileSystem) FileExists(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	file, exists := m.files[path]
	return exists && file.Exists
}

// ReadFile reads a file from the mock filesystem
func (m *MockFileSystem) ReadFile(path string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordOperation("read", path, nil, 0, true, nil)
	
	file, exists := m.files[path]
	if !exists || !file.Exists {
		err := &os.PathError{Op: "read", Path: path, Err: os.ErrNotExist}
		m.updateLastOperation(false, err)
		return nil, err
	}
	
	if file.AccessError != nil {
		m.updateLastOperation(false, file.AccessError)
		return nil, file.AccessError
	}
	
	// Simulate permission error for read-only files
	if m.isReadOnlyPath(path) && file.ReadOnly {
		err := &os.PathError{Op: "read", Path: path, Err: os.ErrPermission}
		m.updateLastOperation(false, err)
		return nil, err
	}
	
	return file.Content, nil
}

// WriteFile writes a file to the mock filesystem
func (m *MockFileSystem) WriteFile(path string, content []byte, mode os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordOperation("write", path, content, mode, true, nil)
	
	// Check if path is read-only
	if m.isReadOnlyPath(path) {
		err := &os.PathError{Op: "write", Path: path, Err: os.ErrPermission}
		m.updateLastOperation(false, err)
		return err
	}
	
	// Check if existing file is read-only
	if file, exists := m.files[path]; exists && file.ReadOnly {
		err := &os.PathError{Op: "write", Path: path, Err: os.ErrPermission}
		m.updateLastOperation(false, err)
		return err
	}
	
	// Create or update the file
	m.files[path] = &MockFile{
		Content: content,
		Mode:    mode,
		ModTime: time.Now(),
		IsDir:   false,
		Exists:  true,
		Owner:   "root",
		Group:   "root",
	}
	
	return nil
}

// CreateDir creates a directory in the mock filesystem
func (m *MockFileSystem) CreateDir(path string, mode os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordOperation("create", path, nil, mode, true, nil)
	
	// Check if path is read-only
	if m.isReadOnlyPath(path) {
		err := &os.PathError{Op: "mkdir", Path: path, Err: os.ErrPermission}
		m.updateLastOperation(false, err)
		return err
	}
	
	m.files[path] = &MockFile{
		Mode:    mode,
		ModTime: time.Now(),
		IsDir:   true,
		Exists:  true,
		Owner:   "root",
		Group:   "root",
	}
	
	return nil
}

// RemoveFile removes a file from the mock filesystem
func (m *MockFileSystem) RemoveFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordOperation("delete", path, nil, 0, true, nil)
	
	file, exists := m.files[path]
	if !exists || !file.Exists {
		err := &os.PathError{Op: "remove", Path: path, Err: os.ErrNotExist}
		m.updateLastOperation(false, err)
		return err
	}
	
	// Check if path is read-only
	if m.isReadOnlyPath(path) || file.ReadOnly {
		err := &os.PathError{Op: "remove", Path: path, Err: os.ErrPermission}
		m.updateLastOperation(false, err)
		return err
	}
	
	file.Exists = false
	return nil
}

// Stat returns file information
func (m *MockFileSystem) Stat(path string) (os.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	file, exists := m.files[path]
	if !exists || !file.Exists {
		return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
	}
	
	if file.AccessError != nil {
		return nil, file.AccessError
	}
	
	return &MockFileInfo{
		name:    filepath.Base(path),
		size:    int64(len(file.Content)),
		mode:    file.Mode,
		modTime: file.ModTime,
		isDir:   file.IsDir,
	}, nil
}

// Chmod changes file permissions
func (m *MockFileSystem) Chmod(path string, mode os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordOperation("chmod", path, nil, mode, true, nil)
	
	file, exists := m.files[path]
	if !exists || !file.Exists {
		err := &os.PathError{Op: "chmod", Path: path, Err: os.ErrNotExist}
		m.updateLastOperation(false, err)
		return err
	}
	
	// Check if path is read-only
	if m.isReadOnlyPath(path) || file.ReadOnly {
		err := &os.PathError{Op: "chmod", Path: path, Err: os.ErrPermission}
		m.updateLastOperation(false, err)
		return err
	}
	
	file.Mode = mode
	return nil
}

// Chown changes file ownership
func (m *MockFileSystem) Chown(path, owner, group string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.recordOperation("chown", path, []byte(fmt.Sprintf("%s:%s", owner, group)), 0, true, nil)
	
	file, exists := m.files[path]
	if !exists || !file.Exists {
		err := &os.PathError{Op: "chown", Path: path, Err: os.ErrNotExist}
		m.updateLastOperation(false, err)
		return err
	}
	
	// Check if path is read-only
	if m.isReadOnlyPath(path) || file.ReadOnly {
		err := &os.PathError{Op: "chown", Path: path, Err: os.ErrPermission}
		m.updateLastOperation(false, err)
		return err
	}
	
	file.Owner = owner
	file.Group = group
	return nil
}

// GetFileContent returns the content of a file (for testing)
func (m *MockFileSystem) GetFileContent(path string) []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if file, exists := m.files[path]; exists && file.Exists {
		return file.Content
	}
	return nil
}

// GetFileMode returns the mode of a file (for testing)
func (m *MockFileSystem) GetFileMode(path string) os.FileMode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if file, exists := m.files[path]; exists && file.Exists {
		return file.Mode
	}
	return 0
}

// GetOperations returns all recorded filesystem operations
func (m *MockFileSystem) GetOperations() []FileOperation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy to prevent modification
	result := make([]FileOperation, len(m.operations))
	copy(result, m.operations)
	return result
}

// GetOperationCount returns the number of operations of a specific type
func (m *MockFileSystem) GetOperationCount(operation string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	count := 0
	for _, op := range m.operations {
		if op.Operation == operation {
			count++
		}
	}
	return count
}

// Reset clears all files and operations
func (m *MockFileSystem) Reset() *MockFileSystem {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.files = make(map[string]*MockFile)
	m.operations = make([]FileOperation, 0)
	m.readOnlyPaths = make([]string, 0)
	return m
}

// AssertFileExists asserts that a file exists
func (m *MockFileSystem) AssertFileExists(path string) {
	if !m.FileExists(path) {
		m.t.Errorf("Expected file '%s' to exist, but it doesn't", path)
	}
}

// AssertFileNotExists asserts that a file does not exist
func (m *MockFileSystem) AssertFileNotExists(path string) {
	if m.FileExists(path) {
		m.t.Errorf("Expected file '%s' to not exist, but it does", path)
	}
}

// AssertFileContent asserts that a file has specific content
func (m *MockFileSystem) AssertFileContent(path string, expectedContent []byte) {
	content := m.GetFileContent(path)
	if string(content) != string(expectedContent) {
		m.t.Errorf("Expected file '%s' to have content '%s', but got '%s'", path, expectedContent, content)
	}
}

// AssertFileMode asserts that a file has a specific mode
func (m *MockFileSystem) AssertFileMode(path string, expectedMode os.FileMode) {
	mode := m.GetFileMode(path)
	if mode != expectedMode {
		m.t.Errorf("Expected file '%s' to have mode %v, but got %v", path, expectedMode, mode)
	}
}

// AssertOperationCount asserts the number of operations of a specific type
func (m *MockFileSystem) AssertOperationCount(operation string, expectedCount int) {
	count := m.GetOperationCount(operation)
	if count != expectedCount {
		m.t.Errorf("Expected %d '%s' operations, but got %d", expectedCount, operation, count)
	}
}

// AssertOperationOccurred asserts that a specific operation occurred on a path
func (m *MockFileSystem) AssertOperationOccurred(operation, path string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, op := range m.operations {
		if op.Operation == operation && op.Path == path && op.Success {
			return
		}
	}
	m.t.Errorf("Expected operation '%s' on path '%s', but it didn't occur", operation, path)
}

// AssertOperationNotOccurred asserts that a specific operation did not occur on a path
func (m *MockFileSystem) AssertOperationNotOccurred(operation, path string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, op := range m.operations {
		if op.Operation == operation && op.Path == path && op.Success {
			m.t.Errorf("Expected operation '%s' on path '%s' to not occur, but it did", operation, path)
			return
		}
	}
}

// isReadOnlyPath checks if a path matches any read-only patterns
func (m *MockFileSystem) isReadOnlyPath(path string) bool {
	for _, pattern := range m.readOnlyPaths {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Also check if pattern is a prefix
		if strings.HasPrefix(path, pattern) {
			return true
		}
	}
	return false
}

// recordOperation records a filesystem operation
func (m *MockFileSystem) recordOperation(operation, path string, content []byte, mode os.FileMode, success bool, err error) {
	m.operations = append(m.operations, FileOperation{
		Operation: operation,
		Path:      path,
		Content:   content,
		Mode:      mode,
		Success:   success,
		Error:     err,
		Timestamp: time.Now(),
	})
}

// updateLastOperation updates the last recorded operation
func (m *MockFileSystem) updateLastOperation(success bool, err error) {
	if len(m.operations) > 0 {
		m.operations[len(m.operations)-1].Success = success
		m.operations[len(m.operations)-1].Error = err
	}
}

// MockFileInfo implements os.FileInfo for testing
type MockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m *MockFileInfo) Name() string       { return m.name }
func (m *MockFileInfo) Size() int64        { return m.size }
func (m *MockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *MockFileInfo) ModTime() time.Time { return m.modTime }
func (m *MockFileInfo) IsDir() bool        { return m.isDir }
func (m *MockFileInfo) Sys() interface{}   { return nil }

// Helper functions for common test scenarios

// CreateStandardFileStructure creates a common file structure for testing
func (m *MockFileSystem) CreateStandardFileStructure() *MockFileSystem {
	m.CreateDirectory("/etc", 0755)
	m.CreateDirectory("/etc/systemd", 0755)
	m.CreateDirectory("/etc/systemd/system", 0755)
	m.CreateDirectory("/var", 0755)
	m.CreateDirectory("/var/log", 0755)
	m.CreateDirectory("/tmp", 0755)
	
	m.CreateFile("/etc/hosts", []byte("127.0.0.1 localhost\n"), 0644)
	m.CreateFile("/etc/passwd", []byte("root:x:0:0:root:/root:/bin/bash\n"), 0644)
	
	return m
}

// SimulatePermissionDenied simulates permission denied errors for a path
func (m *MockFileSystem) SimulatePermissionDenied(path string) *MockFileSystem {
	return m.SetFileError(path, &os.PathError{
		Op:   "access",
		Path: path,
		Err:  os.ErrPermission,
	})
}

// SimulateFileNotFound simulates file not found errors for a path
func (m *MockFileSystem) SimulateFileNotFound(path string) *MockFileSystem {
	return m.SetFileError(path, &os.PathError{
		Op:   "access",
		Path: path,
		Err:  os.ErrNotExist,
	})
}

// SimulateDiskFull simulates disk full errors for write operations
func (m *MockFileSystem) SimulateDiskFull(path string) *MockFileSystem {
	return m.SetFileError(path, &os.PathError{
		Op:   "write",
		Path: path,
		Err:  fmt.Errorf("no space left on device"),
	})
}