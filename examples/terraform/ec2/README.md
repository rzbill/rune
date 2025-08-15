# Terraform: EC2 + Rune

Provisions an EC2 instance and installs Rune Server (`runed`) as a systemd service via cloud-init.

Usage
```bash
terraform init
terraform apply -auto-approve \
  -var "region=us-east-1" \
  -var "key_name=your_ssh_key_name" \
  -var "allowed_cidr=0.0.0.0/0"
```

Outputs
- `instance_public_ip`
- `grpc_endpoint` (port 8080)
- `http_endpoint` (port 8081)

Variables
- `region`, `instance_type`, `key_name`, `allowed_cidr`, `ami_id`, `use_amazon_linux`, `rune_version`


