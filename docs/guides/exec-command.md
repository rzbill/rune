# Rune Exec Command Guide

The `rune exec` command provides powerful debugging and maintenance capabilities for the Rune platform, allowing you to execute commands directly within running service instances. This guide covers the implementation details, usage patterns, and best practices.

## Overview

The exec command enables developers to:
- Execute interactive shell sessions in running instances
- Run one-off commands for debugging and maintenance
- Access specific instances or auto-select from services
- Set environment variables and working directories
- Handle terminal resizing and signal forwarding

## Command Structure

```bash
rune exec TARGET COMMAND [args...] [flags]
```

### Arguments

- `TARGET`: Service name or instance ID (e.g., `api`, `api-instance-123`)
- `COMMAND`: Command to execute (e.g., `bash`, `ls`, `ps`)
- `args...`: Additional arguments for the command

### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--namespace` | `-n` | Namespace of the service/instance | `default` |
| `--workdir` | | Working directory for the command | |
| `--env` | | Environment variables to set (key=value format) | |
| `--tty` | `-t` | Allocate a pseudo-TTY | auto-detected |
| `--no-tty` | | Disable TTY allocation | |
| `--timeout` | | Timeout for the exec session | `5m` |
| `--api-server` | | Address of the API server | `localhost:8443` |
| `--token-file` | | Path to a file containing the bearer token (recommended) | |

## Usage Examples

### Interactive Shell Sessions

Start an interactive bash session in a service:
```bash
rune exec api bash
```

Execute in a specific instance:
```bash
rune exec api-instance-123 bash
```

### One-off Commands

List files in a service:
```bash
rune exec api ls -la /app
```

Check running processes:
```bash
rune exec api ps aux
```

### Environment and Working Directory

Set environment variables and working directory:
```bash
rune exec api --workdir=/app --env=DEBUG=true --env=LOG_LEVEL=debug python debug.py
```

### Non-interactive Commands

Disable TTY for script compatibility:
```bash
rune exec api --no-tty python script.py
```

### Timeout and Configuration

Set a custom timeout:
```bash
rune exec api --timeout=30s python long-running-script.py
```

Connect to a different API server:
```bash
rune exec api bash --api-server=192.168.1.100:8443
```

## Implementation Details

### Architecture

The exec command follows a client-server architecture:

1. **CLI Layer**: Parses arguments and manages user interaction
2. **Client Layer**: Handles gRPC communication with the API server
3. **Server Layer**: Routes exec requests to appropriate instances
4. **Runner Layer**: Executes commands in containers or processes

### Key Components

#### ExecClient (`pkg/api/client/exec_client.go`)

The `ExecClient` provides the core functionality for executing commands:

```go
type ExecClient struct {
    client *Client
    logger log.Logger
    exec   generated.ExecServiceClient
}
```

Key methods:
- `NewExecSession()`: Creates a new exec session
- `Initialize()`: Sets up the exec session with options
- `RunInteractive()`: Runs an interactive TTY session
- `RunNonInteractive()`: Runs a non-interactive session

#### ExecSession

Manages the lifecycle of an exec session:

```go
type ExecSession struct {
    stream generated.ExecService_StreamExecClient
    client *ExecClient
    logger log.Logger
}
```

#### ExecOptions

Configuration for command execution:

```go
type ExecOptions struct {
    Command        []string          // Command and arguments
    Env            map[string]string // Environment variables
    WorkingDir     string            // Working directory
    TTY            bool              // TTY allocation
    TerminalWidth  uint32           // Terminal width
    TerminalHeight uint32           // Terminal height
    Timeout        time.Duration    // Session timeout
}
```

### Target Resolution

The command supports two targeting modes:

1. **Service Targeting**: Automatically selects a running instance
   ```bash
   rune exec api bash  # Selects any running instance of the api service
   ```

2. **Instance Targeting**: Targets a specific instance
   ```bash
   rune exec api-instance-123 bash  # Targets specific instance
   ```

Instance detection uses a simple heuristic: targets containing `-instance-` are treated as instance IDs.

### TTY Detection

The command intelligently determines when to allocate a TTY:

**Interactive Commands** (TTY enabled by default):
- Shells: `bash`, `sh`, `zsh`, `fish`, `tcsh`, `dash`
- Editors: `vim`, `nano`, `vi`
- System tools: `top`, `htop`, `less`, `more`

**Non-interactive Commands** (TTY disabled by default):
- File operations: `ls`, `cat`, `grep`
- Scripts: `python`, `node`, `go`
- Utilities: `ps`, `df`, `du`

### Signal Handling

The exec command properly handles terminal signals:

- `SIGINT` (Ctrl+C): Forwarded to the remote process
- `SIGTERM`: Forwarded to the remote process
- `SIGWINCH`: Terminal resize events are forwarded

### Error Handling

Comprehensive error handling covers:

- **Connection Errors**: Network issues, server unavailable
- **Authentication Errors**: Invalid API keys, permission denied
- **Target Errors**: Service/instance not found, not running
- **Command Errors**: Invalid commands, execution failures
- **Timeout Errors**: Session timeouts, command timeouts

## Security Considerations

### Access Control

- Namespace isolation ensures users can only access instances in permitted namespaces
- API key authentication provides secure access to the exec service
- Instance targeting prevents unauthorized access to other instances

### Command Restrictions

The exec command can be configured with command restrictions:

```yaml
# Example configuration for restricted commands
exec:
  allowed_commands:
    - "ls"
    - "cat"
    - "grep"
    - "ps"
  blocked_commands:
    - "rm -rf"
    - "dd"
    - "mkfs"
```

### Session Management

- Automatic timeout prevents long-running sessions
- Resource limits prevent abuse
- Audit logging tracks all exec sessions

## Best Practices

### Interactive Debugging

1. **Use appropriate shells**: Prefer `bash` for full-featured debugging
2. **Set working directory**: Use `--workdir` to navigate to the correct location
3. **Set environment variables**: Use `--env` to configure the debugging environment

```bash
rune exec api --workdir=/app --env=DEBUG=true bash
```

### Script Execution

1. **Disable TTY**: Use `--no-tty` for automated scripts
2. **Set timeouts**: Use appropriate timeouts for long-running scripts
3. **Handle errors**: Check exit codes and error messages

```bash
rune exec api --no-tty --timeout=10m python maintenance.py
```

### Production Use

1. **Use specific instances**: Target specific instances for predictable behavior
2. **Set appropriate timeouts**: Prevent resource exhaustion
3. **Monitor usage**: Track exec session usage and duration

```bash
rune exec api-instance-123 --timeout=5m bash
```

## Troubleshooting

### Common Issues

**Connection Refused**
```bash
Error: failed to create API client: failed to connect to API server
```
Solution: Verify the API server is running and accessible

**Instance Not Found**
```bash
Error: failed to initialize exec session: instance not found
```
Solution: Check that the instance exists and is running

**Permission Denied**
```bash
Error: failed to initialize exec session: permission denied
```
Solution: Verify API key and namespace permissions

**Command Not Found**
```bash
Error: command not found
```
Solution: Check that the command exists in the container/process

### Debug Mode

Enable verbose logging to debug issues:

```bash
rune exec api bash --verbose
```

## Integration with Existing Infrastructure

The exec command integrates seamlessly with existing Rune components:

- **API Server**: Uses the `ExecService` gRPC service
- **Orchestrator**: Leverages instance management capabilities
- **Runners**: Works with both Docker and Process runners
- **Authentication**: Integrates with existing RBAC systems

## Future Enhancements

Planned improvements include:

1. **Multi-instance exec**: Execute commands across multiple instances
2. **File transfer**: Support file upload/download during sessions
3. **Session recording**: Record sessions for audit and training
4. **Advanced TTY features**: Support for advanced terminal features
5. **Plugin integration**: Custom exec handlers for specialized use cases

## Conclusion

The `rune exec` command provides a powerful, developer-friendly interface for debugging and maintaining running services. By following the patterns and best practices outlined in this guide, you can effectively use the exec command for interactive debugging, automated maintenance, and emergency troubleshooting.

The implementation prioritizes security, usability, and integration with the broader Rune ecosystem while maintaining the platform's commitment to simplicity and developer-centric workflows.
