package api

import (
	"context"
)

type Play struct {
	ID string `json:"id" yaml:"id"`

	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   string `json:"updatedAt" yaml:"updatedAt"`
	LastStateAt string `json:"lastStateAt" yaml:"lastStateAt"`

	ExpiresIn int `json:"expiresIn" yaml:"expiresIn"`

	Playground Playground `json:"playground" yaml:"playground"`

	PageURL   string `json:"pageUrl" yaml:"pageUrl"`
	Active    bool   `json:"active" yaml:"active"`
	Running   bool   `json:"running" yaml:"running"`
	Destroyed bool   `json:"destroyed" yaml:"destroyed"`
	Failed    bool   `json:"failed" yaml:"failed"`

	Machines []Machine `json:"machines" yaml:"machines"`
}

func (p *Play) GetMachine(name string) *Machine {
	for _, m := range p.Machines {
		if m.Name == name {
			return &m
		}
	}
	return nil
}

type Machine struct {
	Name        string `json:"name"`
	CPUCount    int    `json:"cpuCount"`
	RAMSize     string `json:"ramSize"`
	DrivePerf   string `json:"drivePerf"`
	NetworkPerf string `json:"networkPerf"`
}

type CreatePlayRequest struct {
	Playground string `json:"playground"`
}

func (c *Client) CreatePlay(ctx context.Context, req CreatePlayRequest) (*Play, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var p Play
	return &p, c.PostInto(ctx, "/plays", nil, nil, body, &p)
}

func (c *Client) GetPlay(ctx context.Context, id string) (*Play, error) {
	var p Play
	return &p, c.GetInto(ctx, "/plays/"+id, nil, nil, &p)
}

func (c *Client) ListPlays(ctx context.Context) ([]*Play, error) {
	var plays []*Play
	return plays, c.GetInto(ctx, "/plays", nil, nil, &plays)
}

func (c *Client) DeletePlay(ctx context.Context, id string) error {
	resp, err := c.Delete(ctx, "/plays/"+id, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
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
