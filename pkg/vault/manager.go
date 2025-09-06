package vault

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Manager manages multiple vault passwords and IDs
type Manager struct {
	vaults map[string]*Vault
	defaultVaultID string
}

// NewManager creates a new vault manager
func NewManager() *Manager {
	return &Manager{
		vaults: make(map[string]*Vault),
		defaultVaultID: DefaultVaultIDLabel,
	}
}

// AddVault adds a vault with the given ID and password
func (m *Manager) AddVault(vaultID, password string) {
	if vaultID == "" {
		vaultID = DefaultVaultIDLabel
	}
	m.vaults[vaultID] = NewWithVaultID(password, vaultID)
}

// AddVaultFromFile reads a password from a file
func (m *Manager) AddVaultFromFile(vaultID, filename string) error {
	password, err := m.readPasswordFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read password file: %w", err)
	}
	
	m.AddVault(vaultID, password)
	return nil
}

// AddVaultFromScript executes a script to get the password
func (m *Manager) AddVaultFromScript(vaultID, script string) error {
	cmd := exec.Command("sh", "-c", script)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute password script: %w", err)
	}
	
	password := strings.TrimSpace(string(output))
	m.AddVault(vaultID, password)
	return nil
}

// SetDefaultVaultID sets the default vault ID
func (m *Manager) SetDefaultVaultID(vaultID string) {
	m.defaultVaultID = vaultID
}

// GetVault returns a vault by ID
func (m *Manager) GetVault(vaultID string) (*Vault, error) {
	if vaultID == "" {
		vaultID = m.defaultVaultID
	}
	
	vault, exists := m.vaults[vaultID]
	if !exists {
		return nil, fmt.Errorf("vault ID '%s' not found", vaultID)
	}
	
	return vault, nil
}

// Encrypt encrypts data with the specified vault ID
func (m *Manager) Encrypt(data []byte, vaultID string) (string, error) {
	vault, err := m.GetVault(vaultID)
	if err != nil {
		return "", err
	}
	
	return vault.Encrypt(data)
}

// Decrypt attempts to decrypt data with available vaults
func (m *Manager) Decrypt(vaultData string) ([]byte, error) {
	// Extract vault ID from header if present
	vaultID := m.extractVaultID(vaultData)
	
	if vaultID != "" {
		// Try specific vault ID first
		if vault, exists := m.vaults[vaultID]; exists {
			result, err := vault.Decrypt(vaultData)
			if err == nil {
				return result, nil
			}
		}
	}
	
	// Try all vaults
	var lastErr error
	for _, vault := range m.vaults {
		result, err := vault.Decrypt(vaultData)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	
	if lastErr != nil {
		return nil, lastErr
	}
	
	return nil, ErrInvalidPassword
}

// DecryptFile decrypts a file
func (m *Manager) DecryptFile(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	if !IsVaultFile(data) {
		// File is not encrypted
		return data, nil
	}
	
	return m.Decrypt(string(data))
}

// EncryptFile encrypts a file
func (m *Manager) EncryptFile(filename string, vaultID string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	if IsVaultFile(data) {
		return fmt.Errorf("file is already encrypted")
	}
	
	encrypted, err := m.Encrypt(data, vaultID)
	if err != nil {
		return err
	}
	
	return os.WriteFile(filename, []byte(encrypted), 0600)
}

// DecryptToFile decrypts a file and saves the result
func (m *Manager) DecryptToFile(inputFile, outputFile string) error {
	decrypted, err := m.DecryptFile(inputFile)
	if err != nil {
		return err
	}
	
	return os.WriteFile(outputFile, decrypted, 0600)
}

// Rekey changes the encryption password for a file
func (m *Manager) Rekey(filename string, oldVaultID, newVaultID string) error {
	// Decrypt with old password
	oldVault, err := m.GetVault(oldVaultID)
	if err != nil {
		return fmt.Errorf("old vault not found: %w", err)
	}
	
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	if !IsVaultFile(data) {
		return fmt.Errorf("file is not encrypted")
	}
	
	decrypted, err := oldVault.Decrypt(string(data))
	if err != nil {
		return fmt.Errorf("failed to decrypt with old password: %w", err)
	}
	
	// Encrypt with new password
	newVault, err := m.GetVault(newVaultID)
	if err != nil {
		return fmt.Errorf("new vault not found: %w", err)
	}
	
	encrypted, err := newVault.Encrypt(decrypted)
	if err != nil {
		return fmt.Errorf("failed to encrypt with new password: %w", err)
	}
	
	return os.WriteFile(filename, []byte(encrypted), 0600)
}

// View decrypts and displays a file
func (m *Manager) View(filename string, w io.Writer) error {
	decrypted, err := m.DecryptFile(filename)
	if err != nil {
		return err
	}
	
	_, err = w.Write(decrypted)
	return err
}

// Edit decrypts a file for editing and re-encrypts after
func (m *Manager) Edit(filename string) error {
	// Decrypt file
	decrypted, err := m.DecryptFile(filename)
	if err != nil {
		return err
	}
	
	// Create temp file
	tmpFile, err := os.CreateTemp("", "vault-edit-*.yml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	
	// Write decrypted content
	if _, err := tmpFile.Write(decrypted); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()
	
	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	
	// Open editor
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run editor: %w", err)
	}
	
	// Read edited content
	edited, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}
	
	// Re-encrypt
	vaultID := m.extractVaultID(string(decrypted))
	if vaultID == "" {
		vaultID = m.defaultVaultID
	}
	
	encrypted, err := m.Encrypt(edited, vaultID)
	if err != nil {
		return fmt.Errorf("failed to re-encrypt: %w", err)
	}
	
	return os.WriteFile(filename, []byte(encrypted), 0600)
}

// readPasswordFile reads a password from a file
func (m *Manager) readPasswordFile(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	
	if err := scanner.Err(); err != nil {
		return "", err
	}
	
	return "", fmt.Errorf("empty password file")
}

// extractVaultID extracts vault ID from vault data header
func (m *Manager) extractVaultID(vaultData string) string {
	lines := strings.Split(vaultData, "\n")
	if len(lines) == 0 {
		return ""
	}
	
	header := lines[0]
	if !strings.HasPrefix(header, VaultHeader) {
		return ""
	}
	
	// Format: $ANSIBLE_VAULT;1.2;AES256;vault_id
	parts := strings.Split(header, ";")
	if len(parts) >= 4 {
		return parts[3]
	}
	
	return ""
}

// ProcessVariables decrypts any vault-encrypted values in a variables map
func (m *Manager) ProcessVariables(vars map[string]interface{}) error {
	return m.processVariablesRecursive(vars)
}

// processVariablesRecursive recursively processes variables
func (m *Manager) processVariablesRecursive(data interface{}) error {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if strValue, ok := value.(string); ok && IsVaultString(strValue) {
				// Decrypt inline vault string
				decrypted, err := m.decryptInlineVault(strValue)
				if err != nil {
					return fmt.Errorf("failed to decrypt variable '%s': %w", key, err)
				}
				v[key] = decrypted
			} else {
				// Recurse into nested structures
				if err := m.processVariablesRecursive(value); err != nil {
					return err
				}
			}
		}
	case []interface{}:
		for i, item := range v {
			if strItem, ok := item.(string); ok && IsVaultString(strItem) {
				// Decrypt inline vault string
				decrypted, err := m.decryptInlineVault(strItem)
				if err != nil {
					return fmt.Errorf("failed to decrypt array item %d: %w", i, err)
				}
				v[i] = decrypted
			} else {
				// Recurse into nested structures
				if err := m.processVariablesRecursive(item); err != nil {
					return err
				}
			}
		}
	}
	
	return nil
}

// decryptInlineVault decrypts an inline vault string
func (m *Manager) decryptInlineVault(encrypted string) (string, error) {
	// Remove !vault tag and indentation
	encrypted = strings.TrimPrefix(encrypted, "!vault |")
	encrypted = strings.TrimPrefix(encrypted, "!vault |\n")
	
	lines := strings.Split(encrypted, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}
	
	vaultData := strings.Join(cleanLines, "\n")
	decrypted, err := m.Decrypt(vaultData)
	if err != nil {
		return "", err
	}
	
	return string(decrypted), nil
}