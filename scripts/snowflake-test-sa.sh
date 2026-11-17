#!/usr/bin/env bash
#
# snowflake-test-sa.sh -- provision (and tear down) a throwaway Snowflake service
# account + scratch warehouse/db/table for the provider's Snowflake-gated
# acceptance tests, and create a matching Sigma connection.
#
# It emits the env vars those tests read:
#   SIGMA_TEST_SNOWFLAKE_ACCOUNT / _USER / _PRIVATE_KEY   (connection acctests)
#   SIGMA_TEST_CONNECTION_ID                              (data-model / path acctests)
#   SIGMA_TEST_CONNECTION_PATH / SIGMA_TEST_TABLE_ID      (path / table-column acctests)
#
# Usage:
#   eval "$(./scripts/snowflake-test-sa.sh bootstrap)"   # provisions + sets env
#   make acctest                                          # now the gated tests run
#   ./scripts/snowflake-test-sa.sh teardown               # drops everything
#
# Progress goes to stderr; only the `export` block goes to stdout (so `eval` is
# clean). Bootstrap also writes the same block to $STATE_FILE for teardown.
#
# Prereqs: snow CLI with an ACCOUNTADMIN connection to a test Snowflake account,
# openssl, and Sigma OAuth creds in .env (client_id / client_secret).
# Required env: SNOW_CONN (snow connection name), SF_ACCOUNT (<org>-<account>).
# Optional env: other SF_* names, SIGMA_BASE_URL.
set -euo pipefail

SNOW_CONN="${SNOW_CONN:?set SNOW_CONN to a snow CLI connection with ACCOUNTADMIN}"
SF_ACCOUNT="${SF_ACCOUNT:?set SF_ACCOUNT to <org>-<account>}" # Sigma's `account` field
SF_USER="${SF_USER:-TF_ACC_SA}"
SF_ROLE="${SF_ROLE:-TF_ACC_ROLE}"
SF_WH="${SF_WH:-TF_ACC_WH}"
SF_DB="${SF_DB:-TF_ACC_DB}"
SF_SCHEMA="PUBLIC"
SF_TABLE="CUSTOMERS"

WORKDIR="${WORKDIR:-tmp/snowflake-test-sa}" # under gitignored tmp/
KEYFILE="$WORKDIR/rsa_key.p8"
STATE_FILE="$WORKDIR/env.sh"

SIGMA_BASE_URL="${SIGMA_BASE_URL:-https://api.sigmacomputing.com}"

log() { echo "snowflake-test-sa: $*" >&2; }
die() { echo "snowflake-test-sa: ERROR: $*" >&2; exit 1; }

sf() { # run SQL via the snow CLI against the admin connection
	snow sql -c "$SNOW_CONN" --query "$1" >&2
}

# Load Sigma OAuth creds from .env (lowercase client_id/client_secret) if not already set.
load_sigma_creds() {
	if [[ -z "${client_id:-}" || -z "${client_secret:-}" ]]; then
		[[ -f .env ]] || die "no .env and client_id/client_secret unset"
		set -a; # shellcheck disable=SC1091
		source .env; set +a
	fi
	[[ -n "${client_id:-}" && -n "${client_secret:-}" ]] || die "client_id/client_secret missing"
}

sigma_token() {
	curl -sS -X POST "$SIGMA_BASE_URL/v2/auth/token" \
		-H "Content-Type: application/x-www-form-urlencoded" \
		--data-urlencode "grant_type=client_credentials" \
		--data-urlencode "client_id=$client_id" \
		--data-urlencode "client_secret=$client_secret" |
		python3 -c "import sys,json;print(json.load(sys.stdin).get('access_token',''))"
}

# ---------------------------------------------------------------------------
bootstrap() {
	command -v snow >/dev/null || die "snow CLI not found"
	command -v openssl >/dev/null || die "openssl not found"
	command -v python3 >/dev/null || die "python3 not found"
	load_sigma_creds
	mkdir -p "$WORKDIR"; chmod 700 "$WORKDIR"

	log "Generating RSA key-pair for $SF_USER..."
	openssl genrsa 2048 2>/dev/null | openssl pkcs8 -topk8 -inform PEM -out "$KEYFILE" -nocrypt
	chmod 600 "$KEYFILE"
	# Public key body (no PEM header/footer/newlines) for Snowflake RSA_PUBLIC_KEY.
	local pub
	pub="$(openssl rsa -in "$KEYFILE" -pubout 2>/dev/null | sed '1d;$d' | tr -d '\n')"

	log "Provisioning Snowflake objects via snow connection '$SNOW_CONN'..."
	sf "CREATE WAREHOUSE IF NOT EXISTS $SF_WH WAREHOUSE_SIZE=XSMALL AUTO_SUSPEND=60 AUTO_RESUME=TRUE INITIALLY_SUSPENDED=TRUE;"
	sf "CREATE DATABASE IF NOT EXISTS $SF_DB;"
	# OR REPLACE guarantees a known schema even if a prior run left a different one.
	sf "CREATE OR REPLACE TABLE $SF_DB.$SF_SCHEMA.$SF_TABLE (ID NUMBER, NAME STRING, EMAIL STRING);"
	sf "INSERT INTO $SF_DB.$SF_SCHEMA.$SF_TABLE (ID, NAME, EMAIL) VALUES (1,'Ada','ada@example.com'), (2,'Bob','bob@example.com');"
	sf "CREATE ROLE IF NOT EXISTS $SF_ROLE;"
	sf "GRANT USAGE ON WAREHOUSE $SF_WH TO ROLE $SF_ROLE;"
	sf "GRANT USAGE ON DATABASE $SF_DB TO ROLE $SF_ROLE;"
	sf "GRANT USAGE ON SCHEMA $SF_DB.$SF_SCHEMA TO ROLE $SF_ROLE;"
	sf "GRANT SELECT ON ALL TABLES IN SCHEMA $SF_DB.$SF_SCHEMA TO ROLE $SF_ROLE;"
	sf "CREATE USER IF NOT EXISTS $SF_USER RSA_PUBLIC_KEY='$pub' DEFAULT_ROLE=$SF_ROLE DEFAULT_WAREHOUSE=$SF_WH;"
	sf "ALTER USER $SF_USER SET RSA_PUBLIC_KEY='$pub';" # idempotent re-set on reruns
	sf "GRANT ROLE $SF_ROLE TO USER $SF_USER;"

	log "Creating Sigma connection (validates connectivity at create time)..."
	local token; token="$(sigma_token)"
	[[ -n "$token" ]] || die "Sigma auth failed against $SIGMA_BASE_URL"
	local pem; pem="$(cat "$KEYFILE")"
	local body conn_id
	body="$(python3 -c "import json,sys; print(json.dumps({'name':'tf-acc-snowflake','details':{'type':'snowflake','authType':'key-pair','account':sys.argv[1],'user':sys.argv[2],'privateKey':sys.argv[3],'warehouse':sys.argv[4],'role':sys.argv[5]}}))" "$SF_ACCOUNT" "$SF_USER" "$pem" "$SF_WH" "$SF_ROLE")"
	conn_id="$(curl -sS -X POST "$SIGMA_BASE_URL/v2/connections" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$body" |
		python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('connectionId',''))" 2>/dev/null || true)"
	if [[ -z "$conn_id" ]]; then
		log "WARNING: Sigma connection create failed -- connection-id-dependent tests will skip."
	else
		log "Sigma connection created: $conn_id"
	fi

	# Emit + persist the env block.
	{
		echo "export SIGMA_TEST_SNOWFLAKE_ACCOUNT='$SF_ACCOUNT'"
		echo "export SIGMA_TEST_SNOWFLAKE_USER='$SF_USER'"
		echo "export SIGMA_TEST_SNOWFLAKE_PRIVATE_KEY='$(cat "$KEYFILE")'"
		# Path resolver expects a DOT-joined path (DATABASE.SCHEMA[.TABLE]), not slashes.
		[[ -n "$conn_id" ]] && echo "export SIGMA_TEST_CONNECTION_ID='$conn_id'"
		[[ -n "$conn_id" ]] && echo "export SIGMA_TEST_CONNECTION_PATH='$SF_DB.$SF_SCHEMA'"
		# Note: SIGMA_TEST_TABLE_ID must be a *resolved table UUID* (endpoint
		# /v2/connections/tables/{uuid}/columns), which requires Sigma to have crawled
		# the connection's catalog. We don't emit it here — the single
		# table-columns data-source test skips unless you set it manually after sync.
	} | tee "$STATE_FILE"
	log "Wrote env to $STATE_FILE. Run: eval \"\$($0 bootstrap)\"  then  make acctest"
}

# ---------------------------------------------------------------------------
teardown() {
	command -v snow >/dev/null || die "snow CLI not found"
	log "Dropping Snowflake objects..."
	sf "DROP USER IF EXISTS $SF_USER;"
	sf "DROP ROLE IF EXISTS $SF_ROLE;"
	sf "DROP WAREHOUSE IF EXISTS $SF_WH;"
	sf "DROP DATABASE IF EXISTS $SF_DB;"

	# Delete the Sigma connection if we recorded one.
	if [[ -f "$STATE_FILE" ]]; then
		local conn_id; conn_id="$(sed -n "s/^export SIGMA_TEST_CONNECTION_ID='\(.*\)'$/\1/p" "$STATE_FILE")"
		if [[ -n "$conn_id" ]]; then
			load_sigma_creds
			local token; token="$(sigma_token)"
			log "Deleting Sigma connection $conn_id..."
			curl -sS -o /dev/null -X DELETE "$SIGMA_BASE_URL/v2/connections/$conn_id" -H "Authorization: Bearer $token" || true
		fi
	fi
	rm -rf "$WORKDIR"
	log "Teardown complete."
}

case "${1:-}" in
	bootstrap) bootstrap ;;
	teardown)  teardown ;;
	*) echo "Usage: $0 {bootstrap|teardown}" >&2; exit 2 ;;
esac
