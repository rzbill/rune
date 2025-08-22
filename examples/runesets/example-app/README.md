# Example Runeset: example-app

Quick demo of runeset features (values, file/template includes, secret/configmap refs).

## Render (no apply)

```bash
rune cast . --render -f values/values.yaml
```

## Dry-run (validate only)

```bash
rune cast . --dry-run -f values/values.yaml
```

## Apply

```bash
rune cast . -f values/values.yaml
```

## Package and Cast from Archive

```bash
rune pack . -o /tmp/example-app.runeset.tgz --sha256
rune cast /tmp/example-app.runeset.tgz --render -f values/values.yaml
```

Notes:
- `values:runeset.*` is reserved and injected (name, version, description, effectiveNamespace).
- `file:` inserts raw file content; `template:` renders with global values before insert.
- `secret:` and `configmap:` refs are resolved server-side at runtime.
