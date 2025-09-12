# gosible Library Usage Examples

This example demonstrates various ways to use the gosible library programmatically in your Go applications.

## Features Demonstrated

- **Inventory Management**: Loading and managing hosts and groups
- **Module Execution**: Running automation modules directly
- **Template Rendering**: Processing Jinja2-compatible templates
- **Configuration Management**: Working with gosible configuration
- **Playbook Execution**: Running playbooks programmatically
- **Facts Gathering**: Collecting system information

## Running the Example

```bash
go run examples/library-usage/main.go
```

## Example Components

### 1. Inventory Management

- Loading inventory from YAML
- Adding hosts and groups programmatically
- Setting and getting host variables
- Querying inventory structure

### 2. Module Execution

- Running shell commands
- File operations (copy, template)
- Service management
- Package installation
- User management

### 3. Template Rendering

- Jinja2-compatible template processing
- Variable substitution
- Conditional rendering
- Loop processing

### 4. Configuration Management

- Loading configuration from file
- Setting configuration programmatically
- Managing connection parameters
- Configuring parallel execution

### 5. Playbook Execution

- Loading playbooks from YAML
- Executing playbooks with inventory
- Handling results and errors
- Task callbacks

### 6. Facts Gathering

- Collecting system information
- OS details
- Network configuration
- Hardware information
- Custom fact collection

## Example Output

```
=== gosible Library Examples ===

--- Inventory Example ---
All hosts: [webserver1 webserver2 dbserver]
Production hosts: [webserver1 webserver2 dbserver]
Web servers: [webserver1 webserver2]
Host webserver1 variables: ansible_host=192.168.1.10

--- Module Example ---
Module execution completed
Result: Success=true, Changed=true

--- Template Example ---
Rendered template:
Welcome to webserver1!
Server IP: 192.168.1.10

--- Config Example ---
Max parallel: 10
Connection timeout: 30s

--- Playbook Example ---
Running playbook: Deploy Application
Tasks: 5
Play completed successfully

--- Facts Example ---
OS: linux
Distribution: ubuntu
Version: 22.04
Architecture: x86_64
```

## Use Cases

### Embedding in Applications

```go
import (
    "github.com/liliang-cn/gosible/pkg/inventory"
    "github.com/liliang-cn/gosible/pkg/runner"
)

func deployApplication(hosts []string) error {
    inv := inventory.New()
    for _, host := range hosts {
        inv.AddHost(host, "production")
    }

    runner := runner.NewTaskRunner()
    // Run deployment tasks...
    return nil
}
```

### Custom Automation Tools

```go
func createCustomTool() {
    // Use gosible as a library to build
    // custom automation tools with your
    // specific business logic
}
```

### CI/CD Integration

```go
func runDeploymentPipeline() {
    // Integrate gosible into your
    // CI/CD pipeline for infrastructure
    // automation
}
```

## Key Benefits

- **Type Safety**: Full Go type checking
- **IDE Support**: Autocomplete and documentation
- **Error Handling**: Proper Go error handling
- **Performance**: Direct function calls, no subprocess overhead
- **Testability**: Easy to mock and unit test
- **Composability**: Mix and match components as needed

This example provides a foundation for using gosible as a library in your Go applications.
