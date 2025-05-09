package api

import (
	"context"

	"github.com/iximiuz/labctl/content"
)

type Tutorial struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name  string `json:"name" yaml:"name"`
	Title string `json:"title" yaml:"title"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`

	Authors []Author `json:"authors" yaml:"authors"`
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

func (c *Client) ListTutorials(ctx context.Context) ([]Tutorial, error) {
	var tutorials []Tutorial
	return tutorials, c.GetInto(ctx, "/tutorials", nil, nil, &tutorials)
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
