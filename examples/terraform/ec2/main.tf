terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

data "aws_ami" "ubuntu" {
  count       = var.use_amazon_linux ? 0 : 1
  most_recent = true
  owners      = ["099720109477"]

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }
}

data "aws_ami" "amazon_linux" {
  count       = var.use_amazon_linux ? 1 : 0
  most_recent = true
  owners      = ["137112412989"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-x86_64"]
  }
}

locals {
  ami_id = coalesce(var.ami_id, var.use_amazon_linux ? one(data.aws_ami.amazon_linux.*.id) : one(data.aws_ami.ubuntu.*.id))
}

resource "aws_security_group" "rune" {
  name        = "rune-sg"
  description = "Rune security group"

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.allowed_cidr]
  }

  ingress {
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    cidr_blocks = [var.allowed_cidr]
  }

  ingress {
    from_port   = 8081
    to_port     = 8081
    protocol    = "tcp"
    cidr_blocks = [var.allowed_cidr]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "rune" {
  ami                         = local.ami_id
  instance_type               = var.instance_type
  key_name                    = var.key_name
  vpc_security_group_ids      = [aws_security_group.rune.id]
  associate_public_ip_address = true

  user_data = templatefile("${path.module}/user_data.sh", {
    use_amazon_linux = var.use_amazon_linux
    rune_version     = var.rune_version
  })

  tags = {
    Name = "rune-ec2"
  }
}

output "instance_public_ip" {
  value = aws_instance.rune.public_ip
}

output "grpc_endpoint" {
  value = "${aws_instance.rune.public_ip}:7863"
}

output "http_endpoint" {
  value = "http://${aws_instance.rune.public_ip}:7861"
}


