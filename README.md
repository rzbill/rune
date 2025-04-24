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

```bash
# Install from source
go install github.com/rzbill/rune/cmd/rune@latest

# Verify installation
rune version
```

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