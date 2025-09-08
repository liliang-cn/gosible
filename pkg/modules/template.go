package modules

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"
	
	"github.com/liliang-cn/gosinble/pkg/types"
)

// TemplateModule templates files to remote hosts
type TemplateModule struct {
	BaseModule
}

// NewTemplateModule creates a new template module instance
func NewTemplateModule() *TemplateModule {
	return &TemplateModule{
		BaseModule: BaseModule{
			name: "template",
		},
	}
}

// Run executes the template module
func (m *TemplateModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Get arguments
	src, _ := args["src"].(string)
	dest, _ := args["dest"].(string)
	backup, _ := args["backup"].(bool)
	mode, _ := args["mode"].(string)
	owner, _ := args["owner"].(string)
	group, _ := args["group"].(string)
	vars, _ := args["vars"].(map[string]interface{})
	
	result := &types.Result{
		Success: true,
		Changed: false,
		Data:    make(map[string]interface{}),
	}
	
	// Read template file
	templateContent, err := m.readTemplateFile(src)
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to read template file: %v", err)
		return result, nil
	}
	
	// Render template
	rendered, err := m.renderTemplate(templateContent, vars)
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to render template: %v", err)
		return result, nil
	}
	
	// Check if destination exists and get current content
	destExists, currentContent := m.getDestinationContent(ctx, conn, dest)
	
	// Check if content differs
	if destExists && currentContent == rendered {
		result.Message = "File already exists with same content"
		return result, nil
	}
	
	// Backup existing file if requested
	if backup && destExists {
		backupPath := fmt.Sprintf("%s.backup", dest)
		backupCmd := fmt.Sprintf("cp %s %s", dest, backupPath)
		if _, err := conn.Execute(ctx, backupCmd, types.ExecuteOptions{}); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to backup file: %v", err)
			return result, nil
		}
		result.Data["backup_file"] = backupPath
	}
	
	// Copy rendered content to destination
	reader := strings.NewReader(rendered)
	modeInt := 0644
	if mode != "" {
		fmt.Sscanf(mode, "%o", &modeInt)
	}
	
	if err := conn.Copy(ctx, reader, dest, modeInt); err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to copy rendered template: %v", err)
		return result, nil
	}
	
	// Set ownership if specified
	if owner != "" || group != "" {
		if err := m.setOwnership(ctx, conn, dest, owner, group); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to set ownership: %v", err)
			return result, nil
		}
	}
	
	// Set permissions if specified and different from default
	if mode != "" && mode != "0644" {
		if err := m.setMode(ctx, conn, dest, mode); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to set permissions: %v", err)
			return result, nil
		}
	}
	
	result.Changed = true
	result.Message = "Template rendered and copied successfully"
	result.Data["dest"] = dest
	result.Data["checksum"] = m.calculateChecksum(rendered)
	
	return result, nil
}

// readTemplateFile reads the template file from local filesystem
func (m *TemplateModule) readTemplateFile(path string) (string, error) {
	// First try as absolute path
	if data, err := os.ReadFile(path); err == nil {
		return string(data), nil
	}
	
	// Try relative to current directory
	if data, err := os.ReadFile(path); err == nil {
		return string(data), nil
	}
	
	// Try in templates directory
	templatesPath := fmt.Sprintf("templates/%s", path)
	if data, err := os.ReadFile(templatesPath); err == nil {
		return string(data), nil
	}
	
	return "", fmt.Errorf("template file not found: %s", path)
}

// renderTemplate renders a Go template with variables
func (m *TemplateModule) renderTemplate(templateContent string, vars map[string]interface{}) (string, error) {
	// Support both Jinja2-style and Go template syntax
	// Convert common Jinja2 patterns to Go template syntax
	templateContent = m.convertJinja2ToGoTemplate(templateContent)
	
	// Create template
	tmpl, err := template.New("template").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}
	
	// Render template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}
	
	return buf.String(), nil
}

// convertJinja2ToGoTemplate converts common Jinja2 patterns to Go template syntax
func (m *TemplateModule) convertJinja2ToGoTemplate(content string) string {
	// This is a simplified conversion - a full implementation would need more sophisticated parsing
	replacements := []struct {
		from, to string
	}{
		{"{{", "{{"},  // Keep Go template syntax
		{"}}", "}}"},  // Keep Go template syntax
		{"{%", "{{"},  // Convert Jinja2 block syntax
		{"%}", "}}"},  // Convert Jinja2 block syntax
		{"| default(", " | default "},  // Convert filter syntax
		{"| upper", " | upper"},        // Convert filter syntax
		{"| lower", " | lower"},        // Convert filter syntax
	}
	
	result := content
	for _, r := range replacements {
		if r.from != r.to {
			result = strings.ReplaceAll(result, r.from, r.to)
		}
	}
	
	return result
}

// getDestinationContent gets the content of the destination file if it exists
func (m *TemplateModule) getDestinationContent(ctx context.Context, conn types.Connection, dest string) (bool, string) {
	// Check if file exists
	checkCmd := fmt.Sprintf("test -f %s && echo EXISTS || echo NOTEXISTS", dest)
	checkResult, err := conn.Execute(ctx, checkCmd, types.ExecuteOptions{})
	if err != nil || strings.TrimSpace(checkResult.Message) != "EXISTS" {
		return false, ""
	}
	
	// Get file content
	catCmd := fmt.Sprintf("cat %s", dest)
	catResult, err := conn.Execute(ctx, catCmd, types.ExecuteOptions{})
	if err != nil {
		return true, ""
	}
	
	return true, catResult.Message
}

// setOwnership sets file ownership
func (m *TemplateModule) setOwnership(ctx context.Context, conn types.Connection, path, owner, group string) error {
	if owner == "" && group == "" {
		return nil
	}
	
	ownership := ""
	if owner != "" && group != "" {
		ownership = fmt.Sprintf("%s:%s", owner, group)
	} else if owner != "" {
		ownership = owner
	} else {
		ownership = ":" + group
	}
	
	chownCmd := fmt.Sprintf("chown %s %s", ownership, path)
	_, err := conn.Execute(ctx, chownCmd, types.ExecuteOptions{})
	return err
}

// setMode sets file permissions
func (m *TemplateModule) setMode(ctx context.Context, conn types.Connection, path, mode string) error {
	chmodCmd := fmt.Sprintf("chmod %s %s", mode, path)
	_, err := conn.Execute(ctx, chmodCmd, types.ExecuteOptions{})
	return err
}

// calculateChecksum calculates a simple checksum for the content
func (m *TemplateModule) calculateChecksum(content string) string {
	// Simple checksum using Go's hash
	var sum uint32
	for _, b := range []byte(content) {
		sum += uint32(b)
	}
	return fmt.Sprintf("%08x", sum)
}

// Validate checks if the module arguments are valid
func (m *TemplateModule) Validate(args map[string]interface{}) error {
	// Src is required
	src, ok := args["src"]
	if !ok || src == nil || src == "" {
		return types.NewValidationError("src", src, "required field is missing")
	}
	
	// Dest is required
	dest, ok := args["dest"]
	if !ok || dest == nil || dest == "" {
		return types.NewValidationError("dest", dest, "required field is missing")
	}
	
	return nil
}

// Documentation returns the module documentation
func (m *TemplateModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "template",
		Description: "Template a file out to a remote server",
		Parameters: map[string]types.ParamDoc{
			"src": {
				Description: "Path to the template file",
				Required:    true,
				Type:        "string",
			},
			"dest": {
				Description: "Location to render the template to on the remote machine",
				Required:    true,
				Type:        "string",
			},
			"backup": {
				Description: "Create a backup file if the destination already exists",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"mode": {
				Description: "Permissions of the destination file (octal)",
				Required:    false,
				Type:        "string",
				Default:     "0644",
			},
			"owner": {
				Description: "Owner of the destination file",
				Required:    false,
				Type:        "string",
			},
			"group": {
				Description: "Group of the destination file",
				Required:    false,
				Type:        "string",
			},
			"vars": {
				Description: "Variables to use in the template",
				Required:    false,
				Type:        "dict",
			},
		},
		Examples: []string{
			"- name: Template configuration file\n  template:\n    src: nginx.conf.j2\n    dest: /etc/nginx/nginx.conf\n    mode: '0644'\n    backup: true",
			"- name: Template with variables\n  template:\n    src: app.config.j2\n    dest: /opt/app/config.yml\n    vars:\n      port: 8080\n      debug: false",
		},
		Returns: map[string]string{
			"dest":        "Destination file path",
			"checksum":    "Checksum of the rendered file",
			"backup_file": "Path to backup file if created",
		},
	}
}