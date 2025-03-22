package api

import (
	"context"
)

type AccessMode string

const (
	AccessPrivate AccessMode = "private"
	AccessPublic  AccessMode = "public"
)

type Port struct {
	ID string `json:"id"`

	PlayID string `json:"playId"`

	Machine string `json:"machine"`

	Number int `json:"number"`

	Hostname string `json:"hostname"`

	AccessMode AccessMode `json:"access"`

	TLS bool `json:"tls"`

	HostRewrite string `json:"hostRewrite,omitempty"`

	PathRewrite string `json:"pathRewrite,omitempty"`

	URL string `json:"url"`
}

type ExposePortRequest struct {
	Machine     string     `json:"machine"`
	Number      int        `json:"number"`
	Access      AccessMode `json:"access,omitempty"`
	TLS         bool       `json:"tls,omitempty"`
	HostRewrite string     `json:"hostRewrite,omitempty"`
	PathRewrite string     `json:"pathRewrite,omitempty"`
}

func (c *Client) ExposePort(ctx context.Context, id string, req ExposePortRequest) (*Port, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var resp Port
	return &resp, c.PostInto(ctx, "/plays/"+id+"/ports", nil, nil, body, &resp)
}

func (c *Client) ListPorts(ctx context.Context, id string) ([]*Port, error) {
	var resp []*Port
	return resp, c.GetInto(ctx, "/plays/"+id+"/ports", nil, nil, &resp)
}

func (c *Client) UnexposePort(ctx context.Context, id string, portID string) error {
	_, err := c.Delete(ctx, "/plays/"+id+"/ports/"+portID, nil, nil)
	return err
}
