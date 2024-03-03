package api

import (
	"context"
)

type Playground struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
}

func (c *Client) GetPlayground(ctx context.Context, name string) (*Playground, error) {
	var p Playground
	return &p, c.GetInto(ctx, "/playgrounds/"+name, nil, nil, &p)
}

func (c *Client) ListPlaygrounds(ctx context.Context) ([]Playground, error) {
	var plays []Playground
	return plays, c.GetInto(ctx, "/playgrounds", nil, nil, &plays)
}
