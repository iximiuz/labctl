package api

import (
	"context"

	"github.com/iximiuz/labctl/content"
)

type SkillPath struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name  string `json:"name" yaml:"name"`
	Title string `json:"title" yaml:"title"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`

	Authors []Author `json:"authors" yaml:"authors"`
}

var _ content.Content = (*SkillPath)(nil)

func (t *SkillPath) GetKind() content.ContentKind {
	return content.KindSkillPath
}

func (t *SkillPath) GetName() string {
	return t.Name
}

func (t *SkillPath) GetPageURL() string {
	return t.PageURL
}

func (t *SkillPath) IsOfficial() bool {
	for _, author := range t.Authors {
		if !author.Official {
			return false
		}
	}
	return len(t.Authors) > 0
}

func (t *SkillPath) IsAuthoredBy(userID string) bool {
	for _, a := range t.Authors {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

type CreateSkillPathRequest struct {
	Name string `json:"name"`
}

func (c *Client) CreateSkillPath(ctx context.Context, req CreateSkillPathRequest) (*SkillPath, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var t SkillPath
	return &t, c.PostInto(ctx, "/skill-paths", nil, nil, body, &t)
}

func (c *Client) GetSkillPath(ctx context.Context, name string) (*SkillPath, error) {
	var t SkillPath
	return &t, c.GetInto(ctx, "/skill-paths/"+name, nil, nil, &t)
}

func (c *Client) ListSkillPaths(ctx context.Context) ([]SkillPath, error) {
	var skillPaths []SkillPath
	return skillPaths, c.GetInto(ctx, "/skill-paths", nil, nil, &skillPaths)
}

func (c *Client) ListAuthoredSkillPaths(ctx context.Context) ([]SkillPath, error) {
	var skillPaths []SkillPath
	return skillPaths, c.GetInto(ctx, "/author/skill-paths", nil, nil, &skillPaths)
}

func (c *Client) DeleteSkillPath(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/skill-paths/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
