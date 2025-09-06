# Gosinble Library Usage Guide

## 🚀 Using Gosinble as a Go Library

Gosinble is designed as a **Go library first**, CLI second. This means you can import and use all of its powerful automation features directly in your Go applications!

## Installation

```bash
go get github.com/gosinble/gosinble
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/gosinble/gosinble/pkg/inventory"
    "github.com/gosinble/gosinble/pkg/runner"
    "github.com/gosinble/gosinble/pkg/modules"
    "github.com/gosinble/gosinble/internal/common"
)

func main() {
    // 1. Create inventory
    inv := inventory.NewStaticInventory()
    inv.AddHost(common.Host{
        Name:     "web01",
        Address:  "192.168.1.100",
        User:     "admin",
        Password: "secret",
    })
    
    // 2. Create task runner
    taskRunner := runner.NewTaskRunner()
    taskRunner.SetMaxConcurrency(5)
    
    // 3. Define tasks
    tasks := []common.Task{
        {
            Name:   "Install nginx",
            Module: "package",
            Args: map[string]interface{}{
                "name":  "nginx",
                "state": "present",
            },
        },
        {
            Name:   "Start nginx",
            Module: "service",
            Args: map[string]interface{}{
                "name":  "nginx",
                "state": "started",
            },
        },
    }
    
    // 4. Execute tasks
    ctx := context.Background()
    results, err := taskRunner.ExecuteTasks(ctx, tasks, inv, "all")
    if err != nil {
        log.Fatal(err)
    }
    
    // 5. Process results
    for _, result := range results {
        if result.Success {
            log.Printf("✓ %s on %s", result.TaskName, result.Host)
        } else {
            log.Printf("✗ %s on %s: %v", result.TaskName, result.Host, result.Error)
        }
    }
}
```

## 📦 Core Components

### 1. Inventory Management

```go
import "github.com/gosinble/gosinble/pkg/inventory"

// Static inventory
inv := inventory.NewStaticInventory()

// Add hosts
inv.AddHost(common.Host{
    Name:     "db01",
    Address:  "10.0.1.50",
    Port:     22,
    User:     "root",
    Password: "pass123",
    Variables: map[string]interface{}{
        "db_type": "mysql",
        "db_port": 3306,
    },
})

// Add groups
inv.AddGroup(common.Group{
    Name:  "databases",
    Hosts: []string{"db01", "db02"},
    Variables: map[string]interface{}{
        "backup_enabled": true,
    },
})

// Load from YAML
yamlInv, err := inventory.NewFromFile("inventory.yml")

// Dynamic inventory
dynInv := inventory.NewDynamicInventory(myInventorySource)
```

### 2. Module System

```go
import "github.com/gosinble/gosinble/pkg/modules"

// Get module registry
registry := modules.DefaultModuleRegistry

// Register custom module
registry.RegisterModule(myCustomModule)

// Execute module directly
module, _ := registry.GetModule("file")
result, err := module.Run(ctx, connection, map[string]interface{}{
    "path":  "/tmp/test",
    "state": "directory",
    "mode":  "0755",
})
```

### 3. Task Runner

```go
import "github.com/gosinble/gosinble/pkg/runner"

runner := runner.NewTaskRunner()

// Configure runner
runner.SetMaxConcurrency(10)
runner.SetConnectionTimeout(30 * time.Second)
runner.SetTags([]string{"web", "deploy"})

// Execute with callbacks
runner.OnTaskStart(func(task common.Task, host string) {
    log.Printf("Starting %s on %s", task.Name, host)
})

runner.OnTaskComplete(func(result common.Result) {
    log.Printf("Completed: %v", result)
})

results, err := runner.ExecuteTasks(ctx, tasks, inventory, "web_servers")
```

### 4. Connection Management

```go
import "github.com/gosinble/gosinble/pkg/connection"

// Create connection manager
connMgr := connection.NewConnectionManager()

// Register custom connection type
connMgr.RegisterPlugin(connection.ConnectionType("docker"), 
    func() common.Connection {
        return NewDockerConnection()
    })

// Create connection
conn, err := connMgr.GetConnection(ctx, common.ConnectionInfo{
    Type:     "ssh",
    Host:     "192.168.1.100",
    User:     "admin",
    Password: "secret",
})

// Execute commands
result, err := conn.Execute(ctx, "ls -la", common.ExecuteOptions{})

// Copy files
err = conn.Copy(ctx, reader, "/remote/path", 0644)
```

## 🎯 Advanced Library Features

### Playbook Execution

```go
import "github.com/gosinble/gosinble/pkg/playbook"

// Parse playbook
pb, err := playbook.ParseFile("deploy.yml")

// Create executor
executor := playbook.NewExecutor(taskRunner, inventory, varManager)

// Execute playbook
results, err := executor.Execute(ctx, pb)

// Execute specific plays
results, err := executor.ExecutePlays(ctx, pb.Plays[0:2])
```

### Variable Management

```go
import "github.com/gosinble/gosinble/pkg/vars"

varMgr := vars.NewVarManager()

// Set variables at different levels
varMgr.SetGlobalVars(map[string]interface{}{
    "environment": "production",
})

varMgr.SetHostVars("web01", map[string]interface{}{
    "http_port": 8080,
})

varMgr.SetGroupVars("databases", map[string]interface{}{
    "backup_time": "02:00",
})

// Get merged variables for a host
hostVars := varMgr.GetHostVars("web01")
```

### Template Engine

```go
import "github.com/gosinble/gosinble/pkg/template"

engine := template.NewEngine()

// Register custom functions
engine.RegisterFunction("custom_func", myFunc)

// Render template
result, err := engine.RenderString(templateStr, vars)

// Render file
result, err := engine.RenderFile("config.j2", vars)
```

### Vault Encryption

```go
import "github.com/gosinble/gosinble/pkg/vault"

v := vault.New("my-vault-password")

// Encrypt sensitive data
encrypted, err := v.EncryptString("secret-password")

// Decrypt
decrypted, err := v.DecryptString(encrypted)

// Work with files
err = v.EncryptFile("secrets.yml", "secrets.vault")
err = v.DecryptFile("secrets.vault", "secrets.yml")
```

## 🔧 Library Task Builders

### Using CommonTasks

```go
import "github.com/gosinble/gosinble/pkg/library"

tasks := library.NewCommonTasks()

// File operations
fileTasks := tasks.File.EnsureDirectory("/opt/app", 0755)
fileTasks = append(fileTasks, tasks.File.BackupFile("/etc/config", "/backup/")...)

// Service operations
serviceTasks := tasks.Service.InstallBinaryAsService("myapp", 
    "/opt/myapp/bin", serviceConfig)

// Package operations
pkgTasks := tasks.Package.InstallDependencies([]string{"git", "docker"})

// Network operations
netTasks := tasks.Network.ConfigureFirewall([]int{80, 443}, "tcp")

// Execute all tasks
allTasks := append(fileTasks, serviceTasks...)
allTasks = append(allTasks, pkgTasks...)
allTasks = append(allTasks, netTasks...)

results, err := runner.ExecuteTasks(ctx, allTasks, inventory, "all")
```

### Content Distribution

```go
import "github.com/gosinble/gosinble/pkg/library"

// For embedded files
//go:embed assets/*
var assetsFS embed.FS

embedded := library.NewEmbeddedContent(assetsFS, "assets")
tasks := embedded.DeployFile("config.yml", "/etc/app/config.yml")

// For external files
dist := library.NewDistributionTasks()
dist.RegisterFile("binary", "/local/path/app", 0755)
tasks := dist.DistributeBinary("binary", "/usr/local/bin/app", true)
```

## 💡 Real-World Examples

### Example 1: Automated Deployment System

```go
package deployment

import (
    "context"
    "fmt"
    
    "github.com/gosinble/gosinble/pkg/inventory"
    "github.com/gosinble/gosinble/pkg/runner"
    "github.com/gosinble/gosinble/pkg/library"
    "github.com/gosinble/gosinble/internal/common"
)

type Deployer struct {
    runner    *runner.TaskRunner
    inventory *inventory.StaticInventory
    tasks     *library.CommonTasks
}

func NewDeployer(inventoryFile string) (*Deployer, error) {
    inv, err := inventory.NewFromFile(inventoryFile)
    if err != nil {
        return nil, err
    }
    
    return &Deployer{
        runner:    runner.NewTaskRunner(),
        inventory: inv,
        tasks:     library.NewCommonTasks(),
    }, nil
}

func (d *Deployer) DeployApplication(ctx context.Context, app AppConfig) error {
    var tasks []common.Task
    
    // 1. Prepare environment
    tasks = append(tasks, d.tasks.File.EnsureDirectory(app.InstallPath, 0755)...)
    
    // 2. Install dependencies
    tasks = append(tasks, d.tasks.Package.InstallDependencies(app.Dependencies)...)
    
    // 3. Copy application files
    tasks = append(tasks, d.tasks.Distribution.DistributeBinary(
        app.BinaryName, app.BinaryPath, true)...)
    
    // 4. Configure application
    tasks = append(tasks, d.tasks.File.TemplateConfig(
        app.ConfigTemplate, app.ConfigPath, app.ConfigVars)...)
    
    // 5. Setup service
    tasks = append(tasks, d.tasks.Service.InstallBinaryAsService(
        app.ServiceName, app.BinaryPath, app.ServiceTemplate)...)
    
    // 6. Start service
    tasks = append(tasks, common.Task{
        Name:   "Start application",
        Module: "service",
        Args: map[string]interface{}{
            "name":  app.ServiceName,
            "state": "started",
            "enabled": true,
        },
    })
    
    // Execute deployment
    results, err := d.runner.ExecuteTasks(ctx, tasks, d.inventory, app.TargetHosts)
    if err != nil {
        return fmt.Errorf("deployment failed: %w", err)
    }
    
    // Check results
    for _, result := range results {
        if !result.Success {
            return fmt.Errorf("task %s failed on %s: %v", 
                result.TaskName, result.Host, result.Error)
        }
    }
    
    return nil
}
```

### Example 2: Infrastructure Monitoring

```go
package monitoring

import (
    "context"
    "time"
    
    "github.com/gosinble/gosinble/pkg/inventory"
    "github.com/gosinble/gosinble/pkg/runner"
    "github.com/gosinble/gosinble/internal/common"
)

type Monitor struct {
    runner    *runner.TaskRunner
    inventory common.Inventory
}

func (m *Monitor) CheckServices(ctx context.Context, services []string) ([]ServiceStatus, error) {
    var tasks []common.Task
    
    for _, service := range services {
        tasks = append(tasks, common.Task{
            Name:   fmt.Sprintf("Check %s", service),
            Module: "shell",
            Args: map[string]interface{}{
                "cmd": fmt.Sprintf("systemctl is-active %s", service),
            },
        })
    }
    
    results, err := m.runner.ExecuteTasks(ctx, tasks, m.inventory, "all")
    if err != nil {
        return nil, err
    }
    
    // Process results
    var statuses []ServiceStatus
    for _, result := range results {
        statuses = append(statuses, ServiceStatus{
            Host:    result.Host,
            Service: result.TaskName,
            Active:  result.Success,
            Output:  result.Message,
        })
    }
    
    return statuses, nil
}

func (m *Monitor) CollectMetrics(ctx context.Context) ([]HostMetrics, error) {
    task := common.Task{
        Name:   "Collect metrics",
        Module: "shell",
        Args: map[string]interface{}{
            "cmd": "df -h && free -m && uptime",
        },
    }
    
    results, err := m.runner.ExecuteTasks(ctx, []common.Task{task}, 
        m.inventory, "all")
    if err != nil {
        return nil, err
    }
    
    return parseMetrics(results), nil
}
```

### Example 3: Configuration Management

```go
package config

import (
    "context"
    
    "github.com/gosinble/gosinble/pkg/runner"
    "github.com/gosinble/gosinble/pkg/library"
    "github.com/gosinble/gosinble/pkg/vault"
)

type ConfigManager struct {
    runner *runner.TaskRunner
    tasks  *library.CommonTasks
    vault  *vault.Vault
}

func (cm *ConfigManager) UpdateConfiguration(ctx context.Context, cfg ConfigUpdate) error {
    // Decrypt sensitive values
    if cfg.HasSecrets {
        decrypted, err := cm.vault.DecryptString(cfg.EncryptedData)
        if err != nil {
            return err
        }
        cfg.Secrets = parseSecrets(decrypted)
    }
    
    // Build configuration tasks
    var tasks []common.Task
    
    // Backup existing config
    tasks = append(tasks, cm.tasks.File.BackupFile(cfg.Path, "/backup/")...)
    
    // Apply new configuration
    tasks = append(tasks, cm.tasks.File.TemplateConfig(
        cfg.Template, cfg.Path, cfg.Variables)...)
    
    // Validate configuration
    tasks = append(tasks, common.Task{
        Name:   "Validate config",
        Module: "shell",
        Args: map[string]interface{}{
            "cmd": cfg.ValidationCommand,
        },
    })
    
    // Reload service if needed
    if cfg.ReloadService != "" {
        tasks = append(tasks, cm.tasks.Service.ReloadServiceConfig(cfg.ReloadService)...)
    }
    
    return cm.executeWithRollback(ctx, tasks, cfg)
}
```

## 🎨 Custom Module Development

```go
package custom

import (
    "context"
    "github.com/gosinble/gosinble/internal/common"
    "github.com/gosinble/gosinble/pkg/modules"
)

// Custom module for Docker operations
type DockerModule struct {
    modules.BaseModule
}

func NewDockerModule() *DockerModule {
    return &DockerModule{
        BaseModule: modules.BaseModule{
            name: "docker",
        },
    }
}

func (m *DockerModule) Run(ctx context.Context, conn common.Connection, 
    args map[string]interface{}) (*common.Result, error) {
    
    action := args["action"].(string)
    image := args["image"].(string)
    
    var cmd string
    switch action {
    case "pull":
        cmd = fmt.Sprintf("docker pull %s", image)
    case "run":
        name := args["name"].(string)
        cmd = fmt.Sprintf("docker run -d --name %s %s", name, image)
    case "stop":
        name := args["name"].(string)
        cmd = fmt.Sprintf("docker stop %s", name)
    }
    
    result, err := conn.Execute(ctx, cmd, common.ExecuteOptions{})
    if err != nil {
        return nil, err
    }
    
    return &common.Result{
        Success: result.Success,
        Changed: true,
        Message: fmt.Sprintf("Docker %s completed", action),
    }, nil
}

func (m *DockerModule) Validate(args map[string]interface{}) error {
    if _, ok := args["action"]; !ok {
        return fmt.Errorf("action is required")
    }
    return nil
}

// Register the module
func init() {
    modules.DefaultModuleRegistry.RegisterModule(NewDockerModule())
}
```

## 🔗 Integration Examples

### With Kubernetes Operator

```go
func (r *AppReconciler) deployWithGosinble(ctx context.Context, app *v1.App) error {
    // Create inventory from Kubernetes nodes
    inv := inventory.NewStaticInventory()
    for _, node := range app.Spec.Nodes {
        inv.AddHost(common.Host{
            Name:    node.Name,
            Address: node.IP,
            User:    "kubernetes",
        })
    }
    
    // Use gosinble to deploy
    deployer := NewDeployer(inv)
    return deployer.Deploy(ctx, app.Spec.DeploymentConfig)
}
```

### With CI/CD Pipeline

```go
func runDeploymentStage(ctx context.Context, env Environment) error {
    // Load inventory for environment
    inv, err := inventory.NewFromFile(fmt.Sprintf("inventory/%s.yml", env))
    if err != nil {
        return err
    }
    
    // Run deployment playbook
    pb, err := playbook.ParseFile("deploy.yml")
    if err != nil {
        return err
    }
    
    executor := playbook.NewExecutor(runner.NewTaskRunner(), inv, nil)
    results, err := executor.Execute(ctx, pb)
    
    // Report results
    return reportDeploymentResults(results)
}
```

## 📚 Best Practices

1. **Error Handling**: Always check task results
2. **Context Usage**: Pass context for cancellation support
3. **Connection Pooling**: Reuse connections when possible
4. **Concurrent Execution**: Set appropriate concurrency limits
5. **Variable Management**: Use proper variable precedence
6. **Secret Management**: Always use vault for sensitive data
7. **Logging**: Implement proper logging callbacks
8. **Testing**: Mock connections for unit tests

## 🎯 Why Use Gosinble as a Library?

- **Native Go Integration**: No subprocess calls or CLI parsing
- **Type Safety**: Full Go type checking and IDE support
- **Performance**: Direct function calls, no overhead
- **Flexibility**: Customize every aspect programmatically
- **Error Handling**: Proper Go error handling
- **Testing**: Easy to mock and test
- **Concurrency**: Leverage Go's excellent concurrency

---

*Gosinble - Ansible's power, Go's simplicity!*