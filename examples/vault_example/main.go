package main

import (
	"fmt"
	"log"
	"os"
	
	"github.com/liliang-cn/gosible/pkg/vault"
	"gopkg.in/yaml.v3"
)

func main() {
	// Example 1: Basic encryption/decryption
	fmt.Println("=== Basic Vault Encryption ===")
	basicVaultExample()
	
	// Example 2: Inline vault strings
	fmt.Println("\n=== Inline Vault Strings ===")
	inlineVaultExample()
	
	// Example 3: File encryption
	fmt.Println("\n=== File Encryption ===")
	fileVaultExample()
	
	// Example 4: Multiple vault IDs
	fmt.Println("\n=== Multiple Vault IDs ===")
	multiVaultExample()
	
	// Example 5: Variables with vault values
	fmt.Println("\n=== Variables with Vault Values ===")
	variableVaultExample()
}

func basicVaultExample() {
	// Create vault with password
	v := vault.New("my_secret_password")
	
	// Encrypt data
	plaintext := "This is sensitive information!"
	encrypted, err := v.Encrypt([]byte(plaintext))
	if err != nil {
		log.Fatalf("Encryption failed: %v", err)
	}
	
	fmt.Printf("Original: %s\n", plaintext)
	fmt.Printf("Encrypted (first 80 chars):\n%s...\n", encrypted[:80])
	
	// Decrypt data
	decrypted, err := v.Decrypt(encrypted)
	if err != nil {
		log.Fatalf("Decryption failed: %v", err)
	}
	
	fmt.Printf("Decrypted: %s\n", string(decrypted))
}

func inlineVaultExample() {
	// Create vault
	v := vault.New("my_secret_password")
	
	// Create inline vault string (for use in YAML files)
	vs := vault.NewVaultString(v, "super_secret_api_key_12345")
	
	// Encrypt as inline vault string
	encrypted, err := vs.Encrypt()
	if err != nil {
		log.Fatalf("Encryption failed: %v", err)
	}
	
	fmt.Printf("Inline vault string:\n%s", encrypted)
	
	// Decrypt inline vault string
	decrypted, err := vs.Decrypt(encrypted)
	if err != nil {
		log.Fatalf("Decryption failed: %v", err)
	}
	
	fmt.Printf("Decrypted value: %s\n", decrypted)
}

func fileVaultExample() {
	// Create temp directory
	tmpDir := "/tmp/vault_example"
	os.MkdirAll(tmpDir, 0755)
	
	// Create a sample YAML file
	secretsFile := tmpDir + "/secrets.yml"
	secrets := map[string]interface{}{
		"database_password": "db_pass_123",
		"api_key":          "api_key_456",
		"admin_token":      "admin_token_789",
	}
	
	data, _ := yaml.Marshal(secrets)
	os.WriteFile(secretsFile, data, 0600)
	
	fmt.Printf("Original file content:\n%s\n", data)
	
	// Create vault manager
	manager := vault.NewManager()
	manager.AddVault("default", "file_password")
	
	// Encrypt the file
	err := manager.EncryptFile(secretsFile, "default")
	if err != nil {
		log.Fatalf("File encryption failed: %v", err)
	}
	
	// Read encrypted file
	encrypted, _ := os.ReadFile(secretsFile)
	fmt.Printf("Encrypted file (first 160 chars):\n%s...\n", encrypted[:160])
	
	// Decrypt the file
	decrypted, err := manager.DecryptFile(secretsFile)
	if err != nil {
		log.Fatalf("File decryption failed: %v", err)
	}
	
	fmt.Printf("Decrypted content:\n%s\n", decrypted)
	
	// Cleanup
	os.RemoveAll(tmpDir)
}

func multiVaultExample() {
	// Create manager with multiple vault IDs
	manager := vault.NewManager()
	manager.AddVault("dev", "dev_password")
	manager.AddVault("prod", "prod_password")
	manager.AddVault("staging", "staging_password")
	
	// Encrypt different data with different vault IDs
	devSecret := "dev database connection string"
	prodSecret := "production API key"
	
	devEncrypted, err := manager.Encrypt([]byte(devSecret), "dev")
	if err != nil {
		log.Fatalf("Dev encryption failed: %v", err)
	}
	
	prodEncrypted, err := manager.Encrypt([]byte(prodSecret), "prod")
	if err != nil {
		log.Fatalf("Prod encryption failed: %v", err)
	}
	
	fmt.Println("Encrypted dev secret (first 80 chars):")
	fmt.Printf("%s...\n", devEncrypted[:80])
	
	fmt.Println("Encrypted prod secret (first 80 chars):")
	fmt.Printf("%s...\n", prodEncrypted[:80])
	
	// Manager can decrypt both automatically
	devDecrypted, err := manager.Decrypt(devEncrypted)
	if err != nil {
		log.Fatalf("Dev decryption failed: %v", err)
	}
	
	prodDecrypted, err := manager.Decrypt(prodEncrypted)
	if err != nil {
		log.Fatalf("Prod decryption failed: %v", err)
	}
	
	fmt.Printf("Decrypted dev: %s\n", devDecrypted)
	fmt.Printf("Decrypted prod: %s\n", prodDecrypted)
}

func variableVaultExample() {
	// Create manager
	manager := vault.NewManager()
	manager.AddVault("default", "var_password")
	
	// Create vault for inline strings
	v := vault.New("var_password")
	vs := vault.NewVaultString(v, "secret_value")
	encryptedValue, _ := vs.Encrypt()
	
	// Create variables with mix of plain and encrypted values
	vars := map[string]interface{}{
		"plain_var": "plain_value",
		"secret_var": encryptedValue,
		"config": map[string]interface{}{
			"host": "localhost",
			"password": encryptedValue,
			"port": 5432,
		},
		"tokens": []interface{}{
			"public_token",
			encryptedValue,
		},
	}
	
	fmt.Println("Variables before processing:")
	yamlOutput, _ := yaml.Marshal(vars)
	fmt.Printf("%s\n", yamlOutput)
	
	// Process variables to decrypt vault values
	err := manager.ProcessVariables(vars)
	if err != nil {
		log.Fatalf("Failed to process variables: %v", err)
	}
	
	fmt.Println("Variables after processing:")
	yamlOutput, _ = yaml.Marshal(vars)
	fmt.Printf("%s\n", yamlOutput)
}