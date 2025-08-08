# Rune Demo Application - File Structure

This directory contains a comprehensive demo application that showcases all Rune MVP features.

## 📁 File Structure

```
examples/rune-demo-app/
├── main.go                    # Main application code with all features
├── Dockerfile                 # Multi-stage Docker build
├── go.mod                     # Go module definition
├── go.sum                     # Go dependency checksums
├── service.yaml               # Main service configuration
├── config.json                # Application configuration
├── README.md                  # Comprehensive documentation
├── DEMO_GUIDE.md             # Complete feature testing guide
├── test-demo.sh              # Automated test script
├── FILES.md                   # This file
└── environments/              # Environment-specific configurations
    ├── development.yaml       # Development environment config
    └── production.yaml        # Production environment config
```

## 📋 File Descriptions

### Core Application Files

- **`main.go`**: Complete Go application demonstrating:
  - Environment variable configuration
  - Health check endpoints
  - HTTP API endpoints
  - Prometheus metrics
  - Interactive command execution
  - Multi-instance support
  - Graceful shutdown
  - Request counting and logging

- **`Dockerfile`**: Multi-stage Docker build with:
  - Go 1.21 Alpine builder
  - Minimal Alpine runtime
  - Non-root user security
  - Health check configuration
  - Proper signal handling

- **`go.mod`**: Go module definition for the demo app

### Configuration Files

- **`service.yaml`**: Main service configuration with:
  - Environment variables
  - Health checks (liveness/readiness)
  - Resource limits
  - Network policies
  - Service discovery
  - External exposure

- **`config.json`**: Application-level configuration

- **`environments/development.yaml`**: Development environment config
- **`environments/production.yaml`**: Production environment config

### Documentation Files

- **`README.md`**: Comprehensive documentation covering:
  - Feature demonstrations
  - Quick start guide
  - CLI command examples
  - Environment variables
  - API endpoints
  - Health checks
  - Scaling examples
  - Troubleshooting

- **`DEMO_GUIDE.md`**: Complete testing guide with:
  - Feature-by-feature demonstrations
  - Advanced testing scenarios
  - Interactive testing examples
  - Monitoring and metrics
  - Troubleshooting guide

### Testing Files

- **`test-demo.sh`**: Automated test script that:
  - Builds the application
  - Deploys to Rune
  - Tests all features
  - Provides colored output
  - Includes cleanup

## 🚀 Quick Usage

```bash
# Build and deploy
cd examples/rune-demo-app
docker build -t rune-demo-app:latest .
rune cast service.yaml

# Run automated tests
./test-demo.sh

# Manual testing
rune logs rune-demo-app --namespace=demo --follow
rune exec rune-demo-app --namespace=demo bash
rune scale rune-demo-app 5 --namespace=demo
```

## ✅ Features Demonstrated

This demo application showcases all Rune MVP features:

1. **Environment Variables**: Comprehensive configuration management
2. **Health Checks**: Liveness and readiness probes with failure simulation
3. **Logging**: Structured logging with instance identification
4. **Exec**: Interactive command execution and debugging
5. **Scaling**: Multi-instance deployment and management
6. **Service Discovery**: DNS-based service discovery
7. **Resource Management**: CPU and memory limits
8. **Network Policies**: Ingress/egress traffic control
9. **Metrics**: Prometheus-compatible metrics
10. **Multi-Environment**: Development and production configurations

## 🎯 Purpose

This demo application serves as:
- **Learning Tool**: Complete example of Rune features
- **Testing Environment**: Comprehensive testing of all MVP capabilities
- **Development Reference**: Template for building Rune-compatible applications
- **Documentation**: Living example of best practices

The application is designed to be educational, comprehensive, and practical for understanding and testing Rune's capabilities.
