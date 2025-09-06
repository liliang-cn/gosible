# Vault: CLI Tool vs Library Usage

## Summary

**The vault CLI tool is NOT necessary** - it's just a convenience wrapper around the vault library. The entire vault functionality is available as a Go library that can be imported and used directly in your applications.

## Comparison

### When to Use the Library (Recommended for Go Apps)

```go
import "github.com/gosinble/gosinble/pkg/vault"

v := vault.New("password")
encrypted, err := v.Encrypt(data)
```

**Advantages:**
- ✅ **No subprocess overhead** - Direct function calls
- ✅ **Type safety** - Compile-time checking
- ✅ **Better error handling** - Go errors vs exit codes
- ✅ **Easier testing** - Mock the vault interface
- ✅ **Better performance** - No process spawning
- ✅ **Native integration** - Works seamlessly with Go code
- ✅ **Memory operations** - No temporary files needed

**Use Cases:**
- Embedding in Go applications
- Kubernetes operators
- CI/CD tools written in Go
- Microservices needing encryption
- Any Go application needing vault functionality

### When to Use the CLI Tool

```bash
gosinble-vault -action encrypt -password mysecret file.yml
```

**Advantages:**
- ✅ **Shell scripting** - Easy to use in bash scripts
- ✅ **Manual operations** - Quick one-off encryption/decryption
- ✅ **Ansible compatibility** - Drop-in replacement for ansible-vault
- ✅ **Language agnostic** - Can be called from any language
- ✅ **Standalone usage** - No coding required

**Use Cases:**
- Shell scripts and automation
- Manual operations by ops teams
- Non-Go applications (Python, Ruby, etc.)
- Quick command-line encryption tasks
- Existing Ansible workflows

## Code Examples

### Library Usage (Programmatic)

```go
// Create vault manager for multiple environments
manager := vault.NewManager()
manager.AddVault("dev", devPassword)
manager.AddVault("prod", prodPassword)

// Encrypt with specific vault ID
encrypted, err := manager.Encrypt(sensitiveData, "prod")
if err != nil {
    return fmt.Errorf("encryption failed: %w", err)
}

// Decrypt (auto-detects vault ID)
decrypted, err := manager.Decrypt(encrypted)
if err != nil {
    return fmt.Errorf("decryption failed: %w", err)
}

// Check if file is encrypted
if vault.IsVaultFile(data) {
    // Handle encrypted file
}
```

### CLI Usage (Command Line)

```bash
# Encrypt a file
gosinble-vault -action encrypt -password mysecret secrets.yml

# Decrypt to stdout
gosinble-vault -action view -password mysecret secrets.yml

# Edit encrypted file
gosinble-vault -action edit -password mysecret secrets.yml

# Change password
gosinble-vault -action rekey -password old -new-password new secrets.yml
```

## Integration Examples

### 1. Configuration Management (Library)

```go
type Config struct {
    Database struct {
        Host     string
        Username string
        Password string // This will be encrypted
    }
}

func (c *Config) Encrypt(v *vault.Vault) error {
    encrypted, err := v.Encrypt([]byte(c.Database.Password))
    if err != nil {
        return err
    }
    c.Database.Password = encrypted
    return nil
}
```

### 2. Deployment Pipeline (CLI)

```bash
#!/bin/bash
# deployment.sh

# Decrypt production secrets
gosinble-vault -action decrypt \
    -password-file /secure/vault-pass \
    -output /tmp/secrets.yml \
    encrypted/prod-secrets.yml

# Use decrypted secrets for deployment
kubectl apply -f /tmp/secrets.yml

# Clean up
rm /tmp/secrets.yml
```

### 3. Hybrid Approach

```go
// Go application that also supports CLI vault files
func LoadConfig(filename string) (*Config, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    
    // Check if it's vault encrypted
    if vault.IsVaultFile(data) {
        v := vault.New(os.Getenv("VAULT_PASSWORD"))
        data, err = v.DecryptFile(data)
        if err != nil {
            return nil, fmt.Errorf("failed to decrypt: %w", err)
        }
    }
    
    var config Config
    return &config, yaml.Unmarshal(data, &config)
}
```

## Performance Comparison

| Operation | Library | CLI Tool |
|-----------|---------|----------|
| Single encryption | ~1ms | ~50ms (process spawn) |
| Batch (100 items) | ~10ms | ~5000ms |
| Memory usage | Minimal | New process each time |
| Startup time | 0ms | ~40ms |

## Recommendations

### For Gosinble Users

1. **Go Applications**: Always use the library
   ```go
   import "github.com/gosinble/gosinble/pkg/vault"
   ```

2. **Shell Scripts**: Use the CLI tool
   ```bash
   gosinble-vault -action encrypt ...
   ```

3. **Mixed Environments**: Provide both
   - Library for application code
   - CLI for operational tasks

### Migration Path

If you're currently using the CLI from Go:

```go
// OLD: Using CLI from Go (not recommended)
cmd := exec.Command("gosinble-vault", "-action", "encrypt", "-password", pass, file)
output, err := cmd.Output()

// NEW: Using library directly (recommended)
v := vault.New(pass)
encrypted, err := v.Encrypt(data)
```

## Conclusion

The vault CLI tool (`cmd/vault/`) is **optional** and exists primarily for:
- Compatibility with existing Ansible workflows
- Shell scripting convenience
- Manual operations

For Go applications, **always use the vault library directly** for:
- Better performance
- Type safety
- Easier testing
- Native integration

The CLI is just a thin wrapper that calls the same library functions, so you're not missing any functionality by using the library directly.