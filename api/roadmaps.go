package api

import (
	"context"

	"github.com/iximiuz/labctl/content"
)

type Roadmap struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name  string `json:"name" yaml:"name"`
	Title string `json:"title" yaml:"title"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`

	Authors []Author `json:"authors" yaml:"authors"`
}

var _ content.Content = (*Roadmap)(nil)

func (t *Roadmap) GetKind() content.ContentKind {
	return content.KindRoadmap
}

func (t *Roadmap) GetName() string {
	return t.Name
}

func (t *Roadmap) GetPageURL() string {
	return t.PageURL
}

func (t *Roadmap) IsOfficial() bool {
	for _, author := range t.Authors {
		if !author.Official {
			return false
		}
	}
	return len(t.Authors) > 0
}

func (t *Roadmap) IsAuthoredBy(userID string) bool {
	for _, a := range t.Authors {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

type CreateRoadmapRequest struct {
	Name   string `json:"name"`
	Sample bool   `json:"sample"`
}

func (c *Client) CreateRoadmap(ctx context.Context, req CreateRoadmapRequest) (*Roadmap, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var t Roadmap
	return &t, c.PostInto(ctx, "/roadmaps", nil, nil, body, &t)
}

func (c *Client) GetRoadmap(ctx context.Context, name string) (*Roadmap, error) {
	var t Roadmap
	return &t, c.GetInto(ctx, "/roadmaps/"+name, nil, nil, &t)
}

func (c *Client) ListRoadmaps(ctx context.Context) ([]Roadmap, error) {
	var roadmaps []Roadmap
	return roadmaps, c.GetInto(ctx, "/roadmaps", nil, nil, &roadmaps)
}

func (c *Client) ListAuthoredRoadmaps(ctx context.Context) ([]Roadmap, error) {
	var roadmaps []Roadmap
	return roadmaps, c.GetInto(ctx, "/author/roadmaps", nil, nil, &roadmaps)
}

func (c *Client) DeleteRoadmap(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/roadmaps/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
