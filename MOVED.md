# This repository has moved

**⚠️ OpenSQS is now developed in the [tguidoux/openaws](https://github.com/tguidoux/openaws) monorepo** — a collection of lightweight, self-hostable AWS-compatible services sharing components, build and deployment.

This repository (`tguidoux/opensqs`) is no longer the active home of the project. Please update your links, imports and deployments.

## New locations

| Component | New location |
|-----------|--------------|
| Server application | <https://github.com/tguidoux/openaws/tree/main/apps/go/sqs> |
| Queue libraries | <https://github.com/tguidoux/openaws/tree/main/pkgs/v1/sqs> |
| Shared libraries | <https://github.com/tguidoux/openaws/tree/main/pkgs/v1/common> |
| Helm chart | <https://github.com/tguidoux/openaws/tree/main/deploy/helm/sqs> |
| Container image | `ghcr.io/tguidoux/openaws/sqs-server` |

## Migration notes

- **Go import paths** change from `github.com/tguidoux/opensqs/...` to `github.com/tguidoux/openaws/...`
- **Container image names** change — the SQS server image is now `ghcr.io/tguidoux/openaws/sqs-server`
- **Issues and pull requests**: please open new issues and pull requests in [tguidoux/openaws](https://github.com/tguidoux/openaws), not in this repository
