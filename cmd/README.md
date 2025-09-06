# Gosinble Command Line Tool

This directory contains the main command-line interface for gosinble.

## Available Command

### gosinble - Main CLI
**Location**: `cmd/gosinble/`
**Purpose**: Main gosinble CLI for running playbooks and ad-hoc commands

```bash
# Build
go build -o gosinble cmd/gosinble/main.go

# Usage
./gosinble -i inventory.yml -p playbook.yml
./gosinble -i inventory.yml -m shell -a "uptime" all
```

**Features**:
- Run playbooks with inventory
- Execute ad-hoc commands
- Variable management
- Check mode (dry run)
- Verbose output
- Parallel execution control
- Vault support (integrated via library)

## Building

```bash
# Build the CLI
go build -o bin/gosinble cmd/gosinble/main.go

# Or install to $GOPATH/bin
go install ./cmd/gosinble
```

## Installation

```bash
# Install to $GOPATH/bin
go install github.com/gosinble/gosinble/cmd/gosinble@latest

# Or copy built binary to PATH
cp bin/gosinble /usr/local/bin/
```

## Configuration

### Environment Variables
```bash
export GOSINBLE_CONFIG=/path/to/config.yml
export GOSINBLE_INVENTORY=/path/to/inventory
export GOSINBLE_VAULT_PASSWORD_FILE=/path/to/vault_pass
```

### Configuration File
Create `~/.gosinble.yml`:
```yaml
defaults:
  inventory: /path/to/inventory
  remote_user: ubuntu
  private_key_file: ~/.ssh/id_rsa
  host_key_checking: false
  parallel: 10
  timeout: 30
```

## Examples

### Running Playbooks
```bash
# Basic playbook execution
gosinble -i hosts.yml -p deploy.yml

# With extra variables
gosinble -i hosts.yml -p deploy.yml -e "version=1.2.3"

# Check mode (dry run)
gosinble -i hosts.yml -p deploy.yml --check

# Verbose output
gosinble -i hosts.yml -p deploy.yml -v
```

### Ad-hoc Commands
```bash
# Run command on all hosts
gosinble -i hosts.yml -m shell -a "uptime" all

# Copy file to servers
gosinble -i hosts.yml -m copy -a "src=app.conf dest=/etc/app.conf" webservers

# Install package
gosinble -i hosts.yml -m package -a "name=nginx state=present" webservers

# Restart service
gosinble -i hosts.yml -m service -a "name=nginx state=restarted" webservers
```

### Vault Operations (Using Library)

The vault functionality is integrated directly into gosinble via the library. For programmatic vault operations, see the [vault library usage example](../examples/vault-library-usage/).

For manual vault operations in shell scripts or command line, you can:

1. **Use the library programmatically** (recommended for Go applications)
   ```go
   import "github.com/gosinble/gosinble/pkg/vault"
   ```

2. **Create a simple vault utility** using the library if needed:
   ```go
   // See examples/vault-library-usage/main.go
   ```

## Development

### Testing the CLI
```bash
# Run tests
go test ./cmd/gosinble/...

# Build and test locally
go build -o gosinble cmd/gosinble/main.go
./gosinble --version
```

## Comparison with Ansible

| Feature | Gosinble | Ansible |
|---------|----------|---------|
| Language | Go | Python |
| Performance | Fast (compiled) | Slower (interpreted) |
| Dependencies | Single binary | Python + dependencies |
| Modules | Go functions | Python scripts |
| Playbooks | YAML (compatible) | YAML |
| Vault | Library integrated | Separate tool |
| API | Native Go library | Python API |

## Why Only One CLI?

Gosinble follows the **library-first** philosophy:

1. **Core functionality as library**: All features (including vault) are available as importable Go packages
2. **Single CLI for operations**: One command for running playbooks and ad-hoc commands
3. **Programmatic access preferred**: For Go applications, use the library directly for better performance and type safety

This design provides:
- Simpler deployment (single binary)
- Better performance (no subprocess calls)
- Easier testing and mocking
- Type-safe integration for Go applications

## License

Part of the gosinble project - a Go implementation of Ansible's core functionality.