package api

import (
	"context"
)

type Author struct {
	UserID string `json:"userId"`

	DisplayName        string `json:"displayName"`
	ExternalProfileURL string `json:"externalProfileUrl"`

	Official bool `json:"official,omitempty"`
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

func (c *Client) GetAuthor(ctx context.Context) (*Author, error) {
	var author Author
	return &author, c.GetInto(ctx, "/author", nil, nil, &author)
}
