package api

import (
	"context"
)

type Challenge struct {
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`

	Name string `json:"name"`

	PageURL string `json:"pageUrl"`
}

type CreateChallengeRequest struct {
	Name string `json:"name"`
}

func (c *Client) CreateChallenge(ctx context.Context, req CreateChallengeRequest) (*Challenge, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var ch Challenge
	return &ch, c.PostInto(ctx, "/challenges", nil, nil, body, &ch)
}
