#!/bin/bash

# Rune EC2 User Data Script - Install from Source
# This script installs Rune Server (runed) from source on EC2
# Compatible with Ubuntu 22.04+, Debian 11+, and Amazon Linux 2023

set -e

# Log all output to both console and file
exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1

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
    echo "Installing Docker on Ubuntu/Debian..."
    apt-get update -y
    apt-get install -y ca-certificates curl gnupg lsb-release git
    
    # Install Docker
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $UBUNTU_CODENAME || echo "jammy") stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
    apt-get update -y
    apt-get install -y docker-ce docker-ce-cli containerd.io
    systemctl enable --now docker
    
elif [ "$OS" = "amazon" ] || [ "$OS" = "rhel" ] || [ "$OS" = "centos" ]; then
    echo "Installing Docker on Amazon Linux/RHEL/CentOS..."
    dnf update -y
    dnf install -y docker git
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

# Install Go and build from source
echo "Installing Go and building Rune from source..."
GO_VERSION="1.22.5"
curl -LO https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
export PATH=/usr/local/go/bin:$PATH

# Add Go to PATH permanently
echo 'export PATH=/usr/local/go/bin:$PATH' >> /etc/profile
echo 'export PATH=/usr/local/go/bin:$PATH' >> /etc/bash.bashrc

# Clone and build Rune
echo "Cloning Rune repository..."
cd /tmp
git clone https://github.com/rzbill/rune.git
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
systemctl daemon-reload
systemctl enable --now runed

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
if curl -s http://localhost:8081/health > /dev/null; then
    echo "✅ Health endpoint is responding"
else
    echo "⚠️  Health endpoint not yet responding (may need more time)"
fi

# Display version information
echo "Rune installation complete!"
echo "Rune version: $(rune version 2>/dev/null || echo 'CLI not available')"
echo "Rune daemon version: $(runed --version 2>/dev/null || echo 'Daemon not available')"
echo "Service status: $(systemctl is-active runed)"

# Clean up build artifacts
echo "Cleaning up build artifacts..."
rm -rf /tmp/rune
rm -f /tmp/go${GO_VERSION}.linux-amd64.tar.gz

echo "Installation script completed successfully!"
echo "Rune is now running on:"
echo "  - gRPC: localhost:8080"
echo "  - HTTP: localhost:8081"
echo "  - Logs: journalctl -u runed -f"
