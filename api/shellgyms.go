package api

import (
	"context"

	"github.com/iximiuz/labctl/content"
)

type ShellGym struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name        string   `json:"name" yaml:"name"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Categories  []string `json:"categories" yaml:"categories"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Published   bool     `json:"published" yaml:"published"`

	Authors []Author `json:"authors" yaml:"authors"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`

	Status string `json:"status,omitempty" yaml:"status,omitempty"`

	Play *Play `json:"play,omitempty" yaml:"play,omitempty"`
}

var _ content.Content = (*ShellGym)(nil)

func (g *ShellGym) GetKind() content.ContentKind {
	return content.KindShellGym
}

func (g *ShellGym) GetName() string {
	return g.Name
}

func (g *ShellGym) GetPageURL() string {
	return g.PageURL
}

func (g *ShellGym) IsOfficial() bool {
	for _, author := range g.Authors {
		if !author.Official {
			return false
		}
	}
	return len(g.Authors) > 0
}

func (g *ShellGym) IsAuthoredBy(userID string) bool {
	for _, a := range g.Authors {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

type CreateShellGymRequest struct {
	Name string `json:"name"`
}

func (c *Client) CreateShellGym(ctx context.Context, req CreateShellGymRequest) (*ShellGym, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var g ShellGym
	return &g, c.PostInto(ctx, "/shell-gyms", nil, nil, body, &g)
}

func (c *Client) GetShellGym(ctx context.Context, name string) (*ShellGym, error) {
	var g ShellGym
	return &g, c.GetInto(ctx, "/shell-gyms/"+name, nil, nil, &g)
}

func (c *Client) ListShellGyms(ctx context.Context) ([]ShellGym, error) {
	var shellGyms []ShellGym
	return shellGyms, c.GetInto(ctx, "/shell-gyms", nil, nil, &shellGyms)
}

func (c *Client) ListAuthoredShellGyms(ctx context.Context) ([]ShellGym, error) {
	var shellGyms []ShellGym
	return shellGyms, c.GetInto(ctx, "/author/shell-gyms", nil, nil, &shellGyms)
}

type StartShellGymOptions struct {
	SafetyDisclaimerConsent bool
	AsFreeTierUser          bool
}

func (c *Client) StartShellGym(ctx context.Context, name string, opts StartShellGymOptions) (*ShellGym, error) {
	type startShellGymRequest struct {
		Started                 bool `json:"started"`
		SafetyDisclaimerConsent bool `json:"safetyDisclaimerConsent,omitempty"`
		AsFreeTierUser          bool `json:"asFreeTierUser,omitempty"`
	}
	req := startShellGymRequest{
		Started:                 true,
		SafetyDisclaimerConsent: opts.SafetyDisclaimerConsent,
		AsFreeTierUser:          opts.AsFreeTierUser,
	}

	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var g ShellGym
	return &g, c.PatchInto(ctx, "/shell-gyms/"+name, nil, nil, body, &g)
}

func (c *Client) StopShellGym(ctx context.Context, name string) (*ShellGym, error) {
	body, err := toJSONBody(map[string]any{"started": false})
	if err != nil {
		return nil, err
	}

	var g ShellGym
	return &g, c.PatchInto(ctx, "/shell-gyms/"+name, nil, nil, body, &g)
}

func (c *Client) DeleteShellGym(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/shell-gyms/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
