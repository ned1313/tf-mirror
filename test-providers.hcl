provider "hashicorp/random" {
  versions  = ["3.6.0", "3.6.3"]
  platforms = ["linux_amd64", "windows_amd64"]
}

provider "hashicorp/null" {
  versions  = ["3.2.2"]
  platforms = ["linux_amd64"]
}
