<!-- 
Saved from Claude conversation
Date: 2025-09-06T00:00:00Z
Context: How to add new modules to gosinble
-->

# Adding New Modules to Gosinble

To add new modules to gosinble, you need to follow the established pattern. Let me create a todo to plan this out:

Based on my analysis, here's how to add new modules to gosinble:

## Steps to Add New Modules

### 1. **Add ModuleType Constant**
First, add your new module type to `/home/ubuntu/app/lib/gosinble/internal/common/types.go`:

```go
const (
    // Existing modules...
    TypeYourModule ModuleType = "yourmodule"
)
```

### 2. **Create Module Implementation**
Create a new file `/home/ubuntu/app/lib/gosinble/pkg/modules/yourmodule.go`:

```go
package modules

import (
    "context"
    "github.com/gosinble/gosinble/internal/common"
)

// YourModule implements your custom functionality
type YourModule struct {
    BaseModule
}

// NewYourModule creates a new instance
func NewYourModule() *YourModule {
    return &YourModule{
        BaseModule: BaseModule{
            name: "yourmodule",
        },
    }
}

// Run executes the module
func (m *YourModule) Run(ctx context.Context, conn common.Connection, args map[string]interface{}) (*common.Result, error) {
    // Your implementation here
    return m.CreateSuccessResult("localhost", false, "Module executed successfully", nil)
}

// Validate checks module arguments
func (m *YourModule) Validate(args map[string]interface{}) error {
    // Validation logic
    return nil
}

// Documentation returns module docs
func (m *YourModule) Documentation() common.ModuleDoc {
    return common.ModuleDoc{
        Name:        "yourmodule",
        Description: "Your module description",
        Parameters: map[string]common.ParamDoc{
            // Parameter documentation
        },
        Examples: []string{
            "- name: Example usage\n  yourmodule:\n    param: value",
        },
    }
}
```

### 3. **Register in Module Registry**
Add to `/home/ubuntu/app/lib/gosinble/pkg/modules/registry.go:108-144`:

```go
func (r *ModuleRegistry) registerBuiltinModules() {
    // Existing modules...
    r.RegisterModule(NewYourModule())
}
```

### 4. **Update AllModuleTypes Function**
Add to the `AllModuleTypes()` function in `/home/ubuntu/app/lib/gosinble/internal/common/types.go`:

```go
func AllModuleTypes() []ModuleType {
    return []ModuleType{
        // Existing types...
        TypeYourModule,
    }
}
```

## Complete Example: Adding a "Git" Module

Let me show you a complete example by creating a git module:

```go
// Example: Adding a Git Module to gosinble
// This file demonstrates how to add a new module following the established patterns

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/gosinble/gosinble/internal/common"
	"github.com/gosinble/gosinble/pkg/modules"
)

// Step 1: Add to internal/common/types.go
// const (
//     ...existing types...
//     TypeGit ModuleType = "git"
// )

// Step 2: Create the module implementation
type GitModule struct {
	modules.BaseModule
}

// NewGitModule creates a new git module instance
func NewGitModule() *GitModule {
	return &GitModule{
		BaseModule: modules.BaseModule{
			// This would normally be initialized properly
		},
	}
}

// Run executes the git module
func (m *GitModule) Run(ctx context.Context, conn common.Connection, args map[string]interface{}) (*common.Result, error) {
	// Get required parameters
	repo := m.GetStringArg(args, "repo", "")
	dest := m.GetStringArg(args, "dest", "")
	version := m.GetStringArg(args, "version", "HEAD")
	state := m.GetStringArg(args, "state", "present")
	
	// Validate required parameters
	if repo == "" {
		return m.CreateErrorResult("localhost", "repo parameter is required", 
			fmt.Errorf("git: repo parameter cannot be empty"))
	}
	
	if dest == "" {
		return m.CreateErrorResult("localhost", "dest parameter is required",
			fmt.Errorf("git: dest parameter cannot be empty"))
	}
	
	// Handle different states
	switch state {
	case "present":
		return m.cloneOrUpdate(ctx, conn, repo, dest, version)
	case "absent":
		return m.removeRepository(ctx, conn, dest)
	default:
		return m.CreateErrorResult("localhost", "invalid state",
			fmt.Errorf("git: state must be 'present' or 'absent'"))
	}
}

// cloneOrUpdate clones or updates a git repository
func (m *GitModule) cloneOrUpdate(ctx context.Context, conn common.Connection, repo, dest, version string) (*common.Result, error) {
	// Check if destination exists
	checkCmd := fmt.Sprintf("test -d %s/.git", dest)
	checkResult, err := conn.Execute(ctx, checkCmd, common.ExecuteOptions{})
	
	var cmd string
	var message string
	var changed bool
	
	if err != nil || !checkResult.Success {
		// Repository doesn't exist, clone it
		cmd = fmt.Sprintf("git clone %s %s", repo, dest)
		message = fmt.Sprintf("Repository %s cloned to %s", repo, dest)
		changed = true
	} else {
		// Repository exists, update it
		cmd = fmt.Sprintf("cd %s && git fetch origin && git checkout %s", dest, version)
		message = fmt.Sprintf("Repository updated to %s", version)
		changed = true // Could be false if already up to date
	}
	
	result, err := conn.Execute(ctx, cmd, common.ExecuteOptions{})
	if err != nil {
		return m.CreateErrorResult("localhost", "Git operation failed", err)
	}
	
	if !result.Success {
		return m.CreateFailureResult("localhost", "Git command failed", 
			fmt.Errorf("git: %s", result.Stderr), nil)
	}
	
	return m.CreateSuccessResult("localhost", changed, message, map[string]interface{}{
		"repo":    repo,
		"dest":    dest,
		"version": version,
		"before":  "",
		"after":   version,
	})
}

// removeRepository removes a git repository
func (m *GitModule) removeRepository(ctx context.Context, conn common.Connection, dest string) (*common.Result, error) {
	// Check if destination exists
	checkCmd := fmt.Sprintf("test -d %s", dest)
	checkResult, err := conn.Execute(ctx, checkCmd, common.ExecuteOptions{})
	
	if err != nil || !checkResult.Success {
		// Directory doesn't exist, nothing to do
		return m.CreateSuccessResult("localhost", false, "Repository already absent", map[string]interface{}{
			"dest": dest,
		})
	}
	
	// Remove the directory
	cmd := fmt.Sprintf("rm -rf %s", dest)
	result, err := conn.Execute(ctx, cmd, common.ExecuteOptions{})
	if err != nil {
		return m.CreateErrorResult("localhost", "Failed to remove repository", err)
	}
	
	if !result.Success {
		return m.CreateFailureResult("localhost", "Remove command failed",
			fmt.Errorf("git: %s", result.Stderr), nil)
	}
	
	return m.CreateSuccessResult("localhost", true, fmt.Sprintf("Repository removed from %s", dest), map[string]interface{}{
		"dest": dest,
	})
}

// Validate checks if the module arguments are valid
func (m *GitModule) Validate(args map[string]interface{}) error {
	// Required fields
	required := []string{"repo", "dest"}
	if err := m.ValidateRequired(args, required); err != nil {
		return err
	}
	
	// Validate state choices
	if err := m.ValidateChoices(args, "state", []string{"present", "absent"}); err != nil {
		return err
	}
	
	// Validate repository URL format (basic check)
	repo := m.GetStringArg(args, "repo", "")
	if repo != "" && !strings.Contains(repo, "://") && !strings.HasPrefix(repo, "git@") {
		return fmt.Errorf("git: invalid repository URL format")
	}
	
	return nil
}

// Documentation returns the module documentation
func (m *GitModule) Documentation() common.ModuleDoc {
	return common.ModuleDoc{
		Name:        "git",
		Description: "Manage git repositories",
		Parameters: map[string]common.ParamDoc{
			"repo": {
				Description: "Git repository URL",
				Required:    true,
				Type:        "string",
			},
			"dest": {
				Description: "Destination directory path",
				Required:    true,
				Type:        "string",
			},
			"version": {
				Description: "Git branch, tag, or commit to checkout",
				Required:    false,
				Type:        "string",
				Default:     "HEAD",
			},
			"state": {
				Description: "Whether the repository should be present or absent",
				Required:    false,
				Type:        "string",
				Default:     "present",
				Choices:     []string{"present", "absent"},
			},
		},
		Examples: []string{
			"- name: Clone repository\n  git:\n    repo: https://github.com/user/repo.git\n    dest: /opt/myapp",
			"- name: Clone specific branch\n  git:\n    repo: git@github.com:user/repo.git\n    dest: /opt/myapp\n    version: develop",
			"- name: Remove repository\n  git:\n    dest: /opt/myapp\n    state: absent",
		},
		Returns: map[string]string{
			"repo":    "Repository URL",
			"dest":    "Destination directory",
			"version": "Checked out version",
			"before":  "Previous version (if updated)",
			"after":   "New version",
		},
	}
}
```

## Summary: Adding New Modules to Gosinble

To add a new module, follow these **5 steps**:

1. **Add ModuleType constant** in `internal/common/types.go`
2. **Create module implementation** in `pkg/modules/yourmodule.go` 
3. **Register in module registry** in `pkg/modules/registry.go`
4. **Update AllModuleTypes()** function
5. **Create unit tests** in `pkg/modules/yourmodule_test.go`

### Key Requirements:
- **Embed BaseModule** for common functionality
- **Implement 3 methods**: `Run()`, `Validate()`, `Documentation()`
- **Use type-safe constants** for states and module types
- **Follow naming conventions** (`NewXxxModule()` constructor)
- **Handle errors properly** with `CreateErrorResult()`, `CreateSuccessResult()`

The example git module shows a complete implementation following all gosinble patterns and conventions.

---
*Generated by Claude and saved via /save-as command*