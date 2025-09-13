package types

import (
	"testing"
)

func TestModuleType_String(t *testing.T) {
	tests := []struct {
		name       string
		moduleType ModuleType
		expected   string
	}{
		{"file module", TypeFile, "file"},
		{"service module", TypeService, "service"},
		{"package module", TypePackage, "package"},
		{"user module", TypeUser, "user"},
		{"group module", TypeGroup, "group"},
		{"copy module", TypeCopy, "copy"},
		{"template module", TypeTemplate, "template"},
		{"command module", TypeCommand, "command"},
		{"shell module", TypeShell, "shell"},
		{"ping module", TypePing, "ping"},
		{"setup module", TypeSetup, "setup"},
		{"debug module", TypeDebug, "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.moduleType.String(); got != tt.expected {
				t.Errorf("ModuleType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestModuleType_IsValid(t *testing.T) {
	tests := []struct {
		name       string
		moduleType ModuleType
		expected   bool
	}{
		// Valid module types
		{"file module", TypeFile, true},
		{"service module", TypeService, true},
		{"package module", TypePackage, true},
		{"user module", TypeUser, true},
		{"group module", TypeGroup, true},
		{"copy module", TypeCopy, true},
		{"template module", TypeTemplate, true},
		{"command module", TypeCommand, true},
		{"shell module", TypeShell, true},
		{"ping module", TypePing, true},
		{"setup module", TypeSetup, true},
		{"debug module", TypeDebug, true},

		// Invalid module types
		{"invalid module", ModuleType("invalid"), false},
		{"empty module", ModuleType(""), false},
		{"nonexistent module", ModuleType("nonexistent"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.moduleType.IsValid(); got != tt.expected {
				t.Errorf("ModuleType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllModuleTypes(t *testing.T) {
	moduleTypes := AllModuleTypes()

	// Check that we have all expected module types
	expectedCount := 17 // Updated to include systemd, homebrew, apt, yum, dnf modules
	if len(moduleTypes) != expectedCount {
		t.Errorf("AllModuleTypes() returned %d types, want %d", len(moduleTypes), expectedCount)
	}

	// Check that all returned types are valid
	for _, moduleType := range moduleTypes {
		if !moduleType.IsValid() {
			t.Errorf("AllModuleTypes() returned invalid module type: %s", moduleType)
		}
	}

	// Check for specific expected types
	expectedTypes := map[ModuleType]bool{
		TypeFile:     false,
		TypeService:  false,
		TypePackage:  false,
		TypeUser:     false,
		TypeGroup:    false,
		TypeCopy:     false,
		TypeTemplate: false,
		TypeCommand:  false,
		TypeShell:    false,
		TypePing:     false,
		TypeSetup:    false,
		TypeDebug:    false,
	}

	for _, moduleType := range moduleTypes {
		if _, exists := expectedTypes[moduleType]; exists {
			expectedTypes[moduleType] = true
		}
	}

	for moduleType, found := range expectedTypes {
		if !found {
			t.Errorf("AllModuleTypes() missing expected module type: %s", moduleType)
		}
	}
}

func TestTask_WithModuleType(t *testing.T) {
	// Test creating a task with ModuleType
	task := Task{
		Name:   "Test task",
		Module: TypeFile,
		Args: map[string]interface{}{
			"path":  "/tmp/test",
			"state": StateFile,
		},
	}

	if task.Module != TypeFile {
		t.Errorf("Task.Module = %v, want %v", task.Module, TypeFile)
	}

	if task.Module.String() != "file" {
		t.Errorf("Task.Module.String() = %v, want 'file'", task.Module.String())
	}

	if !task.Module.IsValid() {
		t.Error("Task.Module should be valid")
	}
}

func TestStateConstants(t *testing.T) {
	// Test that state constants are properly defined
	expectedStates := map[string]string{
		StatePresent:    "present",
		StateAbsent:     "absent",
		StateStarted:    "started",
		StateStopped:    "stopped",
		StateRestarted:  "restarted",
		StateReloaded:   "reloaded",
		StateLatest:     "latest",
		StateFile:       "file",
		StateDirectory:  "directory",
		StateLink:       "link",
		StateTouch:      "touch",
	}

	for constant, expected := range expectedStates {
		if constant != expected {
			t.Errorf("State constant %s = %v, want %v", expected, constant, expected)
		}
	}
}

func TestTask_TypeSafety(t *testing.T) {
	// Test that we can use constants in task creation for type safety
	tasks := []Task{
		{
			Name:   "Create directory",
			Module: TypeFile,
			Args: map[string]interface{}{
				"path":  "/tmp/testdir",
				"state": StateDirectory,
			},
		},
		{
			Name:   "Start service",
			Module: TypeService,
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": StateStarted,
			},
		},
		{
			Name:   "Install package",
			Module: TypePackage,
			Args: map[string]interface{}{
				"name":  "nginx",
				"state": StatePresent,
			},
		},
	}

	for i, task := range tasks {
		if !task.Module.IsValid() {
			t.Errorf("Task %d has invalid module type: %s", i, task.Module)
		}
	}
}