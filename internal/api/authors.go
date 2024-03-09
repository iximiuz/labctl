package api

import (
	"context"
	"net/url"
)

type Author struct {
	DisplayName        string `json:"displayName"`
	ExternalProfileURL string `json:"externalProfileUrl"`
}

type CreateAuthorRequest struct {
	Author
}

func (c *Client) CreateAuthor(ctx context.Context, req CreateAuthorRequest) (*Author, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var author Author
	return &author, c.PostInto(ctx, "/authors", nil, nil, body, &author)
}

type ListAuthorsFilter struct {
	UserID []string
}

func (c *Client) ListAuthors(ctx context.Context, filter ListAuthorsFilter) ([]Author, error) {
	var authors []Author
	return authors, c.GetInto(ctx, "/authors", url.Values{"userId": filter.UserID}, nil, &authors)
}
