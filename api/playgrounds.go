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
	Networks       []PlaygroundNetwork `yaml:"networks" json:"networks"`
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

type PlaygroundNetwork struct {
	Name    string `yaml:"name" json:"name"`
	Subnet  string `yaml:"subnet" json:"subnet"`
	Gateway string `yaml:"gateway,omitempty" json:"gateway,omitempty"`
	Private bool   `yaml:"private,omitempty" json:"private,omitempty"`
}

type MachineUser struct {
	Name    string `yaml:"name" json:"name"`
	Default bool   `yaml:"default,omitempty" json:"default,omitempty"`
	Welcome string `yaml:"welcome,omitempty" json:"welcome,omitempty"`
}

type MachineDrive struct {
	Source     string `yaml:"source,omitempty" json:"source,omitempty"`
	Mount      string `yaml:"mount,omitempty" json:"mount,omitempty"`
	Size       string `yaml:"size,omitempty" json:"size,omitempty"`
	Filesystem string `yaml:"filesystem,omitempty" json:"filesystem,omitempty"`
	ReadOnly   bool   `yaml:"readOnly,omitempty" json:"readOnly,omitempty"`
}

type MachineNetworkInterface struct {
	Address string `yaml:"address,omitempty" json:"address,omitempty"`
	Network string `yaml:"network,omitempty" json:"network,omitempty"`
}

type MachineNetwork struct {
	Interfaces []MachineNetworkInterface `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
}

type MachineResources struct {
	CPUCount int    `yaml:"cpuCount,omitempty" json:"cpuCount,omitempty"`
	RAMSize  string `yaml:"ramSize,omitempty" json:"ramSize,omitempty"`
}

type MachineStartupFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    string `json:"mode,omitempty"`
	Owner   string `json:"owner,omitempty"`
	Append  bool   `json:"append,omitempty"`
}

type PlaygroundMachine struct {
	Name         string               `yaml:"name" json:"name"`
	Users        []MachineUser        `yaml:"users,omitempty" json:"users,omitempty"`
	Kernel       string               `yaml:"kernel,omitempty" json:"kernel,omitempty"`
	Drives       []MachineDrive       `yaml:"drives,omitempty" json:"drives,omitempty"`
	Network      *MachineNetwork      `yaml:"network,omitempty" json:"network,omitempty"`
	Resources    *MachineResources    `yaml:"resources,omitempty" json:"resources,omitempty"`
	StartupFiles []MachineStartupFile `yaml:"startupFiles,omitempty" json:"startupFiles,omitempty"`
	NoSSH        bool                 `yaml:"noSSH,omitempty" json:"noSSH,omitempty"`
}

type PlaygroundTab struct {
	ID          string `yaml:"id,omitempty" json:"id,omitempty"`
	Kind        string `yaml:"kind" json:"kind"`
	Name        string `yaml:"name" json:"name"`
	Machine     string `yaml:"machine,omitempty" json:"machine,omitempty"`
	Number      int    `yaml:"number,omitempty" json:"number,omitempty"`
	Access      string `yaml:"access,omitempty" json:"access,omitempty"`
	Tls         bool   `yaml:"tls,omitempty" json:"tls,omitempty"`
	HostRewrite string `yaml:"hostRewrite,omitempty" json:"hostRewrite,omitempty"`
	PathRewrite string `yaml:"pathRewrite,omitempty" json:"pathRewrite,omitempty"`
	URL         string `yaml:"url,omitempty" json:"url,omitempty"`
}

type InitCondition struct {
	Key   string `yaml:"key" json:"key"`
	Value string `yaml:"value" json:"value"`
}

type InitTask struct {
	Name           string          `yaml:"name" json:"name"`
	Machine        string          `yaml:"machine,omitempty" json:"machine,omitempty"`
	Init           bool            `yaml:"init,omitempty" json:"init,omitempty"`
	User           string          `yaml:"user" json:"user"`
	TimeoutSeconds int             `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
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
	Networks       []PlaygroundNetwork `yaml:"networks,omitempty" json:"networks,omitempty"`
	Machines       []PlaygroundMachine `yaml:"machines,omitempty" json:"machines,omitempty"`
	Tabs           []PlaygroundTab     `yaml:"tabs,omitempty" json:"tabs,omitempty"`
	InitTasks      map[string]InitTask `yaml:"initTasks,omitempty" json:"initTasks,omitempty"`
	InitConditions InitConditions      `yaml:"initConditions,omitempty" json:"initConditions,omitempty"`
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
	Base        string         `yaml:"base,omitempty" json:"base,omitempty"`
	Title       string         `yaml:"title,omitempty" json:"title,omitempty"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Cover       string         `yaml:"cover,omitempty" json:"cover,omitempty"`
	Categories  []string       `yaml:"categories,omitempty" json:"categories,omitempty"`
	Markdown    string         `yaml:"markdown,omitempty" json:"markdown,omitempty"`
	Playground  PlaygroundSpec `yaml:"playground" json:"playground"`
}

type CreatePlaygroundRequest struct {
	Name           string              `yaml:"name" json:"name"`
	Base           string              `yaml:"base" json:"base"`
	Title          string              `yaml:"title" json:"title"`
	Description    string              `yaml:"description" json:"description"`
	Categories     []string            `yaml:"categories" json:"categories"`
	Markdown       string              `yaml:"markdown,omitempty" json:"markdown,omitempty"`
	Networks       []PlaygroundNetwork `yaml:"networks" json:"networks"`
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
	Networks       []PlaygroundNetwork `yaml:"networks" json:"networks"`
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
