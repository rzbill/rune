# Service Definitions

This guide explains how to define services in Rune, including declaring dependencies between services.

## Basic service

```yaml
name: api
image: nginx:alpine
scale: 1
```

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


