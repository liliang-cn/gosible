# Vault Library Usage

This example demonstrates how to use the gosible vault package as a Go library for programmatic encryption/decryption, without needing the CLI tool.

## Features Demonstrated

1. **Basic Encryption/Decryption** - Simple encrypt and decrypt operations
2. **Vault Manager** - File operations with vault IDs
3. **YAML Encryption** - Encrypting specific fields in YAML structures
4. **Vault Strings** - Inline encryption for specific values
5. **Multiple Vault IDs** - Managing different passwords for different environments

## Running the Example

```bash
go run examples/vault-library-usage/main.go
```

## Use Cases

### Embedding in Applications

Instead of using the CLI tool:

```bash
# CLI approach (requires external command)
gosible-vault -action encrypt -password mysecret file.yml
```

Use the library directly:

```go
import "github.com/liliang-cn/gosible/pkg/vault"

func encryptConfig(data []byte, password string) (string, error) {
    v := vault.New(password)
    return v.Encrypt(data)
}
```

### Configuration Management

```go
// Encrypt sensitive configuration
func protectConfig(config map[string]interface{}) error {
    v := vault.New(os.Getenv("VAULT_PASSWORD"))

    // Encrypt password field
    if password, ok := config["password"].(string); ok {
        encrypted, err := v.Encrypt([]byte(password))
        if err != nil {
            return err
        }
        config["password"] = encrypted
    }

    return nil
}
```

### Multi-Environment Secrets

```go
// Different vault IDs for different environments
manager := vault.NewManager()
manager.AddVault("dev", vault.New(devPassword))
manager.AddVault("prod", vault.New(prodPassword))

// Encrypt with specific vault ID
devSecret, _ := manager.Encrypt(data, "dev")
prodSecret, _ := manager.Encrypt(data, "prod")
```

## Advantages of Library Usage

### Over CLI Tool

| Aspect             | Library               | CLI Tool                   |
| ------------------ | --------------------- | -------------------------- |
| **Performance**    | Direct function calls | Subprocess overhead        |
| **Error Handling** | Go errors             | Exit codes + parsing       |
| **Integration**    | Native Go types       | String parsing             |
| **Testing**        | Easy mocking          | Complex subprocess mocking |
| **Type Safety**    | Compile-time checks   | Runtime errors             |

### API Benefits

- **No External Dependencies**: No need to ship CLI binary
- **Programmatic Control**: Full control over encryption process
- **Batch Operations**: Encrypt multiple items efficiently
- **Custom Workflows**: Build your own encryption workflows
- **Memory Operations**: No temporary files needed

## Common Patterns

### Secret Rotation

```go
func rotateSecrets(oldPassword, newPassword string) error {
    oldVault := vault.New(oldPassword)
    newVault := vault.New(newPassword)

    // Decrypt with old password
    decrypted, err := oldVault.Decrypt(encryptedData)
    if err != nil {
        return err
    }

    // Re-encrypt with new password
    reencrypted, err := newVault.Encrypt(decrypted)
    return err
}
```

### Conditional Encryption

```go
func processConfig(config map[string]interface{}, v *vault.Vault) {
    for key, value := range config {
        if shouldEncrypt(key) {
            if str, ok := value.(string); ok {
                encrypted, _ := v.Encrypt([]byte(str))
                config[key] = encrypted
            }
        }
    }
}
```

### Vault Detection

```go
func loadConfig(data []byte) ([]byte, error) {
    if vault.IsVaultFile(data) {
        // It's encrypted, decrypt it
        v := vault.New(getPassword())
        return v.DecryptFile(data)
    }
    // Not encrypted, use as-is
    return data, nil
}
```

## Integration with gosible

The vault functionality integrates seamlessly with other gosible components:

```go
// Use vault with playbooks
runner := runner.NewTaskRunner()
vaultManager := vault.NewManager()
vaultManager.AddVault("default", vault.New(password))

// Playbook variables are automatically decrypted
runner.SetVaultManager(vaultManager)
runner.RunPlaybook(ctx, playbook, inventory)
```

## Security Best Practices

1. **Never hardcode passwords** - Use environment variables or secure key management
2. **Use different vault IDs** - Separate passwords for different environments
3. **Rotate passwords regularly** - Use the rekey functionality
4. **Limit access** - Only decrypt when necessary
5. **Audit usage** - Log vault operations for security monitoring

## Conclusion

The vault CLI tool (`cmd/vault/main.go`) is simply a convenience wrapper around the vault library. For production Go applications, using the library directly provides better performance, type safety, and integration. The CLI tool is useful for:

- Manual operations by ops teams
- Shell scripts and automation
- Quick encryption/decryption tasks
- Compatibility with existing Ansible workflows

But for Go applications, always prefer the library approach for optimal results.
