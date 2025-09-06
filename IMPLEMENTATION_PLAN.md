# Gosinble Implementation Plan - Ansible Feature Parity

## Overview
This document outlines the implementation plan for adding critical Ansible-compatible features to Gosinble, prioritized by user need and implementation complexity.

## Phase 1: Core Execution Modes (Week 1-2)
Essential features that affect how all modules execute.

### 1.1 Check Mode (`--check`)
**Priority**: CRITICAL - Users need dry-run capability
**Effort**: Medium (3-4 days)
**Implementation**:
```go
// Add to types.ExecuteOptions
type ExecuteOptions struct {
    CheckMode bool  // Don't make actual changes
    DiffMode  bool  // Show what would change
    // ...
}

// Each module checks:
func (m *SystemdModule) Run(ctx context.Context, conn Connection, args map[string]interface{}) (*Result, error) {
    if m.CheckMode {
        return m.simulateChanges(args)
    }
    // ... actual execution
}
```

**Files to modify**:
- `pkg/types/types.go` - Add CheckMode to Task and ExecuteOptions
- `pkg/modules/base.go` - Add CheckMode support to BaseModule
- `pkg/runner/runner.go` - Pass check mode through execution
- `cmd/gosinble/main.go` - Add --check flag

### 1.2 Diff Mode (`--diff`)
**Priority**: HIGH - Shows actual changes
**Effort**: Medium (2-3 days)
**Implementation**:
```go
type DiffResult struct {
    Before string `json:"before"`
    After  string `json:"after"`
}

type Result struct {
    // ... existing fields
    Diff *DiffResult `json:"diff,omitempty"`
}
```

**Files to modify**:
- `pkg/types/types.go` - Add DiffResult type
- File manipulation modules to capture before/after state
- `pkg/runner/runner.go` - Format diff output

## Phase 2: Critical System Modules (Week 2-3)
Most commonly needed modules for system administration.

### 2.1 Systemd Module
**Priority**: CRITICAL - Most modern systems use systemd
**Effort**: Medium (2 days)
**File**: `pkg/modules/systemd.go`
```go
type SystemdModule struct {
    BaseModule
}

// Actions: start, stop, restart, reload, enable, disable, mask, unmask
// States: started, stopped, restarted, reloaded
// Parameters:
// - name: service name (required)
// - state: desired state
// - enabled: bool for boot startup
// - daemon_reload: bool to reload systemd
// - masked: bool to mask/unmask
```

### 2.2 Cron Module  
**Priority**: HIGH - Task scheduling is essential
**Effort**: Medium (2 days)
**File**: `pkg/modules/cron.go`
```go
type CronModule struct {
    BaseModule
}

// Parameters:
// - name: description of the job
// - minute/hour/day/month/weekday: schedule
// - job: command to execute
// - user: user to run as
// - state: present/absent
// - special_time: @reboot, @daily, etc.
```

### 2.3 Mount Module
**Priority**: MEDIUM - Filesystem management
**Effort**: Medium (2 days)
**File**: `pkg/modules/mount.go`
```go
type MountModule struct {
    BaseModule
}

// Parameters:
// - path: mount point (required)
// - src: device/NFS share to mount
// - fstype: filesystem type
// - opts: mount options
// - state: mounted/unmounted/present/absent
// - dump: dump flag for fstab
// - passno: pass number for fsck
```

### 2.4 Sysctl Module
**Priority**: MEDIUM - Kernel tuning
**Effort**: Low (1 day)
**File**: `pkg/modules/sysctl.go`
```go
type SysctlModule struct {
    BaseModule
}

// Parameters:
// - name: sysctl key (required)
// - value: desired value
// - state: present/absent
// - reload: reload sysctl
// - sysctl_file: target file (default /etc/sysctl.conf)
```

### 2.5 Firewall/IPTables Module
**Priority**: HIGH - Security critical
**Effort**: High (3-4 days)
**File**: `pkg/modules/iptables.go`
```go
type IPTablesModule struct {
    BaseModule
}

// Parameters:
// - chain: INPUT/OUTPUT/FORWARD
// - protocol: tcp/udp/icmp
// - source/destination: IP addresses
// - jump: ACCEPT/DROP/REJECT
// - dport/sport: ports
// - state: present/absent
// - table: filter/nat/mangle
```

## Phase 3: File Manipulation Modules (Week 3-4)
Critical for configuration management.

### 3.1 LineInFile Module
**Priority**: CRITICAL - Most used for config editing
**Effort**: High (3 days)
**File**: `pkg/modules/lineinfile.go`
```go
type LineInFileModule struct {
    BaseModule
}

// Parameters:
// - path: file path (required)
// - line: line to ensure exists
// - regexp: pattern to search for
// - state: present/absent
// - insertafter/insertbefore: placement
// - backup: create backup
// - create: create if missing
```

### 3.2 Replace Module
**Priority**: HIGH - Pattern replacement
**Effort**: Medium (2 days)
**File**: `pkg/modules/replace.go`
```go
type ReplaceModule struct {
    BaseModule
}

// Parameters:
// - path: file path (required)
// - regexp: pattern to find (required)
// - replace: replacement string
// - backup: create backup
// - validate: validation command
```

### 3.3 BlockInFile Module
**Priority**: HIGH - Manage text blocks
**Effort**: Medium (2 days)
**File**: `pkg/modules/blockinfile.go`
```go
type BlockInFileModule struct {
    BaseModule
}

// Parameters:
// - path: file path (required)
// - block: content to insert
// - marker: marker pattern
// - insertafter/insertbefore: placement
// - state: present/absent
// - backup: create backup
```

### 3.4 INI File Module
**Priority**: MEDIUM - Common config format
**Effort**: Medium (2 days)
**File**: `pkg/modules/ini_file.go`
```go
type IniFileModule struct {
    BaseModule
}

// Parameters:
// - path: file path (required)
// - section: INI section (required)
// - option: option name
// - value: option value
// - state: present/absent
// - backup: create backup
```

### 3.5 XML Module
**Priority**: LOW - Less common
**Effort**: High (3 days)
**File**: `pkg/modules/xml.go`
```go
type XMLModule struct {
    BaseModule
}

// Parameters:
// - path: file path (required)
// - xpath: XPath expression (required)
// - value: new value
// - attribute: attribute to modify
// - state: present/absent
```

## Phase 4: Package Management Extensions (Week 4-5)

### 4.1 APT Repository Module
**Priority**: HIGH - Ubuntu/Debian systems
**Effort**: Medium (2 days)
**File**: `pkg/modules/apt_repository.go`
```go
type AptRepositoryModule struct {
    BaseModule
}

// Parameters:
// - repo: repository line (required)
// - state: present/absent
// - update_cache: run apt update
// - filename: list file name
```

### 4.2 YUM Repository Module
**Priority**: HIGH - RHEL/CentOS systems
**Effort**: Medium (2 days)
**File**: `pkg/modules/yum_repository.go`
```go
type YumRepositoryModule struct {
    BaseModule
}

// Parameters:
// - name: repo name (required)
// - description: repo description
// - baseurl: repository URL
// - gpgcheck: enable GPG checking
// - enabled: enable repository
// - state: present/absent
```

### 4.3 Language-Specific Package Managers
**Priority**: MEDIUM
**Effort**: Low each (1 day each)

#### PIP Module
**File**: `pkg/modules/pip.go`
```go
// Parameters:
// - name: package name(s)
// - version: specific version
// - requirements: requirements file
// - virtualenv: virtual environment path
// - state: present/absent/latest
```

#### NPM Module
**File**: `pkg/modules/npm.go`
```go
// Parameters:
// - name: package name(s)
// - version: specific version
// - global: install globally
// - path: local install path
// - state: present/absent/latest
```

#### Snap Module
**File**: `pkg/modules/snap.go`
```go
// Parameters:
// - name: snap name(s)
// - channel: stable/edge/beta
// - classic: use classic confinement
// - state: present/absent
```

## Phase 5: Advanced Playbook Features (Week 5-6)

### 5.1 Roles Structure
**Priority**: HIGH - Code organization
**Effort**: High (4-5 days)
**Implementation**:
```
roles/
  webserver/
    tasks/main.yml
    handlers/main.yml
    templates/
    files/
    vars/main.yml
    defaults/main.yml
    meta/main.yml
```

**New Types**:
```go
type Role struct {
    Name     string
    Tasks    []Task
    Handlers []Task
    Vars     map[string]interface{}
    Files    embed.FS
    Templates embed.FS
}
```

### 5.2 Include/Import Tasks
**Priority**: HIGH - Modularity
**Effort**: Medium (3 days)
**Implementation**:
```go
type IncludeTask struct {
    File string
    Vars map[string]interface{}
    Tags []string
}

// Dynamic inclusion at runtime
func (r *TaskRunner) IncludeTasks(file string, vars map[string]interface{}) ([]Task, error)
```

### 5.3 Async/Until Improvements
**Priority**: MEDIUM - Long-running tasks
**Effort**: Medium (3 days)
**Implementation**:
```go
type AsyncTask struct {
    Task
    Async   int  // Max runtime in seconds
    Poll    int  // Check interval
    Until   string // Condition to retry
    Retries int
    Delay   int
}
```

### 5.4 More Template Filters
**Priority**: HIGH - Text processing
**Effort**: Medium (3 days)
**Add filters**:
- `to_json`, `from_json`, `to_yaml`, `from_yaml`
- `regex_replace`, `regex_search`, `regex_findall`
- `b64encode`, `b64decode`
- `hash('sha256')`, `hash('md5')`
- `combine`, `union`, `difference`, `intersect` (for dicts/lists)
- `select`, `reject`, `selectattr`, `rejectattr`
- `default`, `mandatory`
- `basename`, `dirname`, `expanduser`

## Phase 6: Testing & Documentation (Ongoing)

### 6.1 Module Testing Framework
**Priority**: CRITICAL
**Effort**: High (1 week)
```go
type ModuleTestCase struct {
    Name     string
    Module   Module
    Args     map[string]interface{}
    Expected Result
    CheckMode bool
}

func TestModule(t *testing.T, cases []ModuleTestCase)
```

### 6.2 Integration Tests
- Test each module against real systems (Docker containers)
- Test check mode for all modules
- Test diff mode output
- Test role loading and execution

### 6.3 Documentation
- Module documentation generator
- Parameter documentation
- Example playbooks for each module
- Migration guide from Ansible

## Implementation Priority Order

### Sprint 1 (Weeks 1-2): Foundation
1. Check Mode support ✅
2. Diff Mode support ✅
3. Systemd module ✅
4. LineInFile module ✅

### Sprint 2 (Weeks 3-4): Core Modules
1. Cron module ✅
2. Replace module ✅
3. BlockInFile module ✅
4. APT/YUM repository modules ✅

### Sprint 3 (Weeks 5-6): Advanced Features
1. Roles structure ✅
2. Include/Import tasks ✅
3. Template filters ✅
4. Mount & Sysctl modules ✅

### Sprint 4 (Weeks 7-8): Completeness
1. IPTables module ✅
2. Language package managers ✅
3. INI/XML modules ✅
4. Async/Until improvements ✅

## Success Metrics
- [ ] All critical modules implemented and tested
- [ ] Check/Diff modes work with all modules
- [ ] Roles structure supports Ansible-compatible layouts
- [ ] 90% of common Ansible playbooks can be migrated
- [ ] Performance remains 10x+ faster than Ansible
- [ ] Comprehensive test coverage (>80%)

## Migration Strategy
1. Provide ansible2gosinble converter tool
2. Support subset of Ansible YAML syntax
3. Document differences and limitations
4. Provide migration examples for common patterns

## Long-term Vision
- Module marketplace/registry
- Web UI for playbook management
- Terraform provider for Gosinble
- Kubernetes operator using Gosinble library
- VS Code extension with IntelliSense