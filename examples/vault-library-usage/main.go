package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/gosinble/gosinble/pkg/vault"
)

// This example demonstrates how to use the vault package as a library
// instead of using the CLI command
func main() {
	fmt.Println("=== Gosinble Vault Library Usage ===")

	// Example 1: Basic encryption and decryption
	basicEncryptionExample()

	// Example 2: Using Vault Manager for file operations
	vaultManagerExample()

	// Example 3: Working with YAML files
	yamlVaultExample()

	// Example 4: Vault strings (encrypt specific values)
	vaultStringExample()

	// Example 5: Multiple vault IDs
	multipleVaultIDsExample()
}

func basicEncryptionExample() {
	fmt.Println("1. Basic Encryption/Decryption")
	fmt.Println("-------------------------------")

	// Create a vault with password
	password := "mySecretPassword123"
	v := vault.New(password)

	// Original data
	originalData := []byte("This is my secret configuration data")
	fmt.Printf("Original: %s\n", originalData)

	// Encrypt the data
	encrypted, err := v.Encrypt(originalData)
	if err != nil {
		log.Printf("Encryption failed: %v", err)
		return
	}
	fmt.Printf("Encrypted: %s...\n", encrypted[:50]) // Show first 50 chars

	// Decrypt the data
	decrypted, err := v.Decrypt(encrypted)
	if err != nil {
		log.Printf("Decryption failed: %v", err)
		return
	}
	fmt.Printf("Decrypted: %s\n\n", decrypted)
}

func vaultManagerExample() {
	fmt.Println("2. Vault Manager for File Operations")
	fmt.Println("-------------------------------------")

	// Create a vault manager
	manager := vault.NewManager()
	
	// Add a vault with ID
	password := "productionPassword"
	manager.AddVault("production", password)
	
	// Sample data to encrypt
	secretData := []byte(`
database:
  host: prod-db.example.com
  username: admin
  password: SuperSecret123!
  port: 5432
`)

	// Encrypt data with specific vault ID
	encrypted, err := manager.Encrypt(secretData, "production")
	if err != nil {
		log.Printf("Manager encryption failed: %v", err)
		return
	}
	
	fmt.Println("Encrypted with vault ID 'production':")
	fmt.Printf("%s...\n", encrypted[:80]) // Show first 80 chars

	// Decrypt data (manager auto-detects vault ID)
	decrypted, err := manager.Decrypt(encrypted)
	if err != nil {
		log.Printf("Manager decryption failed: %v", err)
		return
	}
	
	fmt.Println("Decrypted content:")
	fmt.Printf("%s\n", decrypted)
}

func yamlVaultExample() {
	fmt.Println("3. YAML File Encryption")
	fmt.Println("------------------------")

	// Create manager with vault
	manager := vault.NewManager()
	manager.AddVault("default", "yamlPassword")

	// Example: Encrypt specific keys in a YAML structure
	yamlData := map[string]interface{}{
		"application": "myapp",
		"environment": "production",
		"database": map[string]interface{}{
			"host":     "db.example.com",
			"username": "dbuser",
			"password": "secretPassword", // This should be encrypted
		},
		"api_key": "sk-1234567890abcdef", // This should be encrypted
	}

	fmt.Println("Original YAML structure:")
	printYAML(yamlData, "  ")

	// Encrypt specific fields
	if dbConfig, ok := yamlData["database"].(map[string]interface{}); ok {
		if password, ok := dbConfig["password"].(string); ok {
			v, err := manager.GetVault("default")
			if err != nil {
				log.Printf("Failed to get vault: %v", err)
				return
			}
			encrypted, _ := v.Encrypt([]byte(password))
			dbConfig["password"] = fmt.Sprintf("!vault |\n%s", encrypted)
		}
	}

	if apiKey, ok := yamlData["api_key"].(string); ok {
		v, err := manager.GetVault("default")
		if err != nil {
			log.Printf("Failed to get vault: %v", err)
			return
		}
		encrypted, _ := v.Encrypt([]byte(apiKey))
		yamlData["api_key"] = fmt.Sprintf("!vault |\n%s", encrypted)
	}

	fmt.Println("\nYAML with encrypted values:")
	printYAML(yamlData, "  ")
}

func vaultStringExample() {
	fmt.Println("4. Vault Strings (Inline Encryption)")
	fmt.Println("-------------------------------------")

	// Create vault
	v := vault.New("inlinePassword")

	// Create vault strings for specific values
	secrets := []string{
		"API_KEY=sk-1234567890",
		"DB_PASSWORD=myDatabasePassword",
		"JWT_SECRET=myJwtSecretKey",
	}

	fmt.Println("Original secrets:")
	for _, secret := range secrets {
		fmt.Printf("  %s\n", secret)
	}

	fmt.Println("\nEncrypted inline:")
	encryptedSecrets := make([]string, 0)
	for _, secret := range secrets {
		parts := strings.Split(secret, "=")
		if len(parts) == 2 {
			vs := vault.NewVaultString(v, parts[1])
			encrypted, err := vs.Encrypt()
			if err != nil {
				log.Printf("Failed to encrypt %s: %v", parts[0], err)
				continue
			}
			encryptedSecret := fmt.Sprintf("%s=!vault %s", parts[0], encrypted)
			encryptedSecrets = append(encryptedSecrets, encryptedSecret)
			fmt.Printf("  %s\n", encryptedSecret[:50]+"...") // Truncate for display
		}
	}

	fmt.Println("\nDecrypted values:")
	for _, encrypted := range encryptedSecrets {
		parts := strings.Split(encrypted, "=!vault ")
		if len(parts) == 2 {
			vs := vault.NewVaultString(v, "")
			decrypted, err := vs.Decrypt(parts[1])
			if err != nil {
				log.Printf("Failed to decrypt %s: %v", parts[0], err)
				continue
			}
			fmt.Printf("  %s=%s\n", parts[0], decrypted)
		}
	}
	fmt.Println()
}

func multipleVaultIDsExample() {
	fmt.Println("5. Multiple Vault IDs")
	fmt.Println("---------------------")

	// Create manager with multiple vaults
	manager := vault.NewManager()
	
	// Add different vaults for different environments
	manager.AddVault("dev", "devPassword")
	manager.AddVault("staging", "stagingPassword")
	manager.AddVault("prod", "prodPassword")

	// Different secrets for different environments
	environments := map[string]string{
		"dev":     "Development database password",
		"staging": "Staging database password",
		"prod":    "Production database password",
	}

	fmt.Println("Encrypting with different vault IDs:")
	encrypted := make(map[string]string)
	for env, secret := range environments {
		enc, err := manager.Encrypt([]byte(secret), env)
		if err != nil {
			log.Printf("Failed to encrypt for %s: %v", env, err)
			continue
		}
		encrypted[env] = enc
		fmt.Printf("  %s: Encrypted with vault ID '%s'\n", env, env)
	}

	fmt.Println("\nDecrypting (auto-detects vault ID):")
	for env, enc := range encrypted {
		dec, err := manager.Decrypt(enc)
		if err != nil {
			log.Printf("Failed to decrypt %s: %v", env, err)
			continue
		}
		fmt.Printf("  %s: %s\n", env, dec)
	}
}

// Helper function to print YAML-like structure
func printYAML(data map[string]interface{}, indent string) {
	for key, value := range data {
		switch v := value.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", indent, key)
			printYAML(v, indent+"  ")
		case string:
			// Check if it's a vault string
			if strings.HasPrefix(v, "!vault") {
				// Truncate encrypted content for display
				lines := strings.Split(v, "\n")
				if len(lines) > 1 {
					fmt.Printf("%s%s: !vault |<encrypted>\n", indent, key)
				} else {
					fmt.Printf("%s%s: %s\n", indent, key, v)
				}
			} else {
				fmt.Printf("%s%s: %s\n", indent, key, v)
			}
		default:
			fmt.Printf("%s%s: %v\n", indent, key, v)
		}
	}
}

func init() {
	fmt.Print(`╔══════════════════════════════════════════════════════════════╗
║         Gosinble Vault - Library Usage Examples             ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  The vault package can be used directly as a Go library     ║
║  without needing the CLI command. This provides:            ║
║                                                              ║
║  • Programmatic encryption/decryption                       ║
║  • Integration with your Go applications                    ║
║  • Multiple vault ID management                             ║
║  • Ansible vault format compatibility                       ║
║  • No subprocess overhead                                   ║
║                                                              ║
║  The CLI tool (cmd/vault) is just a wrapper around this     ║
║  library functionality for command-line convenience.        ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
`)
}