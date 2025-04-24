# Rune Examples

This directory contains examples for using Rune in various scenarios.

## Simple Service

The [simple-service](simple-service) directory contains a basic example of a single service deployment.

```bash
# Deploy the simple service
rune cast examples/simple-service/service.yaml

# View the service
rune status hello-world

# Trace the logs
rune trace hello-world

# Remove the service
rune seal hello-world
```

## Multi-Service

The [multi-service](multi-service) directory contains an example of deploying multiple services with dependencies.

```bash
# Deploy the multi-service example
rune cast examples/multi-service/services.yaml

# View all services
rune status

# Remove all services
rune seal --all
```

## Contributing Examples

If you've created an example that you think would be helpful to others, please submit a pull request. Be sure to include:

1. A clear README explaining the example
2. All required YAML configuration files
3. Any supporting scripts or code
4. Instructions for running the example 