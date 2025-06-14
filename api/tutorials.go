package api

import (
	"context"
	"net/url"

	"github.com/iximiuz/labctl/content"
)

type Tutorial struct {
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
}

var _ content.Content = (*Tutorial)(nil)

func (t *Tutorial) GetKind() content.ContentKind {
	return content.KindTutorial
}

func (t *Tutorial) GetName() string {
	return t.Name
}

func (t *Tutorial) GetPageURL() string {
	return t.PageURL
}

func (t *Tutorial) IsOfficial() bool {
	for _, author := range t.Authors {
		if !author.Official {
			return false
		}
	}
	return len(t.Authors) > 0
}

func (t *Tutorial) IsAuthoredBy(userID string) bool {
	for _, a := range t.Authors {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

type CreateTutorialRequest struct {
	Name   string `json:"name"`
	Sample bool   `json:"sample"`
}

func (c *Client) CreateTutorial(ctx context.Context, req CreateTutorialRequest) (*Tutorial, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var t Tutorial
	return &t, c.PostInto(ctx, "/tutorials", nil, nil, body, &t)
}

func (c *Client) GetTutorial(ctx context.Context, name string) (*Tutorial, error) {
	var t Tutorial
	return &t, c.GetInto(ctx, "/tutorials/"+name, nil, nil, &t)
}

type ListTutorialsOptions struct {
	Category []string
	Status   []string
}

func (c *Client) ListTutorials(ctx context.Context, opts *ListTutorialsOptions) ([]Tutorial, error) {
	var tutorials []Tutorial
	query := url.Values{}

	if opts != nil {
		for _, category := range opts.Category {
			query.Add("category", category)
		}

		for _, status := range opts.Status {
			query.Add("status", status)
		}
	}

	return tutorials, c.GetInto(ctx, "/tutorials", query, nil, &tutorials)
}

func (c *Client) ListAuthoredTutorials(ctx context.Context) ([]Tutorial, error) {
	var tutorials []Tutorial
	return tutorials, c.GetInto(ctx, "/author/tutorials", nil, nil, &tutorials)
}

func (c *Client) DeleteTutorial(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/tutorials/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

type StartTutorialOptions struct {
	SafetyDisclaimerConsent bool
}

func (c *Client) StartTutorial(ctx context.Context, name string, opts StartTutorialOptions) (*Tutorial, error) {
	type startTutorialRequest struct {
		Started                 bool `json:"started"`
		SafetyDisclaimerConsent bool `json:"safetyDisclaimerConsent,omitempty"`
	}
	req := startTutorialRequest{
		Started:                 true,
		SafetyDisclaimerConsent: opts.SafetyDisclaimerConsent,
	}

	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var t Tutorial
	return &t, c.PatchInto(ctx, "/tutorials/"+name, nil, nil, body, &t)
}

func (c *Client) StopTutorial(ctx context.Context, name string) (*Tutorial, error) {
	body, err := toJSONBody(map[string]any{"started": false})
	if err != nil {
		return nil, err
	}

	var t Tutorial
	return &t, c.PatchInto(ctx, "/tutorials/"+name, nil, nil, body, &t)
}

func (c *Client) CompleteTutorial(ctx context.Context, name string) (*Tutorial, error) {
	body, err := toJSONBody(map[string]any{"completed": true})
	if err != nil {
		return nil, err
	}

	var t Tutorial
	return &t, c.PatchInto(ctx, "/tutorials/"+name, nil, nil, body, &t)
}
