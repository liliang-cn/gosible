# OS-Specific Package Manager Modules

## Overview

gosible now includes OS-specific package manager modules for local environment automation. These modules provide native support for package management across different operating systems.

## Implemented Modules

### 1. Homebrew Module (macOS)
- **Module Name**: `homebrew`
- **Manages**: Packages and cask applications on macOS
- **Key Features**:
  - Install/remove packages and casks
  - Update Homebrew
  - Upgrade all packages
  - Link/unlink packages

### 2. APT Module (Debian/Ubuntu)
- **Module Name**: `apt`
- **Manages**: Packages on Debian-based systems
- **Key Features**:
  - Install/remove packages
  - Update cache
  - System upgrades (dist, full, safe)
  - Build dependencies
  - Autoremove and autoclean

### 3. YUM Module (RHEL/CentOS)
- **Module Name**: `yum`
- **Manages**: Packages on RHEL/CentOS systems
- **Key Features**:
  - Install/remove packages
  - Security updates
  - Repository management
  - Cache updates
  - Autoremove

### 4. DNF Module (Fedora/RHEL8+)
- **Module Name**: `dnf`
- **Manages**: Packages on modern Fedora and RHEL8+ systems
- **Key Features**:
  - Install/remove packages
  - Security updates
  - Advanced dependency resolution (allowerasing, nobest)
  - Repository management
  - Autoremove

## Usage Examples

### Homebrew (macOS)
```go
// Install a package
result, _ := homebrewModule.Run(ctx, conn, map[string]interface{}{
    "name": "wget",
    "state": "present",
})

// Install a cask application
result, _ := homebrewModule.Run(ctx, conn, map[string]interface{}{
    "name": "docker",
    "state": "present",
    "cask": true,
})

// Update Homebrew and upgrade all packages
result, _ := homebrewModule.Run(ctx, conn, map[string]interface{}{
    "update_homebrew": true,
    "upgrade_all": true,
})
```

### APT (Debian/Ubuntu)
```go
// Install packages
result, _ := aptModule.Run(ctx, conn, map[string]interface{}{
    "names": []interface{}{"nginx", "postgresql"},
    "state": "present",
    "update_cache": true,
})

// System upgrade
result, _ := aptModule.Run(ctx, conn, map[string]interface{}{
    "upgrade": "dist",
})
```

### YUM (RHEL/CentOS)
```go
// Install with specific repo
result, _ := yumModule.Run(ctx, conn, map[string]interface{}{
    "name": "httpd",
    "state": "present",
    "enablerepo": "epel",
})

// Security updates only
result, _ := yumModule.Run(ctx, conn, map[string]interface{}{
    "security": true,
})
```

### DNF (Fedora/RHEL8+)
```go
// Install with dependency resolution
result, _ := dnfModule.Run(ctx, conn, map[string]interface{}{
    "name": "package",
    "state": "present",
    "allowerasing": true,
    "nobest": true,
})
```

## Common Parameters

All package manager modules support these common parameters:
- `name`: Single package name
- `names`: List of package names
- `state`: Package state (present, absent, latest)
- `update_cache`: Update package cache
- `autoremove`: Remove unnecessary packages

## Testing

All modules include comprehensive unit tests:
```bash
go test ./pkg/modules/ -run "Test.*ModuleValidation"
```

## Type Safety

The modules are registered with type-safe constants:
```go
types.TypeHomebrew
types.TypeApt
types.TypeYum
types.TypeDnf
```

## Integration

The modules are automatically registered in the module registry and can be used through the standard gosible runner:

```go
runner := runner.NewTaskRunner()
task := types.Task{
    Name:   "Install nginx",
    Module: types.TypeApt,
    Args: map[string]interface{}{
        "name": "nginx",
        "state": "present",
    },
}
results, _ := runner.Run(ctx, task, hosts, nil)
```