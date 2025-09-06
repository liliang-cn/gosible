# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
Gosinble is a **Go library first, CLI second** that implements the main features of Ansible for configuration management and automation. It's designed to be imported and used programmatically in other Go applications, providing a powerful API for infrastructure automation, configuration management, and orchestration tasks.

### Key Design Philosophy
- **Library-First Design**: All functionality is exposed through a clean Go API
- **Type-Safe**: Full Go type checking and IDE support
- **Native Integration**: No subprocess calls or CLI parsing needed
- **High Performance**: Direct function calls with Go's excellent concurrency
- **Composable**: Mix and match components as needed
- **Testable**: Easy to mock and unit test

### Primary Use Cases
1. **Embedded Automation**: Add automation capabilities to existing Go applications
2. **Custom Orchestration**: Build custom deployment and configuration tools
3. **Kubernetes Operators**: Implement operators with infrastructure automation
4. **CI/CD Integration**: Native integration in Go-based CI/CD pipelines
5. **Monitoring Systems**: Add remediation capabilities to monitoring tools
6. **Configuration Management**: Programmatic configuration management in Go apps

## Development Commands

### Go Module Management
```bash
go mod tidy          # Clean up dependencies
go mod vendor        # Vendor dependencies (if using vendoring)
```

### Building and Testing
```bash
go build ./...       # Build all packages
go test ./...        # Run all tests
go test -v ./...     # Run tests with verbose output
go test -race ./...  # Run tests with race detection
```

### Single Package Operations
```bash
go test ./pkg/inventory    # Test specific package
go run ./cmd/example       # Run example programs
```

### Code Quality
```bash
go fmt ./...              # Format code
go vet ./...              # Static analysis
golint ./...              # Lint code (requires golint installation)
staticcheck ./...         # Advanced static analysis (requires staticcheck)
```

## Library Usage

### Quick Import Example
```go
import (
    "github.com/gosinble/gosinble/pkg/inventory"
    "github.com/gosinble/gosinble/pkg/runner"
    "github.com/gosinble/gosinble/pkg/modules"
    "github.com/gosinble/gosinble/pkg/playbook"
    "github.com/gosinble/gosinble/pkg/library"
    "github.com/gosinble/gosinble/pkg/vault"
)

// Use gosinble in your application
func deployApp(ctx context.Context) error {
    inv, _ := inventory.NewFromFile("inventory.yml")
    runner := runner.NewTaskRunner()
    tasks := library.NewCommonTasks()
    
    // Build deployment tasks programmatically
    deployTasks := tasks.Package.InstallDependencies([]string{"nginx"})
    deployTasks = append(deployTasks, tasks.Service.EnsureServiceRunning("nginx")...)
    
    results, err := runner.ExecuteTasks(ctx, deployTasks, inv, "web_servers")
    return err
}
```

### Key Library Features
- **Programmatic Task Building**: Create tasks in Go code, not YAML
- **Custom Module Development**: Implement custom modules as Go types
- **Event Callbacks**: Hook into task execution lifecycle
- **Error Handling**: Proper Go error handling, not exit codes
- **Testing Support**: Mock connections and modules for unit tests
- **Concurrent Execution**: Leverage Go's goroutines for parallel execution

For detailed library usage examples, see [LIBRARY_USAGE.md](LIBRARY_USAGE.md)

## Architecture

### Core Components
- **Inventory Management**: Host and group management with dynamic inventory support
- **Module System**: Extensible module architecture for different automation tasks
- **Playbook Engine**: YAML-based playbook parsing and execution
- **Task Runner**: Parallel task execution with proper dependency handling
- **Templating**: Jinja2-compatible template rendering for configurations
- **Connection Plugins**: SSH, WinRM, and local connection support
- **Variable Management**: Fact gathering, variable precedence, and scoping

### Package Structure
```
pkg/
├── inventory/     # Host and group management
├── modules/       # Built-in and custom modules
├── playbook/      # Playbook parsing and execution
├── runner/        # Task execution engine
├── template/      # Template rendering
├── connection/    # Connection plugins (SSH, WinRM, etc.)
├── vars/          # Variable management and fact gathering
└── config/        # Configuration management
```

### Key Design Patterns
- **Plugin Architecture**: Modules, connections, and callbacks use plugin interfaces
- **Context Propagation**: Use context.Context for cancellation and timeouts
- **Concurrent Execution**: Goroutines with proper synchronization for parallel tasks
- **Error Wrapping**: Use fmt.Errorf with %w verb for error chain preservation
- **Configuration**: Environment variables and config files for library settings

## Library Usage Patterns

### Initialization
```go
import "github.com/gosinble/gosinble"

// Initialize with inventory
inv, err := inventory.NewFromFile("hosts.yml")
runner := gosinble.NewRunner(inv)
```

### Common Interfaces
- All modules implement `Module` interface with `Run(ctx context.Context, args map[string]interface{}) error`
- Connection plugins implement `Connection` interface for remote execution
- Inventory sources implement `InventorySource` interface for dynamic inventory

## Testing Strategy
- Unit tests for individual components in `*_test.go` files
- Integration tests in `integration/` directory
- Mock implementations for external dependencies
- Table-driven tests for module validation
- Benchmark tests for performance-critical paths
- no need to think about Backward Compatible we are at the earily stage '