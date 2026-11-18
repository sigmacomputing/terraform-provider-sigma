SHELL := /bin/bash
HOSTNAME ?= registry.terraform.io
NAMESPACE ?= sigmacomputing
NAME      ?= sigma
BINARY    ?= terraform-provider-$(NAME)
VERSION   ?= 0.0.1
OS_ARCH   ?= $(shell go env GOOS)_$(shell go env GOARCH)

.PHONY: all build install fmt vet lint test acctest generate docs docs-check clean

all: build

build:
	go build -o $(BINARY)

# Installs the provider into the local plugin cache for use with `dev_overrides`
# in ~/.terraformrc. See the README "Local development" section.
install: build
	mkdir -p ~/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)
	mv $(BINARY) ~/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)/

fmt:
	gofmt -s -w .

vet:
	go vet ./...

# Lint tools are pinned (go.mod + go.sum) in tools/lint so they don't change the provider's deps.
LINT_TOOL = go tool -modfile=tools/lint/go.mod
lint:
	@test -z "$$(gofmt -l $$(git ls-files '*.go'))" || { echo "gofmt needed:"; gofmt -l $$(git ls-files '*.go'); exit 1; }
	go vet ./...
	$(LINT_TOOL) staticcheck ./...
	$(LINT_TOOL) errcheck ./...

test:
	go test -count=1 ./...

# Acceptance tests: real API calls. Requires SIGMA_CLIENT_ID/SECRET in the env
# (or .env). Costs real Sigma API quota — run sparingly. -p 1 serializes
# package execution; Sigma's API rate-limits parallel test load and produces
# spurious failures otherwise.
acctest:
	TF_ACC=1 go test -count=1 -timeout 30m -p 1 ./internal/services/...

# Regenerate schemas from Sigma's OpenAPI spec into internal/services/_generated/.
# Run from a clean working tree so the diff is meaningful. Tools are version-pinned
# via go.mod (see tools/tools.go), so `go run` resolves the exact pinned binary.
generate:
	go run github.com/hashicorp/terraform-plugin-codegen-openapi/cmd/tfplugingen-openapi generate \
		--config internal/codegen/generator_config.yml \
		--output internal/codegen/provider-spec.json \
		../sigma-docs/fern/apis/sigma-rest-api/openapi.json
	go run github.com/hashicorp/terraform-plugin-codegen-framework/cmd/tfplugingen-framework generate resources \
		--input internal/codegen/provider-spec.json \
		--output internal/services/_generated/

# Generate Terraform Registry docs from schema + examples/. Tool is version-pinned
# via go.mod (see tools/tools.go).
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate

# Fails if generated docs are stale or examples are unformatted.
docs-check: docs
	git diff --exit-code -- docs/
	terraform fmt -check -recursive examples

clean:
	rm -f $(BINARY) coverage.out coverage.html
	rm -rf dist/ internal/services/_generated/ internal/codegen/provider-spec.json
