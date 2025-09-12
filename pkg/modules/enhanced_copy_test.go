package modules

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// Mock connection that implements ProgressCopyConnection
type mockProgressConnection struct {
	copyWithProgressCalled bool
	copyCalled            bool
	copyError             error
	progressUpdates       []types.ProgressInfo
}

func (m *mockProgressConnection) CopyWithProgress(ctx context.Context, src io.Reader, dest string, mode int, totalSize int64, progressCallback func(progress types.ProgressInfo)) error {
	m.copyWithProgressCalled = true
	
	// Simulate progress updates
	for i := 0; i <= 100; i += 25 {
		progress := types.ProgressInfo{
			Stage:      "transferring",
			Percentage: float64(i),
			Message:    "Copying data...",
			Timestamp:  time.Now(),
		}
		progressCallback(progress)
		m.progressUpdates = append(m.progressUpdates, progress)
	}
	
	return m.copyError
}

func (m *mockProgressConnection) Copy(ctx context.Context, src io.Reader, dest string, mode int) error {
	m.copyCalled = true
	return m.copyError
}

func (m *mockProgressConnection) Execute(ctx context.Context, command string, options types.ExecuteOptions) (*types.Result, error) {
	return &types.Result{Success: true}, nil
}

func (m *mockProgressConnection) ExecuteStream(ctx context.Context, command string, options types.ExecuteOptions) (<-chan types.StreamEvent, error) {
	events := make(chan types.StreamEvent, 1)
	close(events)
	return events, nil
}

func (m *mockProgressConnection) Close() error {
	return nil
}

func (m *mockProgressConnection) Connect(ctx context.Context, info types.ConnectionInfo) error {
	return nil
}

func (m *mockProgressConnection) Fetch(ctx context.Context, src string) (io.Reader, error) {
	return nil, nil
}

func (m *mockProgressConnection) IsConnected() bool {
	return true
}

// Mock connection that doesn't support progress
type mockBasicConnection struct {
	copyCalled bool
	copyError  error
}

func (m *mockBasicConnection) Copy(ctx context.Context, src io.Reader, dest string, mode int) error {
	m.copyCalled = true
	return m.copyError
}

func (m *mockBasicConnection) Execute(ctx context.Context, command string, options types.ExecuteOptions) (*types.Result, error) {
	return &types.Result{Success: true}, nil
}

func (m *mockBasicConnection) ExecuteStream(ctx context.Context, command string, options types.ExecuteOptions) (<-chan types.StreamEvent, error) {
	events := make(chan types.StreamEvent, 1)
	close(events)
	return events, nil
}

func (m *mockBasicConnection) Close() error {
	return nil
}

func (m *mockBasicConnection) Connect(ctx context.Context, info types.ConnectionInfo) error {
	return nil
}

func (m *mockBasicConnection) Fetch(ctx context.Context, src string) (io.Reader, error) {
	return nil, nil
}

func (m *mockBasicConnection) IsConnected() bool {
	return true
}

func TestNewEnhancedCopyModule(t *testing.T) {
	module := NewEnhancedCopyModule()
	
	if module == nil {
		t.Fatal("NewEnhancedCopyModule should not return nil")
	}
	
	if module.name != "enhanced_copy" {
		t.Errorf("Expected module name 'enhanced_copy', got '%s'", module.name)
	}
}

func TestEnhancedCopyModule_RunWithProgress(t *testing.T) {
	module := NewEnhancedCopyModule()
	conn := &mockProgressConnection{}
	
	args := map[string]interface{}{
		"content":       "Test file content",
		"dest":          "/tmp/test.txt",
		"mode":          "0644",
		"show_progress": true,
	}
	
	ctx := context.Background()
	result, err := module.Run(ctx, conn, args)
	
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	
	if !result.Success {
		t.Error("Result should indicate success")
	}
	
	if !result.Changed {
		t.Error("Result should indicate change")
	}
	
	if !conn.copyWithProgressCalled {
		t.Error("CopyWithProgress should have been called")
	}
	
	if conn.copyCalled {
		t.Error("Standard Copy should not have been called")
	}
	
	// Verify result data
	if result.Data["source_type"] != "content" {
		t.Errorf("Expected source_type 'content', got '%v'", result.Data["source_type"])
	}
	
	if result.Data["dest"] != "/tmp/test.txt" {
		t.Errorf("Expected dest '/tmp/test.txt', got '%v'", result.Data["dest"])
	}
	
	if result.Data["file_mode"] != "644" {
		t.Errorf("Expected file_mode '644', got '%v'", result.Data["file_mode"])
	}
	
	// Check for progress updates in result
	if _, ok := result.Data["progress_updates"]; !ok {
		t.Error("Result should contain progress_updates")
	}
}

func TestEnhancedCopyModule_RunStandardCopy(t *testing.T) {
	module := NewEnhancedCopyModule()
	conn := &mockBasicConnection{}
	
	args := map[string]interface{}{
		"content": "Test file content",
		"dest":    "/tmp/test.txt",
		"mode":    "0755",
	}
	
	ctx := context.Background()
	result, err := module.Run(ctx, conn, args)
	
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	
	if !result.Success {
		t.Error("Result should indicate success")
	}
	
	if !conn.copyCalled {
		t.Error("Copy should have been called")
	}
	
	// Verify fallback to standard copy
	if result.Data["progress_enabled"] != false {
		t.Error("progress_enabled should be false for standard copy")
	}
	
	if result.Data["file_mode"] != "755" {
		t.Errorf("Expected file_mode '755', got '%v'", result.Data["file_mode"])
	}
}

func TestEnhancedCopyModule_RunWithFile(t *testing.T) {
	module := NewEnhancedCopyModule()
	conn := &mockProgressConnection{}
	
	// Create temporary test file
	tempFile := t.TempDir() + "/source.txt"
	
	// We'll simulate this since the module opens the file
	args := map[string]interface{}{
		"src":           tempFile,
		"dest":          "/tmp/dest.txt",
		"show_progress": true,
	}
	
	ctx := context.Background()
	
	// This will fail because the file doesn't exist, but we can test validation
	_, err := module.Run(ctx, conn, args)
	
	// Expect error for non-existent source file
	if err == nil {
		t.Error("Expected error for non-existent source file")
	}
	
	if !strings.Contains(err.Error(), "failed to open source file") {
		t.Errorf("Expected 'failed to open source file' error, got: %v", err)
	}
}

func TestEnhancedCopyModule_Validate(t *testing.T) {
	module := NewEnhancedCopyModule()
	
	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid content copy",
			args: map[string]interface{}{
				"content": "test content",
				"dest":    "/tmp/test.txt",
			},
			expectError: false,
		},
		{
			name: "valid file copy",
			args: map[string]interface{}{
				"src":  "/source/file.txt",
				"dest": "/tmp/test.txt",
			},
			expectError: false,
		},
		{
			name: "missing src and content",
			args: map[string]interface{}{
				"dest": "/tmp/test.txt",
			},
			expectError: true,
			errorMsg:    "either 'src' or 'content' must be specified",
		},
		{
			name: "both src and content specified",
			args: map[string]interface{}{
				"src":     "/source/file.txt",
				"content": "test content",
				"dest":    "/tmp/test.txt",
			},
			expectError: true,
			errorMsg:    "cannot specify both 'src' and 'content'",
		},
		{
			name: "missing dest",
			args: map[string]interface{}{
				"content": "test content",
			},
			expectError: true,
			errorMsg:    "'dest' parameter is required",
		},
		{
			name: "invalid mode",
			args: map[string]interface{}{
				"content": "test content",
				"dest":    "/tmp/test.txt",
				"mode":    "invalid",
			},
			expectError: true,
			errorMsg:    "invalid mode",
		},
		{
			name: "valid mode with leading zero",
			args: map[string]interface{}{
				"content": "test content",
				"dest":    "/tmp/test.txt",
				"mode":    "0755",
			},
			expectError: false,
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := module.Validate(test.args)
			
			if test.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s'", test.name)
				} else if !strings.Contains(err.Error(), test.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", test.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", test.name, err)
				}
			}
		})
	}
}

func TestEnhancedCopyModule_Documentation(t *testing.T) {
	module := NewEnhancedCopyModule()
	doc := module.Documentation()
	
	if doc.Name != "enhanced_copy" {
		t.Errorf("Expected name 'enhanced_copy', got '%s'", doc.Name)
	}
	
	if doc.Description == "" {
		t.Error("Documentation should have a description")
	}
	
	// Check required parameters
	requiredParams := []string{"dest"}
	for _, param := range requiredParams {
		if paramDoc, exists := doc.Parameters[param]; !exists {
			t.Errorf("Missing parameter '%s' in documentation", param)
		} else if !paramDoc.Required {
			t.Errorf("Parameter '%s' should be marked as required", param)
		}
	}
	
	// Check optional parameters
	optionalParams := []string{"src", "content", "mode", "backup", "show_progress"}
	for _, param := range optionalParams {
		if paramDoc, exists := doc.Parameters[param]; !exists {
			t.Errorf("Missing parameter '%s' in documentation", param)
		} else if paramDoc.Required {
			t.Errorf("Parameter '%s' should be marked as optional", param)
		}
	}
	
	// Check examples
	if len(doc.Examples) == 0 {
		t.Error("Documentation should include examples")
	}
	
	// Check return values
	if len(doc.Returns) == 0 {
		t.Error("Documentation should specify return values")
	}
}

func TestParseFileMode(t *testing.T) {
	tests := []struct {
		input    string
		expected uint32
		hasError bool
	}{
		{"644", 0644, false},
		{"755", 0755, false},
		{"0644", 0644, false},
		{"0755", 0755, false},
		{"600", 0600, false},
		{"777", 0777, false},
		{"invalid", 0, true},
		{"999", 0, true}, // Invalid octal
		{"", 0, true},
	}
	
	for _, test := range tests {
		result, err := parseFileMode(test.input)
		
		if test.hasError {
			if err == nil {
				t.Errorf("Expected error for input '%s'", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", test.input, err)
			} else if uint32(result) != test.expected {
				t.Errorf("Expected mode %o for input '%s', got %o", test.expected, test.input, uint32(result))
			}
		}
	}
}

func TestEnhancedCopyModule_RunWithBackup(t *testing.T) {
	module := NewEnhancedCopyModule()
	conn := &mockProgressConnection{}
	
	args := map[string]interface{}{
		"content": "New content",
		"dest":    "/tmp/existing.txt",
		"backup":  true,
	}
	
	ctx := context.Background()
	result, err := module.Run(ctx, conn, args)
	
	if err != nil {
		t.Fatalf("Run with backup failed: %v", err)
	}
	
	if !result.Success {
		t.Error("Result should indicate success")
	}
	
	// Check backup-related data in result
	if backupCreated, ok := result.Data["backup_created"].(bool); !ok || !backupCreated {
		t.Error("backup_created should be true in result data")
	}
	
	if _, ok := result.Data["backup_path"]; !ok {
		t.Error("backup_path should be present in result data")
	}
}

func TestEnhancedCopyModule_RunWithError(t *testing.T) {
	module := NewEnhancedCopyModule()
	conn := &mockProgressConnection{
		copyError: io.ErrShortWrite, // Set up a proper error
	}
	
	args := map[string]interface{}{
		"content": "Test content",
		"dest":    "/tmp/test.txt",
	}
	
	ctx := context.Background()
	_, err := module.Run(ctx, conn, args)
	
	if err == nil {
		t.Error("Expected error from copy operation")
	}
	
	if !strings.Contains(err.Error(), "copy operation failed") {
		t.Errorf("Expected 'copy operation failed' error, got: %v", err)
	}
}

// Integration test with actual progress tracking
func TestEnhancedCopyModule_ProgressIntegration(t *testing.T) {
	module := NewEnhancedCopyModule()
	conn := &mockProgressConnection{}
	
	// Large content to trigger multiple progress updates
	largeContent := strings.Repeat("This is a line of test content for progress tracking.\n", 100)
	
	args := map[string]interface{}{
		"content":       largeContent,
		"dest":          "/tmp/large_test.txt",
		"show_progress": true,
	}
	
	ctx := context.Background()
	result, err := module.Run(ctx, conn, args)
	
	if err != nil {
		t.Fatalf("Integration test failed: %v", err)
	}
	
	// Verify progress updates were received
	if len(conn.progressUpdates) == 0 {
		t.Error("Expected progress updates")
	}
	
	// Check that progress went from 0 to 100
	firstProgress := conn.progressUpdates[0]
	lastProgress := conn.progressUpdates[len(conn.progressUpdates)-1]
	
	if firstProgress.Percentage != 0.0 {
		t.Errorf("Expected first progress to be 0.0, got %f", firstProgress.Percentage)
	}
	
	if lastProgress.Percentage != 100.0 {
		t.Errorf("Expected last progress to be 100.0, got %f", lastProgress.Percentage)
	}
	
	// Verify result contains progress information
	if progressUpdates, ok := result.Data["progress_updates"].(int); !ok || progressUpdates == 0 {
		t.Error("Result should contain non-zero progress_updates count")
	}
	
	if finalProgress, ok := result.Data["final_progress"].(types.ProgressInfo); !ok {
		t.Error("Result should contain final_progress")
	} else if finalProgress.Percentage != 100.0 {
		t.Errorf("Final progress should be 100.0, got %f", finalProgress.Percentage)
	}
	
	if transferCompleted, ok := result.Data["transfer_completed"].(bool); !ok || !transferCompleted {
		t.Error("transfer_completed should be true")
	}
}

// Benchmark tests
func BenchmarkEnhancedCopyModule_Run(b *testing.B) {
	module := NewEnhancedCopyModule()
	conn := &mockProgressConnection{}
	
	args := map[string]interface{}{
		"content": "Benchmark test content",
		"dest":    "/tmp/benchmark.txt",
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := module.Run(ctx, conn, args)
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
	}
}

func BenchmarkEnhancedCopyModule_Validate(b *testing.B) {
	module := NewEnhancedCopyModule()
	
	args := map[string]interface{}{
		"content": "Test content",
		"dest":    "/tmp/test.txt",
		"mode":    "0644",
		"backup":  true,
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		err := module.Validate(args)
		if err != nil {
			b.Fatalf("Validate failed: %v", err)
		}
	}
}