package testing

import (
	"context"
	"strings"
	"testing"

	"github.com/liliang-cn/gosible/pkg/types"
)

// ModuleTestHelper provides high-level testing orchestration for modules
type ModuleTestHelper struct {
	t          *testing.T
	module     types.Module
	connection *MockConnection
	filesystem *MockFileSystem
	context    context.Context
}

// NewModuleTestHelper creates a new module test helper
func NewModuleTestHelper(t *testing.T, module types.Module) *ModuleTestHelper {
	return &ModuleTestHelper{
		t:          t,
		module:     module,
		connection: NewMockConnection(t),
		filesystem: NewMockFileSystem(t),
		context:    context.Background(),
	}
}

// GetConnection returns the mock connection for setting up expectations
func (h *ModuleTestHelper) GetConnection() *MockConnection {
	return h.connection
}

// GetFileSystem returns the mock filesystem for setting up file expectations
func (h *ModuleTestHelper) GetFileSystem() *MockFileSystem {
	return h.filesystem
}

// SetContext sets the context to use for module execution
func (h *ModuleTestHelper) SetContext(ctx context.Context) *ModuleTestHelper {
	h.context = ctx
	return h
}

// Execute runs the module with the given arguments and modes
func (h *ModuleTestHelper) Execute(args map[string]interface{}, checkMode, diffMode bool) *types.Result {
	return h.ExecuteWithHost(args, checkMode, diffMode, "test-host")
}

// ExecuteWithHost runs the module with the given arguments, modes, and host
func (h *ModuleTestHelper) ExecuteWithHost(args map[string]interface{}, checkMode, diffMode bool, host string) *types.Result {
	// Set up mode flags if needed
	if checkMode {
		args["_check_mode"] = true
	}
	if diffMode {
		args["_diff"] = true
	}
	
	// Set hostname on connection
	h.connection.SetHostname(host)
	
	// First validate the arguments
	if err := h.module.Validate(args); err != nil {
		h.t.Fatalf("Module validation failed: %v", err)
	}
	
	// Execute the module
	result, err := h.module.Run(h.context, h.connection, args)
	if err != nil {
		h.t.Fatalf("Module execution failed: %v", err)
	}
	
	if result == nil {
		h.t.Fatal("Module returned nil result")
	}
	
	return result
}

// ExecuteExpectingError runs the module expecting it to return an error
func (h *ModuleTestHelper) ExecuteExpectingError(args map[string]interface{}) error {
	// First validate the arguments
	if err := h.module.Validate(args); err != nil {
		return err // Validation error is expected
	}
	
	// Execute the module
	_, err := h.module.Run(h.context, h.connection, args)
	if err == nil {
		h.t.Error("Expected module to return an error, but it didn't")
	}
	
	return err
}

// ExecuteValidationTest runs validation tests with expected outcomes
func (h *ModuleTestHelper) ExecuteValidationTest(args map[string]interface{}, expectValid bool) {
	err := h.module.Validate(args)
	if expectValid && err != nil {
		h.t.Errorf("Expected validation to pass, but got error: %v", err)
	} else if !expectValid && err == nil {
		h.t.Error("Expected validation to fail, but it passed")
	}
}

// Verify checks that all mock expectations were met
func (h *ModuleTestHelper) Verify() {
	h.connection.Verify()
	// FileSystem doesn't need explicit verification, but we could add it
}

// Reset clears all mock expectations and history
func (h *ModuleTestHelper) Reset() *ModuleTestHelper {
	h.connection.Reset()
	h.filesystem.Reset()
	return h
}

// Assertion helpers

// AssertSuccess asserts that the result was successful
func (h *ModuleTestHelper) AssertSuccess(result *types.Result) {
	if !result.Success {
		h.t.Errorf("Expected result to be successful, but it failed with error: %v", result.Error)
	}
}

// AssertFailure asserts that the result was not successful
func (h *ModuleTestHelper) AssertFailure(result *types.Result) {
	if result.Success {
		h.t.Error("Expected result to be a failure, but it was successful")
	}
}

// AssertChanged asserts that the result indicates changes were made
func (h *ModuleTestHelper) AssertChanged(result *types.Result) {
	if !result.Changed {
		h.t.Error("Expected result to show changes were made, but Changed=false")
	}
}

// AssertNotChanged asserts that the result indicates no changes were made
func (h *ModuleTestHelper) AssertNotChanged(result *types.Result) {
	if result.Changed {
		h.t.Error("Expected result to show no changes were made, but Changed=true")
	}
}

// AssertSimulated asserts that the result was simulated (check mode)
func (h *ModuleTestHelper) AssertSimulated(result *types.Result) {
	if !result.Simulated {
		h.t.Error("Expected result to be simulated, but Simulated=false")
	}
}

// AssertNotSimulated asserts that the result was not simulated
func (h *ModuleTestHelper) AssertNotSimulated(result *types.Result) {
	if result.Simulated {
		h.t.Error("Expected result to not be simulated, but Simulated=true")
	}
}

// AssertCheckModeSimulated asserts that the result is properly formatted for check mode
func (h *ModuleTestHelper) AssertCheckModeSimulated(result *types.Result) {
	h.AssertSuccess(result)
	h.AssertSimulated(result)
	
	if result.Data == nil {
		h.t.Error("Expected result to have data for check mode")
		return
	}
	
	if checkMode, exists := result.Data["check_mode"]; !exists || checkMode != true {
		h.t.Error("Expected result to have check_mode=true in data")
	}
}

// AssertDiffPresent asserts that the result contains diff information
func (h *ModuleTestHelper) AssertDiffPresent(result *types.Result) {
	if result.Diff == nil {
		h.t.Error("Expected result to contain diff information, but Diff=nil")
	} else if !result.Diff.Prepared {
		h.t.Error("Expected diff to be prepared, but Prepared=false")
	}
}

// AssertDiffNotPresent asserts that the result does not contain diff information
func (h *ModuleTestHelper) AssertDiffNotPresent(result *types.Result) {
	if result.Diff != nil {
		h.t.Error("Expected result to not contain diff information, but Diff is not nil")
	}
}

// AssertMessage asserts that the result contains a specific message
func (h *ModuleTestHelper) AssertMessage(result *types.Result, expectedMessage string) {
	if result.Message != expectedMessage {
		h.t.Errorf("Expected result message to be '%s', but got '%s'", expectedMessage, result.Message)
	}
}

// AssertMessageContains asserts that the result message contains a specific substring
func (h *ModuleTestHelper) AssertMessageContains(result *types.Result, substring string) {
	if result.Message == "" {
		h.t.Error("Expected result to have a message, but it was empty")
		return
	}
	
	if !strings.Contains(result.Message, substring) {
		h.t.Errorf("Expected result message '%s' to contain '%s'", result.Message, substring)
	}
}

// AssertHost asserts that the result is for the expected host
func (h *ModuleTestHelper) AssertHost(result *types.Result, expectedHost string) {
	if result.Host != expectedHost {
		h.t.Errorf("Expected result host to be '%s', but got '%s'", expectedHost, result.Host)
	}
}

// AssertDataValue asserts that the result data contains a specific key-value pair
func (h *ModuleTestHelper) AssertDataValue(result *types.Result, key string, expectedValue interface{}) {
	if result.Data == nil {
		h.t.Error("Expected result to have data, but Data=nil")
		return
	}
	
	actualValue, exists := result.Data[key]
	if !exists {
		h.t.Errorf("Expected result data to contain key '%s', but it doesn't", key)
		return
	}
	
	if actualValue != expectedValue {
		h.t.Errorf("Expected result data[%s] to be %v, but got %v", key, expectedValue, actualValue)
	}
}

// AssertDataContainsKey asserts that the result data contains a specific key
func (h *ModuleTestHelper) AssertDataContainsKey(result *types.Result, key string) {
	if result.Data == nil {
		h.t.Error("Expected result to have data, but Data=nil")
		return
	}
	
	if _, exists := result.Data[key]; !exists {
		h.t.Errorf("Expected result data to contain key '%s', but it doesn't", key)
	}
}

// AssertDiffBefore asserts the before state in diff
func (h *ModuleTestHelper) AssertDiffBefore(result *types.Result, expectedBefore string) {
	h.AssertDiffPresent(result)
	if result.Diff.Before != expectedBefore {
		h.t.Errorf("Expected diff before state to be '%s', but got '%s'", expectedBefore, result.Diff.Before)
	}
}

// AssertDiffAfter asserts the after state in diff
func (h *ModuleTestHelper) AssertDiffAfter(result *types.Result, expectedAfter string) {
	h.AssertDiffPresent(result)
	if result.Diff.After != expectedAfter {
		h.t.Errorf("Expected diff after state to be '%s', but got '%s'", expectedAfter, result.Diff.After)
	}
}

// Batch testing helpers

// TestCase represents a single test case for batch execution
type TestCase struct {
	Name         string
	Args         map[string]interface{}
	CheckMode    bool
	DiffMode     bool
	ExpectError  bool
	Setup        func(helper *ModuleTestHelper) // Optional setup function
	Assertions   func(helper *ModuleTestHelper, result *types.Result) // Custom assertions
}

// RunTestCases executes multiple test cases in sequence
func (h *ModuleTestHelper) RunTestCases(cases []TestCase) {
	for _, tc := range cases {
		h.t.Run(tc.Name, func(t *testing.T) {
			// Create a new helper for this test case to ensure isolation
			caseHelper := &ModuleTestHelper{
				t:          t,
				module:     h.module,
				connection: NewMockConnection(t),
				filesystem: NewMockFileSystem(t),
				context:    h.context,
			}
			
			// Run setup if provided
			if tc.Setup != nil {
				tc.Setup(caseHelper)
			}
			
			// Execute the test case
			if tc.ExpectError {
				caseHelper.ExecuteExpectingError(tc.Args)
			} else {
				result := caseHelper.Execute(tc.Args, tc.CheckMode, tc.DiffMode)
				
				// Run custom assertions if provided
				if tc.Assertions != nil {
					tc.Assertions(caseHelper, result)
				}
			}
			
			// Verify mocks
			caseHelper.Verify()
		})
	}
}

// Validation test helpers

// ValidationTestCase represents a validation test case
type ValidationTestCase struct {
	Name        string
	Args        map[string]interface{}
	ExpectValid bool
}

// RunValidationTests executes multiple validation test cases
func (h *ModuleTestHelper) RunValidationTests(cases []ValidationTestCase) {
	for _, tc := range cases {
		h.t.Run(tc.Name, func(t *testing.T) {
			caseHelper := &ModuleTestHelper{
				t:          t,
				module:     h.module,
				connection: NewMockConnection(t),
				filesystem: NewMockFileSystem(t),
				context:    h.context,
			}
			
			caseHelper.ExecuteValidationTest(tc.Args, tc.ExpectValid)
		})
	}
}