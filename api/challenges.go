package api

import (
	"context"
	"net/url"

	"github.com/iximiuz/labctl/content"
)

type Challenge struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name        string   `json:"name" yaml:"name"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Categories  []string `json:"categories" yaml:"categories"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	Authors []Author `json:"authors" yaml:"authors"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`

	AttemptCount    int `json:"attemptCount" yaml:"attemptCount"`
	CompletionCount int `json:"completionCount" yaml:"completionCount"`

	Play *Play `json:"play,omitempty" yaml:"play,omitempty"`

	Tasks map[string]PlayTask `json:"tasks,omitempty" yaml:"tasks,omitempty"`
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

func (ch *Challenge) IsOfficial() bool {
	for _, author := range ch.Authors {
		if !author.Official {
			return false
		}
	}
	return len(ch.Authors) > 0
}

func (ch *Challenge) IsAuthoredBy(userID string) bool {
	for _, a := range ch.Authors {
		if a.UserID == userID {
			return true
		}
	}
	return false
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
	Category []string
	Status   []string
}

func (c *Client) ListChallenges(ctx context.Context, opts *ListChallengesOptions) ([]*Challenge, error) {
	var challenges []*Challenge
	query := url.Values{}

	for _, category := range opts.Category {
		query.Add("category", category)
	}

	for _, status := range opts.Status {
		query.Add("status", status)
	}

	return challenges, c.GetInto(ctx, "/challenges", query, nil, &challenges)
}

func (c *Client) ListAuthoredChallenges(ctx context.Context) ([]Challenge, error) {
	var challenges []Challenge
	return challenges, c.GetInto(ctx, "/author/challenges", nil, nil, &challenges)
}

type StartChallengeOptions struct {
	SafetyDisclaimerConsent bool
	AsFreeTierUser          bool
}

func (c *Client) StartChallenge(ctx context.Context, name string, opts StartChallengeOptions) (*Challenge, error) {
	type startChallengeRequest struct {
		Started                 bool `json:"started"`
		SafetyDisclaimerConsent bool `json:"safetyDisclaimerConsent,omitempty"`
		AsFreeTierUser          bool `json:"asFreeTierUser,omitempty"`
	}
	req := startChallengeRequest{
		Started:                 true,
		SafetyDisclaimerConsent: opts.SafetyDisclaimerConsent,
		AsFreeTierUser:          opts.AsFreeTierUser,
	}

	body, err := toJSONBody(req)
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
