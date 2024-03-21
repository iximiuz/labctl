package api

import (
	"context"
	"net/url"

	"github.com/iximiuz/labctl/internal/content"
)

type PutContentMarkdownRequest struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (c *Client) PutContentMarkdown(ctx context.Context, req PutContentMarkdownRequest) error {
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

func (c *Client) ListContentFiles(
	ctx context.Context,
	kind content.ContentKind,
	name string,
) ([]string, error) {
	var files []string
	if err := c.GetInto(ctx, "/content/files", url.Values{
		"kind": []string{kind.String()},
		"name": []string{name},
	}, nil, &files); err != nil {
		return nil, err
	}

	return files, nil
}
