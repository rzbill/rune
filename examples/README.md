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

## Cloudflare Tunnel (Expose services over HTTPS without ALB)

Create a Cloudflare Tunnel and token in Cloudflare Zero Trust, then deploy `cloudflared` as a Rune service to publish your local `expose`d ports via Cloudflare's edge.

cloudflared service example:

```yaml
service:
  name: cloudflared
  namespace: default
  image: cloudflare/cloudflared:latest
  scale: 1
  args:
    - tunnel
    - --no-autoupdate
    - run
    - --token
    - <CLOUDFLARE_TUNNEL_TOKEN>
  resources:
    cpu: { request: "50m", limit: "200m" }
    memory: { request: "64Mi", limit: "128Mi" }
```

Map hostnames to local ports (exposed by Rune) in `config.yml` (optional) or in the Cloudflare dashboard:

```yaml
# /etc/cloudflared/config.yml (optional if using --token mode)
ingress:
  - hostname: api.example.com
    service: http://localhost:8080
  - hostname: app.example.com
    service: http://localhost:3000
  - service: http_status:404
```

Deploy:

```bash
rune cast examples/rune-hello-world/service.yaml
rune cast -f <cloudflared-service-yaml>
```

Now `https://api.example.com` will forward to your local service on `127.0.0.1:8080` without opening inbound ports or using an AWS load balancer.

If you've created an example that you think would be helpful to others, please submit a pull request. Be sure to include:

1. A clear README explaining the example
2. All required YAML configuration files
3. Any supporting scripts or code
4. Instructions for running the example 