package api

import (
	"context"
)

type Play struct {
	ID        string `json:"id" yaml:"id"`
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`
	ExpiresIn int    `json:"expiresIn" yaml:"expiresIn"`

	Playground Playground `json:"playground" yaml:"playground"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`
	Active  bool   `json:"active" yaml:"active"`
	Running bool   `json:"running" yaml:"running"`

	Machines []Machine `json:"machines" yaml:"machines"`
}

type Machine struct {
	Name        string `json:"name"`
	CPUCount    int    `json:"cpuCount"`
	RAMSize     string `json:"ramSize"`
	DrivePerf   string `json:"drivePerf"`
	NetworkPerf string `json:"networkPerf"`
}

func (c *Client) GetPlay(ctx context.Context, id string) (*Play, error) {
	var p Play
	return &p, c.GetInto(ctx, "/plays/"+id, nil, nil, &p)
}

func (c *Client) ListPlays(ctx context.Context) ([]Play, error) {
	var plays []Play
	return plays, c.GetInto(ctx, "/plays", nil, nil, &plays)
}

type PortAccess string

const (
	PortAccessPublic  PortAccess = "public"
	PortAccessPrivate PortAccess = "private"
)

type StartTunnelRequest struct {
	Machine          string     `json:"machine"`
	Port             int        `json:"port"`
	Access           PortAccess `json:"access"`
	GenerateLoginURL bool       `json:"generateLoginUrl"`
	SSHPubKey        string     `json:"sshPubKey"`
}

type StartTunnelResponse struct {
	URL      string `json:"url"`
	LoginURL string `json:"loginUrl"`
}

func (c *Client) StartTunnel(ctx context.Context, id string, req StartTunnelRequest) (*StartTunnelResponse, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var resp StartTunnelResponse
	return &resp, c.PostInto(ctx, "/plays/"+id+"/tunnels", nil, nil, body, &resp)
}
