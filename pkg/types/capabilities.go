package types

// ModuleCapabilities defines what features a module supports
type ModuleCapabilities interface {
	SupportsCheckMode() bool
	SupportsDiffMode() bool
	SupportsAsync() bool
}

// ModuleCapability describes the capabilities of a module
type ModuleCapability struct {
	CheckMode   bool   `json:"check_mode"`
	DiffMode    bool   `json:"diff_mode"` 
	AsyncMode   bool   `json:"async"`
	Platform    string `json:"platform"` // "linux", "windows", "all"
	RequiresRoot bool  `json:"requires_root"`
}

// DefaultCapabilities returns default module capabilities
func DefaultCapabilities() *ModuleCapability {
	return &ModuleCapability{
		CheckMode:    true,  // Most modules should support check mode
		DiffMode:     false, // Must be explicitly supported
		AsyncMode:    false, // Must be explicitly supported
		Platform:     "all", // Works on all platforms by default
		RequiresRoot: false, // Doesn't require root by default
	}
}

// ModuleWithCapabilities interface for modules that declare their capabilities
type ModuleWithCapabilities interface {
	Module
	Capabilities() *ModuleCapability
}