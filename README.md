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

### Configuration

Rune has sensible defaults built into the application, so configuration files are completely optional. You can run the server without any additional setup:

```bash
# Run with built-in defaults
runed
```

To customize settings, you can:

1. Use command-line flags (for simple overrides):

```bash
# Override the gRPC port and data directory
runed --grpc-addr=":9090" --data-dir="/path/to/data"
```

2. Use a configuration file (for more complex setups):

```yaml
# rune.yaml
server:
  grpc_address: ":9090"
  http_address: ":9091"
  
data_dir: "/path/to/data"

# Additional settings...
```

Then run with:

```bash
# Point to your config file
runed --config=/path/to/rune.yaml
```

The server looks for a `rune.yaml` file in the current directory, `~/.rune/`, or `/etc/rune/`, but you can always specify a custom location with the `--config` flag.

See the `examples/config/rune.yaml` file for a complete example configuration.

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