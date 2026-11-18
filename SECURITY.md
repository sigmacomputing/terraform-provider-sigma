# Security Policy

## Reporting a Vulnerability

Please report security vulnerabilities through Sigma Computing's
Vulnerability Disclosure Program:

https://www.sigmacomputing.com/product/vdp

Do not open public issues or pull requests for security reports.

## Supported Versions

Security fixes are released on the latest minor version. Upgrade to the latest
release before reporting an issue.

## Handling secrets

The provider writes some secrets to Terraform state (for example, API
credential secrets and warehouse credentials). See
[State file security](README.md#state-file-security) for which attributes are
affected and how to protect state. When sharing `TF_LOG` output, check it for
values your configuration passes in before posting.
