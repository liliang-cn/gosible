package main

import (
	"embed"
	"fmt"
	"log"
	
	"github.com/liliang-cn/gosinble/pkg/library"
	"github.com/liliang-cn/gosinble/pkg/types"
)

// Embed configuration files and scripts
//go:embed configs/* scripts/* templates/*
var contentFS embed.FS

// Example with specific files
//go:embed configs/nginx.conf configs/app.yml
var configFiles embed.FS

// Example with entire directory
//go:embed scripts
var scriptsDir embed.FS

func main() {
	fmt.Println("=== Embedded Content Distribution Examples ===")
	
	// Example 1: Using ContentTasks for programmatic content
	programmaticContentExample()
	
	// Example 2: Using EmbeddedContent with embed.FS
	embeddedContentExample()
	
	// Example 3: Deploying specific configurations
	deployConfigurationExample()
	
	// Example 4: Syncing directories
	syncDirectoryExample()
}

func programmaticContentExample() {
	fmt.Println("Example 1: Programmatic Content Distribution")
	fmt.Println("--------------------------------------------")
	
	ct := library.NewContentTasks()
	
	// Register a configuration file
	nginxConfig := `server {
    listen 80;
    server_name example.com;
    
    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}`
	ct.AddFile("nginx.conf", []byte(nginxConfig), "0644", "root", "root")
	
	// Register a shell script
	deployScript := `#!/bin/bash
set -e

echo "Starting deployment..."
git pull origin main
npm install
npm run build
pm2 restart app
echo "Deployment complete!"
`
	ct.AddFile("deploy.sh", []byte(deployScript), "0755", "deploy", "deploy")
	
	// Register a systemd service file
	serviceFile := `[Unit]
Description=My Application
After=network.target

[Service]
Type=simple
User=app
WorkingDirectory=/opt/app
ExecStart=/usr/bin/node /opt/app/server.js
Restart=on-failure

[Install]
WantedBy=multi-user.target
`
	ct.AddFile("myapp.service", []byte(serviceFile), "0644", "root", "root")
	
	// Create a directory structure with multiple files
	ct.AddDirectory("configs", "0755", "root", "root")
	ct.AddFileToDirectory("configs", "database.yml", []byte("database:\n  host: localhost\n  port: 5432\n"), "0644")
	ct.AddFileToDirectory("configs", "redis.yml", []byte("redis:\n  host: localhost\n  port: 6379\n"), "0644")
	
	// Deploy a single file
	tasks := ct.DeployFile("nginx.conf", "/etc/nginx/sites-available/myapp")
	fmt.Printf("Deploy nginx.conf: %d tasks\n", len(tasks))
	
	// Deploy a directory
	tasks = ct.DeployDirectory("configs", "/etc/myapp")
	fmt.Printf("Deploy configs directory: %d tasks\n", len(tasks))
	
	// Deploy multiple items at once
	deployments := map[string]string{
		"deploy.sh":      "/usr/local/bin/deploy.sh",
		"myapp.service":  "/etc/systemd/system/myapp.service",
		"configs":        "/etc/myapp",
	}
	tasks = ct.BulkDeploy(deployments)
	fmt.Printf("Bulk deployment: %d tasks\n", len(tasks))
	
	// List registered content
	fmt.Printf("Registered files: %v\n", ct.ListFiles())
	fmt.Printf("Registered directories: %v\n", ct.ListDirectories())
	
	fmt.Println()
}

func embeddedContentExample() {
	fmt.Println("Example 2: Embedded Content with embed.FS")
	fmt.Println("-----------------------------------------")
	
	// This would work if we had actual embedded files
	// For demonstration, showing the pattern
	
	// Create embedded content manager
	ec := library.NewEmbeddedContent(contentFS, "configs")
	
	// Load all content
	if err := ec.LoadAll(); err != nil {
		log.Printf("Failed to load embedded content: %v\n", err)
	}
	
	// Deploy a single file
	tasks := ec.DeployFile("nginx.conf", "/etc/nginx/nginx.conf", "root", "root", "0644")
	fmt.Printf("Deploy embedded file: %d tasks\n", len(tasks))
	
	// Deploy an entire directory
	tasks = ec.DeployDirectory("scripts", "/opt/scripts", "root", "root", "0755", "0755")
	fmt.Printf("Deploy embedded directory: %d tasks\n", len(tasks))
	
	// Deploy and render a template
	vars := map[string]interface{}{
		"app_name": "myapp",
		"app_port": 3000,
		"app_user": "deploy",
	}
	tasks = ec.DeployTemplate("templates/app.conf.j2", "/etc/myapp/app.conf", vars, "root", "root", "0644")
	fmt.Printf("Deploy template: %d tasks\n", len(tasks))
	
	// List embedded files
	files, err := ec.ListFiles()
	if err == nil {
		fmt.Printf("Embedded files: %v\n", files)
	}
	
	// Check if file exists
	if ec.FileExists("nginx.conf") {
		fmt.Println("nginx.conf exists in embedded content")
	}
	
	fmt.Println()
}

func deployConfigurationExample() {
	fmt.Println("Example 3: Configuration Deployment Pattern")
	fmt.Println("-------------------------------------------")
	
	ct := library.NewContentTasks()
	
	// Register application configurations
	appConfigs := map[string]string{
		"production.yml": `environment: production
database:
  host: db.prod.example.com
  pool: 20
  timeout: 30
logging:
  level: info
  output: /var/log/app/production.log
`,
		"staging.yml": `environment: staging
database:
  host: db.staging.example.com
  pool: 10
  timeout: 15
logging:
  level: debug
  output: /var/log/app/staging.log
`,
	}
	
	// Register all configs
	for name, content := range appConfigs {
		ct.AddFile(name, []byte(content), "0644", "app", "app")
	}
	
	// Deploy based on environment
	environment := "production" // This would come from variables
	configFile := fmt.Sprintf("%s.yml", environment)
	
	tasks := []types.Task{}
	
	// Backup existing config
	tasks = append(tasks, types.Task{
		Name:   "Backup existing configuration",
		Module: "copy",
		Args: map[string]interface{}{
			"src":  "/etc/app/config.yml",
			"dest": "/etc/app/config.yml.bak",
			"remote_src": true,
		},
		IgnoreErrors: true,
	})
	
	// Deploy new config
	tasks = append(tasks, ct.DeployFile(configFile, "/etc/app/config.yml")...)
	
	// Validate configuration
	tasks = append(tasks, types.Task{
		Name:   "Validate configuration",
		Module: "command",
		Args: map[string]interface{}{
			"cmd": "/usr/bin/app --config-test /etc/app/config.yml",
		},
	})
	
	// Restart service if config is valid
	tasks = append(tasks, types.Task{
		Name:   "Restart application",
		Module: "systemd",
		Args: map[string]interface{}{
			"name":  "myapp",
			"state": "restarted",
		},
	})
	
	fmt.Printf("Configuration deployment: %d tasks\n", len(tasks))
	
	// Show task names
	for _, task := range tasks {
		fmt.Printf("  - %s\n", task.Name)
	}
	
	fmt.Println()
}

func syncDirectoryExample() {
	fmt.Println("Example 4: Directory Synchronization")
	fmt.Println("------------------------------------")
	
	ec := library.NewEmbeddedContent(scriptsDir, "scripts")
	
	// Sync embedded scripts directory with target
	// This will:
	// 1. Deploy all files from embedded content
	// 2. Remove any files in target that don't exist in embedded content
	tasks := ec.SyncDirectory("", "/opt/scripts", "root", "root")
	
	fmt.Printf("Directory sync: %d tasks\n", len(tasks))
	
	// Show what the sync will do
	for i, task := range tasks {
		if i < 5 || i >= len(tasks)-2 { // Show first 5 and last 2 tasks
			fmt.Printf("  - %s\n", task.Name)
		} else if i == 5 {
			fmt.Printf("  ... (%d more tasks) ...\n", len(tasks)-7)
		}
	}
	
	fmt.Println()
}

// Example of a builder pattern for content distribution
type ContentDistribution struct {
	tasks []types.Task
	ct    *library.ContentTasks
}

func NewContentDistribution() *ContentDistribution {
	return &ContentDistribution{
		tasks: []types.Task{},
		ct:    library.NewContentTasks(),
	}
}

func (cd *ContentDistribution) WithConfig(name string, content []byte) *ContentDistribution {
	cd.ct.AddFile(name, content, "0644", "root", "root")
	return cd
}

func (cd *ContentDistribution) WithScript(name string, content []byte) *ContentDistribution {
	cd.ct.AddFile(name, content, "0755", "root", "root")
	return cd
}

func (cd *ContentDistribution) WithTemplate(name string, content []byte) *ContentDistribution {
	cd.ct.AddFile(name, content, "0644", "root", "root")
	return cd
}

func (cd *ContentDistribution) Deploy(deployments map[string]string) *ContentDistribution {
	cd.tasks = append(cd.tasks, cd.ct.BulkDeploy(deployments)...)
	return cd
}

func (cd *ContentDistribution) Build() []types.Task {
	return cd.tasks
}

// Usage example of the builder
func builderExample() {
	dist := NewContentDistribution().
		WithConfig("app.yml", []byte("app: config")).
		WithScript("deploy.sh", []byte("#!/bin/bash\necho 'Deploying...'")).
		WithTemplate("nginx.conf.j2", []byte("server_name {{ domain }};")).
		Deploy(map[string]string{
			"app.yml":        "/etc/app/config.yml",
			"deploy.sh":      "/usr/local/bin/deploy",
			"nginx.conf.j2":  "/tmp/nginx.conf.j2",
		})
	
	tasks := dist.Build()
	fmt.Printf("Built %d distribution tasks\n", len(tasks))
}