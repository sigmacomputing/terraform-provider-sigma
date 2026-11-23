# Changelog

All notable changes to `terraform-provider-sigma` are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## v0.16.2

### Fixed

- Docs: examples pin `~> 0.16`, the getting-started guide has current resource counts, an accurate list of what's stored in state, and links that work on the registry.

## v0.16.1

### Fixed

- Registry docs: resource and data source pages showed their page metadata as text at the top, and weren't grouped into sections in the sidebar.

## v0.16.0

First public release, licensed under the Apache License 2.0.

- 35 resources and 27 data sources covering identity and access, connections and integrations, workspaces and content, organization settings, and tenants and deployment. See the [registry docs](https://registry.terraform.io/providers/sigmacomputing/sigma/latest/docs).
- Every resource supports `terraform import`.
- Releases are built and signed in CI, with a build provenance attestation per artifact. Release archives include `LICENSE` and `NOTICE`.
