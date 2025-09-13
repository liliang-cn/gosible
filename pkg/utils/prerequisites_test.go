package utils

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewCommandChecker(t *testing.T) {
	checker := NewCommandChecker()
	
	if checker == nil {
		t.Fatal("NewCommandChecker returned nil")
	}
	
	if checker.cache == nil {
		t.Error("cache map not initialized")
	}
	
	if checker.installCommands == nil {
		t.Error("installCommands map not initialized")
	}
}

func TestCommandAvailable(t *testing.T) {
	checker := NewCommandChecker()
	
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			name:     "go command should exist",
			command:  "go",
			expected: true,
		},
		{
			name:     "non-existent command",
			command:  "definitely-not-a-real-command-xyz123",
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.CommandAvailable(tt.command)
			if result != tt.expected {
				t.Errorf("CommandAvailable(%s) = %v, want %v", tt.command, result, tt.expected)
			}
			
			// Check that result is cached
			if cached, ok := checker.cache[tt.command]; !ok || cached != result {
				t.Errorf("Command %s not properly cached", tt.command)
			}
		})
	}
}

func TestCommandAvailableWithContext(t *testing.T) {
	checker := NewCommandChecker()
	
	t.Run("with valid context", func(t *testing.T) {
		ctx := context.Background()
		available, err := checker.CommandAvailableWithContext(ctx, "go")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !available {
			t.Error("expected go command to be available")
		}
	})
	
	t.Run("with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		
		_, err := checker.CommandAvailableWithContext(ctx, "go")
		if err == nil {
			t.Error("expected context cancelled error")
		}
	})
}

func TestCheckRequired(t *testing.T) {
	checker := NewCommandChecker()
	
	tests := []struct {
		name         string
		commands     []string
		expectMissing bool
	}{
		{
			name:         "all available commands",
			commands:     []string{"go"},
			expectMissing: false,
		},
		{
			name:         "some missing commands",
			commands:     []string{"go", "fake-command-xyz"},
			expectMissing: true,
		},
		{
			name:         "empty list",
			commands:     []string{},
			expectMissing: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing, err := checker.CheckRequired(tt.commands)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			
			hasMissing := len(missing) > 0
			if hasMissing != tt.expectMissing {
				t.Errorf("CheckRequired() missing = %v, expectMissing = %v", missing, tt.expectMissing)
			}
		})
	}
}

func TestCheckRequiredWithInstallInfo(t *testing.T) {
	checker := NewCommandChecker()
	
	missing, err := checker.CheckRequiredWithInstallInfo([]string{"fake-command-xyz", "curl"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	// Should have install info for fake command
	if _, ok := missing["fake-command-xyz"]; !ok {
		t.Error("expected missing command fake-command-xyz")
	}
	
	// curl might or might not be installed, but if missing should have install info
	if info, ok := missing["curl"]; ok {
		if info == "" {
			t.Error("expected non-empty install info for curl")
		}
	}
}

func TestEnsureCommand(t *testing.T) {
	checker := NewCommandChecker()
	
	tests := []struct {
		name        string
		command     string
		shouldError bool
	}{
		{
			name:        "available command",
			command:     "go",
			shouldError: false,
		},
		{
			name:        "unavailable command with known install",
			command:     "fake-curl-xyz",
			shouldError: true,
		},
		{
			name:        "unavailable command with unknown install",
			command:     "totally-unknown-command",
			shouldError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checker.EnsureCommand(tt.command)
			if (err != nil) != tt.shouldError {
				t.Errorf("EnsureCommand(%s) error = %v, shouldError = %v", tt.command, err, tt.shouldError)
			}
		})
	}
}

func TestGetVersion(t *testing.T) {
	checker := NewCommandChecker()
	
	t.Run("go version", func(t *testing.T) {
		version, err := checker.GetVersion("go")
		if err != nil {
			t.Errorf("unexpected error getting go version: %v", err)
		}
		if !strings.Contains(version, "go") {
			t.Errorf("go version should contain 'go', got: %s", version)
		}
	})
	
	t.Run("non-existent command", func(t *testing.T) {
		_, err := checker.GetVersion("fake-command-xyz")
		if err == nil {
			t.Error("expected error for non-existent command")
		}
	})
}

func TestInstallCommand(t *testing.T) {
	checker := NewCommandChecker()
	
	t.Run("already installed command", func(t *testing.T) {
		ctx := context.Background()
		err := checker.InstallCommand(ctx, "go", false)
		if err != nil {
			t.Errorf("unexpected error for already installed command: %v", err)
		}
	})
	
	t.Run("unknown command", func(t *testing.T) {
		ctx := context.Background()
		err := checker.InstallCommand(ctx, "totally-unknown-xyz", false)
		if err == nil {
			t.Error("expected error for unknown command")
		}
	})
	
	t.Run("with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(10 * time.Millisecond)
		
		err := checker.InstallCommand(ctx, "curl", false)
		// Should either succeed (if curl is installed) or fail (context or install error)
		// Just checking it doesn't panic
		_ = err
	})
}

func TestGetCommonDependencies(t *testing.T) {
	tests := []struct {
		name     string
		useCase  string
		hasRequired bool
	}{
		{
			name:     "web dependencies",
			useCase:  "web",
			hasRequired: true,
		},
		{
			name:     "archive dependencies",
			useCase:  "archive",
			hasRequired: true,
		},
		{
			name:     "build dependencies",
			useCase:  "build",
			hasRequired: true,
		},
		{
			name:     "ansible-like dependencies",
			useCase:  "ansible-like",
			hasRequired: true,
		},
		{
			name:     "default dependencies",
			useCase:  "unknown",
			hasRequired: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := GetCommonDependencies(tt.useCase)
			if deps == nil {
				t.Fatal("GetCommonDependencies returned nil")
			}
			
			hasRequired := len(deps.Required) > 0
			if hasRequired != tt.hasRequired {
				t.Errorf("Expected hasRequired=%v, got %v", tt.hasRequired, hasRequired)
			}
		})
	}
}

func TestCheckDependencies(t *testing.T) {
	checker := NewCommandChecker()
	
	deps := &CommonDependencies{
		Required: []string{"go"},
		Optional: []string{"curl", "wget"},
	}
	
	report, err := checker.CheckDependencies(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if report == nil {
		t.Fatal("CheckDependencies returned nil report")
	}
	
	// Go should be available
	if !report.Required["go"] {
		t.Error("expected go to be available")
	}
	
	// Should check optional dependencies
	if report.Optional == nil {
		t.Error("Optional map not initialized")
	}
	
	// Check AllRequiredPresent flag
	if !report.AllRequiredPresent {
		t.Error("expected all required dependencies to be present (go)")
	}
}

func TestDependencyReportString(t *testing.T) {
	report := &DependencyReport{
		Required: map[string]bool{
			"go":   true,
			"make": false,
		},
		Optional: map[string]bool{
			"curl": true,
			"wget": false,
		},
		Missing: map[string]string{
			"make": "apt-get install make",
		},
		AllRequiredPresent: false,
	}
	
	str := report.String()
	if str == "" {
		t.Error("report.String() returned empty string")
	}
	
	// Check that report contains expected elements
	expectedStrings := []string{
		"Dependency Check Report",
		"Required Commands",
		"Optional Commands",
		"Missing Commands",
		"make",
	}
	
	for _, expected := range expectedStrings {
		if !strings.Contains(str, expected) {
			t.Errorf("report.String() missing expected string: %s", expected)
		}
	}
}

func TestTryInstallWithCommand(t *testing.T) {
	checker := NewCommandChecker()
	
	t.Run("empty command", func(t *testing.T) {
		ctx := context.Background()
		err := checker.tryInstallWithCommand(ctx, "", false)
		if err == nil {
			t.Error("expected error for empty command")
		}
	})
	
	t.Run("non-existent package manager", func(t *testing.T) {
		ctx := context.Background()
		err := checker.tryInstallWithCommand(ctx, "fake-pkg-manager install something", false)
		if err == nil {
			t.Error("expected error for non-existent package manager")
		}
	})
}

func TestInstallCommandsMap(t *testing.T) {
	checker := NewCommandChecker()
	
	// Verify common commands have install instructions
	commonCommands := []string{"curl", "wget", "tar", "zip", "unzip", "git", "make", "rsync"}
	
	for _, cmd := range commonCommands {
		t.Run(cmd, func(t *testing.T) {
			if installCmds, ok := checker.installCommands[cmd]; !ok {
				t.Errorf("no install commands for %s", cmd)
			} else {
				// Check that at least one OS has install instructions
				hasInstruction := false
				for _, osName := range []string{"darwin", "linux", "windows"} {
					if _, ok := installCmds[osName]; ok {
						hasInstruction = true
						break
					}
				}
				if !hasInstruction {
					t.Errorf("no OS-specific install instructions for %s", cmd)
				}
			}
		})
	}
}

func TestCacheConsistency(t *testing.T) {
	checker := NewCommandChecker()
	
	// Check a command twice to ensure caching works
	command := "go"
	result1 := checker.CommandAvailable(command)
	result2 := checker.CommandAvailable(command)
	
	if result1 != result2 {
		t.Error("cache inconsistency: same command returned different results")
	}
	
	// Verify it's actually cached
	if _, ok := checker.cache[command]; !ok {
		t.Error("command not found in cache after check")
	}
}

func TestOSSpecificInstallCommands(t *testing.T) {
	checker := NewCommandChecker()
	osName := runtime.GOOS
	
	// Test that curl has OS-specific install command
	if installCmds, ok := checker.installCommands["curl"]; ok {
		if installCmd, ok := installCmds[osName]; ok {
			if installCmd == "" {
				t.Errorf("empty install command for curl on %s", osName)
			}
		}
	}
}

func BenchmarkCommandAvailable(b *testing.B) {
	checker := NewCommandChecker()
	
	b.Run("uncached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			checker.cache = make(map[string]bool) // Clear cache
			checker.CommandAvailable("go")
		}
	})
	
	b.Run("cached", func(b *testing.B) {
		checker.CommandAvailable("go") // Prime cache
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			checker.CommandAvailable("go")
		}
	})
}

func BenchmarkCheckDependencies(b *testing.B) {
	checker := NewCommandChecker()
	deps := &CommonDependencies{
		Required: []string{"go", "git", "make"},
		Optional: []string{"curl", "wget", "tar"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.CheckDependencies(deps)
	}
}