// Package vault provides Ansible Vault compatible encryption/decryption
package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	
	"golang.org/x/crypto/pbkdf2"
)

const (
	// VaultHeader is the Ansible Vault file header
	VaultHeader = "$ANSIBLE_VAULT"
	
	// DefaultVaultIDLabel is the default vault ID
	DefaultVaultIDLabel = "default"
	
	// Current vault format version
	VaultFormatVersion = "1.1"
	
	// AES256 is the cipher used by Ansible Vault
	VaultCipher = "AES256"
	
	// Salt length in bytes
	SaltLength = 32
	
	// Key derivation iterations
	KeyDerivationIterations = 10000
	
	// Derived key length
	DerivedKeyLength = 32
	
	// HMAC key length
	HMACKeyLength = 32
	
	// AES block size
	AESBlockSize = 16
)

var (
	// ErrInvalidVaultFormat indicates the vault format is invalid
	ErrInvalidVaultFormat = errors.New("invalid vault format")
	
	// ErrInvalidPassword indicates the password is incorrect
	ErrInvalidPassword = errors.New("invalid vault password")
	
	// ErrUnsupportedVersion indicates unsupported vault version
	ErrUnsupportedVersion = errors.New("unsupported vault version")
	
	// ErrInvalidPadding indicates invalid PKCS7 padding
	ErrInvalidPadding = errors.New("invalid padding")
)

// Vault provides encryption and decryption of Ansible Vault format
type Vault struct {
	password string
	vaultID  string
}

// New creates a new Vault with the given password
func New(password string) *Vault {
	return &Vault{
		password: password,
		vaultID:  DefaultVaultIDLabel,
	}
}

// NewWithVaultID creates a new Vault with the given password and vault ID
func NewWithVaultID(password, vaultID string) *Vault {
	return &Vault{
		password: password,
		vaultID:  vaultID,
	}
}

// Encrypt encrypts plaintext data in Ansible Vault format
func (v *Vault) Encrypt(plaintext []byte) (string, error) {
	// Generate random salt
	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	
	// Derive keys using PBKDF2
	derivedKey := pbkdf2.Key([]byte(v.password), salt, KeyDerivationIterations, 
		DerivedKeyLength+HMACKeyLength+AESBlockSize, sha256.New)
	
	// Split derived key into components
	aesKey := derivedKey[:DerivedKeyLength]
	hmacKey := derivedKey[DerivedKeyLength : DerivedKeyLength+HMACKeyLength]
	iv := derivedKey[DerivedKeyLength+HMACKeyLength : DerivedKeyLength+HMACKeyLength+AESBlockSize]
	
	// Pad plaintext using PKCS7
	paddedPlaintext := pkcs7Pad(plaintext, AESBlockSize)
	
	// Encrypt using AES256 in CTR mode
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	
	ciphertext := make([]byte, len(paddedPlaintext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext, paddedPlaintext)
	
	// Create vault payload: salt + HMAC + ciphertext
	vaultPayload := make([]byte, 0, SaltLength+sha256.Size+len(ciphertext))
	vaultPayload = append(vaultPayload, salt...)
	
	// Calculate HMAC
	h := hmac.New(sha256.New, hmacKey)
	h.Write(ciphertext)
	vaultPayload = append(vaultPayload, h.Sum(nil)...)
	vaultPayload = append(vaultPayload, ciphertext...)
	
	// Encode to hex
	hexPayload := hex.EncodeToString(vaultPayload)
	
	// Format as Ansible Vault
	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s;%s;%s\n", VaultHeader, VaultFormatVersion, VaultCipher))
	
	// Wrap hex string at 80 characters
	for i := 0; i < len(hexPayload); i += 80 {
		end := i + 80
		if end > len(hexPayload) {
			end = len(hexPayload)
		}
		result.WriteString(hexPayload[i:end])
		result.WriteString("\n")
	}
	
	return result.String(), nil
}

// Decrypt decrypts Ansible Vault format data
func (v *Vault) Decrypt(vaultData string) ([]byte, error) {
	lines := strings.Split(strings.TrimSpace(vaultData), "\n")
	if len(lines) < 2 {
		return nil, ErrInvalidVaultFormat
	}
	
	// Parse header
	header := lines[0]
	if !strings.HasPrefix(header, VaultHeader) {
		return nil, ErrInvalidVaultFormat
	}
	
	headerParts := strings.Split(header, ";")
	if len(headerParts) != 3 {
		return nil, ErrInvalidVaultFormat
	}
	
	version := headerParts[1]
	if version != VaultFormatVersion && version != "1.2" {
		return nil, ErrUnsupportedVersion
	}
	
	// Join hex lines and decode
	hexPayload := strings.Join(lines[1:], "")
	vaultPayload, err := hex.DecodeString(hexPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex: %w", err)
	}
	
	if len(vaultPayload) < SaltLength+sha256.Size {
		return nil, ErrInvalidVaultFormat
	}
	
	// Extract components
	salt := vaultPayload[:SaltLength]
	storedHMAC := vaultPayload[SaltLength : SaltLength+sha256.Size]
	ciphertext := vaultPayload[SaltLength+sha256.Size:]
	
	// Derive keys using PBKDF2
	derivedKey := pbkdf2.Key([]byte(v.password), salt, KeyDerivationIterations,
		DerivedKeyLength+HMACKeyLength+AESBlockSize, sha256.New)
	
	// Split derived key into components
	aesKey := derivedKey[:DerivedKeyLength]
	hmacKey := derivedKey[DerivedKeyLength : DerivedKeyLength+HMACKeyLength]
	iv := derivedKey[DerivedKeyLength+HMACKeyLength : DerivedKeyLength+HMACKeyLength+AESBlockSize]
	
	// Verify HMAC
	h := hmac.New(sha256.New, hmacKey)
	h.Write(ciphertext)
	calculatedHMAC := h.Sum(nil)
	
	if !hmac.Equal(storedHMAC, calculatedHMAC) {
		return nil, ErrInvalidPassword
	}
	
	// Decrypt using AES256 in CTR mode
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, ciphertext)
	
	// Remove PKCS7 padding
	plaintext, err = pkcs7Unpad(plaintext, AESBlockSize)
	if err != nil {
		return nil, err
	}
	
	return plaintext, nil
}

// EncryptFile encrypts a file's contents
func (v *Vault) EncryptFile(plaintext []byte) ([]byte, error) {
	encrypted, err := v.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	return []byte(encrypted), nil
}

// DecryptFile decrypts a file's contents
func (v *Vault) DecryptFile(ciphertext []byte) ([]byte, error) {
	return v.Decrypt(string(ciphertext))
}

// IsVaultFile checks if data is an Ansible Vault file
func IsVaultFile(data []byte) bool {
	return bytes.HasPrefix(data, []byte(VaultHeader))
}

// IsVaultString checks if a string is vault encrypted
func IsVaultString(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, VaultHeader) || strings.HasPrefix(s, "!vault |")
}

// pkcs7Pad adds PKCS7 padding to data
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// pkcs7Unpad removes PKCS7 padding from data
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, ErrInvalidPadding
	}
	
	padding := int(data[len(data)-1])
	if padding > blockSize || padding == 0 {
		return nil, ErrInvalidPadding
	}
	
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, ErrInvalidPadding
		}
	}
	
	return data[:len(data)-padding], nil
}

// VaultString represents an inline encrypted value
type VaultString struct {
	vault *Vault
	value string
}

// NewVaultString creates a new vault string
func NewVaultString(vault *Vault, value string) *VaultString {
	return &VaultString{
		vault: vault,
		value: value,
	}
}

// Encrypt encrypts the string value
func (vs *VaultString) Encrypt() (string, error) {
	encrypted, err := vs.vault.Encrypt([]byte(vs.value))
	if err != nil {
		return "", err
	}
	
	// Format as inline vault string with !vault tag
	lines := strings.Split(encrypted, "\n")
	var result strings.Builder
	result.WriteString("!vault |\n")
	for _, line := range lines {
		if line != "" {
			result.WriteString("          ")
			result.WriteString(line)
			result.WriteString("\n")
		}
	}
	
	return result.String(), nil
}

// Decrypt decrypts an inline vault string
func (vs *VaultString) Decrypt(encrypted string) (string, error) {
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
	decrypted, err := vs.vault.Decrypt(vaultData)
	if err != nil {
		return "", err
	}
	
	return string(decrypted), nil
}