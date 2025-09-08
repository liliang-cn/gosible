package library

import (
	"github.com/liliang-cn/gosinble/pkg/types"
)

// DevelopmentTasks provides common development environment operations
type DevelopmentTasks struct{}

// NewDevelopmentTasks creates a new DevelopmentTasks instance
func NewDevelopmentTasks() *DevelopmentTasks {
	return &DevelopmentTasks{}
}

// InstallPythonPackage creates tasks to install a Python package via pip
func (dt *DevelopmentTasks) InstallPythonPackage(name, version, virtualenv string) []types.Task {
	args := map[string]interface{}{
		"name":  name,
		"state": "present",
	}
	
	if version != "" {
		args["version"] = version
	}
	
	if virtualenv != "" {
		args["virtualenv"] = virtualenv
	}
	
	return []types.Task{
		{
			Name:   "Install Python package",
			Module: "pip",
			Args:   args,
		},
	}
}

// InstallPythonRequirements creates tasks to install Python requirements from file
func (dt *DevelopmentTasks) InstallPythonRequirements(requirementsFile, virtualenv string) []types.Task {
	args := map[string]interface{}{
		"requirements": requirementsFile,
		"state":        "present",
	}
	
	if virtualenv != "" {
		args["virtualenv"] = virtualenv
	}
	
	return []types.Task{
		{
			Name:   "Install Python requirements",
			Module: "pip",
			Args:   args,
		},
	}
}

// SetupPythonVirtualenv creates tasks to setup a Python virtual environment
func (dt *DevelopmentTasks) SetupPythonVirtualenv(path string, packages []string) []types.Task {
	tasks := []types.Task{
		{
			Name:   "Create virtual environment",
			Module: "command",
			Args: map[string]interface{}{
				"cmd":     "python3 -m venv " + path,
				"creates": path,
			},
		},
	}
	
	// Install packages in the virtualenv
	for _, pkg := range packages {
		tasks = append(tasks, types.Task{
			Name:   "Install " + pkg + " in virtualenv",
			Module: "pip",
			Args: map[string]interface{}{
				"name":       pkg,
				"virtualenv": path,
				"state":      "present",
			},
		})
	}
	
	return tasks
}

// InstallNodePackage creates tasks to install a Node.js package via npm
func (dt *DevelopmentTasks) InstallNodePackage(name, version string, global bool) []types.Task {
	args := map[string]interface{}{
		"name":   name,
		"state":  "present",
		"global": global,
	}
	
	if version != "" {
		args["version"] = version
	}
	
	return []types.Task{
		{
			Name:   "Install Node.js package",
			Module: "npm",
			Args:   args,
		},
	}
}

// InstallNodeDependencies creates tasks to install Node.js dependencies
func (dt *DevelopmentTasks) InstallNodeDependencies(path string, production bool) []types.Task {
	args := map[string]interface{}{
		"path":       path,
		"state":      "present",
		"production": production,
	}
	
	return []types.Task{
		{
			Name:   "Install Node.js dependencies",
			Module: "npm",
			Args:   args,
		},
	}
}

// InstallGlobalNodeTools creates tasks to install common Node.js development tools globally
func (dt *DevelopmentTasks) InstallGlobalNodeTools() []types.Task {
	tools := []string{"typescript", "eslint", "prettier", "nodemon", "pm2"}
	tasks := make([]types.Task, len(tools))
	
	for i, tool := range tools {
		tasks[i] = types.Task{
			Name:   "Install " + tool + " globally",
			Module: "npm",
			Args: map[string]interface{}{
				"name":   tool,
				"global": true,
				"state":  "present",
			},
		}
	}
	
	return tasks
}

// InstallRubyGem creates tasks to install a Ruby gem
func (dt *DevelopmentTasks) InstallRubyGem(name, version string, userInstall bool) []types.Task {
	args := map[string]interface{}{
		"name":         name,
		"state":        "present",
		"user_install": userInstall,
	}
	
	if version != "" {
		args["version"] = version
	}
	
	return []types.Task{
		{
			Name:   "Install Ruby gem",
			Module: "gem",
			Args:   args,
		},
	}
}

// InstallBundler creates tasks to install and use Bundler for Ruby projects
func (dt *DevelopmentTasks) InstallBundler(path string) []types.Task {
	return []types.Task{
		{
			Name:   "Install bundler gem",
			Module: "gem",
			Args: map[string]interface{}{
				"name":  "bundler",
				"state": "present",
			},
		},
		{
			Name:   "Run bundle install",
			Module: "command",
			Args: map[string]interface{}{
				"cmd": "bundle install",
				"chdir": path,
			},
		},
	}
}

// SetupDevelopmentEnvironment creates tasks to setup a complete development environment
func (dt *DevelopmentTasks) SetupDevelopmentEnvironment(languages []string) []types.Task {
	tasks := []types.Task{}
	
	for _, lang := range languages {
		switch lang {
		case "python":
			tasks = append(tasks, types.Task{
				Name:   "Install Python development packages",
				Module: "package",
				Args: map[string]interface{}{
					"name": []string{"python3", "python3-pip", "python3-venv", "python3-dev"},
					"state": "present",
				},
			})
			tasks = append(tasks, types.Task{
				Name:   "Install common Python tools",
				Module: "pip",
				Args: map[string]interface{}{
					"name": []string{"virtualenv", "pipenv", "black", "flake8", "pytest"},
					"state": "present",
				},
			})
			
		case "node", "nodejs":
			tasks = append(tasks, types.Task{
				Name:   "Install Node.js",
				Module: "package",
				Args: map[string]interface{}{
					"name": []string{"nodejs", "npm"},
					"state": "present",
				},
			})
			tasks = append(tasks, dt.InstallGlobalNodeTools()...)
			
		case "ruby":
			tasks = append(tasks, types.Task{
				Name:   "Install Ruby",
				Module: "package",
				Args: map[string]interface{}{
					"name": []string{"ruby", "ruby-dev"},
					"state": "present",
				},
			})
			tasks = append(tasks, types.Task{
				Name:   "Install common Ruby gems",
				Module: "gem",
				Args: map[string]interface{}{
					"name": []string{"bundler", "rake", "rspec", "rubocop"},
					"state": "present",
				},
			})
			
		case "go", "golang":
			tasks = append(tasks, types.Task{
				Name:   "Install Go",
				Module: "package",
				Args: map[string]interface{}{
					"name": "golang",
					"state": "present",
				},
			})
		}
	}
	
	return tasks
}