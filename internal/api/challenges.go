package api

import (
	"context"
)

type Challenge struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name string `json:"name" yaml:"name"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`

	AttemptCount    int `json:"attemptCount" yaml:"attemptCount"`
	CompletionCount int `json:"completionCount" yaml:"completionCount"`
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

func (c *Client) GetChallenge(ctx context.Context, name string) (*Challenge, error) {
	var ch Challenge
	return &ch, c.GetInto(ctx, "/challenges/"+name, nil, nil, &ch)
}

func (c *Client) ListChallenges(ctx context.Context) ([]Challenge, error) {
	var challenges []Challenge
	return challenges, c.GetInto(ctx, "/challenges", nil, nil, &challenges)
}

func (c *Client) ListAuthoredChallenges(ctx context.Context) ([]Challenge, error) {
	var challenges []Challenge
	return challenges, c.GetInto(ctx, "/challenges/authored", nil, nil, &challenges)
}

func (c *Client) DeleteChallenge(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/challenges/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
