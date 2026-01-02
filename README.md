# gosible - Go Library for Infrastructure Automation

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-100%25%20Passing-brightgreen)](TEST_REPORT.md)

gosible is a **Go library first, CLI second** that implements Ansible's core features for configuration management and automation. Built for Go developers who want to embed powerful automation capabilities directly into their applications.

## ✨ Features

- **🎯 Library-First Design**: Import as a library, not a CLI wrapper - full Go API with type safety
- **🚀 High Performance**: 96-340+ ops/sec with native Go concurrency and connection pooling
- **📝 Jinja2-Style Variables**: Full template variable support with `{{ variable }}` syntax
- **🔧 SSH Config Support**: Reads `~/.ssh/config` for seamless integration
- **⚡ Parallel Execution**: Configurable concurrency with efficient connection reuse
- **🧪 Production Ready**: 100% test pass rate, verified on real environments
- **📦 Zero External Runtime**: Single binary, no Python or agent installation needed

## Why gosible?

| Feature | gosible | Ansible |
|---------|---------|---------|
| Integration | Native Go library | Python subprocess |
| Type Safety | Compile-time checking | Runtime errors |
| Performance | 96-340 ops/sec | 10-30 ops/sec |
| Memory Usage | ~50MB | ~200MB |
| Startup Time | <1ms | ~500ms |
| Concurrency | Native goroutines | Process-based |

## Installation

```bash
go get github.com/liliang-cn/gosible
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/liliang-cn/gosible/pkg/inventory"
    "github.com/liliang-cn/gosible/pkg/runner"
    "github.com/liliang-cn/gosible/pkg/types"
)

func main() {
    // Create inventory
    inv := inventory.NewStaticInventory()
    inv.AddHost(types.Host{
        Name: "web1",
        Address: "web1.example.com",
        User: "ubuntu",
        Groups: []string{"webservers"},
    })

    // Get hosts for task execution
    hosts, _ := inv.GetHosts("webservers")

    // Create task runner with configurable concurrency
    taskRunner := runner.NewTaskRunner()
    taskRunner.SetMaxConcurrency(10) // Execute on up to 10 hosts in parallel

    ctx := context.Background()

    // Define variables for template rendering
    vars := map[string]interface{}{
        "package_name": "nginx",
        "service_port": 8080,
    }

    // Execute task with variables
    results, err := taskRunner.ExecuteTask(ctx,
        "Install nginx",
        "apt",
        map[string]interface{}{
            "name": "{{package_name}}",  // Jinja2-style variable
            "state": "present",
            "update_cache": true,
        },
        hosts,
        vars,
    )

    if err != nil {
        log.Fatal(err)
    }

    // Process results
    for _, result := range results {
        if result.Success {
            fmt.Printf("✓ %s: Package installed\n", result.Host)
        } else {
            fmt.Printf("✗ %s: %s\n", result.Host, result.Error)
        }
    }
}
```

## Template Variables

gosible supports Jinja2-style template variables in all module parameters:

```go
vars := map[string]interface{}{
    "app_name": "myapp",
    "version": "1.0.0",
    "config_dir": "/etc/myapp",
}

taskArgs := map[string]interface{}{
    "msg": "Deploying {{app_name}} version {{version}} to {{config_dir}}",
}

results, _ := taskRunner.ExecuteTask(ctx, "Deploy", "debug", taskArgs, hosts, vars)
// Output: "Deploying myapp version 1.0.0 to /etc/myapp"
```

Features:
- ✅ Recursive variable expansion in maps and slices
- ✅ Works with all modules (command, debug, template, copy, etc.)
- ✅ Automatic expansion before task execution

## Primary Use Cases

- **🏗️ Embedded Automation**: Add automation capabilities to existing Go applications
- **🎭 Custom Orchestration**: Build custom deployment and configuration tools
- **☸️ Kubernetes Operators**: Implement operators with infrastructure automation
- **🔄 CI/CD Integration**: Native integration in Go-based CI/CD pipelines
- **📊 Monitoring Systems**: Add remediation capabilities to monitoring tools
- **⚙️ Configuration Management**: Programmatic configuration management in Go apps

## SSH Configuration

gosible automatically reads `~/.ssh/config` for seamless integration:

```ssh
# ~/.ssh/config
Host production
    HostName 192.168.1.100
    User deploy
    Port 2222
    IdentityFile ~/.ssh/deploy_key
```

```go
// Automatically uses SSH config settings
inv.AddHost(types.Host{
    Name: "production",  // Will read HostName, User, Port from SSH config
})
```

Supported SSH config directives:
- `HostName` - Override hostname/address
- `User` - Default username
- `Port` - SSH port number
- `IdentityFile` - Path to private key (with `~` expansion)
- Pattern matching with wildcards (`*`, `?`)

## Concurrency & Performance

### Configurable Concurrency

```go
taskRunner := runner.NewTaskRunner()
taskRunner.SetMaxConcurrency(50)  // Up to 50 parallel hosts
```

**Default**: 5 parallel hosts (balanced for most environments)

**Performance Benchmarks** (Ubuntu 22.04 over SSH):

| Concurrency | Throughput | Latency |
|-------------|------------|---------|
| Serial (1) | 30 ops/sec | ~33ms |
| Default (5) | 96 ops/sec | ~10ms |
| High (20) | 200 ops/sec | ~5ms |
| Max (50) | 340 ops/sec | ~3ms |

See [CONCURRENCY.md](CONCURRENCY.md) for complete performance guide.

### Connection Pooling

gosible implements automatic connection pooling:
- ✅ Connections cached for 30 minutes (configurable)
- ✅ Automatic reuse across tasks
- ✅ No repeated SSH handshakes
- ✅ Thread-safe concurrent access

## Core Components

### Package Structure

```
pkg/
├── inventory/     # Host and group management
├── modules/       # 40+ built-in modules (command, shell, copy, file, etc.)
├── playbook/      # Playbook parsing and execution
├── runner/        # Task execution engine with parallel execution
├── template/      # Jinja2-compatible template rendering
├── connection/    # Connection plugins (SSH, WinRM, Local) with SSH config support
├── vars/          # Variable management and fact gathering
├── vault/         # Ansible Vault-compatible encryption
└── websocket/     # Real-time streaming output
```

### Available Modules

**System Management**:
- `command`, `shell` - Execute commands
- `service`, `systemd` - Service management
- `setup` - Gather system facts

**File Operations**:
- `file` - File/directory management
- `copy` - Copy files with content
- `template` - Template rendering
- `lineinfile`, `blockinfile` - Edit files
- `replace`, `ini_file` - Modify configurations

**Package Management**:
- `apt`, `yum`, `dnf` - Debian/RHEL package managers
- `pip`, `npm`, `gem` - Language package managers
- `package` - Universal package manager

**Utilities**:
- `debug` - Print messages
- `ping` - Connection test
- `user`, `group` - User management
- `cron`, `mount`, `sysctl` - System configuration

## Advanced Usage

### Playbook Execution

```go
import "github.com/liliang-cn/gosible/pkg/playbook"

playbookYAML := `
---
- name: Deploy application
  hosts: webservers
  vars:
    app_version: "1.0.0"
  tasks:
    - name: Deploy app
      debug:
        msg: "Deploying version {{app_version}}"
`

parser := playbook.NewParser()
pb, _ := parser.Parse([]byte(playbookYAML), "deploy.yml")

executor := playbook.NewExecutor(taskRunner, inv, varsMgr)
results, _ := executor.Execute(ctx, pb, nil)
```

### Custom Module Development

```go
import "github.com/liliang-cn/gosible/pkg/modules"

type CustomModule struct {
    modules.BaseModule
}

func (m *CustomModule) Run(ctx context.Context, args map[string]interface{}) (*types.Result, error) {
    // Your custom automation logic
    return &types.Result{
        Success: true,
        Message: "Custom task completed",
    }, nil
}

// Validate required parameters
func (m *CustomModule) Validate(args map[string]interface{}) error {
    return types.ValidateRequiredFields(args, []string{"required_param"})
}

// Register and use
modules.DefaultModuleRegistry.Register("custom", &CustomModule{})
```

### Real-Time Streaming

```go
import "github.com/liliang-cn/gosible/pkg/logging"

// Stream output in real-time
logger := logging.NewStreamingLogger(os.Stdout)
runner.SetLogger(logger)

results, _ := taskRunner.ExecuteTask(ctx, "Long task", "command",
    map[string]interface{}{"cmd": "for i in {1..10}; do echo $i; sleep 1; done"},
    hosts, nil)
// Output streams as it arrives
```

### Testing Your Automation

```go
import "github.com/liliang-cn/gosible/pkg/testing"

// Use mock connections for unit tests
mockConn := testing.NewMockConnection()
mockConn.AddCommandResponse("hostname", "testhost", nil)

results, err := taskRunner.ExecuteTask(ctx, "Test", "command",
    map[string]interface{}{"cmd": "hostname"}, hosts, nil)

assert.NoError(t, err)
assert.True(t, results[0].Success)
assert.Equal(t, "testhost", results[0].Data["stdout"])
```

## Examples

Check out the [examples/](examples/) directory for complete working examples:

- **[library-usage](examples/library-usage/)** - Core library features
- **[common_tasks_example](examples/common_tasks_example/)** - High-level task builders
- **[playbook-example](examples/cli-vs-library/)** - Playbook execution
- **[real-time-output](examples/real-time-output-example/)** - Streaming output
- **[systemd_service](examples/systemd_service/)** - Service management
- **And 15+ more examples...**

## Development

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/runner -v

# Run benchmarks
go test -bench=. -benchmem ./...

# Build all packages
go build ./...
```

## Documentation

- **[TEST_REPORT.md](TEST_REPORT.md)** - Comprehensive test results (100% pass rate)
- **[CONCURRENCY.md](CONCURRENCY.md)** - Performance optimization guide
- **[FIXES_SUMMARY.md](FIXES_SUMMARY.md)** - Recent bug fixes and improvements
- **[LIBRARY_USAGE.md](LIBRARY_USAGE.md)** - Detailed library usage guide
- **[examples/](examples/)** - 20+ working examples

## Performance

Based on production testing on Ubuntu 22.04:

- **✅ 96-340 ops/sec** throughput
- **✅ <30ms average latency** (default concurrency)
- **✅ 100% test pass rate** across 27 test cases
- **✅ Thread-safe** concurrent execution
- **✅ Efficient connection pooling** (30min TTL)

## Production Ready

✅ **Verified on real environments** (orange1, orange2, orange3)
✅ **All functionality tested** - connection, modules, playbook, streaming
✅ **Stress tested** - 100+ concurrent tasks, 300+ operations
✅ **Error handling** - clear error messages, proper recovery
✅ **Documentation** - comprehensive guides and examples

## Contributing

Contributions welcome! Please see:

1. **[CONTRIBUTING.md](CONTRIBUTING.md)** - Contribution guidelines
2. **[CLAUDE.md](CLAUDE.md)** - Project design philosophy
3. **[TEST_REPORT.md](TEST_REPORT.md)** - Testing methodology

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

Inspired by [Ansible](https://www.ansible.com/), built for Go developers.

---

**gosible** - Infrastructure automation as a Go library

Made with ❤️ by [Liang Li](https://github.com/liliang-cn)
