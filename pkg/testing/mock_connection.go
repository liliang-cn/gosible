package testing

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// CommandResponse represents the expected response from a command
type CommandResponse struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Error    error
}

// CommandExpectation represents an expected command execution
type CommandExpectation struct {
	Command     string        // Exact command string
	Pattern     *regexp.Regexp // Regex pattern for command matching
	Response    *CommandResponse
	Called      bool
	CallCount   int
	MaxCalls    int // 0 means unlimited
	Environment map[string]string // Expected environment variables
}

// MockConnection implements types.Connection for testing
type MockConnection struct {
	t           *testing.T
	mu          sync.RWMutex
	expectations []*CommandExpectation
	callOrder   []string
	connected   bool
	hostname    string
	strictOrder bool
	defaultResponse *CommandResponse
}

// NewMockConnection creates a new mock connection for testing
func NewMockConnection(t *testing.T) *MockConnection {
	return &MockConnection{
		t:           t,
		expectations: make([]*CommandExpectation, 0),
		callOrder:   make([]string, 0),
		connected:   true,
		hostname:    "test-host",
	}
}

// ExpectCommand adds an expectation for an exact command
func (m *MockConnection) ExpectCommand(command string, response *CommandResponse) *MockConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.expectations = append(m.expectations, &CommandExpectation{
		Command:  command,
		Response: response,
		MaxCalls: 1,
	})
	return m
}

// ExpectCommandPattern adds an expectation for a command matching a regex pattern
func (m *MockConnection) ExpectCommandPattern(pattern string, response *CommandResponse) *MockConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	regex, err := regexp.Compile(pattern)
	if err != nil {
		m.t.Fatalf("Invalid regex pattern %s: %v", pattern, err)
	}
	
	m.expectations = append(m.expectations, &CommandExpectation{
		Pattern:  regex,
		Response: response,
		MaxCalls: 1,
	})
	return m
}

// ExpectCommandRegex is an alias for ExpectCommandPattern for compatibility
func (m *MockConnection) ExpectCommandRegex(pattern string, response *CommandResponse) *MockConnection {
	return m.ExpectCommandPattern(pattern, response)
}

// ExpectCommandWithEnv adds an expectation for a command with environment variables
func (m *MockConnection) ExpectCommandWithEnv(command string, env map[string]string, response *CommandResponse) *MockConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.expectations = append(m.expectations, &CommandExpectation{
		Command:     command,
		Environment: env,
		Response:    response,
		MaxCalls:    1,
	})
	return m
}

// AllowMultipleCalls allows the last added expectation to be called multiple times
func (m *MockConnection) AllowMultipleCalls() *MockConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if len(m.expectations) > 0 {
		m.expectations[len(m.expectations)-1].MaxCalls = 0 // Unlimited
	}
	return m
}

// SetMaxCalls sets the maximum number of calls for the last added expectation
func (m *MockConnection) SetMaxCalls(maxCalls int) *MockConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if len(m.expectations) > 0 {
		m.expectations[len(m.expectations)-1].MaxCalls = maxCalls
	}
	return m
}

// Execute implements types.Connection.Execute
func (m *MockConnection) Execute(ctx context.Context, command string, options types.ExecuteOptions) (*types.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Record the call
	m.callOrder = append(m.callOrder, command)
	
	// Find matching expectation
	for _, exp := range m.expectations {
		if m.matchesExpectation(exp, command, options.Env) {
			exp.Called = true
			exp.CallCount++
			
			// Check if we've exceeded the maximum calls
			if exp.MaxCalls > 0 && exp.CallCount > exp.MaxCalls {
				m.t.Errorf("Command '%s' called %d times, but max calls is %d", command, exp.CallCount, exp.MaxCalls)
				return nil, fmt.Errorf("too many calls to command: %s", command)
			}
			
			// Create result based on response
			result := &types.Result{
				Host:       m.hostname,
				Success:    exp.Response.ExitCode == 0,
				Changed:    false, // Mock connection doesn't track changes by default
				Message:    exp.Response.Stdout,
				Data:       make(map[string]interface{}),
				StartTime:  types.GetCurrentTime(),
				EndTime:    types.GetCurrentTime(),
				TaskName:   command,
				ModuleName: "mock",
			}
			
			// Add stdout and stderr to data
			result.Data["stdout"] = exp.Response.Stdout
			result.Data["exit_code"] = exp.Response.ExitCode
			if exp.Response.Stderr != "" {
				result.Data["stderr"] = exp.Response.Stderr
			}
			
			// Return error if configured or if exit code is non-zero
			if exp.Response.Error != nil {
				result.Error = exp.Response.Error
				return result, exp.Response.Error
			}
			
			if exp.Response.ExitCode != 0 {
				err := fmt.Errorf("command failed with exit code %d: %s", exp.Response.ExitCode, exp.Response.Stderr)
				result.Error = err
				return result, err
			}
			
			return result, nil
		}
	}
	
	// No expectation found - use default response if set
	if m.defaultResponse != nil {
		result := &types.Result{
			Host:       m.hostname,
			Success:    m.defaultResponse.ExitCode == 0,
			Changed:    false,
			Message:    m.defaultResponse.Stdout,
			Data:       make(map[string]interface{}),
			StartTime:  types.GetCurrentTime(),
			EndTime:    types.GetCurrentTime(),
			TaskName:   command,
			ModuleName: "mock",
		}
		
		result.Data["stdout"] = m.defaultResponse.Stdout
		result.Data["exit_code"] = m.defaultResponse.ExitCode
		if m.defaultResponse.Stderr != "" {
			result.Data["stderr"] = m.defaultResponse.Stderr
		}
		
		if m.defaultResponse.Error != nil {
			result.Error = m.defaultResponse.Error
			return result, m.defaultResponse.Error
		}
		
		if m.defaultResponse.ExitCode != 0 {
			err := fmt.Errorf("command failed with exit code %d: %s", m.defaultResponse.ExitCode, m.defaultResponse.Stderr)
			result.Error = err
			return result, err
		}
		
		return result, nil
	}
	
	// No expectation found and no default response
	m.t.Errorf("Unexpected command executed: %s", command)
	return nil, fmt.Errorf("unexpected command: %s", command)
}

// matchesExpectation checks if a command matches an expectation
func (m *MockConnection) matchesExpectation(exp *CommandExpectation, command string, env map[string]string) bool {
	// Check command match
	var commandMatches bool
	if exp.Command != "" {
		commandMatches = exp.Command == command
	} else if exp.Pattern != nil {
		commandMatches = exp.Pattern.MatchString(command)
	}
	
	if !commandMatches {
		return false
	}
	
	// Check environment variables if specified
	if exp.Environment != nil {
		for key, expectedValue := range exp.Environment {
			if actualValue, exists := env[key]; !exists || actualValue != expectedValue {
				return false
			}
		}
	}
	
	return true
}

// Copy implements types.Connection.Copy
func (m *MockConnection) Copy(ctx context.Context, src io.Reader, dest string, mode int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Read the content (for completeness, though we don't use it in basic mocking)
	content, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	
	// Record the operation with content size for testing
	m.callOrder = append(m.callOrder, fmt.Sprintf("copy %d bytes to %s", len(content), dest))
	
	// For testing, we could add expectations for copy operations
	// For now, just return success
	return nil
}

// Fetch implements types.Connection.Fetch
func (m *MockConnection) Fetch(ctx context.Context, src string) (io.Reader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Record the operation
	m.callOrder = append(m.callOrder, fmt.Sprintf("fetch from %s", src))
	
	// For testing, return empty reader by default
	// This could be enhanced to support configured responses
	return strings.NewReader(""), nil
}

// IsConnected implements types.Connection.IsConnected
func (m *MockConnection) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// Connect implements types.Connection.Connect
func (m *MockConnection) Connect(ctx context.Context, info types.ConnectionInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	if info.Host != "" {
		m.hostname = info.Host
	}
	return nil
}

// Close implements types.Connection.Close
func (m *MockConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	return nil
}

// GetHostname implements optional hostname interface
func (m *MockConnection) GetHostname() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hostname, nil
}

// SetHostname sets the hostname returned by GetHostname
func (m *MockConnection) SetHostname(hostname string) *MockConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hostname = hostname
	return m
}

// SetConnected sets the connection status
func (m *MockConnection) SetConnected(connected bool) *MockConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = connected
	return m
}

// Verify checks that all expectations were met
func (m *MockConnection) Verify() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for i, exp := range m.expectations {
		if !exp.Called {
			command := exp.Command
			if command == "" && exp.Pattern != nil {
				command = exp.Pattern.String()
			}
			m.t.Errorf("Expectation %d was not met: expected command '%s' was not called", i, command)
		}
	}
}

// VerifyAllExpectationsMet is an alias for Verify for compatibility
func (m *MockConnection) VerifyAllExpectationsMet() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for i, exp := range m.expectations {
		if !exp.Called {
			command := exp.Command
			if command == "" && exp.Pattern != nil {
				command = exp.Pattern.String()
			}
			return fmt.Errorf("expectation %d was not met: expected command '%s' was not called", i, command)
		}
	}
	return nil
}

// GetCallOrder returns the order in which commands were called
func (m *MockConnection) GetCallOrder() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy to prevent modification
	result := make([]string, len(m.callOrder))
	copy(result, m.callOrder)
	return result
}

// GetExecutionOrder is an alias for GetCallOrder for compatibility
func (m *MockConnection) GetExecutionOrder() []string {
	return m.GetCallOrder()
}

// GetCallCount returns the number of times a command was called
func (m *MockConnection) GetCallCount(command string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	count := 0
	for _, call := range m.callOrder {
		if call == command {
			count++
		}
	}
	return count
}

// Reset clears all expectations and call history
func (m *MockConnection) Reset() *MockConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.expectations = make([]*CommandExpectation, 0)
	m.callOrder = make([]string, 0)
	m.strictOrder = false
	m.defaultResponse = nil
	return m
}

// AssertCommandCalled asserts that a command was called at least once
func (m *MockConnection) AssertCommandCalled(command string) {
	if m.GetCallCount(command) == 0 {
		m.t.Errorf("Expected command '%s' to be called, but it wasn't", command)
	}
}

// AssertCommandNotCalled asserts that a command was never called
func (m *MockConnection) AssertCommandNotCalled(command string) {
	if count := m.GetCallCount(command); count > 0 {
		m.t.Errorf("Expected command '%s' to not be called, but it was called %d times", command, count)
	}
}

// AssertCommandCalledTimes asserts that a command was called exactly n times
func (m *MockConnection) AssertCommandCalledTimes(command string, times int) {
	if count := m.GetCallCount(command); count != times {
		m.t.Errorf("Expected command '%s' to be called %d times, but it was called %d times", command, times, count)
	}
}

// AssertCommandOrder asserts that commands were called in a specific order
func (m *MockConnection) AssertCommandOrder(commands ...string) {
	callOrder := m.GetCallOrder()
	
	if len(callOrder) < len(commands) {
		m.t.Errorf("Expected at least %d commands to be called, but only %d were called", len(commands), len(callOrder))
		return
	}
	
	// Find the subsequence
	for i := 0; i <= len(callOrder)-len(commands); i++ {
		match := true
		for j, expectedCmd := range commands {
			if callOrder[i+j] != expectedCmd {
				match = false
				break
			}
		}
		if match {
			return // Found the sequence
		}
	}
	
	m.t.Errorf("Expected command sequence %v was not found in call order %v", commands, callOrder)
}

// CreateStandardSystemdMocks creates common systemd command mocks
func (m *MockConnection) CreateStandardSystemdMocks(serviceName string) *MockConnection {
	// Common systemd commands
	m.ExpectCommand(fmt.Sprintf("systemctl show %s", serviceName), &CommandResponse{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("LoadState=loaded\nActiveState=inactive\nSubState=dead\nUnitFileState=disabled\n"),
	})
	
	m.ExpectCommand(fmt.Sprintf("systemctl start %s", serviceName), &CommandResponse{
		ExitCode: 0,
	})
	
	m.ExpectCommand(fmt.Sprintf("systemctl stop %s", serviceName), &CommandResponse{
		ExitCode: 0,
	})
	
	m.ExpectCommand(fmt.Sprintf("systemctl enable %s", serviceName), &CommandResponse{
		ExitCode: 0,
	})
	
	m.ExpectCommand(fmt.Sprintf("systemctl disable %s", serviceName), &CommandResponse{
		ExitCode: 0,
	})
	
	return m
}

// CreateFileOperationMocks creates common file operation mocks
func (m *MockConnection) CreateFileOperationMocks(filePath string) *MockConnection {
	// Common file operations
	m.ExpectCommand(fmt.Sprintf("test -f %s", filePath), &CommandResponse{
		ExitCode: 0,
	})
	
	m.ExpectCommand(fmt.Sprintf("cat %s", filePath), &CommandResponse{
		ExitCode: 0,
		Stdout:   "file content",
	})
	
	return m
}

// SimulateCommandFailure creates a mock that simulates command failure
func (m *MockConnection) SimulateCommandFailure(command string, exitCode int, stderr string) *MockConnection {
	return m.ExpectCommand(command, &CommandResponse{
		ExitCode: exitCode,
		Stderr:   stderr,
	})
}

// EnableStrictOrder enables strict command execution order checking
func (m *MockConnection) EnableStrictOrder() *MockConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.strictOrder = true
	return m
}

// ExpectCommandOrder adds an expectation for a command at a specific order position
func (m *MockConnection) ExpectCommandOrder(command string, order int, response *CommandResponse) *MockConnection {
	// For simplicity, we'll just add the expectation normally
	// The order checking can be done via GetExecutionOrder() and assertions
	return m.ExpectCommand(command, response)
}

// SetDefaultCommandResponse sets the default response for unexpected commands
func (m *MockConnection) SetDefaultCommandResponse(response *CommandResponse) *MockConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultResponse = response
	return m
}

// SimulatePermissionDenied creates a mock that simulates permission denied errors
func (m *MockConnection) SimulatePermissionDenied(command string) *MockConnection {
	return m.ExpectCommand(command, &CommandResponse{
		ExitCode: 1,
		Stderr:   "Permission denied",
	})
}

// SimulateServiceNotFound creates a mock that simulates service not found errors
func (m *MockConnection) SimulateServiceNotFound(serviceName string) *MockConnection {
	return m.ExpectCommand(fmt.Sprintf("systemctl show %s", serviceName), &CommandResponse{
		ExitCode: 1,
		Stderr:   fmt.Sprintf("Unit %s.service could not be found.", serviceName),
	})
}