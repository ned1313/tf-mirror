# Example modules.hcl file for terraform-mirror
# This file defines Terraform modules to be mirrored from the public registry.
#
# Format:
#   module "<namespace>/<name>/<system>" {
#     versions = ["<version1>", "<version2>", ...]
#   }
#
# The source follows the Terraform Registry module format:
#   namespace - The module's namespace (e.g., "hashicorp", "terraform-aws-modules")
#   name      - The module name (e.g., "consul", "vpc")
#   system    - The target provider/system (e.g., "aws", "google", "azurerm")

# HashiCorp official modules
module "hashicorp/consul/aws" {
  versions = ["0.11.0", "0.12.0"]
}

module "hashicorp/vault/aws" {
  versions = ["0.18.0", "0.18.1"]
}

# Popular AWS modules from terraform-aws-modules
module "terraform-aws-modules/vpc/aws" {
  versions = ["5.0.0", "5.1.0", "5.2.0"]
}

module "terraform-aws-modules/eks/aws" {
  versions = ["19.0.0", "20.0.0"]
}

module "terraform-aws-modules/s3-bucket/aws" {
  versions = ["3.15.0", "4.0.0"]
}

module "terraform-aws-modules/rds/aws" {
  versions = ["6.0.0", "6.1.0"]
}

# Azure modules
module "Azure/avm-res-compute-virtualmachine/azurerm" {
  versions = ["0.1.0"]
}

module "Azure/naming/azurerm" {
  versions = ["0.4.0", "0.4.1"]
}

# Google Cloud modules
module "terraform-google-modules/network/google" {
  versions = ["9.0.0", "9.1.0"]
}

module "terraform-google-modules/kubernetes-engine/google" {
  versions = ["29.0.0", "30.0.0"]
}
