package api

import (
	"context"
	"net/url"
)

type PremiumAccess struct {
	Until    string `json:"until,omitempty" yaml:"until,omitempty"`
	Lifetime bool   `json:"lifetime,omitempty" yaml:"lifetime,omitempty"`
	Trial    bool   `json:"trial,omitempty" yaml:"trial,omitempty"`
}

type Me struct {
	ID string `json:"id" yaml:"userId"`

	GithubProfileId string `json:"githubProfileId" yaml:"githubProfileId"`

	PremiumAccess *PremiumAccess `json:"premiumAccess,omitempty" yaml:"premiumAccess,omitempty"`
}

func (c *Client) GetMe(ctx context.Context) (*Me, error) {
	var me Me
	return &me, c.GetInto(ctx, "/auth/me", url.Values{"authenticate": []string{"true"}}, nil, &me)
}
