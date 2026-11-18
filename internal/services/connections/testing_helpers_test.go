package connections_test

import (
	"os"
	"testing"
)

// snowflakeEnv returns Snowflake credentials from env vars, or skips the
// test if any are missing. Connection acctests need a live warehouse —
// Sigma validates connectivity at create time.
func snowflakeEnv(t *testing.T) (account, user, privateKey string) {
	t.Helper()
	account = os.Getenv("SIGMA_TEST_SNOWFLAKE_ACCOUNT")
	user = os.Getenv("SIGMA_TEST_SNOWFLAKE_USER")
	privateKey = os.Getenv("SIGMA_TEST_SNOWFLAKE_PRIVATE_KEY")
	if account == "" || user == "" || privateKey == "" {
		t.Skip("SIGMA_TEST_SNOWFLAKE_ACCOUNT/USER/PRIVATE_KEY not set; skipping connection test")
	}
	return
}

// pathEnv returns the optional table path (e.g. "SAMPLE_DB/PUBLIC/CUSTOMERS")
// used by connection-path tests, or skips if unset.
func pathEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("%s not set; skipping path-dependent test", key)
	}
	return v
}
