package api

import (
	"context"

	"github.com/iximiuz/labctl/content"
)

type BlogPost struct {
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
}

var _ content.Content = (*BlogPost)(nil)

func (b *BlogPost) GetKind() content.ContentKind {
	return content.KindBlogPost
}

func (b *BlogPost) GetName() string {
	return b.Name
}

func (b *BlogPost) GetPageURL() string {
	return b.PageURL
}

func (b *BlogPost) IsAuthoredBy(userID string) bool {
	for _, a := range b.Authors {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

type CreateBlogPostRequest struct {
	Name string `json:"name"`
}

func (c *Client) CreateBlogPost(ctx context.Context, req CreateBlogPostRequest) (*BlogPost, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var b BlogPost
	return &b, c.PostInto(ctx, "/blog-posts", nil, nil, body, &b)
}

func (c *Client) GetBlogPost(ctx context.Context, name string) (*BlogPost, error) {
	var b BlogPost
	return &b, c.GetInto(ctx, "/blog-posts/"+name, nil, nil, &b)
}

func (c *Client) ListBlogPosts(ctx context.Context) ([]BlogPost, error) {
	var blogPosts []BlogPost
	return blogPosts, c.GetInto(ctx, "/blog-posts", nil, nil, &blogPosts)
}

func (c *Client) ListAuthoredBlogPosts(ctx context.Context) ([]BlogPost, error) {
	var blogPosts []BlogPost
	return blogPosts, c.GetInto(ctx, "/author/blog-posts", nil, nil, &blogPosts)
}

func (c *Client) DeleteBlogPost(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/blog-posts/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
