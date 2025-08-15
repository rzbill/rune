#!/bin/bash
set -euo pipefail

RUNE_VERSION="${rune_version}"
GIT_BRANCH="${git_branch}"

echo "[cloud-init] Installing Rune via installer script"
if command -v curl >/dev/null 2>&1; then
  FETCH="curl -fsSL"
elif command -v wget >/dev/null 2>&1; then
  FETCH="wget -qO-"
else
  apt-get update -y || true
  apt-get install -y curl || true
  FETCH="curl -fsSL"
fi

if [ -n "$RUNE_VERSION" ]; then
  bash -c "$($FETCH https://raw.githubusercontent.com/rzbill/rune/master/scripts/install-rune.sh)" -- --version "$RUNE_VERSION"
else
  bash -c "$($FETCH https://raw.githubusercontent.com/rzbill/rune/master/scripts/install-rune.sh)" -- --from-source --branch "$GIT_BRANCH"
fi

echo "[cloud-init] Rune install complete"


