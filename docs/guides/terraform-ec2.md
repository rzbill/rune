### Provision EC2 with Terraform and Bootstrap Rune

This example provisions an EC2 instance and installs Rune as a systemd service via cloud-init `user_data`.

Repo path: `examples/terraform/ec2/`

What it creates:
- Security group allowing 22, 8080, 8081 (configurable)
- Ubuntu 22.04 t3.small (configurable)
- Cloud-init that: installs Docker, installs Rune, writes `/etc/rune`, enables `runed`, and sets up CLI access

Quick start
```bash
cd examples/terraform/ec2
terraform init
terraform apply -auto-approve \
  -var "region=us-east-1" \
  -var "key_name=your_ssh_key_name" \
  -var "allowed_cidr=0.0.0.0/0"

# After apply, note the outputs, then:
echo "gRPC: $(terraform output -raw grpc_endpoint)"
echo "HTTP:  $(terraform output -raw http_endpoint)"
```

Variables
- `region`: AWS region
- `key_name`: existing EC2 key pair name
- `instance_type`: default `t3.small`
- `allowed_cidr`: allowed source CIDR for ports 22/8080/8081
- `ami_id`: optional custom AMI. If empty, the latest Ubuntu 22.04 is used
- `rune_version`: optional release tag to download; if empty, builds from source

Destroy
```bash
terraform destroy -auto-approve
```

Notes
- For Amazon Linux, set `use_amazon_linux=true` and optionally provide an AL2023 AMI
- Ensure IAM credentials are configured for Terraform (via env or profile)


