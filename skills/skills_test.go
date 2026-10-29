package skills

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDefaultKit(t *testing.T) {
	k := Default()

	skillRef := regexp.MustCompile("`(labs-[a-z0-9-]+)` skill")

	for _, item := range k.Items() {
		var content []byte

		switch item.Kind {
		case KindSkill:
			content = item.skill["SKILL.md"]
			meta := frontmatter(t, item.Name, content)

			assert.Equal(t, item.Name, meta["name"], "%s: name must match the folder name", item.Name)
			assert.NotEmpty(t, meta["description"], "%s: description is required", item.Name)
			assert.LessOrEqual(t, len(meta["description"].(string)), 1024, "%s: description is too long for some agents", item.Name)
			assert.True(t, strings.HasPrefix(item.Name, "labs-"), "%s: skill names start with labs-", item.Name)

		case KindRule:
			src, err := embedded.ReadFile(item.Bundle + "/rules/" + item.Name + ".md")
			require.NoError(t, err)
			content = src

			bundle, _ := k.Bundle(item.Bundle)
			assert.True(t, bundle.ProjectOnly, "%s: rules need a project-only bundle", item.Name)

			// The source files are in Claude Code's format, which must render back unchanged.
			assert.Equal(t, string(src), string(renderClaudeRule(item.rule)), "%s: not in the canonical format", item.Name)

			assert.NotEmpty(t, item.rule.Description, "%s: a rule needs a # heading", item.Name)
			assert.NotContains(t, item.rule.Description, ":", "%s: Cursor's description is unquoted", item.Name)

			for _, f := range []*ruleFormat{&claudeRules, &copilotRules, &agentsRules} {
				frontmatter(t, item.Name, f.render(item.rule))
			}

			// Antigravity's per-file limit.
			assert.LessOrEqual(t, len(agentsRules.render(item.rule)), 24000, "%s: too big for Antigravity", item.Name)
		}

		for _, m := range skillRef.FindAllStringSubmatch(string(content), -1) {
			ref, ok := k.Item(m[1])
			if assert.True(t, ok, "%s refers to an unknown skill %s", item.Name, m[1]) {
				assert.Equal(t, KindSkill, ref.Kind)
				assertReachable(t, k, item.Bundle, ref.Bundle)
			}
		}
	}

	for _, b := range k.Bundles() {
		_, err := k.ResolveBundles(b.Requires)
		assert.NoError(t, err, "bundle %s", b.Name)
	}

	// Every agent gets everything without conflicts.
	_, err := k.files(ScopeProject, AgentNames(), k.BundleNames())
	assert.NoError(t, err)
}

// assertReachable checks that installing the from bundle also installs the to
// bundle.
func assertReachable(t *testing.T, k *Kit, from, to string) {
	t.Helper()

	resolved, err := k.ResolveBundles([]string{from})
	require.NoError(t, err)
	assert.Contains(t, resolved, to, "the %s bundle refers to the %s bundle without requiring it", from, to)
}

func frontmatter(t *testing.T, name string, content []byte) map[string]any {
	t.Helper()

	s := string(content)
	if !strings.HasPrefix(s, "---\n") {
		return map[string]any{}
	}

	end := strings.Index(s[4:], "\n---\n")
	require.NotEqual(t, -1, end, "%s: unterminated frontmatter", name)

	meta := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(s[4:4+end]), &meta), "%s: invalid frontmatter", name)
	return meta
}

func TestRuleRendering(t *testing.T) {
	scoped := Rule{Name: "x", Description: "X Rules", Paths: []string{"**/a/**", "**/b/**"}, Body: []byte("# X Rules\n")}
	always := Rule{Name: "y", Description: "Y Rules", Body: []byte("# Y Rules\n")}

	assert.Equal(t, "---\npaths:\n  - \"**/a/**\"\n  - \"**/b/**\"\n---\n\n# X Rules\n", string(renderClaudeRule(scoped)))
	assert.Equal(t, "# Y Rules\n", string(renderClaudeRule(always)))

	assert.Equal(t, "---\ndescription: X Rules\nglobs: **/a/**, **/b/**\nalwaysApply: false\n---\n\n# X Rules\n", string(renderCursorRule(scoped)))
	assert.Equal(t, "---\ndescription: Y Rules\nalwaysApply: true\n---\n\n# Y Rules\n", string(renderCursorRule(always)))

	assert.Equal(t, "---\napplyTo: \"**/a/**,**/b/**\"\n---\n\n# X Rules\n", string(renderCopilotRule(scoped)))
	assert.Equal(t, "---\napplyTo: \"**\"\n---\n\n# Y Rules\n", string(renderCopilotRule(always)))

	assert.Equal(t, "---\ntrigger: glob\nglobs: \"**/a/**, **/b/**\"\ndescription: \"X Rules\"\n---\n\n# X Rules\n", string(renderAntigravityRule(scoped)))
	assert.Equal(t, "---\ntrigger: always_on\ndescription: \"Y Rules\"\n---\n\n# Y Rules\n", string(renderAntigravityRule(always)))

	index := string(renderIndex("base", []Rule{always, scoped}, &agentsRules))
	assert.True(t, strings.HasPrefix(index, "<!-- BEGIN labctl skills: base -->\n"))
	assert.True(t, strings.HasSuffix(index, "<!-- END labctl skills: base -->"))
	assert.Contains(t, index, "- `.agents/rules/labctl-y.md` - Y Rules\n")
	assert.Contains(t, index, "- `.agents/rules/labctl-x.md` - X Rules (`**/a/**`, `**/b/**`)\n")
}

func TestBlocks(t *testing.T) {
	block := []byte("<!-- BEGIN labctl skills: base -->\nhi\n<!-- END labctl skills: base -->")
	block2 := []byte("<!-- BEGIN labctl skills: base -->\nhello\n<!-- END labctl skills: base -->")

	assert.Equal(t, string(block)+"\n", string(spliceBlock(nil, "base", block)))

	mine := []byte("# My Notes\n\nKeep it short.\n")
	withBlock := spliceBlock(mine, "base", block)
	assert.Equal(t, "# My Notes\n\nKeep it short.\n\n"+string(block)+"\n", string(withBlock))

	updated := spliceBlock(withBlock, "base", block2)
	assert.Equal(t, "# My Notes\n\nKeep it short.\n\n"+string(block2)+"\n", string(updated))

	assert.Equal(t, string(mine), string(cutBlock(updated, "base")))
	assert.Empty(t, cutBlock(spliceBlock(nil, "base", block), "base"))

	// A block in the middle of the file.
	middle := []byte("intro\n\n" + string(block) + "\n\noutro\n")
	assert.Equal(t, "intro\n\noutro\n", string(cutBlock(middle, "base")))
	assert.Equal(t, string(middle), string(cutBlock(middle, "other")))
}

func testKit(t *testing.T, files map[string]string) *Kit {
	t.Helper()

	fsys := fstest.MapFS{}
	for p, content := range files {
		fsys[p] = &fstest.MapFile{Data: []byte(content)}
	}

	k, err := New(fsys, []Bundle{
		{Name: "base", ProjectOnly: true},
		{Name: "extra", Requires: []string{"base"}},
	})
	require.NoError(t, err)

	return k
}

func projectTarget(t *testing.T) Target {
	t.Helper()

	root := t.TempDir()
	return Target{Scope: ScopeProject, Root: root, Manifest: filepath.Join(root, ".labctl-skills.json")}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func actions(changes []Change) map[string]Action {
	m := make(map[string]Action)
	for _, c := range changes {
		m[c.Path] = c.Action
	}
	return m
}

func TestNewRejectsBrokenKits(t *testing.T) {
	_, err := New(fstest.MapFS{"base/skills/labs-a/README.md": &fstest.MapFile{Data: []byte("x")}}, []Bundle{{Name: "base"}})
	assert.ErrorContains(t, err, "has no SKILL.md")

	_, err = New(fstest.MapFS{
		"base/rules/a.md":  &fstest.MapFile{Data: []byte("x")},
		"extra/rules/a.md": &fstest.MapFile{Data: []byte("y")},
	}, []Bundle{{Name: "base"}, {Name: "extra"}})
	assert.ErrorContains(t, err, "provided by both")

	_, err = New(fstest.MapFS{"base/rules/Bad_Name.md": &fstest.MapFile{Data: []byte("x")}}, []Bundle{{Name: "base"}})
	assert.ErrorContains(t, err, "invalid rule name")
}

func TestResolve(t *testing.T) {
	k := testKit(t, map[string]string{"base/rules/a.md": "# A\n"})

	resolved, err := k.ResolveBundles([]string{"extra"})
	require.NoError(t, err)
	assert.Equal(t, []string{"base", "extra"}, resolved)

	_, err = k.ResolveBundles([]string{"nope"})
	assert.ErrorContains(t, err, `unknown bundle "nope"`)

	agents, err := ResolveAgents([]string{"zed", "claude", "zed"})
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "zed"}, agents)

	_, err = ResolveAgents([]string{"vim"})
	assert.ErrorContains(t, err, `unknown agent "vim"`)
}

func TestSkillDirsAvoidDuplicates(t *testing.T) {
	agent := func(name string) Agent {
		a, err := lookupAgent(name)
		require.NoError(t, err)
		return a
	}

	assert.Equal(t, []string{".claude/skills"}, skillDirs(ScopeProject, []Agent{agent("claude"), agent("cursor"), agent("copilot")}))
	assert.Equal(t, []string{".agents/skills"}, skillDirs(ScopeProject, []Agent{agent("cursor")}))
	assert.Equal(t, []string{".claude/skills", ".agents/skills"}, skillDirs(ScopeProject, []Agent{agent("claude"), agent("codex"), agent("cursor")}))
	assert.Equal(t, []string{".gemini/config/skills", ".gemini/antigravity-cli/skills"}, skillDirs(ScopeUser, []Agent{agent("antigravity")}))
}

func TestSyncLifecycle(t *testing.T) {
	tgt := projectTarget(t)
	root := tgt.Root

	k := testKit(t, map[string]string{
		"base/skills/labs-one/SKILL.md":  "one v1",
		"base/skills/labs-one/notes.txt": "notes v1",
		"base/rules/style.md":            "# Style\n\nv1\n",
		"extra/skills/labs-two/SKILL.md": "two v1",
	})

	sync := func(k *Kit, bundles []string, force bool, version string) []Change {
		t.Helper()
		changes, err := k.Sync(tgt, SyncOptions{Agents: []string{"claude"}, Bundles: bundles, Version: version, Force: force})
		require.NoError(t, err)
		return changes
	}

	// Fresh install of the base bundle only.
	assert.Equal(t, map[string]Action{
		".claude/skills/labs-one/SKILL.md":  ActionCreate,
		".claude/skills/labs-one/notes.txt": ActionCreate,
		".claude/rules/labctl/style.md":     ActionCreate,
	}, actions(sync(k, []string{"base"}, false, "v1")))
	assert.Equal(t, "one v1", readFile(t, filepath.Join(root, ".claude/skills/labs-one/SKILL.md")))
	assert.Equal(t, "# Style\n\nv1\n", readFile(t, filepath.Join(root, ".claude/rules/labctl/style.md")))

	m, exists, err := LoadManifest(tgt)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "v1", m.Version)
	assert.Equal(t, []string{"claude"}, m.Agents)
	assert.Equal(t, []string{"base"}, m.Bundles)

	// Re-syncing changes nothing, not even the recorded version.
	assert.Empty(t, sync(k, []string{"base"}, false, "v1.1"))
	m, _, err = LoadManifest(tgt)
	require.NoError(t, err)
	assert.Equal(t, "v1", m.Version)

	outdated, err := k.Outdated(tgt)
	require.NoError(t, err)
	assert.False(t, outdated)

	// The user edits a skill, and a new labctl version changes the kit.
	writeFile(t, filepath.Join(root, ".claude/skills/labs-one/SKILL.md"), "one - my version")

	k2 := testKit(t, map[string]string{
		"base/skills/labs-one/SKILL.md":  "one v2",
		"base/rules/style.md":            "# Style\n\nv2\n",
		"base/rules/new.md":              "# New\n",
		"extra/skills/labs-two/SKILL.md": "two v1",
	})

	outdated, err = k2.Outdated(tgt)
	require.NoError(t, err)
	assert.True(t, outdated)

	status, err := k2.Status(tgt, []string{"claude"})
	require.NoError(t, err)
	assert.Equal(t, StateModified, status["labs-one"])
	assert.Equal(t, StateOutdated, status["style"])
	assert.Equal(t, StateNotInstalled, status["new"])
	assert.Equal(t, StateNotInstalled, status["labs-two"])

	assert.Equal(t, map[string]Action{
		".claude/skills/labs-one/SKILL.md":  ActionSkip,   // modified locally
		".claude/skills/labs-one/notes.txt": ActionRemove, // dropped from the kit
		".claude/rules/labctl/style.md":     ActionUpdate,
		".claude/rules/labctl/new.md":       ActionCreate,
	}, actions(sync(k2, []string{"base"}, false, "v2")))
	assert.Equal(t, "one - my version", readFile(t, filepath.Join(root, ".claude/skills/labs-one/SKILL.md")))
	assert.NoFileExists(t, filepath.Join(root, ".claude/skills/labs-one/notes.txt"))

	// A locally modified file doesn't make the kit look outdated forever.
	outdated, err = k2.Outdated(tgt)
	require.NoError(t, err)
	assert.False(t, outdated)

	// --force overwrites the local changes.
	assert.Equal(t, map[string]Action{".claude/skills/labs-one/SKILL.md": ActionUpdate}, actions(sync(k2, []string{"base"}, true, "v2")))
	assert.Equal(t, "one v2", readFile(t, filepath.Join(root, ".claude/skills/labs-one/SKILL.md")))

	// Another bundle.
	assert.Equal(t, map[string]Action{".claude/skills/labs-two/SKILL.md": ActionCreate}, actions(sync(k2, []string{"base", "extra"}, false, "v2")))

	// Removing everything keeps the locally modified files tracked, so that
	// a forced run can still remove them.
	writeFile(t, filepath.Join(root, ".claude/rules/labctl/new.md"), "mine")
	writeFile(t, filepath.Join(root, ".claude/skills/my-own/SKILL.md"), "not labctl's")

	changes, err := k2.Sync(tgt, SyncOptions{})
	require.NoError(t, err)
	assert.Equal(t, map[string]Action{
		".claude/skills/labs-one/SKILL.md": ActionRemove,
		".claude/skills/labs-two/SKILL.md": ActionRemove,
		".claude/rules/labctl/style.md":    ActionRemove,
		".claude/rules/labctl/new.md":      ActionSkip,
	}, actions(changes))
	assert.NoDirExists(t, filepath.Join(root, ".claude/skills/labs-one"))
	assert.FileExists(t, filepath.Join(root, ".claude/skills/my-own/SKILL.md"))
	assert.FileExists(t, tgt.Manifest)

	changes, err = k2.Sync(tgt, SyncOptions{Force: true})
	require.NoError(t, err)
	assert.Equal(t, map[string]Action{".claude/rules/labctl/new.md": ActionRemove}, actions(changes))
	assert.NoDirExists(t, filepath.Join(root, ".claude/rules"))
	assert.NoFileExists(t, tgt.Manifest)
}

func TestSyncManyAgents(t *testing.T) {
	tgt := projectTarget(t)
	root := tgt.Root

	k := testKit(t, map[string]string{
		"base/skills/labs-one/SKILL.md": "one",
		"base/rules/always.md":          "# Always\n",
		"base/rules/scoped.md":          "---\npaths:\n  - \"**/challenges/**\"\n---\n\n# Scoped\n",
	})

	writeFile(t, filepath.Join(root, "AGENTS.md"), "# My Project\n\nMy own instructions.\n")

	all := []string{"claude", "codex", "cursor", "copilot", "gemini", "antigravity"}
	changes, err := k.Sync(tgt, SyncOptions{Agents: all, Bundles: []string{"base"}})
	require.NoError(t, err)
	assert.Equal(t, map[string]Action{
		".claude/skills/labs-one/SKILL.md":                   ActionCreate,
		".agents/skills/labs-one/SKILL.md":                   ActionCreate,
		".claude/rules/labctl/always.md":                     ActionCreate,
		".claude/rules/labctl/scoped.md":                     ActionCreate,
		".cursor/rules/labctl/always.mdc":                    ActionCreate,
		".cursor/rules/labctl/scoped.mdc":                    ActionCreate,
		".github/instructions/labctl/always.instructions.md": ActionCreate,
		".github/instructions/labctl/scoped.instructions.md": ActionCreate,
		".agents/rules/labctl-always.md":                     ActionCreate,
		".agents/rules/labctl-scoped.md":                     ActionCreate,
		"AGENTS.md#base":                                     ActionCreate,
		"GEMINI.md#base":                                     ActionCreate,
	}, actions(changes))

	agentsMD := readFile(t, filepath.Join(root, "AGENTS.md"))
	assert.True(t, strings.HasPrefix(agentsMD, "# My Project\n\nMy own instructions.\n\n<!-- BEGIN labctl skills: base -->\n"))
	assert.Contains(t, agentsMD, "`.agents/rules/labctl-scoped.md` - Scoped (`**/challenges/**`)")
	assert.Contains(t, readFile(t, filepath.Join(root, ".agents/rules/labctl-scoped.md")), "trigger: glob\n")

	gemini := readFile(t, filepath.Join(root, "GEMINI.md"))
	assert.True(t, strings.HasPrefix(gemini, "<!-- BEGIN labctl skills: base -->\n"))

	// The user edits the file around the section - that's fine.
	writeFile(t, filepath.Join(root, "AGENTS.md"), "# My Project\n\nMy own instructions, updated.\n"+agentsMD[len("# My Project\n\nMy own instructions.\n"):])
	changes, err = k.Sync(tgt, SyncOptions{Agents: all, Bundles: []string{"base"}})
	require.NoError(t, err)
	assert.Empty(t, changes)

	// Dropping Codex removes the AGENTS.md section (no other agent here needs
	// it) but keeps .agents/ (Gemini CLI and Antigravity still read it).
	changes, err = k.Sync(tgt, SyncOptions{Agents: []string{"claude", "cursor", "copilot", "gemini", "antigravity"}, Bundles: []string{"base"}})
	require.NoError(t, err)
	assert.Equal(t, map[string]Action{"AGENTS.md#base": ActionRemove}, actions(changes))
	assert.Equal(t, "# My Project\n\nMy own instructions, updated.\n", readFile(t, filepath.Join(root, "AGENTS.md")))

	// Dropping everything else removes GEMINI.md (labctl created it) and the
	// agents' folders.
	changes, err = k.Sync(tgt, SyncOptions{})
	require.NoError(t, err)
	assert.Equal(t, ActionRemove, actions(changes)["GEMINI.md#base"])
	assert.NoFileExists(t, filepath.Join(root, "GEMINI.md"))
	for _, d := range []string{".claude", ".agents", ".cursor", ".github"} {
		assert.NoDirExists(t, filepath.Join(root, d))
	}
	assert.FileExists(t, filepath.Join(root, "AGENTS.md"))
}

func TestModifiedSectionIsKept(t *testing.T) {
	tgt := projectTarget(t)

	k := testKit(t, map[string]string{"base/rules/always.md": "# Always\n"})

	_, err := k.Sync(tgt, SyncOptions{Agents: []string{"codex"}, Bundles: []string{"base"}})
	require.NoError(t, err)

	agentsMD := filepath.Join(tgt.Root, "AGENTS.md")
	edited := strings.Replace(readFile(t, agentsMD), "## iximiuz Labs Rules", "## Our Rules", 1)
	writeFile(t, agentsMD, edited)

	k2 := testKit(t, map[string]string{"base/rules/always.md": "# Always\n", "base/rules/more.md": "# More\n"})
	changes, err := k2.Sync(tgt, SyncOptions{Agents: []string{"codex"}, Bundles: []string{"base"}})
	require.NoError(t, err)
	assert.Equal(t, ActionSkip, actions(changes)["AGENTS.md#base"])
	assert.Equal(t, edited, readFile(t, agentsMD))
}

func TestUserScope(t *testing.T) {
	home := t.TempDir()
	tgt := Target{Scope: ScopeUser, Root: home, Manifest: filepath.Join(home, ".iximiuz/labctl/skills.json")}

	k := testKit(t, map[string]string{
		"extra/skills/labs-two/SKILL.md": "two",
		"base/rules/always.md":           "# Always\n",
	})

	changes, err := k.Sync(tgt, SyncOptions{Agents: []string{"codex", "antigravity"}, Bundles: []string{"extra"}})
	require.NoError(t, err)
	assert.Equal(t, map[string]Action{
		".agents/skills/labs-two/SKILL.md":                 ActionCreate,
		".gemini/config/skills/labs-two/SKILL.md":          ActionCreate,
		".gemini/antigravity-cli/skills/labs-two/SKILL.md": ActionCreate,
	}, actions(changes))
	assert.FileExists(t, tgt.Manifest)
}

func TestManifestPathsAreConfined(t *testing.T) {
	for _, p := range []string{
		"../outside.txt",
		".claude/skills/../../outside.txt",
		"/etc/passwd",
		".claude/skills/./labs-x/SKILL.md",
		".claude/skills/not-labs/SKILL.md",
		"CLAUDE.md",
		".agents/rules/mine.md",
		"README.md#base",
		"AGENTS.md#../x",
	} {
		t.Run(p, func(t *testing.T) {
			tgt := projectTarget(t)

			victim := filepath.Join(filepath.Dir(tgt.Root), "outside.txt")
			writeFile(t, victim, "precious")

			manifest := `{"version": "v1", "agents": ["claude"], "bundles": ["base"], "files": {"` + p + `": {"bundle": "base", "sha256": "` + hash([]byte("precious")) + `"}}}`
			writeFile(t, tgt.Manifest, manifest)

			k := testKit(t, map[string]string{"base/rules/style.md": "# Style\n"})

			_, err := k.Sync(tgt, SyncOptions{Agents: []string{"claude"}, Bundles: []string{"base"}, Force: true})
			assert.ErrorContains(t, err, "unexpected path")

			_, err = k.Sync(tgt, SyncOptions{Force: true})
			assert.ErrorContains(t, err, "unexpected path")

			assert.Equal(t, "precious", readFile(t, victim))
		})
	}
}

func TestSymlinksAreNotFollowed(t *testing.T) {
	tgt := projectTarget(t)
	outside := t.TempDir()

	victim := filepath.Join(outside, "SKILL.md")
	writeFile(t, victim, "precious")

	// A repository plants a symlinked skill folder and a matching manifest entry.
	require.NoError(t, os.MkdirAll(filepath.Join(tgt.Root, ".claude/skills"), 0755))
	require.NoError(t, os.Symlink(outside, filepath.Join(tgt.Root, ".claude/skills/labs-one")))
	writeFile(t, tgt.Manifest, `{"version": "v1", "agents": ["claude"], "bundles": ["base"], "files": {".claude/skills/labs-one/SKILL.md": {"bundle": "base", "sha256": "`+hash([]byte("precious"))+`"}}}`)

	k := testKit(t, map[string]string{"base/skills/labs-one/SKILL.md": "one"})

	changes, err := k.Sync(tgt, SyncOptions{Agents: []string{"claude"}, Bundles: []string{"base"}, Force: true})
	require.NoError(t, err)
	assert.Equal(t, map[string]Action{".claude/skills/labs-one/SKILL.md": ActionSkip}, actions(changes))
	assert.Equal(t, "precious", readFile(t, victim))

	changes, err = k.Sync(tgt, SyncOptions{Force: true})
	require.NoError(t, err)
	assert.Equal(t, map[string]Action{".claude/skills/labs-one/SKILL.md": ActionSkip}, actions(changes))
	assert.Equal(t, "precious", readFile(t, victim))

	// A symlinked index file (e.g., AGENTS.md -> CLAUDE.md) is left alone too.
	require.NoError(t, os.Symlink(victim, filepath.Join(tgt.Root, "AGENTS.md")))
	k2 := testKit(t, map[string]string{"base/rules/always.md": "# Always\n"})
	changes, err = k2.Sync(tgt, SyncOptions{Agents: []string{"codex"}, Bundles: []string{"base"}, Force: true})
	require.NoError(t, err)
	assert.Equal(t, ActionSkip, actions(changes)["AGENTS.md#base"])
	assert.Equal(t, "precious", readFile(t, victim))
}

func TestConflictsWithUserFiles(t *testing.T) {
	tgt := projectTarget(t)

	k := testKit(t, map[string]string{
		"base/skills/labs-one/SKILL.md": "one v1",
		"base/rules/style.md":           "# Style\n",
	})

	// The user already has a skill with the same name and an identical copy of a rule.
	writeFile(t, filepath.Join(tgt.Root, ".claude/skills/labs-one/SKILL.md"), "someone else's skill")
	writeFile(t, filepath.Join(tgt.Root, ".claude/rules/labctl/style.md"), "# Style\n")

	status, err := k.Status(tgt, []string{"claude"})
	require.NoError(t, err)
	assert.Equal(t, StateConflict, status["labs-one"])
	assert.Equal(t, StateInstalled, status["style"])

	changes, err := k.Sync(tgt, SyncOptions{Agents: []string{"claude"}, Bundles: []string{"base"}})
	require.NoError(t, err)
	assert.Equal(t, map[string]Action{
		".claude/skills/labs-one/SKILL.md": ActionSkip,
		".claude/rules/labctl/style.md":    ActionTrack,
	}, actions(changes))
	assert.Equal(t, "someone else's skill", readFile(t, filepath.Join(tgt.Root, ".claude/skills/labs-one/SKILL.md")))

	// Uninstalling never touches the files labctl doesn't own.
	_, err = k.Sync(tgt, SyncOptions{Force: true})
	require.NoError(t, err)
	assert.Equal(t, "someone else's skill", readFile(t, filepath.Join(tgt.Root, ".claude/skills/labs-one/SKILL.md")))
	assert.NoFileExists(t, filepath.Join(tgt.Root, ".claude/rules/labctl/style.md"))
}

func TestDetectAgents(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".codex/config.toml"), "")
	writeFile(t, filepath.Join(home, ".config/zed/settings.json"), "")

	assert.Equal(t, []string{"codex", "zed"}, DetectAgents(home))
}
