package api

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type PlayState string

const (
	StateCreated    PlayState = "CREATED"
	StateWarmingUp  PlayState = "WARMING_UP"
	StateWarmedUp   PlayState = "WARMED_UP"
	StateStarting   PlayState = "STARTING"
	StateRunning    PlayState = "RUNNING"
	StateStopping   PlayState = "STOPPING"
	StateStopped    PlayState = "STOPPED"
	StateDestroying PlayState = "DESTROYING"
	StateDestroyed  PlayState = "DESTROYED"
	StateFailed     PlayState = "FAILED"
)

type StateEvent struct {
	State PlayState `json:"state" yaml:"state"`
	Error bool      `json:"error,omitempty" yaml:"error,omitempty"`
	At    string    `json:"at" yaml:"at"`
}

type PlayStatus struct {
	FactoryID   string       `json:"factoryId" yaml:"factoryId"`
	StateEvents []StateEvent `json:"stateEvents" yaml:"stateEvents"`
}

type Play struct {
	ID string `json:"id" yaml:"id"`

	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   string `json:"updatedAt" yaml:"updatedAt"`
	LastStateAt string `json:"lastStateAt" yaml:"lastStateAt"`

	ExpiresIn int `json:"expiresIn" yaml:"expiresIn"`

	Status *PlayStatus `json:"status,omitempty" yaml:"status,omitempty"`

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

func (p *Play) FactoryID() string {
	if p.Status == nil {
		return ""
	}
	return p.Status.FactoryID
}

func (p *Play) State() PlayState {
	// Older plays don't have a status
	if p.Status == nil {
		return ""
	}
	return p.Status.StateEvents[len(p.Status.StateEvents)-1].State
}

func (p *Play) StateIs(state PlayState) bool {
	return p.State() == state
}

func (p *Play) IsActive() bool {
	// All but the terminal states
	return !p.StateIs(StateStopped) && !p.StateIs(StateDestroyed)
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
		if !task.Helper && task.Status != PlayTaskStatusCompleted {
			return false
		}
	}
	return true
}

func (p *Play) CountTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if !task.Helper && !task.Init {
			count++
		}
	}
	return count
}

func (p *Play) CountCompletedTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if !task.Helper && !task.Init && task.Status == PlayTaskStatusCompleted {
			count++
		}
	}
	return count
}

type Machine struct {
	Name      string           `json:"name"`
	Users     []MachineUser    `json:"users"`
	Resources MachineResources `json:"resources"`
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
	Playground              string              `json:"playground"`
	Tabs                    []PlaygroundTab     `json:"tabs,omitempty"`
	Networks                []PlaygroundNetwork `json:"networks,omitempty"`
	Machines                []PlaygroundMachine `json:"machines,omitempty"`
	InitTasks               map[string]InitTask `json:"initTasks,omitempty"`
	InitConditions          map[string]string   `json:"initConditions,omitempty"`
	SafetyDisclaimerConsent bool                `json:"safetyDisclaimerConsent"`
	AsFreeTierUser          bool                `json:"asFreeTierUser,omitempty"`
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

func (c *Client) StopPlay(ctx context.Context, id string) (*Play, error) {
	body, err := toJSONBody(map[string]any{"action": "stop"})
	if err != nil {
		return nil, err
	}

	var p Play
	return &p, c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &p)
}

func (c *Client) RestartPlay(ctx context.Context, id string) (*Play, error) {
	body, err := toJSONBody(map[string]any{"action": "restart"})
	if err != nil {
		return nil, err
	}

	var p Play
	return &p, c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &p)
}

func (c *Client) DestroyPlay(ctx context.Context, id string) error {
	body, err := toJSONBody(map[string]any{"action": "destroy"})
	if err != nil {
		return err
	}

	var p Play
	return c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &p)
}

func (c *Client) PersistPlay(ctx context.Context, id string) error {
	body, err := toJSONBody(map[string]any{"action": "make_persistent"})
	if err != nil {
		return err
	}

	var p Play
	return c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &p)
}

// Deprecated: Use DestroyPlay instead
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
	for attempt := range 5 {
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
