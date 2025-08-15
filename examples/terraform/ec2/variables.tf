variable "region" {
  type        = string
  description = "AWS region"
}

variable "instance_type" {
  type        = string
  default     = "t3.small"
  description = "EC2 instance type"
}

variable "key_name" {
  type        = string
  description = "Existing EC2 key pair name"
}

variable "allowed_cidr" {
  type        = string
  description = "CIDR allowed to access SSH and HTTP ports"
}

variable "ami_id" {
  type        = string
  default     = null
  description = "Optional custom AMI ID"
}

variable "use_amazon_linux" {
  type        = bool
  default     = false
  description = "Use Amazon Linux instead of Ubuntu"
}

variable "rune_version" {
  type        = string
  default     = ""
  description = "Optional Rune release version tag (e.g., v0.1.0). If empty, build from source"
}

variable "git_branch" {
  type        = string
  default     = "master"
  description = "Git branch to build from when rune_version is empty"
}


