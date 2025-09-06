<!-- 
Saved from Claude conversation
Date: 2025-09-06T14:32:00Z
Context: Gosinble Library API Documentation and Usage Guide
-->

# Gosinble Library API Documentation

Based on the gosinble library code and documentation, here's a comprehensive guide on how to use it as a Go library:

## 📚 Installation & Import

```go
import (
    "github.com/gosinble/gosinble"
    "github.com/gosinble/gosinble/pkg/inventory"
    "github.com/gosinble/gosinble/pkg/modules"
    "github.com/gosinble/gosinble/pkg/connection"
    "github.com/gosinble/gosinble/pkg/playbook"
    "github.com/gosinble/gosinble/pkg/runner"
    "github.com/gosinble/gosinble/pkg/template"
    "github.com/gosinble/gosinble/pkg/vars"
    "github.com/gosinble/gosinble/pkg/vault"
)
```

## 🚀 Core API Usage Patterns

### 1. Basic Module Execution

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/gosinble/gosinble/pkg/modules"
    "github.com/gosinble/gosinble/pkg/connection"
    "github.com/gosinble/gosinble/internal/common"
)

func main() {
    ctx := context.Background()
    
    // Create connection
    conn := connection.NewLocalConnection()
    connInfo := common.ConnectionInfo{
        Type: "local",
        Host: "localhost",
    }
    
    if err := conn.Connect(ctx, connInfo); err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    
    // Use a module
    fileModule := modules.NewFileModule()
    args := map[string]interface{}{
        "path":  "/tmp/test.txt",
        "state": "file",
        "mode":  "0644",
    }
    
    result, err := fileModule.Run(ctx, conn, args)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Success: %v, Changed: %v, Message: %s\n", 
        result.Success, result.Changed, result.Message)
}
```

### 2. SSH Connection

```go
func connectToRemoteHost() {
    ctx := context.Background()
    
    // SSH with password
    conn := connection.NewSSHConnection()
    connInfo := common.ConnectionInfo{
        Type:     "ssh",
        Host:     "192.168.1.100",
        Port:     22,
        User:     "ubuntu",
        Password: "mypassword",
    }
    
    // Or SSH with key
    connInfo = common.ConnectionInfo{
        Type:    "ssh",
        Host:    "192.168.1.100", 
        Port:    22,
        User:    "ubuntu",
        KeyFile: "/home/user/.ssh/id_rsa",
    }
    
    if err := conn.Connect(ctx, connInfo); err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    
    // Now use modules on remote host
    serviceModule := modules.NewServiceModule()
    result, _ := serviceModule.Run(ctx, conn, map[string]interface{}{
        "name":  "nginx",
        "state": "started",
    })
    
    fmt.Printf("Service result: %+v\n", result)
}
```

### 3. Inventory Management

```go
func useInventory() {
    // Create static inventory
    inv := inventory.NewStaticInventory()
    
    // Add hosts
    host := &common.Host{
        Name:    "web1",
        Address: "192.168.1.10",
        Port:    22,
        User:    "ubuntu",
        Variables: map[string]interface{}{
            "env": "production",
            "role": "webserver",
        },
    }
    inv.AddHost(host)
    
    // Add to groups
    inv.AddHostToGroup("web1", "webservers")
    inv.AddHostToGroup("web1", "production")
    
    // Load from YAML
    yamlContent := `
all:
  hosts:
    web1:
      ansible_host: 192.168.1.10
      ansible_user: ubuntu
    web2:
      ansible_host: 192.168.1.11
      ansible_user: ubuntu
  children:
    webservers:
      hosts:
        web1:
        web2:
`
    
    inv, err := inventory.NewFromYAML([]byte(yamlContent))
    if err != nil {
        log.Fatal(err)
    }
    
    // Get hosts by pattern
    hosts := inv.GetHosts("webservers")
    for _, host := range hosts {
        fmt.Printf("Host: %s (%s)\n", host.Name, host.Address)
    }
}
```

### 4. Playbook Execution

```go
func runPlaybook() {
    ctx := context.Background()
    
    // Load playbook
    playbookContent := `
- hosts: all
  tasks:
    - name: Install nginx
      package:
        name: nginx
        state: present
        
    - name: Start nginx service
      service:
        name: nginx
        state: started
        enabled: true
        
    - name: Create web directory
      file:
        path: /var/www/html
        state: directory
        mode: '0755'
`
    
    pb, err := playbook.LoadFromYAML([]byte(playbookContent))
    if err != nil {
        log.Fatal(err)
    }
    
    // Create inventory
    inv := inventory.NewStaticInventory()
    // ... add hosts ...
    
    // Create runner and execute
    runner := runner.NewTaskRunner()
    results, err := runner.ExecutePlaybook(ctx, pb, inv)
    if err != nil {
        log.Fatal(err)
    }
    
    for _, result := range results {
        fmt.Printf("Task: %s, Success: %v\n", result.TaskName, result.Success)
    }
}
```

### 5. Template Engine

```go
func useTemplates() {
    // Create template engine
    engine := template.NewEngine()
    
    // Render template
    templateStr := `
server {
    listen {{.port}};
    server_name {{.domain}};
    
    {{if .ssl_enabled}}
    ssl on;
    ssl_certificate {{.ssl_cert}};
    ssl_certificate_key {{.ssl_key}};
    {{end}}
    
    location / {
        proxy_pass http://{{.backend_host}}:{{.backend_port}};
    }
}
`
    
    vars := map[string]interface{}{
        "port":         80,
        "domain":       "example.com", 
        "ssl_enabled":  false,
        "backend_host": "127.0.0.1",
        "backend_port": 8080,
    }
    
    result, err := engine.Render(templateStr, vars)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("Rendered template:")
    fmt.Println(result)
    
    // Render from file
    result, err = engine.RenderFile("/path/to/template.j2", vars)
    if err != nil {
        log.Fatal(err)
    }
}
```

### 6. Variable Management & Facts

```go
func useVariables() {
    ctx := context.Background()
    
    // Create variable manager
    varManager := vars.NewVarManager()
    
    // Set variables
    varManager.SetVar("app_name", "myapp")
    varManager.SetVar("app_version", "1.0.0")
    
    // Set multiple variables
    appVars := map[string]interface{}{
        "database_host": "db.example.com",
        "database_port": 5432,
        "debug_mode":    false,
    }
    varManager.SetVars(appVars)
    
    // Gather system facts
    conn := connection.NewLocalConnection()
    // ... connect ...
    
    facts, err := varManager.GatherFacts(ctx, conn)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("OS: %s, Architecture: %s\n", 
        facts["ansible_os_family"], facts["ansible_architecture"])
    
    // Get all variables (including facts)
    allVars := varManager.GetVars()
    fmt.Printf("Total variables: %d\n", len(allVars))
}
```

### 7. Vault Encryption

```go
func useVault() {
    // Create vault with password
    vault := vault.New("mypassword")
    
    // Encrypt sensitive data
    secretData := []byte("database_password: supersecret123")
    encrypted := vault.Encrypt(secretData)
    
    fmt.Println("Encrypted data:")
    fmt.Println(encrypted)
    
    // Decrypt
    decrypted := vault.Decrypt(encrypted)
    fmt.Printf("Decrypted: %s\n", string(decrypted))
    
    // Work with vault manager
    vaultManager := vault.NewManager()
    vaultManager.SetPassword("mypassword")
    
    // Process variables (decrypt vault strings)
    variables := map[string]interface{}{
        "db_host": "localhost",
        "db_pass": "$ANSIBLE_VAULT;1.1;AES256;...", // encrypted
    }
    
    processed, err := vaultManager.ProcessVariables(variables)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Processed variables: %+v\n", processed)
}
```

## 🏗️ Advanced Usage Patterns

### 8. Custom Module Development

```go
type MyCustomModule struct {
    modules.BaseModule
}

func NewMyCustomModule() *MyCustomModule {
    return &MyCustomModule{
        BaseModule: modules.BaseModule{
            name: "mycustom",
        },
    }
}

func (m *MyCustomModule) Run(ctx context.Context, conn common.Connection, args map[string]interface{}) (*common.Result, error) {
    // Your custom logic here
    result := &common.Result{
        Success: true,
        Changed: false,
        Message: "Custom module executed successfully",
        Data:    make(map[string]interface{}),
    }
    
    // Perform operations...
    
    return result, nil
}

func (m *MyCustomModule) Validate(args map[string]interface{}) error {
    // Validate arguments
    if _, ok := args["required_field"]; !ok {
        return fmt.Errorf("required_field is missing")
    }
    return nil
}

func useCustomModule() {
    // Register custom module
    registry := modules.NewModuleRegistry()
    registry.RegisterModule(NewMyCustomModule())
    
    // Use it
    module, _ := registry.GetModule("mycustom")
    // ... execute module ...
}
```

### 9. High-Level Task Library

```go
func useTaskLibrary() {
    ctx := context.Background()
    conn := connection.NewLocalConnection()
    // ... connect ...
    
    // File operations
    fileTasks := library.NewFileTasks()
    
    // Deploy configuration
    err := fileTasks.EnsureFile("/etc/app.conf", "config content", 0644)
    if err != nil {
        log.Fatal(err)
    }
    
    // Backup important files
    err = fileTasks.BackupFile("/etc/important.conf", "/backup/")
    if err != nil {
        log.Fatal(err)
    }
    
    // Service operations
    serviceTasks := library.NewServiceTasks()
    
    // Ensure service is running
    err = serviceTasks.EnsureServiceRunning("nginx")
    if err != nil {
        log.Fatal(err)
    }
    
    // Package operations
    packageTasks := library.NewPackageTasks()
    
    // Install dependencies
    packages := []string{"git", "build-essential", "curl"}
    err = packageTasks.InstallDependencies(packages)
    if err != nil {
        log.Fatal(err)
    }
}
```

### 10. Parallel Execution

```go
func runParallelTasks() {
    ctx := context.Background()
    
    // Create inventory with multiple hosts
    inv := inventory.NewStaticInventory()
    // ... add multiple hosts ...
    
    // Create task runner with concurrency
    runner := runner.NewTaskRunner()
    runner.SetMaxConcurrency(5) // Run on 5 hosts in parallel
    
    // Define tasks
    tasks := []runner.Task{
        {
            Name:   "Update packages",
            Module: "package",
            Args: map[string]interface{}{
                "name":         "nginx,vim,git",
                "state":        "latest", 
                "update_cache": true,
            },
        },
        {
            Name:   "Start nginx",
            Module: "service",
            Args: map[string]interface{}{
                "name":    "nginx",
                "state":   "started",
                "enabled": true,
            },
        },
    }
    
    // Execute tasks on all hosts
    results, err := runner.ExecuteTasks(ctx, tasks, inv)
    if err != nil {
        log.Fatal(err)
    }
    
    // Process results
    for host, hostResults := range results {
        fmt.Printf("Host: %s\n", host)
        for _, result := range hostResults {
            fmt.Printf("  Task: %s, Success: %v\n", 
                result.TaskName, result.Success)
        }
    }
}
```

## 📋 Available Modules

The library includes all essential Ansible-compatible modules:

- **file**: File/directory management
- **copy**: Copy files to remote hosts  
- **template**: Template rendering and deployment
- **service**: Service management (systemd, sysvinit, etc.)
- **package**: Package management (apt, yum, dnf, etc.)
- **user**: User account management
- **group**: Group management
- **command**: Execute commands
- **shell**: Execute shell commands with pipes/redirection
- **ping**: Test connectivity
- **setup**: Gather system facts
- **debug**: Debug output

Each module follows the same interface:
```go
result, err := module.Run(ctx, connection, args)
```

## 🔄 Error Handling

```go
func handleErrors() {
    result, err := module.Run(ctx, conn, args)
    if err != nil {
        // Handle connection/execution errors
        log.Printf("Execution error: %v", err)
        return
    }
    
    if !result.Success {
        // Handle module failure
        log.Printf("Module failed: %s", result.Error)
        if result.Error != nil {
            log.Printf("Detailed error: %v", result.Error)
        }
        return
    }
    
    // Success case
    fmt.Printf("Task completed successfully: %s\n", result.Message)
    if result.Changed {
        fmt.Println("System state was modified")
    }
}
```

This comprehensive API allows you to use gosinble as a powerful Go library for configuration management, automation, and infrastructure orchestration in your Go applications!

---
*Generated by Claude and saved via /save-as command*