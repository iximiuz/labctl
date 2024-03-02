package api

import (
	"context"
)

type Me struct {
	ID              string `json:"id" yaml:"userId"`
	GithubProfileId string `json:"githubProfileId" yaml:"githubProfileId"`
}

func (c *Client) GetMe(ctx context.Context) (*Me, error) {
	var me Me
	return &me, c.GetInto(ctx, "/account", nil, nil, &me)
}
