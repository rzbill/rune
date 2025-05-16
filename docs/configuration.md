# Rune Configuration Guide

Rune provides a flexible configuration system that follows a clear precedence model:

1. Command-line flags
2. Environment variables
3. Configuration file
4. Default values

## Configuration File

Rune looks for configuration in the following locations:
- Path specified with `--config` flag
- `/etc/rune/rune.yaml`
- `$HOME/.rune/rune.yaml`
- `./rune.yaml` (in the current directory)

## Configuration Structure

```yaml
# Server endpoints
server:
  # gRPC endpoint for API services
  grpc_address: ":8080"
  
  # HTTP endpoint for REST gateway
  http_address: ":8081"
  
  # TLS configuration
  tls:
    enabled: false
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"

# Data directory for persistent storage
data_dir: "/var/lib/rune"

# Docker runner configuration
docker:
  # API version to use (if not specified, auto-negotiation is used)
  # api_version: "1.43"
  
  # Fallback API version to use if auto-negotiation fails
  fallback_api_version: "1.43"
  
  # Timeout for API version negotiation in seconds
  negotiation_timeout_seconds: 3

# Default namespace
namespace: "default"

# Authentication configuration
auth:
  # Comma-separated list of API keys
  api_keys: ""
  
  # Authentication provider (token, oidc, none)
  provider: "token"
  
  # Static token for simple auth
  token: ""

# Logging configuration
log:
  # Log level (debug, info, warn, error)
  level: "info"
  
  # Log format (text, json)
  format: "text"
```

## Environment Variables

All configuration options can be set via environment variables using the `RUNE_` prefix and underscores instead of dots. Examples:

- `RUNE_SERVER_GRPC_ADDRESS=":8443"`
- `RUNE_LOG_LEVEL="debug"`
- `RUNE_AUTH_API_KEYS="key1,key2"`

## Docker Configuration

Rune supports configuring Docker API version settings to ensure compatibility with different Docker daemon versions:

| Configuration Option | Environment Variable | Description | Default |
|---------------------|----------------------|-------------|---------|
| `docker.api_version` | `RUNE_DOCKER_API_VERSION` | Specific Docker API version to use | Auto-negotiation |
| `docker.fallback_api_version` | `RUNE_DOCKER_FALLBACK_API_VERSION` | Fallback API version when negotiation fails | "1.43" |
| `docker.negotiation_timeout_seconds` | `RUNE_DOCKER_NEGOTIATION_TIMEOUT` | Timeout seconds for API version negotiation | 3 |

### Docker API Version Selection Process

Rune uses the following process to determine which Docker API version to use:

1. If `docker.api_version` is explicitly set (via flag, env var, or config), that exact version is used.
2. If not set, Rune attempts to auto-negotiate with the Docker daemon to find the highest supported version.
3. If negotiation encounters compatibility issues, the `fallback_api_version` is used.

This approach ensures maximum compatibility across different Docker deployments while still allowing users to specify exact versions when needed.

## Command-line Flags

Rune provides several command-line flags that override any configuration from files or environment variables:

```
--config string       Configuration file path
--grpc-addr string    gRPC server address (default ":8080")
--http-addr string    HTTP server address (default ":8081")
--data-dir string     Data directory
--log-level string    Log level (debug, info, warn, error) (default "info")
--debug               Enable debug mode (shorthand for --log-level=debug)
--log-format string   Log format (text, json) (default "text")
--pretty              Enable pretty text log format
--api-keys string     Comma-separated list of API keys (empty to disable auth)
--help                Show help
--version             Show version
``` 