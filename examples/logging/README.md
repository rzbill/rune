# Logging Service Examples for Rune

This directory contains example services that generate logs for testing Rune's `trace` command and other log-related features.

## Contents

- `main.go` - A simple logging service using Rune's structured logging library
- `log-generator.go` - A continuous log generator that outputs logs at different levels and formats
- `Dockerfile` - Dockerfile for the first example
- `Dockerfile-generator` - Dockerfile for the log generator
- `service.yaml` - Rune service manifest for deploying the logging service
- `service-generator.yaml` - Rune service manifest for deploying the log generator

## Building and Running the Docker Images

### Building the Log Generator

```bash
# From the project root
docker build -t rune-log-generator -f examples/logging/Dockerfile-generator .
```

### Running the Log Generator Locally

```bash
docker run -it --name log-generator rune-log-generator
```

You can adjust the log generation interval using the `LOG_INTERVAL` environment variable (in milliseconds):

```bash
docker run -it --name log-generator -e LOG_INTERVAL=200 rune-log-generator
```

## Deploying to Rune

### Deploy the Log Generator Service

```bash
rune cast examples/logging/service-generator.yaml
```

### View Logs with the Trace Command

```bash
rune trace log-generator
```

To filter for specific log levels:

```bash
rune trace log-generator --pattern="ERROR"
```

To show only the last 10 log entries:

```bash
rune trace log-generator --tail=10
```

## Testing the fixed trace functionality

If you've fixed the trace functionality, you can use these services to verify that logs are being correctly captured and displayed. The log generator produces a continuous stream of well-structured logs at various levels, making it ideal for testing the trace command.

## Cleanup

To remove the services from Rune:

```bash
rune delete service log-generator
```

To remove the Docker containers:

```bash
docker rm -f log-generator
``` 