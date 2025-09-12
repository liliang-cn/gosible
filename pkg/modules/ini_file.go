package modules

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/liliang-cn/gosible/pkg/types"
)

type IniFileModule struct {
	BaseModule
}

func NewIniFileModule() *IniFileModule {
	return &IniFileModule{
		BaseModule: BaseModule{
			name: "ini_file",
		},
	}
}

func (m *IniFileModule) Validate(args map[string]interface{}) error {
	path, hasPath := args["path"].(string)
	if !hasPath || path == "" {
		return fmt.Errorf("path is required")
	}

	section, hasSection := args["section"].(string)
	option, hasOption := args["option"].(string)
	state, hasState := args["state"].(string)

	if !hasState {
		state = "present"
		args["state"] = state
	}

	if state != "present" && state != "absent" {
		return fmt.Errorf("state must be 'present' or 'absent'")
	}

	if state == "present" {
		if !hasSection || section == "" {
			return fmt.Errorf("section is required when state is present")
		}
		if !hasOption || option == "" {
			return fmt.Errorf("option is required when state is present")
		}
		if _, hasValue := args["value"]; !hasValue {
			return fmt.Errorf("value is required when state is present")
		}
	}

	if state == "absent" {
		if !hasSection && !hasOption {
			return fmt.Errorf("at least one of section or option is required when state is absent")
		}
	}

	return nil
}

func (m *IniFileModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	if err := m.Validate(args); err != nil {
		return &types.Result{
			Success: false,
			Message: err.Error(),
		}, err
	}

	path := args["path"].(string)
	section, _ := args["section"].(string)
	option, _ := args["option"].(string)
	value, _ := args["value"].(string)
	state := args["state"].(string)
	backup, _ := args["backup"].(bool)
	createDir, _ := args["create"].(bool)
	noExtraSpaces, _ := args["no_extra_spaces"].(bool)
	allowNoValue, _ := args["allow_no_value"].(bool)
	exclusive, _ := args["exclusive"].(bool)

	exists, err := m.fileExists(ctx, conn, path)
	if err != nil {
		return &types.Result{
			Success: false,
			Message: fmt.Sprintf("Failed to check file: %v", err),
		}, err
	}

	if !exists && state == "absent" {
		return &types.Result{
			Success: true,
			Changed: false,
			Message: "File does not exist, nothing to remove",
		}, nil
	}

	if !exists && state == "present" {
		if createDir {
			dir := filepath.Dir(path)
			if err := m.createDirectory(ctx, conn, dir); err != nil {
				return &types.Result{
					Success: false,
					Message: fmt.Sprintf("Failed to create directory: %v", err),
				}, err
			}
		}

		if m.CheckMode(args) {
			return &types.Result{
				Success: true,
				Changed: true,
				Message: "Would create new INI file",
			}, nil
		}

		content := m.createNewIniContent(section, option, value, noExtraSpaces, allowNoValue)
		if err := m.writeFile(ctx, conn, path, content); err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to create file: %v", err),
			}, err
		}

		return &types.Result{
			Success: true,
			Changed: true,
			Message: "INI file created",
		}, nil
	}

	currentContent, err := m.readFile(ctx, conn, path)
	if err != nil {
		return &types.Result{
			Success: false,
			Message: fmt.Sprintf("Failed to read file: %v", err),
		}, err
	}

	if backup && !m.CheckMode(args) {
		if err := m.createBackup(ctx, conn, path, currentContent); err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to create backup: %v", err),
			}, err
		}
	}

	parser := &iniParser{
		content:       currentContent,
		noExtraSpaces: noExtraSpaces,
		allowNoValue:  allowNoValue,
	}

	changed := false
	var newContent string
	var diff string

	if state == "present" {
		newContent, changed = parser.setValue(section, option, value, exclusive)
	} else {
		if option != "" {
			newContent, changed = parser.removeOption(section, option)
		} else {
			newContent, changed = parser.removeSection(section)
		}
	}

	if changed && m.DiffMode(args) {
		diff = m.generateDiff(currentContent, newContent, path)
	}

	if m.CheckMode(args) {
		result := &types.Result{
			Success: true,
			Changed: changed,
			Message: "Check mode: no changes made",
		}
		if diff != "" {
			result.Diff = &types.DiffResult{
				Diff: diff,
			}
		}
		return result, nil
	}

	if changed {
		if err := m.writeFile(ctx, conn, path, newContent); err != nil {
			return &types.Result{
				Success: false,
				Message: fmt.Sprintf("Failed to write file: %v", err),
			}, err
		}
	}

	result := &types.Result{
		Success: true,
		Changed: changed,
		Message: "INI file updated",
	}
	if diff != "" {
		result.Diff = &types.DiffResult{
			Diff: diff,
		}
	}
	return result, nil
}

type iniParser struct {
	content       string
	noExtraSpaces bool
	allowNoValue  bool
}

func (p *iniParser) setValue(section, option, value string, exclusive bool) (string, bool) {
	lines := strings.Split(p.content, "\n")
	inSection := false
	sectionFound := false
	optionFound := false
	sectionStart := -1
	sectionEnd := -1
	result := []string{}
	changed := false

	sectionHeader := fmt.Sprintf("[%s]", section)
	optionPattern := p.getOptionPattern(option)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == sectionHeader {
			inSection = true
			sectionFound = true
			sectionStart = len(result)
			result = append(result, line)
			continue
		}

		if inSection && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inSection = false
			sectionEnd = len(result)
		}

		if inSection && optionPattern.MatchString(trimmed) {
			optionFound = true
			newLine := p.formatOption(option, value)
			if line != newLine {
				changed = true
				result = append(result, newLine)
			} else {
				result = append(result, line)
			}
			continue
		}

		if inSection && exclusive && trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, ";") {
			if strings.Contains(trimmed, "=") || (p.allowNoValue && trimmed != "") {
				changed = true
				continue
			}
		}

		result = append(result, line)
	}

	if inSection {
		sectionEnd = len(result)
	}

	if !optionFound {
		newLine := p.formatOption(option, value)
		if sectionFound {
			if sectionEnd > sectionStart+1 {
				result = append(result[:sectionEnd], append([]string{newLine}, result[sectionEnd:]...)...)
			} else {
				result = append(result[:sectionStart+1], append([]string{newLine}, result[sectionStart+1:]...)...)
			}
		} else {
			result = append(result, "", sectionHeader, newLine)
		}
		changed = true
	}

	return strings.Join(result, "\n"), changed
}

func (p *iniParser) removeOption(section, option string) (string, bool) {
	lines := strings.Split(p.content, "\n")
	inSection := false
	result := []string{}
	changed := false
	sectionHeader := fmt.Sprintf("[%s]", section)
	optionPattern := p.getOptionPattern(option)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == sectionHeader {
			inSection = true
			result = append(result, line)
			continue
		}

		if inSection && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inSection = false
		}

		if inSection && optionPattern.MatchString(trimmed) {
			changed = true
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n"), changed
}

func (p *iniParser) removeSection(section string) (string, bool) {
	lines := strings.Split(p.content, "\n")
	inSection := false
	result := []string{}
	changed := false
	sectionHeader := fmt.Sprintf("[%s]", section)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == sectionHeader {
			inSection = true
			changed = true
			continue
		}

		if inSection && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inSection = false
		}

		if !inSection {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n"), changed
}

func (p *iniParser) getOptionPattern(option string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(option)
	if p.noExtraSpaces {
		return regexp.MustCompile(fmt.Sprintf("^%s=", escaped))
	}
	return regexp.MustCompile(fmt.Sprintf("^%s\\s*=", escaped))
}

func (p *iniParser) formatOption(option, value string) string {
	if p.allowNoValue && value == "" {
		return option
	}
	if p.noExtraSpaces {
		return fmt.Sprintf("%s=%s", option, value)
	}
	return fmt.Sprintf("%s = %s", option, value)
}

func (m *IniFileModule) createNewIniContent(section, option, value string, noExtraSpaces, allowNoValue bool) string {
	parser := &iniParser{
		noExtraSpaces: noExtraSpaces,
		allowNoValue:  allowNoValue,
	}
	content := fmt.Sprintf("[%s]\n%s\n", section, parser.formatOption(option, value))
	return content
}

func (m *IniFileModule) fileExists(ctx context.Context, conn types.Connection, path string) (bool, error) {
	result, err := conn.Execute(ctx, fmt.Sprintf("test -f %s", path), types.ExecuteOptions{})
	if err != nil {
		return false, nil
	}
	return result.Success, nil
}

func (m *IniFileModule) readFile(ctx context.Context, conn types.Connection, path string) (string, error) {
	result, err := conn.Execute(ctx, fmt.Sprintf("cat %s", path), types.ExecuteOptions{})
	if err != nil {
		return "", err
	}
	if !result.Success {
		return "", fmt.Errorf("failed to read file")
	}
	if stdout, ok := result.Data["stdout"].(string); ok {
		return stdout, nil
	}
	return "", fmt.Errorf("no stdout in result")
}

func (m *IniFileModule) writeFile(ctx context.Context, conn types.Connection, path, content string) error {
	tempFile := "/tmp/ini_file_temp"
	
	escaped := strings.ReplaceAll(content, "'", "'\"'\"'")
	cmd := fmt.Sprintf("cat > %s << 'EOF'\n%s\nEOF", tempFile, escaped)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("failed to write temp file")
	}

	cmd = fmt.Sprintf("mv %s %s", tempFile, path)
	result, err = conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("failed to move file")
	}

	return nil
}

func (m *IniFileModule) createDirectory(ctx context.Context, conn types.Connection, dir string) error {
	cmd := fmt.Sprintf("mkdir -p %s", dir)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("failed to create directory")
	}
	return nil
}

func (m *IniFileModule) createBackup(ctx context.Context, conn types.Connection, path, content string) error {
	backupPath := fmt.Sprintf("%s.backup", path)
	return m.writeFile(ctx, conn, backupPath, content)
}

func (m *IniFileModule) generateDiff(oldContent, newContent, path string) string {
	// oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	
	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- %s\n", path))
	diff.WriteString(fmt.Sprintf("+++ %s\n", path))
	
	scanner := bufio.NewScanner(strings.NewReader(oldContent))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		oldLine := scanner.Text()
		if lineNum <= len(newLines) {
			if oldLine != newLines[lineNum-1] {
				diff.WriteString(fmt.Sprintf("-%s\n", oldLine))
				diff.WriteString(fmt.Sprintf("+%s\n", newLines[lineNum-1]))
			}
		} else {
			diff.WriteString(fmt.Sprintf("-%s\n", oldLine))
		}
	}
	
	for i := lineNum; i < len(newLines); i++ {
		diff.WriteString(fmt.Sprintf("+%s\n", newLines[i]))
	}
	
	return diff.String()
}