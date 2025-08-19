# Terraform EC2 Examples

This directory contains Terraform examples for provisioning EC2 instances with Rune.

## User Data Script

### `user_data.sh`
- **Uses**: Official `install-server.sh` installer
- **Pros**: 
  - Clean, maintainable code
  - Uses tested installer script
  - Automatic Docker installation
  - Better error handling and verification
  - Easier to debug and maintain
  - Automatic CLI access setup for primary users
- **Best for**: All production deployments, cloud-init, CI/CD

## Usage

### Quick Start with Official Installer
```bash
# Use the recommended user_data.sh
terraform apply -var="user_data_file=user_data.sh"
```

### Custom Version/Branch
```bash
# Install specific version
terraform apply \
  -var="user_data_file=user_data.sh" \
  -var="rune_version=v0.1.0"

# Install from source branch
terraform apply \
  -var="user_data_file=user_data.sh" \
  -var="git_branch=feature/new-feature"
```



## Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `rune_version` | `""` | Rune version to install (e.g., "v0.1.0") |
| `git_branch` | `"master"` | Git branch to use when building from source |

## What Gets Installed

Both scripts install:
- Docker engine
- Rune server (`runed`) and CLI (`rune`)
- Systemd service
- Configuration files
- CLI access for primary user

## Post-Installation

After successful installation:
1. **Service**: `runed` runs as systemd service
2. **Endpoints**: 
   - gRPC: `localhost:7863`
   - HTTP: `localhost:7861`
   - Dashboard: `localhost:7862`
3. **CLI Access**: Primary user can run `rune status` immediately
4. **Logs**: `journalctl -u runed -f`

## Troubleshooting

### Check Installation Logs
```bash
# On the EC2 instance
sudo tail -f /var/log/user-data.log
```

### Service Status
```bash
sudo systemctl status runed --no-pager
```

### Manual CLI Setup
If CLI config wasn't copied automatically:
```bash
# Copy config manually
sudo mkdir -p ~/.rune
sudo cp /var/lib/rune/.rune/config.yaml ~/.rune/config.yaml
sudo chown -R $USER:$USER ~/.rune
chmod 700 ~/.rune
chmod 600 ~/.rune/config.yaml

# Test
rune status
```




