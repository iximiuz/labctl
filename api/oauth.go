package api

import (
	"context"
)

// OAuthTokenScopes is the catalog of scopes an access token can be granted.
// The author:* scopes are pro-only - the server rejects them for free accounts.
var OAuthTokenScopes = []string{
	"account:read",
	"account:write",
	"learning:read",
	"learning:write",
	"playground:read",
	"playground:write",
	"author:read",
	"author:write",
}

// OAuthTokenDefaultScopes is the subset of OAuthTokenScopes available to all accounts.
var OAuthTokenDefaultScopes = []string{
	"account:read",
	"account:write",
	"learning:read",
	"learning:write",
	"playground:read",
	"playground:write",
}

type CreateOAuthTokenRequest struct {
	Name string `json:"name"`

	Scope []string `json:"scope"`

	ExpiresInDays int `json:"expiresInDays,omitempty"`
}

type OAuthToken struct {
	AccessToken string `json:"accessToken"`

	ID string `json:"id"`

	Kind string `json:"kind"`

	Name string `json:"name"`

	Scope []string `json:"scope"`

	CreatedAt string `json:"createdAt"`

	ExpiresAt string `json:"expiresAt"`
}

func (c *Client) CreateOAuthToken(ctx context.Context, req CreateOAuthTokenRequest) (*OAuthToken, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var token OAuthToken
	return &token, c.PostInto(ctx, "/oauth/tokens", nil, nil, body, &token)
}
