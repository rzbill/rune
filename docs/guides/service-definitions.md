# Service Definitions

This guide explains how to define services in Rune, including declaring dependencies between services.

## Basic service

```yaml
name: api
image: nginx:alpine
scale: 1
```

## Resources (CPU and Memory)

Rune follows Kubernetes-style resource strings. Both `request` and `limit` are optional and may be set independently. CPU values represent cores; memory values represent bytes with unit suffixes.

Example:

```yaml
name: api
image: my-api:latest
resources:
  cpu:
    request: "500m"   # 0.5 cores
    limit:   "1"      # 1 core
  memory:
    request: "256Mi"  # 268,435,456 bytes
    limit:   "2Gi"    # 2,147,483,648 bytes
```

CPU formats:
- "1" → 1 core (vCPU)
- "0.5" → 0.5 cores
- "500m" → 0.5 cores (millicores)

Memory formats (case-insensitive):
- Binary units: Ki, Mi, Gi, Ti, Pi, Ei (base-2). Example: `2Gi` (2,147,483,648 bytes)
- SI units: K, M, G, T, P, E (base-10). Example: `2.5G` (2,500,000,000 bytes)

Notes:
- If a value is omitted it is treated as 0 for scheduling/limits.
- Use binary units (Gi/Mi) for exact powers-of-two sizing; use SI (G/M) for decimal sizing.

## Declaring dependencies (MVP)

Dependencies can be declared using simple strings (same namespace), FQDN-like strings (service.namespace), or structured objects.

```yaml
name: api
namespace: default
image: my-api:latest
dependencies:
  - "db"                 # same namespace
  - "cache.shared"      # cross-namespace (service.namespace)
  - service: auth        # structured form
    namespace: security
```

Rune normalizes these into `service` and `namespace`. If `namespace` is omitted, it defaults to the service's own namespace.

### Readiness semantics (MVP)

- A dependency is Ready if:
  - The dependency service defines a readiness probe and at least one instance is Running and readiness=true; or
  - No readiness probe is defined and at least one instance is Running.
- The orchestrator delays instance creation for a service with dependencies until all dependencies are Ready.

### Delete safety (MVP)

- Deleting a service is blocked if other services depend on it.
- You can override with:

```bash
rune delete service <name> --no-dependencies
```

### CLI helpers

```bash
rune deps validate <service>           # format, existence, cycle checks
rune deps graph <service> --format=dot # visualize dependency graph
rune deps check <service>              # readiness check for dependencies
rune deps dependents <service>         # list services that depend on target
```


