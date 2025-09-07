package modules

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gosinble/gosinble/pkg/types"
)

// RepositoryModule manages package repositories (APT/YUM/DNF)
type RepositoryModule struct {
	BaseModule
}

// RepositoryState represents the current state of repositories
type RepositoryState struct {
	System       string              `json:"system"`        // apt, yum, dnf
	Repositories map[string]*RepoInfo `json:"repositories"` // repo name -> info
}

// RepoInfo contains information about a repository
type RepoInfo struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Enabled     bool              `json:"enabled"`
	GPGCheck    bool              `json:"gpg_check"`
	GPGKey      string            `json:"gpg_key,omitempty"`
	Description string            `json:"description,omitempty"`
	Components  []string          `json:"components,omitempty"` // APT only
	Distribution string           `json:"distribution,omitempty"` // APT only
	Architecture string           `json:"architecture,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// NewRepositoryModule creates a new repository module instance
func NewRepositoryModule() *RepositoryModule {
	return &RepositoryModule{
		BaseModule: BaseModule{},
	}
}

// Name returns the module name
func (m *RepositoryModule) Name() string {
	return "repository"
}

// Capabilities returns the module capabilities
func (m *RepositoryModule) Capabilities() *types.ModuleCapability {
	return &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     true,
		AsyncMode:    false,
		Platform:     "linux",
		RequiresRoot: true, // Repository management typically requires root
	}
}

// Validate validates the module arguments
func (m *RepositoryModule) Validate(args map[string]interface{}) error {
	// Required parameters
	repo := m.GetStringArg(args, "repo", "")
	name := m.GetStringArg(args, "name", "")
	
	if repo == "" && name == "" {
		return types.NewValidationError("repo/name", nil, "either repo or name parameter is required")
	}
	
	// State validation
	state := m.GetStringArg(args, "state", "present")
	validStates := []string{"present", "absent"}
	if !m.isValidChoice(state, validStates) {
		return types.NewValidationError("state", state, fmt.Sprintf("state must be one of: %v", validStates))
	}
	
	// If state is present and repo is provided, validate repo format
	if state == "present" && repo != "" {
		if err := m.validateRepoFormat(repo); err != nil {
			return types.NewValidationError("repo", repo, fmt.Sprintf("invalid repository format: %v", err))
		}
	}
	
	// Validate baseurl format if provided
	baseurl := m.GetStringArg(args, "baseurl", "")
	if baseurl != "" {
		if !strings.HasPrefix(baseurl, "http://") && !strings.HasPrefix(baseurl, "https://") && !strings.HasPrefix(baseurl, "ftp://") && !strings.HasPrefix(baseurl, "file://") {
			return types.NewValidationError("baseurl", baseurl, "baseurl must be a valid URL")
		}
	}
	
	return nil
}

// Run executes the repository module
func (m *RepositoryModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)
	diffMode := m.DiffMode(args)
	
	// Parse arguments
	repo := m.GetStringArg(args, "repo", "")
	name := m.GetStringArg(args, "name", "")
	state := m.GetStringArg(args, "state", "present")
	description := m.GetStringArg(args, "description", "")
	baseurl := m.GetStringArg(args, "baseurl", "")
	gpgcheck := m.GetBoolArg(args, "gpgcheck", true)
	gpgkey := m.GetStringArg(args, "gpgkey", "")
	enabled := m.GetBoolArg(args, "enabled", true)
	
	// Detect package management system
	system, err := m.detectPackageSystem(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("failed to detect package management system: %w", err)
	}
	
	// Get current repository state
	currentState, err := m.getRepositoryState(ctx, conn, system)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository state: %w", err)
	}
	
	// Store original state for diff mode
	beforeState := *currentState
	
	// Determine repository name
	repoName := name
	if repoName == "" && repo != "" {
		repoName = m.extractRepoName(repo, system)
	}
	
	// Track changes
	var changes []string
	actuallyChanged := false
	
	// Initialize result
	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"system":       system,
		"repo_name":    repoName,
		"before_state": beforeState,
	})
	
	// Process based on state
	if state == "present" {
		// Ensure repository is present
		changed, changeMsg, newState, err := m.handlePresentRepository(ctx, conn, currentState, system, repoName, repo, description, baseurl, gpgcheck, gpgkey, enabled, checkMode)
		if err != nil {
			return nil, fmt.Errorf("failed to handle present repository: %w", err)
		}
		
		if changed {
			changes = append(changes, changeMsg)
			currentState = newState
			if !checkMode {
				actuallyChanged = true
			}
		}
	} else if state == "absent" {
		// Remove repository
		changed, changeMsg, newState, err := m.handleAbsentRepository(ctx, conn, currentState, system, repoName, checkMode)
		if err != nil {
			return nil, fmt.Errorf("failed to handle absent repository: %w", err)
		}
		
		if changed {
			changes = append(changes, changeMsg)
			currentState = newState
			if !checkMode {
				actuallyChanged = true
			}
		}
	}
	
	// Set result properties
	result.Changed = actuallyChanged || (checkMode && len(changes) > 0)
	result.Data["after_state"] = *currentState
	result.Data["changes"] = changes
	
	if checkMode {
		result.Simulated = true
		result.Data["check_mode"] = true
	}
	
	// Generate diff if requested
	if diffMode && (actuallyChanged || (checkMode && len(changes) > 0)) {
		diff := m.generateRepositoryDiff(&beforeState, currentState, changes)
		result.Diff = diff
	}
	
	// Set appropriate message
	if len(changes) > 0 {
		if checkMode {
			result.Message = fmt.Sprintf("Would make changes: %s", strings.Join(changes, ", "))
		} else {
			result.Message = fmt.Sprintf("Made changes: %s", strings.Join(changes, ", "))
		}
	} else {
		result.Message = "Repository is already in desired state"
	}
	
	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	return result, nil
}

// detectPackageSystem detects the package management system
func (m *RepositoryModule) detectPackageSystem(ctx context.Context, conn types.Connection) (string, error) {
	// Check for APT
	result, err := conn.Execute(ctx, "which apt-get", types.ExecuteOptions{})
	if err == nil && result.Success {
		return "apt", nil
	}
	
	// Check for DNF
	result, err = conn.Execute(ctx, "which dnf", types.ExecuteOptions{})
	if err == nil && result.Success {
		return "dnf", nil
	}
	
	// Check for YUM
	result, err = conn.Execute(ctx, "which yum", types.ExecuteOptions{})
	if err == nil && result.Success {
		return "yum", nil
	}
	
	return "", fmt.Errorf("no supported package management system found (apt, yum, dnf)")
}

// getRepositoryState retrieves the current state of repositories
func (m *RepositoryModule) getRepositoryState(ctx context.Context, conn types.Connection, system string) (*RepositoryState, error) {
	state := &RepositoryState{
		System:       system,
		Repositories: make(map[string]*RepoInfo),
	}
	
	switch system {
	case "apt":
		return m.getAPTRepositories(ctx, conn, state)
	case "yum", "dnf":
		return m.getYumRepositories(ctx, conn, state, system)
	default:
		return nil, fmt.Errorf("unsupported package system: %s", system)
	}
}

// getAPTRepositories gets APT repository information
func (m *RepositoryModule) getAPTRepositories(ctx context.Context, conn types.Connection, state *RepositoryState) (*RepositoryState, error) {
	// Read sources.list and sources.list.d/*
	result, err := conn.Execute(ctx, "cat /etc/apt/sources.list 2>/dev/null || true", types.ExecuteOptions{})
	if err != nil {
		return state, nil
	}
	
	if result.Success && result.Message != "" {
		m.parseAPTSources(result.Message, state.Repositories)
	}
	
	// Read additional sources
	result, err = conn.Execute(ctx, "find /etc/apt/sources.list.d -name '*.list' -exec cat {} \\; 2>/dev/null || true", types.ExecuteOptions{})
	if err != nil {
		return state, nil
	}
	
	if result.Success && result.Message != "" {
		m.parseAPTSources(result.Message, state.Repositories)
	}
	
	return state, nil
}

// getYumRepositories gets YUM/DNF repository information
func (m *RepositoryModule) getYumRepositories(ctx context.Context, conn types.Connection, state *RepositoryState, system string) (*RepositoryState, error) {
	// List repositories
	cmd := fmt.Sprintf("%s repolist all", system)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return state, nil
	}
	
	m.parseYumRepoList(result.Message, state.Repositories)
	
	return state, nil
}

// parseAPTSources parses APT sources.list content
func (m *RepositoryModule) parseAPTSources(content string, repositories map[string]*RepoInfo) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		
		if fields[0] == "deb" || fields[0] == "deb-src" {
			url := fields[1]
			dist := fields[2]
			var components []string
			if len(fields) > 3 {
				components = fields[3:]
			}
			
			repoName := m.generateRepoName(url, dist)
			repositories[repoName] = &RepoInfo{
				Name:         repoName,
				URL:          url,
				Enabled:      true, // APT repos in sources.list are enabled by default
				Distribution: dist,
				Components:   components,
			}
		}
	}
}

// parseYumRepoList parses YUM/DNF repository list output
func (m *RepositoryModule) parseYumRepoList(content string, repositories map[string]*RepoInfo) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "repo id") {
			continue
		}
		
		// Parse repo list format: "repo-id   repo-name   status"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		
		repoID := fields[0]
		enabled := !strings.Contains(line, "disabled")
		
		repositories[repoID] = &RepoInfo{
			Name:    repoID,
			Enabled: enabled,
		}
	}
}

// handlePresentRepository ensures repository is present
func (m *RepositoryModule) handlePresentRepository(ctx context.Context, conn types.Connection, state *RepositoryState, system, repoName, repo, description, baseurl string, gpgcheck bool, gpgkey string, enabled bool, checkMode bool) (bool, string, *RepositoryState, error) {
	newState := *state
	
	existingRepo, exists := state.Repositories[repoName]
	
	if exists {
		// Repository exists, check if it needs updates
		needsUpdate := m.repositoryNeedsUpdate(existingRepo, repo, description, baseurl, gpgcheck, gpgkey, enabled)
		if !needsUpdate {
			return false, "", &newState, nil
		}
		
		// Update repository
		if !checkMode {
			err := m.updateRepository(ctx, conn, system, repoName, repo, description, baseurl, gpgcheck, gpgkey, enabled)
			if err != nil {
				return false, "", &newState, fmt.Errorf("failed to update repository: %w", err)
			}
		}
		
		// Update state
		newState.Repositories[repoName] = &RepoInfo{
			Name:        repoName,
			URL:         baseurl,
			Enabled:     enabled,
			GPGCheck:    gpgcheck,
			GPGKey:      gpgkey,
			Description: description,
		}
		
		return true, fmt.Sprintf("updated repository %s", repoName), &newState, nil
	} else {
		// Repository doesn't exist, add it
		if !checkMode {
			err := m.addRepository(ctx, conn, system, repoName, repo, description, baseurl, gpgcheck, gpgkey, enabled)
			if err != nil {
				return false, "", &newState, fmt.Errorf("failed to add repository: %w", err)
			}
		}
		
		// Update state
		if newState.Repositories == nil {
			newState.Repositories = make(map[string]*RepoInfo)
		}
		newState.Repositories[repoName] = &RepoInfo{
			Name:        repoName,
			URL:         baseurl,
			Enabled:     enabled,
			GPGCheck:    gpgcheck,
			GPGKey:      gpgkey,
			Description: description,
		}
		
		return true, fmt.Sprintf("added repository %s", repoName), &newState, nil
	}
}

// handleAbsentRepository ensures repository is absent
func (m *RepositoryModule) handleAbsentRepository(ctx context.Context, conn types.Connection, state *RepositoryState, system, repoName string, checkMode bool) (bool, string, *RepositoryState, error) {
	newState := *state
	
	_, exists := state.Repositories[repoName]
	if !exists {
		return false, "", &newState, nil
	}
	
	// Remove repository
	if !checkMode {
		err := m.removeRepository(ctx, conn, system, repoName)
		if err != nil {
			return false, "", &newState, fmt.Errorf("failed to remove repository: %w", err)
		}
	}
	
	// Update state
	delete(newState.Repositories, repoName)
	
	return true, fmt.Sprintf("removed repository %s", repoName), &newState, nil
}

// addRepository adds a new repository
func (m *RepositoryModule) addRepository(ctx context.Context, conn types.Connection, system, repoName, repo, description, baseurl string, gpgcheck bool, gpgkey string, enabled bool) error {
	switch system {
	case "apt":
		return m.addAPTRepository(ctx, conn, repoName, repo)
	case "yum", "dnf":
		return m.addYumRepository(ctx, conn, system, repoName, description, baseurl, gpgcheck, gpgkey, enabled)
	default:
		return fmt.Errorf("unsupported package system: %s", system)
	}
}

// addAPTRepository adds an APT repository
func (m *RepositoryModule) addAPTRepository(ctx context.Context, conn types.Connection, repoName, repo string) error {
	// Add repository using add-apt-repository or directly to sources.list.d
	cmd := fmt.Sprintf("add-apt-repository -y '%s'", repo)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		// Fallback: add directly to sources.list.d
		filename := fmt.Sprintf("/etc/apt/sources.list.d/%s.list", repoName)
		cmd = fmt.Sprintf("echo '%s' > %s", repo, filename)
		result, err = conn.Execute(ctx, cmd, types.ExecuteOptions{})
		if err != nil || !result.Success {
			return fmt.Errorf("failed to add APT repository: %s", result.Message)
		}
	}
	
	// Update APT cache
	_, err = conn.Execute(ctx, "apt-get update", types.ExecuteOptions{})
	if err != nil {
		return fmt.Errorf("failed to update APT cache: %w", err)
	}
	
	return nil
}

// addYumRepository adds a YUM/DNF repository
func (m *RepositoryModule) addYumRepository(ctx context.Context, conn types.Connection, system, repoName, description, baseurl string, gpgcheck bool, gpgkey string, enabled bool) error {
	// Create repo file
	repoContent := fmt.Sprintf(`[%s]
name=%s
baseurl=%s
enabled=%d
gpgcheck=%d`, repoName, description, baseurl, m.boolToInt(enabled), m.boolToInt(gpgcheck))
	
	if gpgkey != "" {
		repoContent += fmt.Sprintf("\ngpgkey=%s", gpgkey)
	}
	
	filename := fmt.Sprintf("/etc/yum.repos.d/%s.repo", repoName)
	cmd := fmt.Sprintf("echo '%s' > %s", repoContent, filename)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return fmt.Errorf("failed to create repository file: %s", result.Message)
	}
	
	return nil
}

// updateRepository updates an existing repository
func (m *RepositoryModule) updateRepository(ctx context.Context, conn types.Connection, system, repoName, repo, description, baseurl string, gpgcheck bool, gpgkey string, enabled bool) error {
	// For updates, remove and re-add
	if err := m.removeRepository(ctx, conn, system, repoName); err != nil {
		return err
	}
	return m.addRepository(ctx, conn, system, repoName, repo, description, baseurl, gpgcheck, gpgkey, enabled)
}

// removeRepository removes a repository
func (m *RepositoryModule) removeRepository(ctx context.Context, conn types.Connection, system, repoName string) error {
	switch system {
	case "apt":
		return m.removeAPTRepository(ctx, conn, repoName)
	case "yum", "dnf":
		return m.removeYumRepository(ctx, conn, repoName)
	default:
		return fmt.Errorf("unsupported package system: %s", system)
	}
}

// removeAPTRepository removes an APT repository
func (m *RepositoryModule) removeAPTRepository(ctx context.Context, conn types.Connection, repoName string) error {
	// Remove from sources.list.d
	filename := fmt.Sprintf("/etc/apt/sources.list.d/%s.list", repoName)
	cmd := fmt.Sprintf("rm -f %s", filename)
	_, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err
}

// removeYumRepository removes a YUM/DNF repository
func (m *RepositoryModule) removeYumRepository(ctx context.Context, conn types.Connection, repoName string) error {
	filename := fmt.Sprintf("/etc/yum.repos.d/%s.repo", repoName)
	cmd := fmt.Sprintf("rm -f %s", filename)
	_, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err
}

// Helper methods
func (m *RepositoryModule) validateRepoFormat(repo string) error {
	// Basic validation for APT format: "deb http://... dist component"
	if strings.HasPrefix(repo, "deb ") {
		fields := strings.Fields(repo)
		if len(fields) < 3 {
			return fmt.Errorf("APT repository format should be: deb URL DISTRIBUTION [COMPONENTS]")
		}
		return nil
	}
	
	// Basic validation for URL format
	if strings.HasPrefix(repo, "http://") || strings.HasPrefix(repo, "https://") {
		return nil
	}
	
	return fmt.Errorf("repository should be a valid APT line or URL")
}

func (m *RepositoryModule) extractRepoName(repo, system string) string {
	if system == "apt" {
		// Extract meaningful name from APT repo
		if strings.Contains(repo, "/") {
			parts := strings.Split(repo, "/")
			for _, part := range parts {
				if part != "" && part != "http:" && part != "https:" && !strings.Contains(part, ".") {
					return part
				}
			}
		}
	}
	
	// Fallback to sanitized version
	reg := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	return reg.ReplaceAllString(repo, "-")
}

func (m *RepositoryModule) generateRepoName(url, dist string) string {
	// Generate a name from URL and distribution
	name := strings.ReplaceAll(url, "http://", "")
	name = strings.ReplaceAll(name, "https://", "")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ".", "-")
	if dist != "" {
		name += "-" + dist
	}
	return name
}

func (m *RepositoryModule) repositoryNeedsUpdate(existing *RepoInfo, repo, description, baseurl string, gpgcheck bool, gpgkey string, enabled bool) bool {
	if baseurl != "" && existing.URL != baseurl {
		return true
	}
	if existing.Enabled != enabled {
		return true
	}
	if existing.GPGCheck != gpgcheck {
		return true
	}
	if gpgkey != "" && existing.GPGKey != gpgkey {
		return true
	}
	if description != "" && existing.Description != description {
		return true
	}
	return false
}

func (m *RepositoryModule) generateRepositoryDiff(before, after *RepositoryState, changes []string) *types.DiffResult {
	beforeContent := m.repositoryStateToString(before)
	afterContent := m.repositoryStateToString(after)
	return m.GenerateDiff(beforeContent, afterContent)
}

func (m *RepositoryModule) repositoryStateToString(state *RepositoryState) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("System: %s", state.System))
	
	for name, repo := range state.Repositories {
		lines = append(lines, fmt.Sprintf("Repository: %s", name))
		lines = append(lines, fmt.Sprintf("  URL: %s", repo.URL))
		lines = append(lines, fmt.Sprintf("  Enabled: %v", repo.Enabled))
		if repo.GPGCheck {
			lines = append(lines, fmt.Sprintf("  GPGCheck: %v", repo.GPGCheck))
		}
		if repo.GPGKey != "" {
			lines = append(lines, fmt.Sprintf("  GPGKey: %s", repo.GPGKey))
		}
	}
	
	return strings.Join(lines, "\n")
}

func (m *RepositoryModule) boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (m *RepositoryModule) isValidChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}