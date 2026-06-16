package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
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

type MachineState string

const (
	MachineStateCreated   MachineState = "CREATED"
	MachineStateWarmingUp MachineState = "WARMING_UP"
	MachineStateWarmedUp  MachineState = "WARMED_UP"
	MachineStateStarting  MachineState = "STARTING"
	MachineStateRunning   MachineState = "RUNNING"
	MachineStateRebooting MachineState = "REBOOTING"
	MachineStateStopping  MachineState = "STOPPING"
	MachineStateStopped   MachineState = "STOPPED"
)

type MachineStatus struct {
	Name  string       `json:"name" yaml:"name"`
	State MachineState `json:"state" yaml:"state"`
}

type PlayStatus struct {
	FactoryID   string          `json:"factoryId" yaml:"factoryId"`
	StateEvents []StateEvent    `json:"stateEvents" yaml:"stateEvents"`
	Machines    []MachineStatus `json:"machines,omitempty" yaml:"machines,omitempty"`
}

type Play struct {
	ID string `json:"id" yaml:"id"`

	Title       string `json:"title" yaml:"title"`
	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   string `json:"updatedAt" yaml:"updatedAt"`
	LastStateAt string `json:"lastStateAt" yaml:"lastStateAt"`

	ExpiresIn int `json:"expiresIn" yaml:"expiresIn"`

	MaxPlayTime string `json:"maxPlayTime,omitempty" yaml:"maxPlayTime,omitempty"`

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

func (p *Play) MachineState(name string) MachineState {
	if p.Status == nil {
		return ""
	}
	for _, m := range p.Status.Machines {
		if m.Name == name {
			return m.State
		}
	}
	return ""
}

func (p *Play) FactoryID() string {
	if p.Status == nil {
		return ""
	}
	return p.Status.FactoryID
}

func (p *Play) State() PlayState {
	// Older plays don't have a status
	if p.Status == nil || len(p.Status.StateEvents) == 0 {
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

// HasFailedTask reports whether any non-helper task has failed.
func (p *Play) HasFailedTask() bool {
	for _, task := range p.Tasks {
		if !task.Helper && task.Status == PlayTaskStatusFailed {
			return true
		}
	}
	return false
}

// CountMachines returns the number of machines defined for the play.
func (p *Play) CountMachines() int {
	return len(p.Machines)
}

// CountRunningMachines returns how many of the play's machines currently report
// the RUNNING state in the latest status.
func (p *Play) CountRunningMachines() int {
	count := 0
	for _, m := range p.Machines {
		if p.MachineState(m.Name) == MachineStateRunning {
			count++
		}
	}
	return count
}

// AllMachinesRunning reports whether every machine of the play has reached the
// RUNNING state. It returns false until a status with the full machine list has
// been observed.
func (p *Play) AllMachinesRunning() bool {
	if p.Status == nil || len(p.Machines) == 0 {
		return false
	}
	for _, m := range p.Machines {
		if p.MachineState(m.Name) != MachineStateRunning {
			return false
		}
	}
	return true
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

var validPlayIDRegex = regexp.MustCompile(`^[0-9a-f]{24}$`)

func LooksLikePlayID(v string) bool {
	return validPlayIDRegex.MatchString(v)
}

type Machine struct {
	Name      string           `json:"name"`
	Users     []MachineUser    `json:"users"`
	Resources MachineResources `json:"resources"`
}

// ResolveMachine returns the given machine name if it exists in the playground,
// or defaults to the first machine's name if name is empty.
func (p *Play) ResolveMachine(name string) (string, error) {
	if name == "" {
		return p.Machines[0].Name, nil
	}
	if p.GetMachine(name) == nil {
		return "", fmt.Errorf("machine %q not found in the playground", name)
	}
	return name, nil
}

// ResolveUser returns the given user if it exists on the machine,
// or defaults to the machine's default user (or "root") if user is empty.
func (p *Play) ResolveUser(machine, user string) (string, error) {
	m := p.GetMachine(machine)
	if m == nil {
		return "", fmt.Errorf("machine %q not found in the playground", machine)
	}
	if user == "" {
		if u := m.DefaultUser(); u != nil {
			return u.Name, nil
		}
		return "root", nil
	}
	if !m.HasUser(user) {
		return "", fmt.Errorf("user %q not found in the machine %q", user, machine)
	}
	return user, nil
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

type ListPlaysQueryParams struct {
	Persistent bool `json:"persistent,omitempty" yaml:"persistent,omitempty"`
}

func (c *Client) ListPlays(ctx context.Context, listPlaysQueryParams ListPlaysQueryParams) ([]*Play, error) {
	var plays []*Play
	query := url.Values{}
	if listPlaysQueryParams.Persistent {
		query.Add("persistent", "true")
	}
	return plays, c.GetInto(ctx, "/plays", query, nil, &plays)
}

func (c *Client) SetPlayTitle(ctx context.Context, id string, title string) (*Play, error) {
	body, err := toJSONBody(map[string]any{"action": "set_title", "title": title})
	if err != nil {
		return nil, err
	}

	var p Play
	return &p, c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &p)
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

func (c *Client) SetPlayMaxPlayTime(ctx context.Context, id string, maxPlayTimeMinutes int) (*Play, error) {
	body, err := toJSONBody(map[string]any{
		"action":             "set_max_play_time",
		"maxPlayTimeMinutes": maxPlayTimeMinutes,
	})
	if err != nil {
		return nil, err
	}

	var p Play
	return &p, c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &p)
}

func (c *Client) RebootPlayMachine(ctx context.Context, id, machine string) (*Play, error) {
	body, err := toJSONBody(map[string]any{"action": "machine.reboot", "machine": machine})
	if err != nil {
		return nil, err
	}

	var p Play
	return &p, c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &p)
}

func (c *Client) StopPlayMachine(ctx context.Context, id, machine string) (*Play, error) {
	body, err := toJSONBody(map[string]any{"action": "machine.stop", "machine": machine})
	if err != nil {
		return nil, err
	}

	var p Play
	return &p, c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &p)
}

func (c *Client) RestartPlayMachine(ctx context.Context, id, machine string) (*Play, error) {
	body, err := toJSONBody(map[string]any{"action": "machine.restart", "machine": machine})
	if err != nil {
		return nil, err
	}

	var p Play
	return &p, c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &p)
}

func (c *Client) ListPlayMachineConsoles(ctx context.Context, id, machine string) ([]string, error) {
	body, err := toJSONBody(map[string]any{"action": "machine.console.list", "machine": machine})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Consoles []string `json:"consoles"`
	}
	if err := c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &resp); err != nil {
		return nil, err
	}
	return resp.Consoles, nil
}

func (c *Client) ReadPlayMachineConsole(ctx context.Context, id, machine, console string) (string, error) {
	body, err := toJSONBody(map[string]any{
		"action":  "machine.console.read",
		"machine": machine,
		"console": console,
	})
	if err != nil {
		return "", err
	}

	var resp struct {
		Content string `json:"content"`
	}
	if err := c.PostInto(ctx, "/plays/"+id+"/actions", nil, nil, body, &resp); err != nil {
		return "", err
	}
	return resp.Content, nil
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

// GetPlayTasks returns the merged control-plane + data-plane view of a play's
// tasks. When machines is non-empty, only those machines' tasks are returned.
func (c *Client) GetPlayTasks(ctx context.Context, id string, machines []string) ([]PlayTaskDetails, error) {
	query := url.Values{}
	for _, m := range machines {
		query.Add("machine", m)
	}

	var tasks []PlayTaskDetails
	return tasks, c.GetInto(ctx, "/plays/"+id+"/tasks", query, nil, &tasks)
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
