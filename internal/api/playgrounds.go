package api

import (
	"context"
	"net/url"
)

type Playground struct {
	ID             string              `yaml:"id" json:"id"`
	Owner          string              `yaml:"owner" json:"owner"`
	Name           string              `yaml:"name" json:"name"`
	Base           string              `yaml:"base" json:"base"`
	Title          string              `yaml:"title" json:"title"`
	Description    string              `yaml:"description" json:"description"`
	Categories     []string            `yaml:"categories" json:"categories"`
	Cover          string              `yaml:"cover,omitempty" json:"cover,omitempty"`
	Markdown       string              `yaml:"markdown,omitempty" json:"markdown,omitempty"`
	Published      bool                `yaml:"published" json:"published"`
	PageURL        string              `yaml:"pageUrl" json:"pageUrl"`
	Machines       []PlaygroundMachine `yaml:"machines" json:"machines"`
	Tabs           []PlaygroundTab     `yaml:"tabs" json:"tabs"`
	InitTasks      map[string]InitTask `yaml:"initTasks,omitempty" json:"initTasks,omitempty"`
	InitConditions InitConditions      `yaml:"initConditions,omitempty" json:"initConditions,omitempty"`
	RegistryAuth   string              `yaml:"registryAuth,omitempty" json:"registryAuth,omitempty"`

	AccessControl PlaygroundAccessControl `yaml:"accessControl" json:"accessControl"`
	UserAccess    PlaygroundUserAccess    `yaml:"userAccess,omitempty" json:"userAccess,omitempty"`
}

type GetPlaygroundOptions struct {
	Format string // <none> | extended
}

func (c *Client) GetPlayground(ctx context.Context, name string, opts *GetPlaygroundOptions) (*Playground, error) {
	var p Playground
	q := url.Values{}

	if opts != nil && opts.Format != "" {
		q.Add("format", opts.Format)
	}

	return &p, c.GetInto(ctx, "/playgrounds/"+name, q, nil, &p)
}

type ListPlaygroundsOptions struct {
	Filter string
}

func (c *Client) ListPlaygrounds(ctx context.Context, opts *ListPlaygroundsOptions) ([]Playground, error) {
	var plays []Playground

	q := url.Values{}
	if opts != nil && opts.Filter != "" {
		q.Add("filter", opts.Filter)
	}

	return plays, c.GetInto(ctx, "/playgrounds", q, nil, &plays)
}

type MachineUser struct {
	Name    string `yaml:"name" json:"name"`
	Default bool   `yaml:"default,omitempty" json:"default,omitempty"`
}

type MachineResources struct {
	CPUCount int    `yaml:"cpuCount" json:"cpuCount"`
	RAMSize  string `yaml:"ramSize" json:"ramSize"`
}

type PlaygroundMachine struct {
	Name      string           `yaml:"name" json:"name"`
	Users     []MachineUser    `yaml:"users" json:"users"`
	Resources MachineResources `yaml:"resources" json:"resources"`
}

type PlaygroundTab struct {
	ID          string `yaml:"id" json:"id"`
	Kind        string `yaml:"kind" json:"kind"`
	Name        string `yaml:"name" json:"name"`
	Machine     string `yaml:"machine" json:"machine"`
	Number      int    `yaml:"number,omitempty" json:"number,omitempty"`
	Tls         bool   `yaml:"tls,omitempty" json:"tls,omitempty"`
	HostRewrite string `yaml:"hostRewrite,omitempty" json:"hostRewrite,omitempty"`
	PathRewrite string `yaml:"pathRewrite,omitempty" json:"pathRewrite,omitempty"`
}

type InitCondition struct {
	Key   string `yaml:"key" json:"key"`
	Value string `yaml:"value" json:"value"`
}

type InitTask struct {
	Name           string          `yaml:"name" json:"name"`
	Machine        string          `yaml:"machine,omitempty" json:"machine,omitempty"`
	Init           bool            `yaml:"init" json:"init"`
	User           string          `yaml:"user" json:"user"`
	TimeoutSeconds int             `yaml:"timeout_seconds" json:"timeout_seconds"`
	Needs          []string        `yaml:"needs,omitempty" json:"needs,omitempty"`
	Run            string          `yaml:"run" json:"run"`
	Status         int             `yaml:"status,omitempty" json:"status,omitempty"`
	Conditions     []InitCondition `yaml:"conditions,omitempty" json:"conditions,omitempty"`
}

type InitConditionValue struct {
	Key      string   `yaml:"key" json:"key"`
	Default  string   `yaml:"default,omitempty" json:"default,omitempty"`
	Nullable bool     `yaml:"nullable,omitempty" json:"nullable,omitempty"`
	Options  []string `yaml:"options" json:"options"`
}

type InitConditions struct {
	Values []InitConditionValue `yaml:"values,omitempty" json:"values,omitempty"`
}

// Deprecated: Use PlaygroundAccessControl instead
type PlaygroundAccess struct {
	Mode  string   `yaml:"mode" json:"mode"`
	Users []string `yaml:"users,omitempty" json:"users,omitempty"`
}

type PlaygroundAccessControl struct {
	CanList  []string `yaml:"canList,omitempty" json:"canList,omitempty"`
	CanRead  []string `yaml:"canRead,omitempty" json:"canRead,omitempty"`
	CanStart []string `yaml:"canStart,omitempty" json:"canStart,omitempty"`
}

type PlaygroundUserAccess struct {
	CanList  bool `yaml:"canList" json:"canList"`
	CanRead  bool `yaml:"canRead" json:"canRead"`
	CanStart bool `yaml:"canStart" json:"canStart"`
}

type PlaygroundSpec struct {
	Machines       []PlaygroundMachine `yaml:"machines" json:"machines"`
	Tabs           []PlaygroundTab     `yaml:"tabs" json:"tabs"`
	InitTasks      map[string]InitTask `yaml:"initTasks" json:"initTasks"`
	InitConditions InitConditions      `yaml:"initConditions" json:"initConditions"`
	RegistryAuth   string              `yaml:"registryAuth,omitempty" json:"registryAuth,omitempty"`

	// Deprecated: Use PlaygroundAccessControl instead
	Access *PlaygroundAccess `yaml:"access,omitempty" json:"access,omitempty"`

	AccessControl PlaygroundAccessControl `yaml:"accessControl" json:"accessControl"`
}

func (s *PlaygroundSpec) HasAccessControl() bool {
	return len(s.AccessControl.CanList) > 0 ||
		len(s.AccessControl.CanRead) > 0 ||
		len(s.AccessControl.CanStart) > 0
}

type PlaygroundManifest struct {
	Kind        string         `yaml:"kind" json:"kind"`
	Name        string         `yaml:"name" json:"name"`
	Base        string         `yaml:"base" json:"base"`
	Title       string         `yaml:"title" json:"title"`
	Description string         `yaml:"description" json:"description"`
	Cover       string         `yaml:"cover" json:"cover"`
	Categories  []string       `yaml:"categories" json:"categories"`
	Markdown    string         `yaml:"markdown" json:"markdown"`
	Playground  PlaygroundSpec `yaml:"playground" json:"playground"`
}

type CreatePlaygroundRequest struct {
	Name           string              `yaml:"name" json:"name"`
	Base           string              `yaml:"base" json:"base"`
	Title          string              `yaml:"title" json:"title"`
	Description    string              `yaml:"description" json:"description"`
	Categories     []string            `yaml:"categories" json:"categories"`
	Markdown       string              `yaml:"markdown" json:"markdown"`
	Machines       []PlaygroundMachine `yaml:"machines" json:"machines"`
	Tabs           []PlaygroundTab     `yaml:"tabs" json:"tabs"`
	InitTasks      map[string]InitTask `yaml:"initTasks" json:"initTasks"`
	InitConditions InitConditions      `yaml:"initConditions" json:"initConditions"`
	RegistryAuth   string              `yaml:"registryAuth" json:"registryAuth"`

	AccessControl PlaygroundAccessControl `yaml:"accessControl" json:"accessControl"`
}

func (c *Client) CreatePlayground(ctx context.Context, req CreatePlaygroundRequest) (*Playground, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var p Playground
	return &p, c.PostInto(ctx, "/playgrounds", nil, nil, body, &p)
}

type UpdatePlaygroundRequest struct {
	Title          string              `yaml:"title" json:"title"`
	Description    string              `yaml:"description" json:"description"`
	Categories     []string            `yaml:"categories" json:"categories"`
	Cover          string              `yaml:"cover,omitempty" json:"cover,omitempty"`
	Markdown       string              `yaml:"markdown,omitempty" json:"markdown,omitempty"`
	Machines       []PlaygroundMachine `yaml:"machines" json:"machines"`
	Tabs           []PlaygroundTab     `yaml:"tabs" json:"tabs"`
	InitTasks      map[string]InitTask `yaml:"initTasks" json:"initTasks"`
	InitConditions InitConditions      `yaml:"initConditions" json:"initConditions"`
	RegistryAuth   string              `yaml:"registryAuth" json:"registryAuth"`

	AccessControl PlaygroundAccessControl `yaml:"accessControl" json:"accessControl"`
}

func (c *Client) UpdatePlayground(ctx context.Context, name string, req UpdatePlaygroundRequest) (*Playground, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var p Playground
	return &p, c.PutInto(ctx, "/playgrounds/"+name, nil, nil, body, &p)
}

func (c *Client) DeletePlayground(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/playgrounds/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
