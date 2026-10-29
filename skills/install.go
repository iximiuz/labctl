package skills

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Target is where the kit is installed.
type Target struct {
	Scope Scope

	// Root is the project directory (ScopeProject) or the home directory
	// (ScopeUser). The installed files' paths are relative to it.
	Root string

	// Manifest is the path of the manifest file, where labctl records what it
	// installed, so that it can later tell its own unmodified files from the
	// user's ones.
	Manifest string
}

type Manifest struct {
	// Version is the labctl version that made the last change.
	Version string `json:"version"`

	// Agents and Bundles are what the user asked to install.
	Agents  []string `json:"agents"`
	Bundles []string `json:"bundles"`

	// Files maps the installed files (relative to the target root) to their
	// bundles and content hashes at install time. A "<file>#<bundle>" key is
	// the bundle's section in an index file (e.g., AGENTS.md#authoring).
	Files map[string]ManifestEntry `json:"files"`
}

type ManifestEntry struct {
	Bundle string `json:"bundle"`
	SHA256 string `json:"sha256"`
}

// LoadManifest reads the target's manifest. A missing manifest yields an
// empty one and exists == false.
func LoadManifest(t Target) (m *Manifest, exists bool, err error) {
	m = &Manifest{Files: make(map[string]ManifestEntry)}

	data, err := os.ReadFile(t.Manifest)
	if os.IsNotExist(err) {
		return m, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	if err := json.Unmarshal(data, m); err != nil {
		return nil, true, fmt.Errorf("couldn't parse %s: %w", t.Manifest, err)
	}
	if m.Files == nil {
		m.Files = make(map[string]ManifestEntry)
	}

	// The manifest usually comes with the (possibly untrusted) repository, so
	// it must never point labctl at files outside of its own folders.
	for p := range m.Files {
		if !isKitPath(p) {
			return nil, true, fmt.Errorf("refusing to use %s: unexpected path %q", t.Manifest, p)
		}
	}

	return m, true, nil
}

func (m *Manifest) save(t Target) error {
	if len(m.Files) == 0 {
		err := os.Remove(t.Manifest)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(t.Manifest), 0755); err != nil {
		return err
	}

	return os.WriteFile(t.Manifest, append(data, '\n'), 0644)
}

// isKitPath reports whether p is a clean relative path to one of the files
// (or index file sections) that labctl manages.
func isKitPath(p string) bool {
	if file, bundle, ok := strings.Cut(p, "#"); ok {
		for _, a := range agents {
			if a.index != "" && file == a.index && validName(bundle) {
				return true
			}
		}
		return false
	}

	if p == "" || path.Clean(p) != p || !filepath.IsLocal(filepath.FromSlash(p)) {
		return false
	}

	for _, d := range allSkillDirs() {
		if rest, ok := strings.CutPrefix(p, d+"/"); ok && strings.HasPrefix(rest, "labs-") {
			return true
		}
	}

	for _, f := range ruleFormats {
		if f.owns(p) {
			return true
		}
	}

	return false
}

type Action string

const (
	ActionCreate Action = "created"
	ActionUpdate Action = "updated"
	ActionRemove Action = "removed"

	// ActionTrack means an identical copy of the file was already in place,
	// and labctl started tracking it.
	ActionTrack Action = "tracked"

	// ActionSkip means the file was left as is because it was modified
	// locally (or is not labctl's) and --force wasn't given.
	ActionSkip Action = "skipped"
)

type Change struct {
	// Path is the file path relative to the target root.
	Path   string
	Action Action
	Note   string

	bundle  string
	content []byte
	sha     string
}

type SyncOptions struct {
	// Agents and Bundles to have installed (resolved, see ResolveAgents and
	// Kit.ResolveBundles). Everything else that labctl installed earlier is
	// removed.
	Agents  []string
	Bundles []string

	// Version of labctl to record in the manifest.
	Version string

	// Force overwrites and removes locally modified files (and overwrites the
	// files that labctl didn't install) instead of skipping them.
	Force bool
}

// Sync brings the target in line with the given agents and bundles: it
// creates and updates their files and removes the files labctl installed
// earlier that are no longer needed. It returns the changes it made.
func (k *Kit) Sync(t Target, opts SyncOptions) ([]Change, error) {
	m, _, err := LoadManifest(t)
	if err != nil {
		return nil, err
	}

	changes, err := k.plan(t, m, opts.Agents, opts.Bundles, opts.Force)
	if err != nil {
		return nil, err
	}

	if err := apply(t, m, changes); err != nil {
		return nil, err
	}

	// Record the version only when something changed, so that an upgrade of
	// labctl with the same kit doesn't touch the (committed) manifest.
	for _, c := range changes {
		if c.Action != ActionSkip || m.Version == "" {
			m.Version = opts.Version
			break
		}
	}

	m.Agents = opts.Agents
	m.Bundles = opts.Bundles

	if err := m.save(t); err != nil {
		return nil, fmt.Errorf("couldn't save %s: %w", t.Manifest, err)
	}

	return changes, nil
}

// Outdated reports whether syncing the installed agents and bundles would
// change anything in the target.
func (k *Kit) Outdated(t Target) (bool, error) {
	m, exists, err := LoadManifest(t)
	if err != nil || !exists {
		return false, err
	}

	agents, bundles, err := k.installed(m)
	if err != nil {
		return false, err
	}

	changes, err := k.plan(t, m, agents, bundles, false)
	if err != nil {
		return false, err
	}

	for _, c := range changes {
		if c.Action != ActionSkip {
			return true, nil
		}
	}

	return false, nil
}

// installed returns the manifest's agents and bundles that this kit knows.
func (k *Kit) installed(m *Manifest) (agents []string, bundles []string, err error) {
	for _, name := range m.Bundles {
		if _, ok := k.Bundle(name); ok {
			bundles = append(bundles, name)
		}
	}

	for _, name := range m.Agents {
		if _, err := lookupAgent(name); err == nil {
			agents = append(agents, name)
		}
	}

	return agents, bundles, nil
}

type ItemState string

const (
	StateNotInstalled ItemState = "-"
	StateInstalled    ItemState = "installed"
	StateOutdated     ItemState = "outdated"
	StateModified     ItemState = "modified"
	StateMissing      ItemState = "missing"

	// StateConflict means a file that labctl didn't install takes the item's
	// place.
	StateConflict ItemState = "conflict"
)

// Status reports the state of every kit item for the given agents (keyed by
// the item name).
func (k *Kit) Status(t Target, agents []string) (map[string]ItemState, error) {
	m, _, err := LoadManifest(t)
	if err != nil {
		return nil, err
	}

	status := make(map[string]ItemState)
	for _, item := range k.items {
		status[item.Name] = StateNotInstalled
	}

	if len(agents) == 0 {
		return status, nil
	}

	desired, err := k.files(t.Scope, agents, k.BundleNames())
	if err != nil {
		return nil, err
	}

	changes, err := k.plan(t, m, agents, k.BundleNames(), false)
	if err != nil {
		return nil, err
	}

	byPath := make(map[string]Change, len(changes))
	for _, c := range changes {
		byPath[c.Path] = c
	}

	states := make(map[string]map[ItemState]bool)
	for p, f := range desired {
		if f.item == "" {
			continue // an index file section
		}

		if states[f.item] == nil {
			states[f.item] = make(map[ItemState]bool)
		}

		_, managed := m.Files[p]
		c, changed := byPath[p]

		switch {
		case !changed:
			states[f.item][StateInstalled] = true
		case c.Action == ActionCreate && managed:
			states[f.item][StateMissing] = true
		case c.Action == ActionCreate:
			states[f.item][StateNotInstalled] = true
		case c.Action == ActionTrack:
			states[f.item][StateInstalled] = true
		case c.Action == ActionUpdate:
			states[f.item][StateOutdated] = true
		case c.Action == ActionSkip && managed:
			states[f.item][StateModified] = true
		default:
			states[f.item][StateConflict] = true
		}
	}

	for name, s := range states {
		status[name] = summarize(s)
	}

	return status, nil
}

func summarize(states map[ItemState]bool) ItemState {
	if len(states) == 1 && states[StateNotInstalled] {
		return StateNotInstalled
	}

	for _, s := range []ItemState{StateModified, StateMissing, StateConflict, StateOutdated} {
		if states[s] {
			return s
		}
	}

	if states[StateNotInstalled] {
		return StateOutdated // installed partially (e.g., for some of the agents)
	}

	return StateInstalled
}

type file struct {
	bundle string

	// item is the kit item the file belongs to ("" for index file sections).
	item string

	content []byte
}

// files returns the files that the given agents need for the given bundles,
// keyed by their paths relative to the target root.
func (k *Kit) files(scope Scope, agentNames []string, bundles []string) (map[string]file, error) {
	var selected []Agent
	for _, name := range agentNames {
		a, err := lookupAgent(name)
		if err != nil {
			return nil, err
		}
		selected = append(selected, a)
	}

	out := make(map[string]file)
	put := func(p string, f file) error {
		if old, ok := out[p]; ok && !bytes.Equal(old.content, f.content) {
			return fmt.Errorf("conflicting content for %s", p)
		}
		out[p] = f
		return nil
	}

	dirs := skillDirs(scope, selected)

	for _, item := range k.items {
		if !contains(bundles, item.Bundle) {
			continue
		}

		switch item.Kind {
		case KindSkill:
			for _, d := range dirs {
				for rel, content := range item.skill {
					if err := put(path.Join(d, item.Name, rel), file{bundle: item.Bundle, item: item.Name, content: content}); err != nil {
						return nil, err
					}
				}
			}

		case KindRule:
			if scope != ScopeProject {
				continue // rules are project-only (see Bundle.ProjectOnly)
			}

			for _, a := range selected {
				if err := put(a.rules.path(item.Name), file{bundle: item.Bundle, item: item.Name, content: a.rules.render(item.rule)}); err != nil {
					return nil, err
				}
			}
		}
	}

	if scope != ScopeProject {
		return out, nil
	}

	for _, a := range selected {
		if a.index == "" {
			continue
		}

		for _, bundle := range bundles {
			var rules []Rule
			for _, item := range k.items {
				if item.Bundle == bundle && item.Kind == KindRule {
					rules = append(rules, item.rule)
				}
			}

			if len(rules) == 0 {
				continue
			}

			if err := put(indexKey(a.index, bundle), file{bundle: bundle, content: renderIndex(bundle, rules, a.rules)}); err != nil {
				return nil, err
			}
		}
	}

	return out, nil
}

func (k *Kit) plan(t Target, m *Manifest, agents []string, bundles []string, force bool) ([]Change, error) {
	desired, err := k.files(t.Scope, agents, bundles)
	if err != nil {
		return nil, err
	}

	var changes []Change

	for _, p := range sortedKeys(desired) {
		f := desired[p]
		c := Change{Path: p, bundle: f.bundle, content: f.content, sha: hash(f.content)}

		if err := checkNoSymlinks(t.Root, p); err != nil {
			c.Action = ActionSkip
			c.Note = err.Error()
			changes = append(changes, c)
			continue
		}

		onDisk, exists, err := readEntry(t.Root, p)
		if err != nil {
			return nil, err
		}

		entry, managed := m.Files[p]

		switch {
		case !exists:
			c.Action = ActionCreate

		case hash(onDisk) == c.sha:
			if managed && entry.SHA256 == c.sha && entry.Bundle == c.bundle {
				continue // up to date
			}
			c.Action = ActionTrack // the content is already there

		case managed && hash(onDisk) == entry.SHA256:
			c.Action = ActionUpdate

		case force:
			c.Action = ActionUpdate
			c.Note = "overwrote local changes"

		case managed:
			c.Action = ActionSkip
			c.Note = "modified locally - use --force to overwrite it"

		default:
			c.Action = ActionSkip
			c.Note = "not installed by labctl - use --force to overwrite it"
		}

		changes = append(changes, c)
	}

	// The files that are no longer needed. Locally modified ones are kept
	// (and stay tracked, so that a later forced run can remove them).
	for _, p := range sortedKeys(m.Files) {
		if _, ok := desired[p]; ok {
			continue
		}

		entry := m.Files[p]
		c := Change{Path: p, Action: ActionRemove, bundle: entry.Bundle}

		if err := checkNoSymlinks(t.Root, p); err != nil {
			c.Action = ActionSkip
			c.Note = err.Error()
		} else if onDisk, exists, err := readEntry(t.Root, p); err != nil {
			return nil, err
		} else if exists && hash(onDisk) != entry.SHA256 && !force {
			c.Action = ActionSkip
			c.Note = "modified locally - use --force to remove it"
		}

		changes = append(changes, c)
	}

	return changes, nil
}

func apply(t Target, m *Manifest, changes []Change) error {
	for _, c := range changes {
		switch c.Action {
		case ActionCreate, ActionUpdate:
			if err := writeEntry(t.Root, c.Path, c.content); err != nil {
				return err
			}
			m.Files[c.Path] = ManifestEntry{Bundle: c.bundle, SHA256: c.sha}

		case ActionTrack:
			m.Files[c.Path] = ManifestEntry{Bundle: c.bundle, SHA256: c.sha}

		case ActionRemove:
			if err := removeEntry(t.Root, c.Path); err != nil {
				return err
			}
			delete(m.Files, c.Path)
		}
	}

	return nil
}

// readEntry returns the current content of a file or an index file section.
func readEntry(root, p string) (content []byte, exists bool, err error) {
	name, bundle, isBlock := strings.Cut(p, "#")

	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	if !isBlock {
		return data, true, nil
	}

	start, end, found := findBlock(data, bundle)
	if !found {
		return nil, false, nil
	}

	return data[start:end], true, nil
}

func writeEntry(root, p string, content []byte) error {
	name, bundle, isBlock := strings.Cut(p, "#")
	full := filepath.Join(root, filepath.FromSlash(name))

	if isBlock {
		data, err := os.ReadFile(full)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		content = spliceBlock(data, bundle, content)
	}

	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}

	return os.WriteFile(full, content, 0644)
}

func removeEntry(root, p string) error {
	name, bundle, isBlock := strings.Cut(p, "#")
	full := filepath.Join(root, filepath.FromSlash(name))

	if isBlock {
		data, err := os.ReadFile(full)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}

		if rest := cutBlock(data, bundle); len(rest) > 0 {
			return os.WriteFile(full, rest, 0644)
		}
	}

	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}

	pruneEmptyDirs(root, path.Dir(name))
	return nil
}

// checkNoSymlinks makes sure that none of the existing elements of p
// (relative to root) is a symlink, so that labctl never writes or deletes
// files outside of the folders it manages.
func checkNoSymlinks(root, p string) error {
	name, _, _ := strings.Cut(p, "#")

	cur := root
	for _, part := range strings.Split(name, "/") {
		cur = filepath.Join(cur, part)

		fi, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink - labctl doesn't follow symlinks", cur)
		}
	}

	return nil
}

// pruneEmptyDirs removes the empty directories from rel (relative to root) up
// to, but not including, root itself.
func pruneEmptyDirs(root, rel string) {
	for rel != "." && rel != "/" && rel != "" {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			return // not empty (or already gone)
		}
		rel = path.Dir(rel)
	}
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}
