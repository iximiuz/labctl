package api

import (
	"context"
)

type Session struct {
	ID string `json:"id"`

	UserID string `json:"userId"`

	Authenticated bool `json:"authenticated"`

	AccessToken string `json:"accessToken"`

	AuthURL string `json:"authUrl"`
}

func (c *Client) CreateSession(ctx context.Context) (*Session, error) {
	var s Session
	return &s, c.PostInto(ctx, "/sessions", nil, nil, nil, &s)
}

func (c *Client) GetSession(ctx context.Context, id string) (*Session, error) {
	var s Session
	return &s, c.GetInto(ctx, "/sessions/"+id, nil, nil, &s)
}

func (c *Client) DeleteSession(ctx context.Context, id string) error {
	resp, err := c.Delete(ctx, "/sessions/"+id, nil, nil)
	if err != nil {
		return err
	}

	return resp.Body.Close()
}
