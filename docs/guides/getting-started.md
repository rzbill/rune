# Getting Started with Rune

This guide will help you get started with Rune, a lightweight orchestration platform.

## Prerequisites

- Go 1.19 or later
- Docker (for container-based services)

## Installation

### From Binary

Download the latest release for your platform from the [releases page](https://github.com/rzbill/rune/releases).

```bash
# Linux/macOS
chmod +x rune
sudo mv rune /usr/local/bin/
```

### From Source

```bash
# Install from source
go install github.com/rzbill/rune/cmd/rune@latest

# Verify installation
rune version
```

## Your First Service

Let's deploy a simple web service using Rune.

1. Create a file named `hello.yaml` with the following content:

```yaml
name: hello-world
image: nginx:alpine
ports:
  - 8080:80
```

2. Deploy the service:

```bash
rune cast hello.yaml
```

3. Check the service status:

```bash
rune status hello-world
```

4. View the service logs:

```bash
rune trace hello-world
```

5. Open a web browser and navigate to [http://localhost:8080](http://localhost:8080). You should see the Nginx welcome page.

6. When you're done, remove the service:

```bash
rune seal hello-world
```

## Next Steps

Now that you've deployed your first service, you can:

- Learn more about [Service Definitions](service-definitions.md)
- Explore [Deployment](deployment.md) options
- Learn about [Scaling](scaling.md) services
- Discover how to [Manage Services](managing-services.md)

## Troubleshooting

If you encounter any issues, check the following:

- Make sure Docker is running
- Verify that the ports are not already in use
- Check the service logs with `rune trace <service-name>`

For more help, please [open an issue](https://github.com/rzbill/rune/issues) on GitHub. 