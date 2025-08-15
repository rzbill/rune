### CI/CD with GitHub Actions

This repo includes workflows for CI and releases using GitHub Actions.

Workflows added:
- `.github/workflows/ci.yml` — Lint, test, and build on pushes and PRs
- `.github/workflows/release.yml` — Build and attach binaries to a GitHub Release on version tags

Release tags should follow `vX.Y.Z`.

Secrets (optional):
- `GH_TOKEN` if using release creation that requires explicit token in forks; Actions default token often suffices

Future enhancements:
- Build and push Docker image to GHCR
- Adopt goreleaser for richer release assets


