// Package jwks implements sigma_jwk_set and sigma_jwk_set_key.
package jwks

import (
	"context"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

type validForReq struct {
	ValidForDays *int64 `json:"validForDays,omitempty"`
}

type keyResp struct {
	JSONWebKeyID string `json:"jsonWebKeyId"`
	JwkSetID     string `json:"jwkSetId"`
	Algorithm    string `json:"algorithm"`
	PublicKey    string `json:"publicKey"`
	ExpiresAt    string `json:"expiresAt"`
	Status       string `json:"status"`
}

func setPath(setID string) string { return "/v2/jwkSets/" + client.EscapeID(setID) }

func keysPath(setID string) string { return setPath(setID) + "/jsonWebKeys" }

func keyPath(setID, keyID string) string { return keysPath(setID) + "/" + client.EscapeID(keyID) }

// listKeys returns a set's keys; Sigma returns an empty list (not 404) for an unknown set.
// The endpoint rejects pageSize, so no page params are sent.
func listKeys(ctx context.Context, c *client.Client, setID string) ([]keyResp, error) {
	return client.ListAllPageToken[keyResp](ctx, c, keysPath(setID), nil)
}

func keysCacheKey(setID string) string { return "jwk-keys:" + setID }

// cachedKeys is listKeys shared by every Read of one set during a run.
func cachedKeys(ctx context.Context, c *client.Client, setID string) ([]keyResp, error) {
	return client.CachedList(c, keysCacheKey(setID), func() ([]keyResp, error) {
		return listKeys(ctx, c, setID)
	})
}

func invalidateKeys(c *client.Client, setID string) { client.InvalidateList(c, keysCacheKey(setID)) }
