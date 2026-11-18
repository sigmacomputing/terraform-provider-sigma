#!/usr/bin/env bash
#
# publish-provider.sh — publish terraform-provider-sigma to the HCP Terraform
# (Terraform Cloud) PRIVATE provider registry from goreleaser artifacts.
#
# Providers (unlike modules) do NOT need a VCS connection. They are published by
# API from GPG-signed release artifacts — exactly what `goreleaser release`
# produces in ./dist.
#
# Usage:
#   # one-time: register the GPG public key that signs SHA256SUMS
#   ./scripts/publish-provider.sh register-gpg path/to/public-key.asc
#
#   # per release: download the CI-built release, then publish it
#   gh release download vX.Y.Z -D dist
#   ./scripts/publish-provider.sh publish
#
#   # check dist/ without publishing (publish runs the same checks first)
#   ./scripts/publish-provider.sh verify
#
# publish refuses artifacts unless the SHA256SUMS signature verifies against a
# key in scripts/release-keys/, every file matches SHA256SUMS, each zip and
# SHA256SUMS carry a provenance attestation from release.yml for the tag, and
# the tag is annotated with a signature GitHub verifies. Needs gpg and gh.
#
# Auth: reads the HCP Terraform token from $TFE_TOKEN, falling back to
# $terraform_api (the name used in this repo's .env). Never pass it on the CLI.
#
# Override defaults via env: TFE_HOST, TFE_ORG, PROVIDER_NAME, DIST_DIR, VERSION,
# GPG_KEY_ID.
set -euo pipefail

TFE_HOST="${TFE_HOST:-app.terraform.io}"
TFE_ORG="${TFE_ORG:-sigmacomputing}"
PROVIDER_NAME="${PROVIDER_NAME:-sigma}"
PROJECT="terraform-provider-${PROVIDER_NAME}"
REPO="${REPO:-sigmacomputing/terraform-provider-sigma}"
KEYS_DIR="${KEYS_DIR:-$(cd "$(dirname "$0")" && pwd)/release-keys}"
DIST_DIR="${DIST_DIR:-dist}"
API="https://${TFE_HOST}/api/v2"

TOKEN="${TFE_TOKEN:-${terraform_api:-}}"
require_token() {
  if [[ -z "${TOKEN}" ]]; then
    echo "ERROR: set \$TFE_TOKEN (or \$terraform_api). Tip: 'set -a; source .env; set +a'." >&2
    exit 1
  fi
}

command -v jq >/dev/null || { echo "ERROR: jq is required." >&2; exit 1; }

# api METHOD PATH [json-body] — calls the JSON:API, fails on >=400 except where
# the caller inspects the captured status via the trailing "HTTP <code>" line.
api() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-sS -X "$method"
    -H "Authorization: Bearer ${TOKEN}"
    -H "Content-Type: application/vnd.api+json"
    -w $'\nHTTP %{http_code}')
  [[ -n "$body" ]] && args+=(-d "$body")
  curl "${args[@]}" "${API}${path}"
}

# upload FILE URL CONTENT_TYPE — PUT a release artifact to a pre-signed link.
upload() {
  local file="$1" url="$2" ctype="$3"
  curl -sS -f -X PUT -H "Content-Type: ${ctype}" --data-binary "@${file}" "$url"
}

die() { echo "ERROR: $*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# dist_inputs sets shasums, sig and version from $DIST_DIR (or $VERSION).
dist_inputs() {
  [[ -d "$DIST_DIR" ]] || die "no $DIST_DIR/ — run 'gh release download vX.Y.Z -D $DIST_DIR' first."
  shasums="$(ls "${DIST_DIR}/${PROJECT}"_*_SHA256SUMS 2>/dev/null | head -1)" || true
  sig="${shasums}.sig"
  [[ -f "$shasums" ]] || die "SHA256SUMS not found in $DIST_DIR/"
  [[ -f "$sig" ]] || die "SHA256SUMS.sig not found in $DIST_DIR/"
  version="${VERSION:-}"
  if [[ -z "$version" ]]; then
    version="$(basename "$shasums" | sed -E "s/^${PROJECT}_(.+)_SHA256SUMS$/\1/")"
  fi
  [[ -n "$version" ]] || die "could not determine version."
}

# verify_release_inputs SHASUMS SIG VERSION KEY_ID — dies unless dist/ is the
# CI-built, signed release for tag vVERSION.
verify_release_inputs() {
  local shasums="$1" sig="$2" version="$3" key_id="$4" tag="v$3"
  command -v gpg >/dev/null || die "gpg is required."
  command -v gh >/dev/null || die "gh is required."
  echo ">> Verifying release inputs for ${tag}…"

  # 1) SHA256SUMS is signed by the key the registry will be told about, checked
  #    against only the keys committed in scripts/release-keys/.
  local gnupg; gnupg="$(mktemp -d)"
  ls "$KEYS_DIR"/*.asc >/dev/null 2>&1 || { rm -rf "$gnupg"; die "no public keys in $KEYS_DIR"; }
  GNUPGHOME="$gnupg" gpg --batch --quiet --import "$KEYS_DIR"/*.asc 2>/dev/null
  local status
  status="$(GNUPGHOME="$gnupg" gpg --batch --status-fd 1 --verify "$sig" "$shasums" 2>/dev/null)" || true
  rm -rf "$gnupg"
  local signer; signer="$(awk '$2=="VALIDSIG" {print $NF}' <<<"$status")"
  [[ -n "$signer" ]] || die "SHA256SUMS signature does not verify against $KEYS_DIR"
  [[ "$signer" == *"$key_id" ]] || die "SHA256SUMS is signed by ${signer}, not GPG_KEY_ID ${key_id}"
  echo "   ok  signature by ${signer}"

  # 2) every file listed in SHA256SUMS exists and matches.
  local sha fname n=0
  while read -r sha fname; do
    [[ -f "${DIST_DIR}/${fname}" ]] || die "missing ${fname} listed in SHA256SUMS"
    [[ "$(shasum -a 256 "${DIST_DIR}/${fname}" | cut -d' ' -f1)" == "$sha" ]] \
      || die "checksum mismatch: ${fname}"
    n=$((n + 1))
  done < "$shasums"
  (( n > 0 )) || die "SHA256SUMS lists no files"
  echo "   ok  ${n} files match SHA256SUMS"

  # 3) each zip and SHA256SUMS was attested by release.yml for this tag.
  local f
  for f in "${DIST_DIR}/${PROJECT}_${version}"_*.zip "$shasums"; do
    gh attestation verify "$f" --repo "$REPO" \
      --signer-workflow "${REPO}/.github/workflows/release.yml" \
      --source-ref "refs/tags/${tag}" >/dev/null 2>&1 \
      || die "no valid provenance attestation for $(basename "$f") from release.yml at ${tag}"
  done
  echo "   ok  provenance attestations from release.yml at ${tag}"

  # 4) the tag is annotated and its signature verifies on GitHub.
  local ref; ref="$(gh api "repos/${REPO}/git/ref/tags/${tag}" --jq '.object.type + " " + .object.sha' 2>/dev/null)" \
    || die "tag ${tag} not found in ${REPO}"
  [[ "${ref%% *}" == "tag" ]] || die "tag ${tag} is lightweight; release tags must be annotated and signed"
  [[ "$(gh api "repos/${REPO}/git/tags/${ref#* }" --jq .verification.verified)" == "true" ]] \
    || die "tag ${tag} has no signature GitHub verifies (git tag -s)"
  echo "   ok  ${tag} is a signed annotated tag"
}

verify() {
  local shasums sig version
  dist_inputs
  local gpg_key_id="${GPG_KEY_ID:-}"
  [[ -n "$gpg_key_id" ]] || die "set \$GPG_KEY_ID (see 'register-gpg')."
  verify_release_inputs "$shasums" "$sig" "$version" "$gpg_key_id"
}

# ---------------------------------------------------------------------------
register_gpg() {
  local keyfile="${1:?usage: register-gpg <public-key.asc>}"
  [[ -f "$keyfile" ]] || die "key file not found: $keyfile"
  echo ">> Registering GPG public key for namespace ${TFE_ORG}…"
  local payload
  payload="$(jq -n --arg ns "$TFE_ORG" --rawfile armor "$keyfile" \
    '{data:{type:"gpg-keys",attributes:{namespace:$ns,"ascii-armor":$armor}}}')"
  local resp; resp="$(curl -sS -X POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/vnd.api+json" \
    -d "$payload" \
    "https://${TFE_HOST}/api/registry/private/v2/gpg-keys")"
  local kid; kid="$(echo "$resp" | jq -r '.data.attributes."key-id" // empty')"
  [[ -n "$kid" ]] || die "GPG registration failed: $resp"
  echo ">> Registered. key-id = ${kid}"
  echo ">> Export it for publish:  export GPG_KEY_ID=${kid}"
}

# ---------------------------------------------------------------------------
ensure_provider() {
  echo ">> Ensuring provider ${TFE_ORG}/${PROVIDER_NAME} (registry: private) exists…"
  local out; out="$(api GET "/organizations/${TFE_ORG}/registry-providers/private/${TFE_ORG}/${PROVIDER_NAME}")"
  if [[ "$(echo "$out" | tail -1)" == "HTTP 200" ]]; then
    echo ">> Provider already exists."
    return
  fi
  local payload
  payload="$(jq -n --arg n "$PROVIDER_NAME" --arg ns "$TFE_ORG" \
    '{data:{type:"registry-providers",attributes:{name:$n,namespace:$ns,"registry-name":"private"}}}')"
  out="$(api POST "/organizations/${TFE_ORG}/registry-providers" "$payload")"
  [[ "$(echo "$out" | tail -1)" =~ HTTP\ 20 ]] || die "create provider failed:\n$out"
  echo ">> Provider created."
}

# ---------------------------------------------------------------------------
publish() {
  local shasums sig version
  dist_inputs

  local gpg_key_id="${GPG_KEY_ID:-}"
  [[ -n "$gpg_key_id" ]] || die "set \$GPG_KEY_ID (see 'register-gpg')."
  verify_release_inputs "$shasums" "$sig" "$version" "$gpg_key_id"

  echo ">> Publishing ${PROVIDER_NAME} v${version} to ${TFE_ORG} (private registry)…"
  ensure_provider

  local base="/organizations/${TFE_ORG}/registry-providers/private/${TFE_ORG}/${PROVIDER_NAME}"

  echo ">> Creating version ${version}…"
  local vpayload vout
  vpayload="$(jq -n --arg v "$version" --arg k "$gpg_key_id" \
    '{data:{type:"registry-provider-versions",attributes:{version:$v,"key-id":$k,protocols:["6.0"]}}}')"
  vout="$(api POST "${base}/versions" "$vpayload")"
  if [[ ! "$(echo "$vout" | tail -1)" =~ HTTP\ 20 ]]; then
    echo "$vout" | grep -qi "has already been taken" \
      && die "version ${version} already exists in the registry — bump the tag." \
      || die "create version failed:\n$vout"
  fi
  local vjson; vjson="$(echo "$vout" | sed '$d')"
  local up_shasums up_sig
  up_shasums="$(echo "$vjson" | jq -r '.data.links."shasums-upload"')"
  up_sig="$(echo "$vjson" | jq -r '.data.links."shasums-sig-upload"')"

  echo ">> Uploading SHA256SUMS + signature…"
  upload "$shasums" "$up_shasums" "application/octet-stream"
  upload "$sig" "$up_sig" "application/octet-stream"

  echo ">> Creating platforms + uploading binaries…"
  # SHA256SUMS lines:  <sha>  terraform-provider-sigma_<ver>_<os>_<arch>.zip
  while read -r sha fname; do
    [[ "$fname" =~ ^${PROJECT}_${version}_([a-z0-9]+)_([a-z0-9]+)\.zip$ ]] || continue
    local os="${BASH_REMATCH[1]}" arch="${BASH_REMATCH[2]}"
    local zip="${DIST_DIR}/${fname}"
    [[ -f "$zip" ]] || { echo "   ! missing $zip, skipping"; continue; }
    echo "   - ${os}/${arch}"
    local ppayload pout
    ppayload="$(jq -n --arg os "$os" --arg arch "$arch" --arg sha "$sha" --arg fn "$fname" \
      '{data:{type:"registry-provider-version-platforms",attributes:{os:$os,arch:$arch,shasum:$sha,filename:$fn}}}')"
    pout="$(api POST "${base}/versions/${version}/platforms" "$ppayload")"
    [[ "$(echo "$pout" | tail -1)" =~ HTTP\ 20 ]] || die "create platform ${os}/${arch} failed:\n$pout"
    local up_bin; up_bin="$(echo "$pout" | sed '$d' | jq -r '.data.links."provider-binary-upload"')"
    upload "$zip" "$up_bin" "application/octet-stream"
  done < "$shasums"

  echo ">> Done. Consume with:  source = \"${TFE_HOST}/${TFE_ORG}/${PROVIDER_NAME}\""
}

# ---------------------------------------------------------------------------
case "${1:-}" in
  register-gpg) shift; require_token; register_gpg "$@" ;;
  publish)      shift; require_token; publish "$@" ;;
  verify)       shift; verify "$@" ;;
  *) echo "Usage: $0 {register-gpg <public-key.asc> | publish | verify}" >&2; exit 2 ;;
esac
