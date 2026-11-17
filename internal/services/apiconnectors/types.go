// Package apiconnectors implements sigma_api_connector_credential and
// sigma_api_connector — Sigma's "API connectors" feature, which lets Sigma call
// EXTERNAL HTTP endpoints from within workbooks.
//
// This is distinct from sigma_api_credential (package credentials), which models
// OAuth2 client credentials for calling Sigma's OWN REST API via /v2/credentials.
// The two are intentionally separate resource types despite the similar names.
//
// Both resources carry complex, polymorphic sub-objects (an api-connector's
// `params`/`config` request shape, and an api-credential's `credential` oneOf for
// the auth method). Following the sigma_connection `details_json` convention, those
// are accepted as opaque JSON-string attributes and forwarded verbatim. The trade
// off — like details_json — is no drift detection on those fields: GET responses
// are not used to refresh them (and credentials are never returned).
package apiconnectors

import "time"

// apiCredential is the GET/POST response shape for /v2/api-credentials.
// The `credential` secret is write-only (not refreshed into state).
type apiCredential struct {
	APICredentialID string    `json:"apiCredentialId"`
	Name            string    `json:"name"`
	AuthMethod      string    `json:"authMethod"`
	Allowlist       []string  `json:"allowlist"`
	Description     string    `json:"description,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// apiConnector is the GET/POST response shape for /v2/api-connectors.
// `params`/`config` are write-only (not refreshed into state).
type apiConnector struct {
	APIConnectorID string    `json:"apiConnectorId"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	AuthID         *string   `json:"authId,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

const tsLayout = "2006-01-02T15:04:05Z07:00"

// grantGrantee mirrors the /v2/grants grantee oneOf — exactly one field is set.
type grantGrantee struct {
	MemberID string `json:"memberId,omitempty"`
	TeamID   string `json:"teamId,omitempty"`
}

// createGrantReq is the POST /v2/grants body; inodeId is the API connector ID.
type createGrantReq struct {
	Grantee    grantGrantee `json:"grantee"`
	Permission string       `json:"permission"`
	InodeID    string       `json:"inodeId"`
}

// inodeGrant is the /v2/grants entry shape.
type inodeGrant struct {
	GrantID    string  `json:"grantId"`
	InodeID    string  `json:"inodeId"`
	Permission string  `json:"permission"`
	MemberID   *string `json:"memberId"`
	TeamID     *string `json:"teamId"`
}

func (x inodeGrant) matches(g grantGrantee, perm string) bool {
	if x.Permission != perm {
		return false
	}
	if g.MemberID != "" {
		return x.MemberID != nil && *x.MemberID == g.MemberID
	}
	return x.TeamID != nil && *x.TeamID == g.TeamID
}
