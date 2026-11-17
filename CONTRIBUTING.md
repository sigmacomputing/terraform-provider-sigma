# Contributing

Thanks for helping improve the Sigma Terraform provider. Bug reports and feature
requests go in [GitHub issues](../../issues/new/choose). Report security issues
through [SECURITY.md](SECURITY.md), not in public issues.

## Development setup

Requirements: Go (version from `go.mod`), Terraform 1.5+, and `make`.

```bash
make build      # compile
make install    # build and install into ~/.terraform.d/plugins
```

To run a local build against a config, see [Local development](README.md#local-development).

## Checks

Run these before opening a pull request. CI runs the same ones.

```bash
make test        # unit tests
make lint        # gofmt, go vet, staticcheck, errcheck
make docs-check  # regenerates docs and fails if anything under docs/ changed
```

Acceptance tests call the real Sigma API and create and delete objects. Run them
only against a test organization:

```bash
export SIGMA_CLIENT_ID=... SIGMA_CLIENT_SECRET=... SIGMA_BASE_URL=...
make acctest
```

See [Acceptance tests](README.md#acceptance-tests) for the warehouse-dependent tests.

## Documentation

Files under `docs/` are generated. Don't edit them by hand. Edit the source,
then run `make docs`:

- Resource and attribute descriptions: the schema in `internal/services/...`.
- Examples: `examples/resources/<name>/resource.tf` and `import.sh`,
  `examples/data-sources/<name>/data-source.tf`.
- Guides: `templates/guides/`.
- Registry sidebar groups: the `subcategory` lookup in
  `templates/resources.md.tmpl` and `templates/data-sources.md.tmpl`.

## Adding a resource or data source

1. Implement it under `internal/services/<area>/` and register it in
   `Resources()` or `DataSources()` in `internal/provider/provider.go`.
2. Add unit tests and, where the test organization supports it, an acceptance test.
3. Add an example (`resource.tf` or `data-source.tf`). For resources, also add an
   `import.sh` that shows the import ID format.
4. Add the name to the `subcategory` lookup in both templates.
5. Run `make docs` and commit the generated files.

## Pull requests

- Start the title with a short area prefix, for example `feat:`, `fix:`,
  `docs:`, `ci:` or `deps:`.
- Add a line to `CHANGELOG.md` under **Unreleased** for any user-visible change.
- Every pull request needs an approving review from a code owner (see
  `.github/CODEOWNERS`) before it can merge.

## License

By contributing, you agree that your contributions are licensed under the
[Apache License, Version 2.0](LICENSE).
