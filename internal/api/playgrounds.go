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
	PageURL        string              `yaml:"pageUrl" json:"pageUrl"`
	Access         PlaygroundAccess    `yaml:"access" json:"access"`
	Machines       []PlaygroundMachine `yaml:"machines" json:"machines"`
	Tabs           []PlaygroundTab     `yaml:"tabs" json:"tabs"`
	InitTasks      map[string]InitTask `yaml:"initTasks" json:"initTasks"`
	InitConditions InitConditions      `yaml:"initConditions" json:"initConditions"`
	RegistryAuth   string              `yaml:"registryAuth,omitempty" json:"registryAuth,omitempty"`
}

func (c *Client) GetPlayground(ctx context.Context, name string) (*Playground, error) {
	var p Playground
	return &p, c.GetInto(ctx, "/playgrounds/"+name, nil, nil, &p)
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
	ID      string `yaml:"id" json:"id"`
	Kind    string `yaml:"kind" json:"kind"`
	Name    string `yaml:"name" json:"name"`
	Machine string `yaml:"machine" json:"machine"`
	Number  int    `yaml:"number,omitempty" json:"number,omitempty"`
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

type PlaygroundAccess struct {
	Mode  string   `yaml:"mode" json:"mode"`
	Users []string `yaml:"users,omitempty" json:"users,omitempty"`
}

type PlaygroundSpec struct {
	Access         PlaygroundAccess    `yaml:"access" json:"access"`
	Machines       []PlaygroundMachine `yaml:"machines" json:"machines"`
	Tabs           []PlaygroundTab     `yaml:"tabs" json:"tabs"`
	InitTasks      map[string]InitTask `yaml:"initTasks" json:"initTasks"`
	InitConditions InitConditions      `yaml:"initConditions" json:"initConditions"`
	RegistryAuth   string              `yaml:"registryAuth,omitempty" json:"registryAuth,omitempty"`
}

type PlaygroundManifest struct {
	Kind        string         `yaml:"kind" json:"kind"`
	Title       string         `yaml:"title" json:"title"`
	Description string         `yaml:"description" json:"description"`
	Categories  []string       `yaml:"categories" json:"categories"`
	Playground  PlaygroundSpec `yaml:"playground" json:"playground"`
}

type CreatePlaygroundRequest struct {
	Base           string              `yaml:"base" json:"base"`
	Title          string              `yaml:"title" json:"title"`
	Description    string              `yaml:"description" json:"description"`
	Categories     []string            `yaml:"categories" json:"categories"`
	Access         PlaygroundAccess    `yaml:"access" json:"access"`
	Machines       []PlaygroundMachine `yaml:"machines" json:"machines"`
	Tabs           []PlaygroundTab     `yaml:"tabs" json:"tabs"`
	InitTasks      map[string]InitTask `yaml:"initTasks" json:"initTasks"`
	InitConditions InitConditions      `yaml:"initConditions" json:"initConditions"`
	RegistryAuth   string              `yaml:"registryAuth" json:"registryAuth"`
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
	Access         PlaygroundAccess    `yaml:"access" json:"access"`
	Machines       []PlaygroundMachine `yaml:"machines" json:"machines"`
	Tabs           []PlaygroundTab     `yaml:"tabs" json:"tabs"`
	InitTasks      map[string]InitTask `yaml:"initTasks" json:"initTasks"`
	InitConditions InitConditions      `yaml:"initConditions" json:"initConditions"`
	RegistryAuth   string              `yaml:"registryAuth" json:"registryAuth"`
}

func (c *Client) UpdatePlayground(ctx context.Context, name string, req UpdatePlaygroundRequest) (*Playground, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var p Playground
	return &p, c.PutInto(ctx, "/playgrounds/"+name, nil, nil, body, &p)
}
