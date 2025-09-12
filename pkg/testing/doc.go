// Package testing provides comprehensive testing utilities and mock implementations for gosible modules.
//
// This package includes:
//
//   - MockConnection: A mock implementation of types.Connection for simulating command execution
//   - MockFileSystem: A mock filesystem for testing file operations without touching the real filesystem
//   - ModuleTestHelper: A high-level testing framework that simplifies module testing
//   - Specialized helpers: SystemdTestHelper, FileTestHelper, and PackageTestHelper for common scenarios
//
// Usage Example:
//
//	func TestMyModule(t *testing.T) {
//		module := NewMyModule()
//		helper := testing.NewModuleTestHelper(t, module)
//		
//		// Setup mock expectations
//		helper.GetConnection().ExpectCommand("echo hello", &testing.CommandResponse{
//			Stdout: "hello",
//			ExitCode: 0,
//		})
//		
//		// Execute module
//		result := helper.Execute(map[string]interface{}{
//			"message": "hello",
//		}, false, false)
//		
//		// Verify results
//		helper.AssertSuccess(result)
//		helper.AssertChanged(result)
//	}
//
// The testing framework supports:
//   - Check mode and diff mode testing
//   - Command execution mocking with pattern matching
//   - File system operation simulation
//   - Error injection and failure scenarios
//   - Test case batching and organization
//   - Specialized helpers for systemd, file, and package operations
package testing