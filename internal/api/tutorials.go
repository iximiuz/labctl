package api

import (
	"context"
)

type Tutorial struct {
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`

	Name string `json:"name"`

	PageURL string `json:"pageUrl"`
}

type CreateTutorialRequest struct {
	Name string `json:"name"`
}

func (c *Client) CreateTutorial(ctx context.Context, req CreateTutorialRequest) (*Tutorial, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var t Tutorial
	return &t, c.PostInto(ctx, "/tutorials", nil, nil, body, &t)
}

func (c *Client) GetTutorial(ctx context.Context, name string) (*Tutorial, error) {
	var t Tutorial
	return &t, c.GetInto(ctx, "/tutorials/"+name, nil, nil, &t)
}

func (c *Client) ListTutorials(ctx context.Context) ([]Tutorial, error) {
	var tutorials []Tutorial
	return tutorials, c.GetInto(ctx, "/tutorials", nil, nil, &tutorials)
}

func (c *Client) ListAuthoredTutorials(ctx context.Context) ([]Tutorial, error) {
	var tutorials []Tutorial
	return tutorials, c.GetInto(ctx, "/tutorials/authored", nil, nil, &tutorials)
}

func (c *Client) DeleteTutorial(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/tutorials/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
