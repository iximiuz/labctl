package api

import (
	"context"
)

type Shell struct {
	ID string `json:"id"`

	PlayID string `json:"playId"`

	Machine string `json:"machine"`

	User string `json:"user"`

	Hostname string `json:"hostname"`

	AccessMode AccessMode `json:"access"`

	URL string `json:"url"`
}

type ExposeShellRequest struct {
	Machine string     `json:"machine"`
	User    string     `json:"user"`
	Access  AccessMode `json:"access,omitempty"`
}

func (c *Client) ExposeShell(ctx context.Context, id string, req ExposeShellRequest) (*Shell, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var resp Shell
	return &resp, c.PostInto(ctx, "/plays/"+id+"/shells", nil, nil, body, &resp)
}

func (c *Client) ListShells(ctx context.Context, id string) ([]*Shell, error) {
	var resp []*Shell
	return resp, c.GetInto(ctx, "/plays/"+id+"/shells", nil, nil, &resp)
}

func (c *Client) UnexposeShell(ctx context.Context, id string, shellID string) error {
	_, err := c.Delete(ctx, "/plays/"+id+"/shells/"+shellID, nil, nil)
	return err
}
