# Rune

Rune is a lightweight, powerful orchestration platform designed to simplify the deployment and management of services across environments.

## Features

- **Simple deployment**: Deploy services with a single command
- **Multi-environment**: Seamlessly run services across development, testing, and production
- **Lightweight**: Minimal resource footprint
- **Container-native**: First-class support for containerized applications
- **Process-aware**: Run and manage local processes when containers aren't needed
- **Dependency-aware**: Automatically manage service dependencies
- **Multi-node**: Scale across multiple nodes

## Getting Started

### Prerequisites

- Go 1.19 or later
- Docker (for container-based services)

### Installation

#### Quick Install (Recommended)

```bash
# Install CLI only (for developers)
curl -fsSL https://raw.githubusercontent.com/rzbill/rune/master/scripts/install-cli.sh | bash

# Install complete server environment (recommended for most users)
curl -fsSL https://raw.githubusercontent.com/rzbill/rune/master/scripts/install-server.sh | sudo bash -s -- --version v0.1.0

# Install binary-only (assumes Docker already configured)
curl -fsSL https://raw.githubusercontent.com/rzbill/rune/master/scripts/install.sh | sudo bash -s -- --version v0.1.0

# For automated deployment (cloud-init, CI/CD) - runs as root automatically
curl -fsSL https://raw.githubusercontent.com/rzbill/rune/master/scripts/install-server.sh | bash -s -- --version v0.1.0
```

#### From Source

```bash
# Install from source
go install github.com/rzbill/rune/cmd/rune@latest

# Verify installation
rune version
```

### Configuration

Rune uses a clean separation between server and client configuration:

#### Server Configuration (`runefile.yaml`)

The server configuration file contains registry authentication, Docker settings, and server options:

```bash
# Run with built-in defaults (auto-creates runefile.yaml)
runed

# Use custom server configuration
runed --config=/path/to/runefile.yaml
```

**Configuration locations (in order of precedence):**
1. **`--config` flag** - Explicitly specified file
2. **`./runefile.yaml`** - Local development override
3. **`/etc/rune/runefile.yaml`** - System-wide production config

**Example server configuration:**
```yaml
# runefile.yaml
server:
  grpc_address: ":7863"
  http_address: ":7861"

docker:
  registries:
    - name: "ecr-prod"
      registry: "123456789012.dkr.ecr.us-west-2.amazonaws.com"
      auth:
        type: "ecr"
        region: "us-west-2"

log:
  level: "info"
  format: "text"
```

#### Client Configuration (`$HOME/.rune/config.yaml`)

Client configuration manages connections to Rune servers:

```yaml
# $HOME/.rune/config.yaml
current-context: production
contexts:
  production:
    server: "https://rune.company.com:7863"
    token: "your-auth-token"
    defaultNamespace: "production"
```

**Client config locations:**
- **`$HOME/.rune/config.yaml`** - User's home directory
- **`$RUNE_CLI_CONFIG`** - Environment variable override

#### First Run

On first startup, Rune will:
1. **Auto-create** a default `runefile.yaml` if none exists
2. **Bootstrap** an admin user if no users exist
3. **Create** a CLI config file for the admin user

See the `examples/runefile.yaml` file for a complete server configuration example.

### Quick Start

1. Create a simple service definition:

```yaml
# service.yaml
name: hello-world
image: nginx:alpine
ports:
  - 8080:80
```

2. Deploy the service:

```bash
rune cast service.yaml
```

3. View the service:

```bash
rune trace hello-world
```

## CI/CD with GitHub Actions

Deploy services automatically using the Rune CLI in GitHub Actions:

```bash
# Copy the workflow to your repository
mkdir -p .github/workflows
cp examples/github-actions/rune-deploy.yml .github/workflows/

# Add your service definitions
mkdir services
cp examples/github-actions/services/example-app.yaml services/
```

See [CI/CD Guide](docs/guides/ci-cd-github-actions.md) for complete setup instructions.

## Documentation

For more detailed documentation:

- [User Guide](docs/guides/README.md)
- [API Documentation](docs/api/README.md)
- [Examples](examples/README.md)

## Development

### Setup Development Environment

```bash
# Clone the repository
git clone https://github.com/rzbill/rune.git
cd rune

# Install development dependencies
make setup

# Build the project
make build

# Run tests
make test
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. 