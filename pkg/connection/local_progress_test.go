package connection

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liliang-cn/gosinble/pkg/types"
)

func TestLocalConnection_CopyWithProgress(t *testing.T) {
	// Create temporary files
	tmpDir := t.TempDir()
	destPath := tmpDir + "/test_copy.txt"
	
	// Test data
	testContent := "This is test content for copy with progress tracking. " +
		"It should be long enough to see multiple progress updates during the copy operation. " +
		"We want to test that the progress callback is called with appropriate percentages."
	
	conn := NewLocalConnection()
	err := conn.Connect(context.Background(), types.ConnectionInfo{})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	
	// Track progress updates
	var progressUpdates []types.ProgressInfo
	progressCallback := func(progress types.ProgressInfo) {
		progressUpdates = append(progressUpdates, progress)
	}
	
	// Test copy with progress
	reader := strings.NewReader(testContent)
	ctx := context.Background()
	totalSize := int64(len(testContent))
	
	err = conn.CopyWithProgress(ctx, reader, destPath, 0644, totalSize, progressCallback)
	if err != nil {
		t.Fatalf("CopyWithProgress failed: %v", err)
	}
	
	// Verify file was created
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatal("Destination file was not created")
	}
	
	// Verify content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	
	if string(content) != testContent {
		t.Fatalf("Content mismatch. Expected: %s, Got: %s", testContent, string(content))
	}
	
	// Verify progress updates
	if len(progressUpdates) == 0 {
		t.Fatal("No progress updates received")
	}
	
	// Check first progress update
	firstProgress := progressUpdates[0]
	if firstProgress.Stage != "transferring" {
		t.Errorf("Expected stage 'transferring', got '%s'", firstProgress.Stage)
	}
	
	if firstProgress.Percentage < 0 || firstProgress.Percentage > 100 {
		t.Errorf("Invalid percentage: %f", firstProgress.Percentage)
	}
	
	// Check last progress update should be 100%
	lastProgress := progressUpdates[len(progressUpdates)-1]
	if lastProgress.Percentage != 100.0 {
		t.Errorf("Expected final percentage 100.0, got %f", lastProgress.Percentage)
	}
	
	// Verify progress messages contain file size info
	found := false
	for _, progress := range progressUpdates {
		if strings.Contains(progress.Message, "B") { // Should contain byte information
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Progress messages should contain byte information")
	}
}

func TestLocalConnection_CopyWithProgress_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := tmpDir + "/large_test.txt"
	
	// Create larger content to test multiple progress updates
	var largeContent strings.Builder
	for i := 0; i < 1000; i++ {
		largeContent.WriteString("This is line ")
		largeContent.WriteString(string(rune('0' + (i % 10))))
		largeContent.WriteString(" of the large test file for progress tracking.\n")
	}
	
	content := largeContent.String()
	conn := NewLocalConnection()
	err := conn.Connect(context.Background(), types.ConnectionInfo{})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	
	var progressUpdates []types.ProgressInfo
	progressCallback := func(progress types.ProgressInfo) {
		progressUpdates = append(progressUpdates, progress)
	}
	
	reader := strings.NewReader(content)
	ctx := context.Background()
	totalSize := int64(len(content))
	
	err = conn.CopyWithProgress(ctx, reader, destPath, 0644, totalSize, progressCallback)
	if err != nil {
		t.Fatalf("CopyWithProgress failed: %v", err)
	}
	
	// Should have multiple progress updates for larger file
	if len(progressUpdates) < 2 {
		t.Fatalf("Expected multiple progress updates, got %d", len(progressUpdates))
	}
	
	// Verify progress is monotonically increasing
	for i := 1; i < len(progressUpdates); i++ {
		if progressUpdates[i].Percentage < progressUpdates[i-1].Percentage {
			t.Errorf("Progress should be monotonically increasing. Got %f after %f", 
				progressUpdates[i].Percentage, progressUpdates[i-1].Percentage)
		}
	}
	
	// Verify timestamps are increasing
	for i := 1; i < len(progressUpdates); i++ {
		if progressUpdates[i].Timestamp.Before(progressUpdates[i-1].Timestamp) {
			t.Error("Progress timestamps should be increasing")
		}
	}
}

func TestLocalConnection_CopyWithProgress_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := tmpDir + "/cancelled_test.txt"
	
	// Create content for copy
	testContent := "This is test content that will be cancelled"
	
	conn := NewLocalConnection()
	err := conn.Connect(context.Background(), types.ConnectionInfo{})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	
	// Create cancellable context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	
	// Give it a moment to start then cancel
	go func() {
		time.Sleep(2 * time.Millisecond)
		cancel()
	}()
	
	progressCallback := func(progress types.ProgressInfo) {
		// This might not be called due to quick cancellation
	}
	
	reader := strings.NewReader(testContent)
	totalSize := int64(len(testContent))
	
	err = conn.CopyWithProgress(ctx, reader, destPath, 0644, totalSize, progressCallback)
	
	// Should get context cancellation error or succeed quickly
	if err != nil && !strings.Contains(err.Error(), "context") {
		t.Fatalf("Expected context cancellation error or success, got: %v", err)
	}
}

func TestLocalConnection_CopyWithProgress_InvalidDestination(t *testing.T) {
	// Try to copy to an invalid destination
	invalidPath := "/nonexistent/invalid/path/test.txt"
	testContent := "This should fail"
	
	conn := NewLocalConnection()
	// No need to connect for invalid destination test
	
	progressCallback := func(progress types.ProgressInfo) {
		t.Error("Progress callback should not be called for invalid destination")
	}
	
	reader := strings.NewReader(testContent)
	ctx := context.Background()
	totalSize := int64(len(testContent))
	
	err := conn.CopyWithProgress(ctx, reader, invalidPath, 0644, totalSize, progressCallback)
	if err == nil {
		t.Fatal("Expected error for invalid destination path")
	}
}

func TestProgressReader(t *testing.T) {
	testContent := "This is test content for progress reader testing"
	reader := strings.NewReader(testContent)
	totalSize := int64(len(testContent))
	
	var progressUpdates []types.ProgressInfo
	progressCallback := func(progress types.ProgressInfo) {
		progressUpdates = append(progressUpdates, progress)
	}
	
	progressReader := &progressReader{
		reader:           reader,
		totalSize:        totalSize,
		progressCallback: progressCallback,
		dest:             "test",
	}
	
	// Read data in chunks to trigger progress updates
	buffer := make([]byte, 10)
	var totalRead int64
	
	for {
		n, err := progressReader.Read(buffer)
		totalRead += int64(n)
		
		// Add delay to allow time-based progress updates (>100ms threshold)
		time.Sleep(110 * time.Millisecond)
		
		if err != nil {
			break
		}
	}
	
	// Verify all data was read
	if totalRead != totalSize {
		t.Errorf("Expected to read %d bytes, got %d", totalSize, totalRead)
	}
	
	// Verify progress updates were triggered
	if len(progressUpdates) == 0 {
		t.Fatal("No progress updates received from progressReader")
	}
	
	// Verify final progress is 100% or close (due to chunked reading)
	lastProgress := progressUpdates[len(progressUpdates)-1]
	if lastProgress.Percentage < 95.0 { // Allow some leeway for chunked reading
		t.Errorf("Expected final progress near 100.0, got %f", lastProgress.Percentage)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}
	
	for _, test := range tests {
		result := formatBytes(test.bytes)
		if result != test.expected {
			t.Errorf("formatBytes(%d) = %s, expected %s", test.bytes, result, test.expected)
		}
	}
}

// Benchmark tests for progress tracking performance
func BenchmarkCopyWithProgress(b *testing.B) {
	tmpDir := b.TempDir()
	conn := NewLocalConnection()
	err := conn.Connect(context.Background(), types.ConnectionInfo{})
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	
	// Create test content
	testContent := strings.Repeat("benchmark test content ", 1000)
	totalSize := int64(len(testContent))
	
	progressCallback := func(progress types.ProgressInfo) {
		// Minimal callback to test overhead
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		destPath := tmpDir + "/" + string(rune(i)) + "_benchmark.txt"
		reader := strings.NewReader(testContent)
		ctx := context.Background()
		
		err := conn.CopyWithProgress(ctx, reader, destPath, 0644, totalSize, progressCallback)
		if err != nil {
			b.Fatalf("CopyWithProgress failed: %v", err)
		}
	}
}

func BenchmarkProgressReader(b *testing.B) {
	testContent := strings.Repeat("benchmark test content ", 100)
	totalSize := int64(len(testContent))
	
	progressCallback := func(progress types.ProgressInfo) {
		// Minimal callback
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(testContent)
		progressReader := &progressReader{
			reader:           reader,
			totalSize:        totalSize,
			progressCallback: progressCallback,
			dest:             "benchmark",
		}
		
		buffer := make([]byte, 1024)
		for {
			_, err := progressReader.Read(buffer)
			if err != nil {
				break
			}
		}
	}
}