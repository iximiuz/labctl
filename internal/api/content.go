package api

import (
	"context"
)

type PutMarkdownRequest struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (c *Client) PutMarkdown(ctx context.Context, req PutMarkdownRequest) error {
	body, err := toJSONBody(req)
	if err != nil {
		return err
	}

	resp, err := c.Put(ctx, "/content/markdown", nil, nil, body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
