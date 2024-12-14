package api

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Play struct {
	ID string `json:"id" yaml:"id"`

	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   string `json:"updatedAt" yaml:"updatedAt"`
	LastStateAt string `json:"lastStateAt" yaml:"lastStateAt"`

	ExpiresIn int `json:"expiresIn" yaml:"expiresIn"`

	Playground Playground `json:"playground" yaml:"playground"`

	TutorialName string `json:"tutorialName,omitempty" yaml:"tutorialName"`

	Tutorial *struct {
		Name  string `json:"name" yaml:"name"`
		Title string `json:"title" yaml:"title"`
	} `json:"tutorial,omitempty" yaml:"tutorial"`

	ChallengeName string `json:"challengeName,omitempty" yaml:"challengeName"`

	Challenge *struct {
		Name  string `json:"name" yaml:"name"`
		Title string `json:"title" yaml:"title"`
	} `json:"challenge,omitempty" yaml:"challenge"`

	CourseName string `json:"courseName,omitempty" yaml:"courseName"`

	Course *struct {
		Name  string `json:"name" yaml:"name"`
		Title string `json:"title" yaml:"title"`
	} `json:"course,omitempty" yaml:"course"`

	LessonPath string `json:"lessonPath,omitempty" yaml:"lessonPath"`

	Lesson *struct {
		Name  string `json:"name" yaml:"name"`
		Title string `json:"title" yaml:"title"`
	} `json:"lesson,omitempty" yaml:"lesson"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`

	Active    bool `json:"active" yaml:"active"`
	Running   bool `json:"running" yaml:"running"`
	Destroyed bool `json:"destroyed" yaml:"destroyed"`
	Failed    bool `json:"failed" yaml:"failed"`

	Machines []Machine `json:"machines" yaml:"machines"`

	Tasks map[string]PlayTask `json:"tasks,omitempty" yaml:"tasks,omitempty"`
}

func (p *Play) GetMachine(name string) *Machine {
	for _, m := range p.Machines {
		if m.Name == name {
			return &m
		}
	}
	return nil
}

func (p *Play) IsInitialized() bool {
	for _, task := range p.Tasks {
		if task.Init && task.Status != PlayTaskStatusCompleted {
			return false
		}
	}
	return true
}

func (p *Play) IsFailed() bool {
	for _, task := range p.Tasks {
		if task.Status == PlayTaskStatusFailed {
			return true
		}
	}
	return false
}

func (p *Play) CountInitTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if task.Init {
			count++
		}
	}
	return count
}

func (p *Play) CountCompletedInitTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if task.Init && task.Status == PlayTaskStatusCompleted {
			count++
		}
	}
	return count
}

func (p *Play) IsCompletable() bool {
	for _, task := range p.Tasks {
		if task.Status != PlayTaskStatusCompleted {
			return false
		}
	}
	return true
}

type MachineUser struct {
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

type Machine struct {
	Name        string        `json:"name"`
	Users       []MachineUser `json:"users"`
	CPUCount    int           `json:"cpuCount"`
	RAMSize     string        `json:"ramSize"`
	DrivePerf   string        `json:"drivePerf"`
	NetworkPerf string        `json:"networkPerf"`
}

func (m *Machine) DefaultUser() *MachineUser {
	for _, u := range m.Users {
		if u.Default {
			return &u
		}
	}
	return nil
}

func (m *Machine) HasUser(name string) bool {
	if name == "root" {
		// Everyone has root
		return true
	}

	for _, u := range m.Users {
		if u.Name == name {
			return true
		}
	}
	return false
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

type PlayConnHandle struct {
	URL string `json:"url"`
}

func (c *Client) RequestPlayConn(ctx context.Context, id string) (*PlayConnHandle, error) {
	var conn PlayConnHandle
	return &conn, c.PostInto(ctx, "/plays/"+id+"/conns", nil, nil, nil, &conn)
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
	SSHUser          string     `json:"sshUser"`
	SSHPubKey        string     `json:"sshPubKey"`
}

type StartTunnelResponse struct {
	URL      string `json:"url"`
	LoginURL string `json:"loginUrl"`
}

func (c *Client) StartTunnel(ctx context.Context, id string, req StartTunnelRequest) (*StartTunnelResponse, error) {
	// A hacky workaround for the fact that the CLI currently
	// doesn't check for playground readiness before establishing
	// a tunnel.
	backoff := 200 * time.Millisecond
	for attempt := 0; attempt < 5; attempt++ {
		body, err := toJSONBody(req)
		if err != nil {
			return nil, err
		}

		var resp StartTunnelResponse
		err = c.PostInto(ctx, "/plays/"+id+"/tunnels", nil, nil, body, &resp)
		if err == nil {
			return &resp, nil
		}
		if !errors.Is(err, ErrGatewayTimeout) {
			return nil, err
		}
		if attempt == 2 {
			return nil, fmt.Errorf("max retries exceeded: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}

	return nil, ctx.Err()
}
