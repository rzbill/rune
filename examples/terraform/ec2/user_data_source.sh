#!/bin/bash

# Rune EC2 User Data Script - Install from Source
# This script installs Rune Server (runed) from source on EC2
# Compatible with Ubuntu 22.04+, Debian 11+, and Amazon Linux 2023

set -e

# Log all output to both console and file
exec > /var/log/user-data.log 2>&1

echo "Starting Rune installation from source..."

# Detect OS and install Docker
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
    VERSION=$VERSION_ID
else
    echo "Cannot detect OS, defaulting to Ubuntu"
    OS="ubuntu"
fi

echo "Detected OS: $OS $VERSION"

# Install Docker based on OS
if [ "$OS" = "ubuntu" ] || [ "$OS" = "debian" ]; then
    echo "Installing Docker on $OS..."
    # Force IPv4 for apt to avoid IPv6-only mirror issues and normalize Debian sources
    if [ "$OS" = "debian" ]; then
        echo 'Acquire::ForceIPv4 "true";' > /etc/apt/apt.conf.d/99force-ipv4
        cat >/etc/apt/sources.list <<'EOF'
deb http://deb.debian.org/debian bookworm main contrib non-free non-free-firmware
deb http://security.debian.org/debian-security bookworm-security main contrib non-free non-free-firmware
deb http://deb.debian.org/debian bookworm-updates main contrib non-free non-free-firmware
deb http://deb.debian.org/debian bookworm-backports main contrib non-free non-free-firmware
EOF
    fi

    apt-get -o Acquire::ForceIPv4=true update -y
    apt-get -o Acquire::ForceIPv4=true install -y ca-certificates curl gnupg lsb-release git build-essential make openssl

    # Import Docker's official GPG key for the correct distro (skip if already present)
    if [ ! -f /usr/share/keyrings/docker.gpg ]; then
        curl -fsSL https://download.docker.com/linux/$OS/gpg | gpg --dearmor -o /usr/share/keyrings/docker.gpg
    fi

    # Determine codename per distro
    if [ "$OS" = "ubuntu" ]; then
        CODENAME=$(. /etc/os-release && echo $${UBUNTU_CODENAME:-jammy})
        DIST="ubuntu"
    else
        CODENAME=$(. /etc/os-release && echo $VERSION_CODENAME)
        DIST="debian"
    fi

    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker.gpg] https://download.docker.com/linux/$DIST $CODENAME stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
    apt-get -o Acquire::ForceIPv4=true update -y
    apt-get -o Acquire::ForceIPv4=true install -y docker-ce docker-ce-cli containerd.io
    systemctl enable --now docker
    
elif [ "$OS" = "amazon" ] || [ "$OS" = "rhel" ] || [ "$OS" = "centos" ]; then
    echo "Installing Docker on Amazon Linux/RHEL/CentOS..."
    dnf update -y
    dnf install -y docker git make gcc openssl || dnf groupinstall -y "Development Tools"
    systemctl enable --now docker
    usermod -aG docker ec2-user || usermod -aG docker ubuntu || true
else
    echo "Unsupported OS: $OS"
    exit 1
fi

# Create rune user and directories
echo "Creating rune user and directories..."
useradd --system --home /var/lib/rune --shell /usr/sbin/nologin rune || true
mkdir -p /etc/rune /var/lib/rune
chown -R rune:rune /var/lib/rune

# Grant Docker access to service and current users (if docker group exists)
if getent group docker >/dev/null 2>&1; then
    usermod -aG docker rune || true
    if id -u ubuntu >/dev/null 2>&1; then usermod -aG docker ubuntu || true; fi
    if [ -n "$${SUDO_USER:-}" ]; then usermod -aG docker "$SUDO_USER" || true; fi
fi

# Install Go and build from source
echo "Installing Go and building Rune from source..."
GO_VERSION="${go_version}"
if [ -z "$GO_VERSION" ]; then
    GO_VERSION="1.22.5"
fi
curl -fSL -o /tmp/go$GO_VERSION.linux-amd64.tar.gz https://go.dev/dl/go$GO_VERSION.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go$GO_VERSION.linux-amd64.tar.gz
export PATH=/usr/local/go/bin:$PATH

# Ensure Go module cache env so builds don't fail in non-interactive shells
if [ -z "$${HOME:-}" ]; then
    export HOME=/root
fi
export GOPATH=$${GOPATH:-$HOME/go}
export GOMODCACHE=$${GOMODCACHE:-$GOPATH/pkg/mod}
mkdir -p "$GOPATH" "$GOMODCACHE"
go env -w GOPATH="$GOPATH" >/dev/null 2>&1 || true
go env -w GOMODCACHE="$GOMODCACHE" >/dev/null 2>&1 || true

# Add Go to PATH permanently
echo 'export PATH=/usr/local/go/bin:$PATH' >> /etc/profile
echo 'export PATH=/usr/local/go/bin:$PATH' >> /etc/bash.bashrc

# Clone and build Rune
echo "Cloning Rune repository..."
cd /tmp
GIT_BRANCH="${git_branch}"
if [ -z "$GIT_BRANCH" ]; then
    GIT_BRANCH="master"
fi

# Clean previous clone to make script idempotent
if [ -d /tmp/rune ]; then
    echo "Removing existing /tmp/rune directory from previous run..."
    rm -rf /tmp/rune
fi

git clone -b "$GIT_BRANCH" https://github.com/rzbill/rune.git
cd rune

echo "Building Rune binaries..."
make build

# Install binaries
echo "Installing Rune binaries..."
install -m 0755 bin/rune /usr/local/bin/rune
install -m 0755 bin/runed /usr/local/bin/runed

# Configure Rune
echo "Configuring Rune..."
cp examples/config/rune.yaml /etc/rune/rune.yaml
sed -i 's#data_dir: "/var/lib/rune"#data_dir: "/var/lib/rune"#' /etc/rune/rune.yaml

# Generate KEK for secret encryption
echo "Generating encryption key..."
openssl rand -base64 32 > /etc/rune/kek.b64
chmod 600 /etc/rune/kek.b64
chown root:root /etc/rune/kek.b64

# Install and enable systemd service
echo "Installing systemd service..."
cp examples/config/runed.service /etc/systemd/system/runed.service

# Ensure the service has Docker group access regardless of user group cache
mkdir -p /etc/systemd/system/runed.service.d
cat >/etc/systemd/system/runed.service.d/override.conf <<'EOF'
[Service]
SupplementaryGroups=docker
EOF

systemctl daemon-reload
systemctl enable --now runed

# Path where runed writes the CLI context
CLI_CONFIG_PATH="/var/lib/rune/.rune/config.yaml"

# Wait for service to start
echo "Waiting for Rune service to start..."
sleep 10

# Verify installation
echo "Verifying installation..."
if systemctl is-active --quiet runed; then
    echo "✅ Rune service is running successfully"
else
    echo "❌ Rune service failed to start"
    systemctl status runed --no-pager || true
fi

# Test health endpoint
echo "Testing health endpoint..."
if curl -s http://localhost:7861/health > /dev/null; then
    echo "✅ Health endpoint is responding"
else
    echo "⚠️  Health endpoint not yet responding (may need more time)"
fi

# Also copy the CLI config to the primary login user's home so `rune status` works immediately
echo "Propagating CLI config to primary user..."

# Detect a primary non-system user (prefer common cloud users, then first UID >= 1000)
TARGET_USER=""
for candidate in admin ubuntu ec2-user debian; do
    if id -u "$candidate" >/dev/null 2>&1; then
        TARGET_USER="$candidate"
        break
    fi
done
if [ -z "$TARGET_USER" ]; then
    TARGET_USER=$(awk -F: '$3>=1000 && $1!="nobody" {print $1; exit}' /etc/passwd || true)
fi

if [ -n "$TARGET_USER" ]; then
    TARGET_HOME=$(getent passwd "$TARGET_USER" | cut -d: -f6)
    if [ -f "$CLI_CONFIG_PATH" ]; then
        mkdir -p "$TARGET_HOME/.rune"
        cp "$CLI_CONFIG_PATH" "$TARGET_HOME/.rune/config.yaml"
        chown -R "$TARGET_USER":"$TARGET_USER" "$TARGET_HOME/.rune"
        echo "Copied CLI config to $TARGET_HOME/.rune/config.yaml for user $TARGET_USER"
    else
        echo "Warning: CLI config not found at $CLI_CONFIG_PATH; skipping copy"
    fi
fi

# Display version information
echo "Rune installation complete!"
echo "Rune version: $(rune version 2>/dev/null || echo 'CLI not available')"
echo "Rune daemon version: $(runed --version 2>/dev/null || echo 'Daemon not available')"
echo "Service status: $(systemctl is-active runed)"

# Clean up build artifacts
echo "Cleaning up build artifacts..."
rm -rf /tmp/rune
rm -f /tmp/go$GO_VERSION.linux-amd64.tar.gz

echo "Installation script completed successfully!"
echo "Rune is now running on:"
echo "  - gRPC: localhost:7863"
echo "  - Dashboard: localhost:7862"
echo "  - HTTP: localhost:7861"
echo "  - Logs: journalctl -u runed -f"
