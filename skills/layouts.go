package skills

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Scope tells where the kit is installed: into a project or into the user's
// home directory.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeUser    Scope = "user"
)

// Agent describes where (and in which format) an AI agent reads skills and
// rules. The layouts follow the agents' docs as of October 2026.
type Agent struct {
	Name  string
	Title string

	// projectSkills and userSkills list the skill directories the agent needs
	// (relative to the project or the home directory). Every group must be
	// covered by one of its directories, and the first one is preferred.
	projectSkills [][]string
	userSkills    [][]string

	// rules is the format of the agent's project rules.
	rules *ruleFormat

	// index is an always-on instructions file (e.g., AGENTS.md) where labctl
	// keeps a section that lists the rules, for agents that can't load rule
	// files on their own.
	index string

	// markers are paths (relative to the home directory) that suggest the
	// agent is installed on the machine.
	markers []string
}

var (
	claudeSkills = ".claude/skills"
	agentsSkills = ".agents/skills" // the cross-agent location
)

var agents = []Agent{
	{
		Name:          "claude",
		Title:         "Claude Code",
		projectSkills: [][]string{{claudeSkills}},
		userSkills:    [][]string{{claudeSkills}},
		rules:         &claudeRules,
		markers:       []string{".claude"},
	},
	{
		Name:          "codex",
		Title:         "Codex",
		projectSkills: [][]string{{agentsSkills}},
		userSkills:    [][]string{{agentsSkills}},
		rules:         &agentsRules,
		index:         "AGENTS.md",
		markers:       []string{".codex"},
	},
	{
		Name:          "cursor",
		Title:         "Cursor",
		projectSkills: [][]string{{agentsSkills, claudeSkills}},
		userSkills:    [][]string{{agentsSkills, claudeSkills}},
		rules:         &cursorRules,
		markers:       []string{".cursor"},
	},
	{
		Name:          "copilot",
		Title:         "GitHub Copilot",
		projectSkills: [][]string{{agentsSkills, claudeSkills}},
		userSkills:    [][]string{{agentsSkills}},
		rules:         &copilotRules,
		markers:       []string{".copilot"},
	},
	{
		Name:          "opencode",
		Title:         "OpenCode",
		projectSkills: [][]string{{agentsSkills, claudeSkills}},
		userSkills:    [][]string{{agentsSkills, claudeSkills}},
		rules:         &agentsRules,
		index:         "AGENTS.md",
		markers:       []string{".config/opencode"},
	},
	{
		Name:          "gemini",
		Title:         "Gemini CLI",
		projectSkills: [][]string{{agentsSkills}},
		userSkills:    [][]string{{agentsSkills}},
		rules:         &agentsRules,
		index:         "GEMINI.md",
		markers:       []string{".gemini/settings.json"},
	},
	{
		Name:          "antigravity",
		Title:         "Antigravity",
		projectSkills: [][]string{{agentsSkills}},
		// The app/IDE and the CLI read different global skill directories.
		userSkills: [][]string{{".gemini/config/skills"}, {".gemini/antigravity-cli/skills"}},
		rules:      &agentsRules, // read natively from .agents/rules/*.md
		markers:    []string{".gemini/antigravity", ".gemini/antigravity-cli", ".gemini/config"},
	},
	{
		Name:          "devin",
		Title:         "Devin",
		projectSkills: [][]string{{agentsSkills, claudeSkills}},
		userSkills:    [][]string{{agentsSkills}},
		rules:         &agentsRules,
		index:         "AGENTS.md",
		markers:       []string{".config/devin"},
	},
	{
		Name:          "warp",
		Title:         "Warp",
		projectSkills: [][]string{{agentsSkills, claudeSkills}},
		userSkills:    [][]string{{agentsSkills, claudeSkills}},
		rules:         &agentsRules,
		index:         "AGENTS.md",
		markers:       []string{".warp"},
	},
	{
		Name:          "zed",
		Title:         "Zed",
		projectSkills: [][]string{{agentsSkills}},
		userSkills:    [][]string{{agentsSkills}},
		rules:         &agentsRules,
		index:         "AGENTS.md",
		markers:       []string{".config/zed"},
	},
}

// Agents returns the supported agents.
func Agents() []Agent {
	return agents
}

func AgentNames() []string {
	var names []string
	for _, a := range agents {
		names = append(names, a.Name)
	}

	return names
}

func lookupAgent(name string) (Agent, error) {
	for _, a := range agents {
		if a.Name == name {
			return a, nil
		}
	}

	return Agent{}, fmt.Errorf("unknown agent %q (supported: %s)", name, strings.Join(AgentNames(), ", "))
}

// ResolveAgents validates the agent names and returns them in the canonical
// order (without duplicates).
func ResolveAgents(names []string) ([]string, error) {
	for _, name := range names {
		if _, err := lookupAgent(name); err != nil {
			return nil, err
		}
	}

	var resolved []string
	for _, a := range agents {
		for _, name := range names {
			if a.Name == name {
				resolved = append(resolved, a.Name)
				break
			}
		}
	}

	return resolved, nil
}

// DetectAgents returns the agents that seem to be installed on the machine
// (best effort, based on their files in the home directory).
func DetectAgents(home string) []string {
	var found []string
	for _, a := range agents {
		for _, m := range a.markers {
			if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(m))); err == nil {
				found = append(found, a.Name)
				break
			}
		}
	}

	return found
}

// skillDirs returns the skill directories that cover all the given agents,
// preferring the directories that are already chosen (so that agents reading
// several locations don't get duplicates).
func skillDirs(scope Scope, selected []Agent) []string {
	var dirs []string
	for _, a := range selected {
		groups := a.projectSkills
		if scope == ScopeUser {
			groups = a.userSkills
		}

		for _, group := range groups {
			covered := false
			for _, d := range group {
				if contains(dirs, d) {
					covered = true
					break
				}
			}

			if !covered {
				dirs = append(dirs, group[0])
			}
		}
	}

	return dirs
}

func allSkillDirs() []string {
	var dirs []string
	for _, a := range agents {
		for _, groups := range [][][]string{a.projectSkills, a.userSkills} {
			for _, group := range groups {
				for _, d := range group {
					if !contains(dirs, d) {
						dirs = append(dirs, d)
					}
				}
			}
		}
	}

	return dirs
}

type ruleFormat struct {
	// dir is relative to the project directory.
	dir string

	// prefix marks labctl's files in a folder shared with other rules.
	prefix string

	ext string

	render func(r Rule) []byte
}

var (
	claudeRules = ruleFormat{dir: ".claude/rules/labctl", ext: ".md", render: renderClaudeRule}

	cursorRules = ruleFormat{dir: ".cursor/rules/labctl", ext: ".mdc", render: renderCursorRule}

	copilotRules = ruleFormat{dir: ".github/instructions/labctl", ext: ".instructions.md", render: renderCopilotRule}

	// Antigravity loads .agents/rules/*.md (but not its subfolders) natively.
	// The agents that read only AGENTS.md (or GEMINI.md) get a section there
	// that points at the same files.
	agentsRules = ruleFormat{dir: ".agents/rules", prefix: "labctl-", ext: ".md", render: renderAntigravityRule}

	ruleFormats = []*ruleFormat{&claudeRules, &cursorRules, &copilotRules, &agentsRules}
)

func (f *ruleFormat) path(name string) string {
	return path.Join(f.dir, f.prefix+name+f.ext)
}

func (f *ruleFormat) owns(p string) bool {
	rest, ok := strings.CutPrefix(p, f.dir+"/")
	return ok && !strings.Contains(rest, "/") && strings.HasPrefix(rest, f.prefix) && strings.HasSuffix(rest, f.ext)
}

func renderClaudeRule(r Rule) []byte {
	var b bytes.Buffer
	if len(r.Paths) > 0 {
		b.WriteString("---\npaths:\n")
		for _, p := range r.Paths {
			fmt.Fprintf(&b, "  - %q\n", p)
		}
		b.WriteString("---\n\n")
	}

	b.Write(r.Body)
	return b.Bytes()
}

func renderCursorRule(r Rule) []byte {
	var b bytes.Buffer
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %s\n", r.Description)
	if len(r.Paths) > 0 {
		// Cursor documents comma-separated, unquoted globs.
		fmt.Fprintf(&b, "globs: %s\n", strings.Join(r.Paths, ", "))
		b.WriteString("alwaysApply: false\n")
	} else {
		b.WriteString("alwaysApply: true\n")
	}
	b.WriteString("---\n\n")

	b.Write(r.Body)
	return b.Bytes()
}

func renderCopilotRule(r Rule) []byte {
	applyTo := "**"
	if len(r.Paths) > 0 {
		applyTo = strings.Join(r.Paths, ",")
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "---\napplyTo: %q\n---\n\n", applyTo)

	b.Write(r.Body)
	return b.Bytes()
}

func renderAntigravityRule(r Rule) []byte {
	var b bytes.Buffer
	b.WriteString("---\n")
	if len(r.Paths) > 0 {
		b.WriteString("trigger: glob\n")
		fmt.Fprintf(&b, "globs: %q\n", strings.Join(r.Paths, ", "))
	} else {
		b.WriteString("trigger: always_on\n")
	}
	fmt.Fprintf(&b, "description: %q\n", r.Description)
	b.WriteString("---\n\n")

	b.Write(r.Body)
	return b.Bytes()
}

// The section that labctl keeps in an index file (AGENTS.md, GEMINI.md). Its
// manifest key is "<file>#<bundle>".

func indexKey(file, bundle string) string {
	return file + "#" + bundle
}

func blockMarkers(bundle string) (begin, end string) {
	return fmt.Sprintf("<!-- BEGIN labctl skills: %s -->", bundle), fmt.Sprintf("<!-- END labctl skills: %s -->", bundle)
}

func renderIndex(bundle string, rules []Rule, f *ruleFormat) []byte {
	begin, end := blockMarkers(bundle)

	var always, scoped bytes.Buffer
	for _, r := range rules {
		if len(r.Paths) == 0 {
			fmt.Fprintf(&always, "- `%s` - %s\n", f.path(r.Name), r.Description)
		} else {
			fmt.Fprintf(&scoped, "- `%s` - %s (`%s`)\n", f.path(r.Name), r.Description, strings.Join(r.Paths, "`, `"))
		}
	}

	var b bytes.Buffer
	b.WriteString(begin + "\n")
	b.WriteString("## iximiuz Labs Rules\n\n")
	if always.Len() > 0 {
		b.WriteString("Read these rule files before working on the project, unless they are already in your context:\n\n")
		b.Write(always.Bytes())
		b.WriteString("\n")
	}
	if scoped.Len() > 0 {
		b.WriteString("Read these rule files before working on the files matching their globs:\n\n")
		b.Write(scoped.Bytes())
		b.WriteString("\n")
	}
	b.WriteString("This section is managed by `labctl skills install` - edit the rule files instead.\n")
	b.WriteString(end)

	return b.Bytes()
}

// findBlock returns the byte range of the bundle's section in data.
func findBlock(data []byte, bundle string) (start, end int, found bool) {
	begin, stop := blockMarkers(bundle)

	start = bytes.Index(data, []byte(begin))
	if start == -1 {
		return 0, 0, false
	}

	n := bytes.Index(data[start:], []byte(stop))
	if n == -1 {
		return 0, 0, false
	}

	return start, start + n + len(stop), true
}

func spliceBlock(data []byte, bundle string, block []byte) []byte {
	if start, end, found := findBlock(data, bundle); found {
		return bytes.Join([][]byte{data[:start], block, data[end:]}, nil)
	}

	var b bytes.Buffer
	if before := bytes.TrimRight(data, "\n"); len(before) > 0 {
		b.Write(before)
		b.WriteString("\n\n")
	}
	b.Write(block)
	b.WriteString("\n")

	return b.Bytes()
}

// cutBlock removes the bundle's section from data. The result is empty if
// nothing but the section was there.
func cutBlock(data []byte, bundle string) []byte {
	start, end, found := findBlock(data, bundle)
	if !found {
		return data
	}

	before := bytes.TrimRight(data[:start], "\n")
	after := bytes.TrimLeft(data[end:], "\n")

	switch {
	case len(before) == 0 && len(after) == 0:
		return nil
	case len(before) == 0:
		return after
	case len(after) == 0:
		return bytes.Join([][]byte{before, nil}, []byte("\n"))
	default:
		return bytes.Join([][]byte{before, after}, []byte("\n\n"))
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}

	return false
}
