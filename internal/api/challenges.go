package api

import (
	"context"
	"net/url"

	"github.com/iximiuz/labctl/internal/content"
)

type Challenge struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name        string   `json:"name" yaml:"name"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Categories  []string `json:"categories" yaml:"categories"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`

	AttemptCount    int `json:"attemptCount" yaml:"attemptCount"`
	CompletionCount int `json:"completionCount" yaml:"completionCount"`

	Play *Play `json:"play,omitempty" yaml:"play,omitempty"`
}

var _ content.Content = (*Challenge)(nil)

func (ch *Challenge) GetKind() content.ContentKind {
	return content.KindChallenge
}

func (ch *Challenge) GetName() string {
	return ch.Name
}

func (ch *Challenge) GetPageURL() string {
	return ch.PageURL
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

type ListChallengesOptions struct {
	Category string
}

func (c *Client) ListChallenges(ctx context.Context, opts *ListChallengesOptions) ([]Challenge, error) {
	var challenges []Challenge
	query := url.Values{}
	if opts.Category != "" {
		query.Set("category", opts.Category)
	}
	return challenges, c.GetInto(ctx, "/challenges", query, nil, &challenges)
}

func (c *Client) ListAuthoredChallenges(ctx context.Context) ([]Challenge, error) {
	var challenges []Challenge
	return challenges, c.GetInto(ctx, "/challenges/authored", nil, nil, &challenges)
}

func (c *Client) StartChallenge(ctx context.Context, name string) (*Challenge, error) {
	body, err := toJSONBody(map[string]any{"started": true})
	if err != nil {
		return nil, err
	}

	var ch Challenge
	return &ch, c.PatchInto(ctx, "/challenges/"+name, nil, nil, body, &ch)
}

func (c *Client) CompleteChallenge(ctx context.Context, name string) (*Challenge, error) {
	body, err := toJSONBody(map[string]any{"completed": true})
	if err != nil {
		return nil, err
	}

	var ch Challenge
	return &ch, c.PatchInto(ctx, "/challenges/"+name, nil, nil, body, &ch)
}

func (c *Client) StopChallenge(ctx context.Context, name string) (*Challenge, error) {
	body, err := toJSONBody(map[string]any{"started": false})
	if err != nil {
		return nil, err
	}

	var ch Challenge
	return &ch, c.PatchInto(ctx, "/challenges/"+name, nil, nil, body, &ch)
}

func (c *Client) DeleteChallenge(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/challenges/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
