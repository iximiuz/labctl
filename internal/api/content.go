package api

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/iximiuz/labctl/internal/content"
)

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

func (c *Client) PutContentMarkdown(
	ctx context.Context,
	kind content.ContentKind,
	name string,
	file string,
	content string,
) error {
	body, err := toJSONBody(map[string]string{
		"kind":    kind.String(),
		"name":    name,
		"file":    file,
		"content": content,
	})
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

func (c *Client) DownloadContentFile(
	ctx context.Context,
	kind content.ContentKind,
	name string,
	file string,
	dest string,
) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return c.DownloadTo(ctx, filepath.Join("/content", "files", kind.Plural(), name, file), nil, nil, dest)
}

func (c *Client) UploadContentFile(
	ctx context.Context,
	kind content.ContentKind,
	name string,
	file string,
	src string,
) error {
	body, err := toJSONBody(map[string]string{
		"kind": kind.String(),
		"name": name,
		"file": file,
	})
	if err != nil {
		return err
	}

	resp := struct {
		UploadURL string `json:"uploadUrl"`
	}{}
	if err := c.PutInto(ctx, "/content/files", nil, nil, body, &resp); err != nil {
		return err
	}

	r, err := c.UploadFrom(ctx, resp.UploadURL, src)
	if err != nil {
		return err
	}
	r.Body.Close()

	return nil
}

func (c *Client) DeleteContentFile(
	ctx context.Context,
	kind content.ContentKind,
	name string,
	file string,
) error {
	resp, err := c.Delete(ctx, "/content/files", url.Values{
		"kind": []string{kind.String()},
		"name": []string{name},
		"file": []string{file},
	}, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
