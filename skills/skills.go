// Package skills holds the AI agent skills and rules for working with
// iximiuz Labs that `labctl skills install` puts into a project (or the user's
// home directory) in the formats that different AI agents read.
//
// The kit is organized in bundles. Every bundle is a top-level folder with
// skills/<name>/SKILL.md (plus optional supporting files) and rules/<name>.md
// files. Skills are copied as is, and rules are rendered in each agent's own
// format (see layouts.go).
package skills

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed playgrounds authoring
var embedded embed.FS

type Bundle struct {
	Name string

	Description string

	// ProjectOnly bundles rely on the content repository layout (e.g., their
	// rules are scoped to challenges/**, tutorials/**, etc.), so they can be
	// installed into a project but not into the user's home directory.
	ProjectOnly bool

	// Requires lists the bundles whose skills this bundle's skills refer to.
	Requires []string
}

var defaultBundles = []Bundle{
	{
		Name:        "playgrounds",
		Description: "Start, inspect, and destroy playgrounds, run commands on them, and check their tasks",
	},
	{
		Name:        "authoring",
		Description: "Create, push, and debug content (challenges, tutorials, courses, skill paths, Shell Gyms)",
		ProjectOnly: true,
		Requires:    []string{"playgrounds"},
	},
}

type ItemKind string

const (
	KindSkill ItemKind = "skill"
	KindRule  ItemKind = "rule"
)

// Item is a single skill or rule of the kit.
type Item struct {
	Name   string
	Kind   ItemKind
	Bundle string

	// skill maps the skill's files (relative to the skill folder) to their
	// content.
	skill map[string][]byte

	rule Rule
}

type Kit struct {
	bundles []Bundle
	items   []Item
}

// Default returns the kit embedded into the labctl binary.
func Default() *Kit {
	k, err := New(embedded, defaultBundles)
	if err != nil {
		panic(fmt.Sprintf("broken embedded skills kit: %s", err))
	}

	return k
}

func New(fsys fs.FS, bundles []Bundle) (*Kit, error) {
	k := &Kit{bundles: bundles}

	for _, b := range bundles {
		if err := k.load(fsys, b.Name); err != nil {
			return nil, fmt.Errorf("bundle %s: %w", b.Name, err)
		}
	}

	return k, nil
}

func (k *Kit) load(fsys fs.FS, bundle string) error {
	skills, err := fs.ReadDir(fsys, path.Join(bundle, "skills"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	for _, entry := range skills {
		if !entry.IsDir() {
			return fmt.Errorf("unexpected file %s in skills/ (expected skills/<name>/SKILL.md)", entry.Name())
		}

		item := Item{
			Name:   entry.Name(),
			Kind:   KindSkill,
			Bundle: bundle,
			skill:  make(map[string][]byte),
		}

		root := path.Join(bundle, "skills", entry.Name())
		if err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}

			content, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}

			item.skill[strings.TrimPrefix(p, root+"/")] = content
			return nil
		}); err != nil {
			return err
		}

		if _, ok := item.skill["SKILL.md"]; !ok {
			return fmt.Errorf("skill %s has no SKILL.md", item.Name)
		}

		if err := k.add(item); err != nil {
			return err
		}
	}

	rules, err := fs.ReadDir(fsys, path.Join(bundle, "rules"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	for _, entry := range rules {
		if entry.IsDir() || path.Ext(entry.Name()) != ".md" {
			return fmt.Errorf("unexpected entry %s in rules/ (expected rules/<name>.md)", entry.Name())
		}

		content, err := fs.ReadFile(fsys, path.Join(bundle, "rules", entry.Name()))
		if err != nil {
			return err
		}

		rule, err := parseRule(strings.TrimSuffix(entry.Name(), ".md"), content)
		if err != nil {
			return err
		}

		if err := k.add(Item{Name: rule.Name, Kind: KindRule, Bundle: bundle, rule: rule}); err != nil {
			return err
		}
	}

	return nil
}

func (k *Kit) add(item Item) error {
	if !validName(item.Name) {
		return fmt.Errorf("invalid %s name %q (use lowercase letters, digits, and single dashes)", item.Kind, item.Name)
	}

	if other, ok := k.Item(item.Name); ok {
		return fmt.Errorf("%s %s is provided by both %s and %s bundles", item.Kind, item.Name, other.Bundle, item.Bundle)
	}

	k.items = append(k.items, item)
	return nil
}

// validName follows the strictest agent's naming rules (Zed, VS Code): 1-64
// lowercase letters, digits, and single dashes, no leading or trailing dash.
func validName(name string) bool {
	if len(name) == 0 || len(name) > 64 || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return false
	}

	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}

	return true
}

func (k *Kit) Bundles() []Bundle {
	return k.bundles
}

func (k *Kit) Bundle(name string) (Bundle, bool) {
	for _, b := range k.bundles {
		if b.Name == name {
			return b, true
		}
	}

	return Bundle{}, false
}

func (k *Kit) BundleNames() []string {
	var names []string
	for _, b := range k.bundles {
		names = append(names, b.Name)
	}

	return names
}

// Items returns all skills and rules of the kit (bundle by bundle, skills
// first).
func (k *Kit) Items() []Item {
	return k.items
}

// Item finds a skill or rule by its name.
func (k *Kit) Item(name string) (Item, bool) {
	for _, item := range k.items {
		if item.Name == name {
			return item, true
		}
	}

	return Item{}, false
}

// Show returns the main file of an item as the given agent sees it: the
// SKILL.md of a skill or the rule rendered in the agent's format.
func (k *Kit) Show(name, agent string) ([]byte, error) {
	item, ok := k.Item(name)
	if !ok {
		return nil, fmt.Errorf("unknown skill or rule %q", name)
	}

	if item.Kind == KindSkill {
		return item.skill["SKILL.md"], nil
	}

	a, err := lookupAgent(agent)
	if err != nil {
		return nil, err
	}

	return a.rules.render(item.rule), nil
}

// ResolveBundles validates the bundle names and adds the bundles they require.
// The result follows the kit's bundle order.
func (k *Kit) ResolveBundles(names []string) ([]string, error) {
	wanted := make(map[string]bool)

	var add func(name string) error
	add = func(name string) error {
		b, ok := k.Bundle(name)
		if !ok {
			return fmt.Errorf("unknown bundle %q (available: %s)", name, strings.Join(k.BundleNames(), ", "))
		}

		if wanted[name] {
			return nil
		}
		wanted[name] = true

		for _, req := range b.Requires {
			if err := add(req); err != nil {
				return err
			}
		}

		return nil
	}

	for _, name := range names {
		if err := add(name); err != nil {
			return nil, err
		}
	}

	var resolved []string
	for _, b := range k.bundles {
		if wanted[b.Name] {
			resolved = append(resolved, b.Name)
		}
	}

	return resolved, nil
}
