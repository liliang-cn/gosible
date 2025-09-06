# Module Test Helper Framework

The Module Test Helper Framework provides comprehensive testing utilities for gosinble modules, making it easy to write isolated, reliable tests without affecting the real system.

## Features

- **MockConnection**: Simulates command execution with configurable responses
- **MockFileSystem**: In-memory filesystem for testing file operations
- **ModuleTestHelper**: High-level testing framework with built-in assertions
- **Specialized Helpers**: Pre-configured helpers for systemd, file, and package modules
- **Test Case Batching**: Run multiple related test cases efficiently
- **Check/Diff Mode Testing**: Full support for testing Ansible-style check and diff modes

## Quick Start

```go
func TestMyModule(t *testing.T) {
    module := NewMyModule()
    helper := testing.NewModuleTestHelper(t, module)
    
    // Setup mock command expectations
    helper.GetConnection().ExpectCommand("echo hello", &testing.CommandResponse{
        Stdout: "hello",
        ExitCode: 0,
    })
    
    // Execute module
    result := helper.Execute(map[string]interface{}{
        "message": "hello",
    }, false, false)
    
    // Verify results
    helper.AssertSuccess(result)
    helper.AssertChanged(result)
    helper.AssertCommandExecuted("echo hello")
}
```

## Test Case Batch Execution

```go
func TestMyModuleScenarios(t *testing.T) {
    module := NewMyModule()
    helper := testing.NewModuleTestHelper(t, module)
    
    testCases := []testing.ModuleTestCase{
        {
            Name: "SuccessfulOperation",
            Args: map[string]interface{}{"param": "value"},
            Setup: func(h *testing.ModuleTestHelper) error {
                h.GetConnection().ExpectCommand("my-command", &testing.CommandResponse{
                    Stdout: "success",
                    ExitCode: 0,
                })
                return nil
            },
            ExpectedResult: &testing.ExpectedResult{
                Success: testing.BoolPtr(true),
                Changed: testing.BoolPtr(true),
                MessageContains: "success",
            },
        },
        // Add more test cases...
    }
    
    helper.RunTestCases(testCases)
}
```

## Systemd Module Testing

```go
func TestSystemdModule(t *testing.T) {
    module := NewSystemdModule()
    helper := testing.NewModuleTestHelper(t, module)
    
    // Mock systemd service configuration
    helper.GetSystemdHelper().MockSystemdService("nginx", testing.SystemdServiceConfig{
        ActiveState:  "inactive",
        EnabledState: "disabled",
        LoadState:    "loaded",
        Description:  "Nginx HTTP Server",
    })
    
    // Mock allowed operations
    helper.GetSystemdHelper().MockSystemdOperations("nginx", testing.SystemdOperations{
        AllowStart: true,
        AllowEnable: true,
    })
    
    // Execute module
    result := helper.Execute(map[string]interface{}{
        "name": "nginx",
        "state": "started",
        "enabled": true,
    }, false, false)
    
    helper.AssertSuccess(result)
    helper.AssertChanged(result)
}
```

## Check Mode Testing

```go
func TestCheckMode(t *testing.T) {
    module := NewMyModule()
    helper := testing.NewModuleTestHelper(t, module)
    
    // Setup mocks...
    
    // Execute in check mode
    result := helper.Execute(args, true, false) // checkMode = true
    
    helper.AssertSuccess(result)
    helper.AssertCheckModeSimulated(result)
    // In check mode, commands should not be executed
    helper.AssertCommandNotExecuted("dangerous-command")
}
```

## Mock Connection Features

### Command Expectations

```go
conn := testing.NewMockConnection("test-host")

// Exact command match
conn.ExpectCommand("systemctl start nginx", &testing.CommandResponse{
    Stdout: "Started nginx",
    ExitCode: 0,
})

// Regex pattern matching
conn.ExpectCommandRegex("systemctl (start|stop) .*", &testing.CommandResponse{
    Stdout: "Service operation completed",
    ExitCode: 0,
})

// Environment variable requirements
conn.ExpectCommandWithEnv("MY_COMMAND", map[string]string{
    "ENV_VAR": "expected_value",
}, &testing.CommandResponse{
    ExitCode: 0,
})

// Multiple calls to same command
conn.ExpectCommandCount("systemctl status nginx", 3, &testing.CommandResponse{
    Stdout: "active",
    ExitCode: 0,
})
```

### Error Simulation

```go
// Simulate connection failures
conn.SimulateFailures()

// Simulate slow responses
conn.SimulateLatency()

// Fail after N commands
conn.FailAfterCommands(5)

// Default error response
conn.SetDefaultCommandResponse(&testing.CommandResponse{
    Stderr: "Command not found",
    ExitCode: 127,
})
```

## Mock Filesystem Features

```go
fs := testing.NewMockFileSystem()

// Create files and directories
fs.AddFile("/etc/config.conf", []byte("config=value"), 0644)
fs.AddDir("/var/log", 0755)

// Simulate errors
fs.SimulatePermissionErrors()  // Files with "restricted" in path will fail
fs.SimulateIOErrors()         // Files with "broken" in path will fail
fs.SetReadOnly("/etc/passwd") // Specific files are read-only

// Verify operations
operations := fs.GetOperations()
counts := fs.GetOperationsCount()
```

## Best Practices

### Test Organization

1. **Group Related Tests**: Use subtests (`t.Run`) to organize related test cases
2. **Reset Between Tests**: Call `helper.Reset()` between independent tests
3. **Use Descriptive Names**: Test names should clearly describe the scenario
4. **Test Edge Cases**: Include error conditions, missing services, permission denied, etc.

### Mock Configuration

1. **Be Specific**: Use exact command matches when possible
2. **Handle Fallbacks**: Mock fallback commands that modules might use
3. **Realistic Responses**: Use realistic command outputs and exit codes
4. **Order Matters**: Commands are expected in the order they're registered

### Assertions

1. **Check Multiple Aspects**: Verify success, changes, messages, and command execution
2. **Use Custom Assertions**: For complex validations, use custom assertion functions
3. **Test Both Modes**: Test both normal execution and check mode for state-changing modules

### Error Testing

1. **Test Error Paths**: Ensure modules handle errors gracefully
2. **Use Scenarios**: Use pre-configured error scenarios for common failures
3. **Verify Error Messages**: Check that error messages are helpful and accurate

## Architecture

The testing framework follows a layered architecture:

```
┌─────────────────────┐
│  ModuleTestHelper   │  ← High-level test orchestration
├─────────────────────┤
│ Specialized Helpers │  ← Domain-specific test utilities
├─────────────────────┤
│   MockConnection    │  ← Command execution simulation
│   MockFileSystem    │  ← File system simulation
└─────────────────────┘
```

Each layer provides progressively higher-level abstractions, allowing you to choose the appropriate level for your testing needs.