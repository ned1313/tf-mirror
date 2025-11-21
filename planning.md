# Terraform Mirror

This project is intended to provide two useful functions to Terraform users in air-gapped or low-bandwidth environments.

## Provider Network Mirror

The first benefit is to provide a network mirror for terraform providers. The mirror will be enabled through the client settings for the Terraform client as detailed here: https://developer.hashicorp.com/terraform/cli/config/config-file#provider-installation

The network mirror protocol should be implemented as detailed here: https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol

In addition to implementing the protocol, the provider mirror should also have two additional features:

1. Automatically download new providers on demand when requested by a terraform client. The server will download the requested version only. This feature can be disabled by the administrator.
1. Load a set of predefined providers from a file. The file will include the provider source addresses, architectures, and versions to include.

## Provider Module Mirror

The second benefit is to provide a mirror for Terraform modules from the public registry. The Terraform client doesn't currently support a mirror setting for modules, so we need to take a different approach.

To use the module mirror, the source property for a module will be updated to include the hostname of the module mirror: e.g. if the current module source is "terraform-aws-modules/iam/aws", the new source will be "mirror.hostname.local/terraform-aws-modules/iam/aws"

The module will then be served by the local cache on the Terraform Mirror server. To serve the modules properly, the server will need to implement the Terraform Registry protocol as detailed here: https://developer.hashicorp.com/terraform/internals/module-registry-protocol

In addition to implementing the protocol, the module mirror should also have two additional features:

1. Automatically download new modules on demand when requested by a terraform client. The server will download the requested version only. This feature can be disabled by the administrator.
1. Load a set of predefined modules from a file. The file will include the module source addresses and versions to include.

## Architecture

The backend logic should be developed using Go. The frontend should use Typescript for the Web UI and to handle requests. There should be two endpoints: `providers` and `modules` to handle the two distinct functionalities supported by the server.

For the server, it should be available to run as a container. I'd like to start with a minimal container, maybe alpine, and layer in only the necessary components. TBD whether the backend and frontend run as separate containers? The persistent storage for the modules and providers should be S3 compliant object store. That can be actual AWS S3 or MinIO.

For the initial implementation, there should be two personas. Admins are able to configure the server options, add and remove modules and providers, and pre-load modules and providers. Consumers will have read-only access to the web UI for discovery and read-only access to download modules and providers. Consumers will not require authentication initially.

Admins will require authentication. We'll use a simple username/password login. Later on, I'd like to introduce SSO. Admin credentials should be created during the setup process and should be changeable by the admin. Credentials should be stored hashed and salted.