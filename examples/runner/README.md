# Rune Runner Examples

This directory contains examples of using the Rune Runner implementations to manage different types of service instances.

## Directory Structure

- `docker-runner/` - Examples of using the Docker runner to manage containerized services
- `process-runner/` - Examples of using the Process runner to manage local processes

## Docker Runner

The Docker Runner provides an implementation of the Runner interface for Docker containers. It allows you to:

- Create and manage Docker containers
- Stream logs from containers
- Control container lifecycle (create, start, stop, remove)
- Apply resource limits to containers

### Running the Docker Runner Example

```bash
go run examples/runner/docker-runner/main.go
```

This example demonstrates:
- Creating a Docker container from the nginx image
- Starting the container
- Monitoring container status
- Streaming container logs
- Listing managed containers
- Stopping and removing the container

**Note**: You need to have Docker installed and running on your machine for this example to work.

## Process Runner

The Process Runner provides an implementation of the Runner interface for local processes. It allows you to:

- Create and manage local processes
- Stream logs from process output
- Control process lifecycle (create, start, stop, remove)
- Apply resource limits to processes (on Linux with cgroups)

### Simple Process Example

```bash
go run examples/runner/process-runner/simple/main.go
```

This example demonstrates:
- Creating a process instance with resource limits
- Starting the process
- Getting the process status
- Retrieving process logs
- Listing all managed processes
- Removing the process

### Long-Running Process Example

```bash
go run examples/runner/process-runner/long-running/main.go
```

This example demonstrates:
- Creating a long-running process (a counter that updates every second)
- Following logs in real-time with a timeout
- Checking process status while it runs
- Gracefully stopping a process
- Retrieving the last few log lines after process completion

### Path Validation and Security Context Example

```bash
go run examples/runner/process-runner/path-validation/main.go
```

This example demonstrates:
- Using commands from the system PATH
- Using absolute paths to executables
- Setting up security contexts for processes
- Handling validation errors for invalid paths
- Testing non-executable files

### Complete Example

```bash
go run examples/runner/process-runner/complete/main.go
```

This comprehensive example demonstrates all the features of the Process Runner:
- Creating and running processes with commands from PATH
- Using absolute path validation for executables
- Configuring security contexts (note: user/group features require root privileges)
- Testing validation errors for non-existent and non-executable files
- Managing process lifecycle and capturing logs

**Note**: Some features like security contexts with user/group settings require root privileges. When run as a regular user, you'll see permission errors when trying to use these features, which is expected behavior.

## Implementation Notes

Both runners implement the common Runner interface defined in `pkg/runner/interface.go`:

```go
type Runner interface {
    // Create creates a new service instance but does not start it
    Create(ctx context.Context, instance *types.Instance) error
    
    // Start starts an existing service instance
    Start(ctx context.Context, instanceID string) error
    
    // Stop stops a running service instance
    Stop(ctx context.Context, instanceID string, timeout time.Duration) error
    
    // Remove removes a service instance
    Remove(ctx context.Context, instanceID string, force bool) error
    
    // GetLogs retrieves logs from a service instance
    GetLogs(ctx context.Context, instanceID string, options LogOptions) (io.ReadCloser, error)
    
    // Status retrieves the current status of a service instance
    Status(ctx context.Context, instanceID string) (types.InstanceStatus, error)
    
    // List lists all service instances managed by this runner
    List(ctx context.Context) ([]*types.Instance, error)
}
```

This unified interface allows for consistent management of different types of service instances, whether they are local processes or containerized applications. 