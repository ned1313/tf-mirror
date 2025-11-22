# Small test file for quick testing
# This downloads only one provider with one version/platform

provider "hashicorp/random" {
  versions  = ["3.5.0"]
  platforms = ["linux_amd64"]
}
