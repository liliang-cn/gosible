package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	
	"gopkg.in/yaml.v3"
)

// VaultConfig holds vault configuration
type VaultConfig struct {
	PasswordFile   string   `yaml:"vault_password_file"`
	PasswordFiles  []string `yaml:"vault_password_files"`
	IdentityList   []string `yaml:"vault_identity_list"`
	EncryptVars    bool     `yaml:"vault_encrypt_vars"`
	DefaultVaultID string   `yaml:"vault_id"`
}

// LoadVaultConfig loads vault configuration from a file
func LoadVaultConfig(filename string) (*VaultConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	
	var config VaultConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

// InitManagerFromConfig initializes a vault manager from configuration
func InitManagerFromConfig(config *VaultConfig) (*Manager, error) {
	manager := NewManager()
	
	// Load single password file
	if config.PasswordFile != "" {
		if err := manager.AddVaultFromFile(DefaultVaultIDLabel, config.PasswordFile); err != nil {
			return nil, fmt.Errorf("failed to load password file: %w", err)
		}
	}
	
	// Load multiple password files
	for _, file := range config.PasswordFiles {
		// Format: vault_id@password_file
		parts := strings.SplitN(file, "@", 2)
		vaultID := DefaultVaultIDLabel
		passwordFile := file
		
		if len(parts) == 2 {
			vaultID = parts[0]
			passwordFile = parts[1]
		}
		
		if err := manager.AddVaultFromFile(vaultID, passwordFile); err != nil {
			return nil, fmt.Errorf("failed to load password file %s: %w", passwordFile, err)
		}
	}
	
	// Load vault identity list
	for _, identity := range config.IdentityList {
		// Format: vault_id@source
		parts := strings.SplitN(identity, "@", 2)
		if len(parts) != 2 {
			continue
		}
		
		vaultID := parts[0]
		source := parts[1]
		
		// Check if source is a script (executable) or file
		if fileInfo, err := os.Stat(source); err == nil {
			if fileInfo.Mode()&0111 != 0 {
				// Executable script
				if err := manager.AddVaultFromScript(vaultID, source); err != nil {
					return nil, fmt.Errorf("failed to load from script %s: %w", source, err)
				}
			} else {
				// Regular file
				if err := manager.AddVaultFromFile(vaultID, source); err != nil {
					return nil, fmt.Errorf("failed to load from file %s: %w", source, err)
				}
			}
		}
	}
	
	// Set default vault ID
	if config.DefaultVaultID != "" {
		manager.SetDefaultVaultID(config.DefaultVaultID)
	}
	
	return manager, nil
}

// InitManagerFromEnv initializes a vault manager from environment variables
func InitManagerFromEnv() (*Manager, error) {
	manager := NewManager()
	
	// Check for ANSIBLE_VAULT_PASSWORD_FILE
	if passwordFile := os.Getenv("ANSIBLE_VAULT_PASSWORD_FILE"); passwordFile != "" {
		if err := manager.AddVaultFromFile(DefaultVaultIDLabel, passwordFile); err != nil {
			return nil, fmt.Errorf("failed to load password from ANSIBLE_VAULT_PASSWORD_FILE: %w", err)
		}
	}
	
	// Check for ANSIBLE_VAULT_PASSWORD (not recommended)
	if password := os.Getenv("ANSIBLE_VAULT_PASSWORD"); password != "" {
		manager.AddVault(DefaultVaultIDLabel, password)
	}
	
	// Check for ANSIBLE_VAULT_IDENTITY_LIST
	if identityList := os.Getenv("ANSIBLE_VAULT_IDENTITY_LIST"); identityList != "" {
		identities := strings.Split(identityList, ",")
		for _, identity := range identities {
			parts := strings.SplitN(identity, "@", 2)
			if len(parts) != 2 {
				continue
			}
			
			vaultID := parts[0]
			source := parts[1]
			
			if fileInfo, err := os.Stat(source); err == nil {
				if fileInfo.Mode()&0111 != 0 {
					// Executable script
					if err := manager.AddVaultFromScript(vaultID, source); err != nil {
						return nil, err
					}
				} else {
					// Regular file
					if err := manager.AddVaultFromFile(vaultID, source); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	
	return manager, nil
}

// DecryptYAMLFile decrypts a YAML file containing vault-encrypted values
func (m *Manager) DecryptYAMLFile(filename string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Check if entire file is encrypted
	if IsVaultFile(data) {
		decrypted, err := m.Decrypt(string(data))
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt file: %w", err)
		}
		data = decrypted
	}
	
	// Parse YAML
	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	
	// Process any inline vault-encrypted values
	if err := m.ProcessVariables(result); err != nil {
		return nil, fmt.Errorf("failed to process vault variables: %w", err)
	}
	
	return result, nil
}

// EncryptYAMLFile encrypts sensitive values in a YAML file
func (m *Manager) EncryptYAMLFile(filename string, keys []string, vaultID string) error {
	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	// Parse YAML
	var content map[string]interface{}
	if err := yaml.Unmarshal(data, &content); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}
	
	// Get vault
	vault, err := m.GetVault(vaultID)
	if err != nil {
		return err
	}
	
	// Encrypt specified keys
	for _, key := range keys {
		if value, exists := content[key]; exists {
			if strValue, ok := value.(string); ok {
				vs := NewVaultString(vault, strValue)
				encrypted, err := vs.Encrypt()
				if err != nil {
					return fmt.Errorf("failed to encrypt key '%s': %w", key, err)
				}
				content[key] = encrypted
			}
		}
	}
	
	// Write back
	output, err := yaml.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	
	return os.WriteFile(filename, output, 0600)
}

// FindVaultFiles finds all vault-encrypted files in a directory
func FindVaultFiles(dir string) ([]string, error) {
	var vaultFiles []string
	
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() {
			return nil
		}
		
		// Check common Ansible file extensions
		ext := filepath.Ext(path)
		if ext == ".yml" || ext == ".yaml" || ext == ".json" {
			// Read first line to check if it's a vault file
			file, err := os.Open(path)
			if err != nil {
				return nil // Skip files we can't read
			}
			defer file.Close()
			
			header := make([]byte, len(VaultHeader))
			if _, err := file.Read(header); err == nil {
				if string(header) == VaultHeader {
					vaultFiles = append(vaultFiles, path)
				}
			}
		}
		
		return nil
	})
	
	return vaultFiles, err
}

// VaultFilter is a template filter for decrypting vault strings
type VaultFilter struct {
	manager *Manager
}

// NewVaultFilter creates a new vault filter
func NewVaultFilter(manager *Manager) *VaultFilter {
	return &VaultFilter{manager: manager}
}

// Filter decrypts a vault-encrypted string in templates
func (f *VaultFilter) Filter(value interface{}) (interface{}, error) {
	strValue, ok := value.(string)
	if !ok {
		return value, nil
	}
	
	if !IsVaultString(strValue) {
		return value, nil
	}
	
	decrypted, err := f.manager.decryptInlineVault(strValue)
	if err != nil {
		return nil, err
	}
	
	return decrypted, nil
}

// GetTemplateFilters returns template filters for vault operations
func (m *Manager) GetTemplateFilters() map[string]interface{} {
	filter := NewVaultFilter(m)
	return map[string]interface{}{
		"vault_decrypt": filter.Filter,
		"unvault":       filter.Filter,
	}
}