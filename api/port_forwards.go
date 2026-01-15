package api

import (
	"context"
	"strconv"
)

type PortForward struct {
	Kind       string `json:"kind"`
	Machine    string `json:"machine"`
	LocalHost  string `json:"localHost,omitempty"`
	LocalPort  int    `json:"localPort,omitempty"`
	RemotePort int    `json:"remotePort,omitempty"`
	RemoteHost string `json:"remoteHost,omitempty"`
}

func (c *Client) ListPortForwards(ctx context.Context, playID string) ([]*PortForward, error) {
	var resp []*PortForward
	return resp, c.GetInto(ctx, "/plays/"+playID+"/port-forwards", nil, nil, &resp)
}

func (c *Client) AddPortForward(ctx context.Context, playID string, pf PortForward) (*PortForward, error) {
	body, err := toJSONBody(pf)
	if err != nil {
		return nil, err
	}

	var resp PortForward
	return &resp, c.PostInto(ctx, "/plays/"+playID+"/port-forwards", nil, nil, body, &resp)
}

func (c *Client) RemovePortForward(ctx context.Context, playID string, index int) error {
	_, err := c.Delete(ctx, "/plays/"+playID+"/port-forwards/"+strconv.Itoa(index), nil, nil)
	return err
}
