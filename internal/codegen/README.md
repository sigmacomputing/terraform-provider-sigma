# Codegen pipeline

This provider uses HashiCorp's two-stage codegen — `tfplugingen-openapi` and
`tfplugingen-framework` — to scaffold schema and model skeletons from Sigma's
OpenAPI spec. **CRUD logic is hand-written**, not generated. Regenerating
schemas does not (and should not) touch the resource files under
`internal/services/<group>/`.

## When to regenerate

When Sigma adds a new endpoint, field, or resource you want to expose:

```bash
# 1. Pull the latest OpenAPI spec into ../sigma-docs/reference/
# 2. From the repo root:
make generate
```

The generated artifacts land in `internal/services/_generated/` and are
diffed against the hand-written schemas as part of CI (so drift between the
spec and the provider is visible in PRs).

## What the generator handles

- Top-level field types and constraints (string, bool, enum, format)
- Required vs optional vs computed
- Basic nested objects

## What the generator does NOT handle (hand-finish required)

- **Polymorphic `details`** on `POST /v2/connections` (oneOf per warehouse).
  We accept `details_json` as a string and forward verbatim.
- **Composite-ID nested resources** (`sigma_team_membership`,
  `sigma_user_attribute_*_assignment`, `sigma_connection_grant`,
  `sigma_connection_path_grant`). The composite ID lives in our
  `ImportState` implementation.
- **Pagination shape differences** (v2 array vs v2.1 cursor). Handled by
  `internal/client/pagination.go`.
- **Idempotency** for non-idempotent POSTs. Lookup-or-create logic lives in
  the resource's `Create` method.

## Tooling versions

Pinned via `tools/tools.go`. `go mod tidy` keeps them in sync with the rest
of the module.
