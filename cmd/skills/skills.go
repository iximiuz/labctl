package skills

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/iximiuz/labctl/internal/labcli"
	kit "github.com/iximiuz/labctl/skills"
)

// projectManifest is the manifest file name in the project directory.
const projectManifest = ".labctl-skills.json"

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "skills <list|show|install|uninstall> [flags]",
		Aliases: []string{"skill"},
		Short:   "Install AI agent skills and rules for working with iximiuz Labs",
		Long: `Install AI agent skills and rules for working with iximiuz Labs.

The skills teach AI agents to use labctl to start and debug playgrounds and
to create, push, and test content. The rules describe the content formats and
the authoring conventions. labctl writes them where (and in the format that)
each agent reads them.

Supported agents:
` + agentsHelp() + `
The skills are versioned with labctl - upgrade labctl and run
'labctl skills install' again to update them. Locally modified files are never
overwritten without --force.`,
	}

	cmd.AddCommand(
		newListCommand(cli),
		newShowCommand(cli),
		newInstallCommand(cli),
		newUninstallCommand(cli),
	)

	return cmd
}

type targetOptions struct {
	user bool
	dir  string
}

func (o *targetOptions) addFlags(fs *pflag.FlagSet) {
	fs.BoolVar(
		&o.user,
		"user",
		false,
		"Target the user's home directory (skills for all projects) instead of the project",
	)
	fs.StringVarP(
		&o.dir,
		"dir",
		"d",
		"",
		"Project directory to target (default: the current directory)",
	)
}

func (o *targetOptions) target(cli labcli.CLI) (kit.Target, error) {
	if o.user && o.dir != "" {
		return kit.Target{}, fmt.Errorf("--user and --dir are mutually exclusive")
	}

	if o.user {
		home, err := os.UserHomeDir()
		if err != nil {
			return kit.Target{}, fmt.Errorf("couldn't determine the home directory: %w", err)
		}

		return kit.Target{
			Scope:    kit.ScopeUser,
			Root:     home,
			Manifest: filepath.Join(filepath.Dir(cli.Config().FilePath), "skills.json"),
		}, nil
	}

	return projectTarget(o.dir)
}

func projectTarget(dir string) (kit.Target, error) {
	if dir == "" {
		dir = "."
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return kit.Target{}, fmt.Errorf("couldn't get the absolute path of %s: %w", dir, err)
	}

	return kit.Target{
		Scope:    kit.ScopeProject,
		Root:     abs,
		Manifest: filepath.Join(abs, projectManifest),
	}, nil
}

type agentOptions struct {
	agents []string
}

func (o *agentOptions) addFlags(fs *pflag.FlagSet, usage string) {
	fs.StringSliceVarP(
		&o.agents,
		"agent",
		"a",
		nil,
		usage+" (one or more of: "+strings.Join(kit.AgentNames(), ", ")+")",
	)
}

// PrintTip suggests installing the skills into the project that contentDir
// belongs to, or updating them when they are outdated. The install tip is
// shown only in a terminal, while the update one also reaches AI agents and
// CI logs - the project has opted into the skills already.
func PrintTip(cli labcli.CLI, contentDir string) {
	root, installed := findProject(contentDir)

	command := "labctl skills install"
	if cwd, err := os.Getwd(); err != nil || cwd != root {
		dir := root
		if rel, err := filepath.Rel(cwd, root); err == nil && cwd != "" {
			dir = rel
		}
		if strings.ContainsAny(dir, " \t'\"") {
			dir = "'" + strings.ReplaceAll(dir, "'", `'\''`) + "'"
		}
		command += " --dir " + dir
	}

	if !installed {
		if cli.AuxStream().IsTerminal() {
			cli.PrintAux("Tip: Authoring with AI agents? Run '%s' to add the iximiuz Labs skills and rules to the project.\n", command)
		}
		return
	}

	t, err := projectTarget(root)
	if err != nil {
		return
	}

	outdated, err := kit.Default().Outdated(t)
	if err != nil {
		slog.Debug("Couldn't check the installed skills: " + err.Error())
		return
	}

	if outdated {
		cli.PrintAux("Tip: The project's labctl skills are outdated. Run '%s' to update them.\n", command)
	}
}

// findProject returns the closest directory above contentDir that has the
// skills manifest, looking up to the git repository root. Without one, it
// returns the directory to install the skills into: the repository root, or
// else the parent of contentDir. The search starts above contentDir because
// the skills installed into a content folder would be pushed with it.
func findProject(contentDir string) (string, bool) {
	dir := filepath.Dir(contentDir)
	for {
		if _, err := os.Stat(filepath.Join(dir, projectManifest)); err == nil {
			return dir, true
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, false
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Dir(contentDir), false
		}
		dir = parent
	}
}

// chooseAgents asks the user to pick the agents (in a terminal) or explains
// how to pass them.
func chooseAgents(cli labcli.CLI) ([]string, error) {
	var detected []string
	if home, err := os.UserHomeDir(); err == nil {
		detected = kit.DetectAgents(home)
	}

	if !cli.InputStream().IsTerminal() || !cli.OutputStream().IsTerminal() {
		hint := ""
		if len(detected) > 0 {
			hint = fmt.Sprintf("\n\nHint: Detected on this machine: --agent %s", strings.Join(detected, ","))
		}

		return nil, labcli.NewStatusError(1,
			"choose the AI agents to install the skills for with --agent (one or more of: %s)%s",
			strings.Join(kit.AgentNames(), ", "), hint,
		)
	}

	var options []huh.Option[string]
	for _, a := range kit.Agents() {
		options = append(options, huh.NewOption(a.Title, a.Name).Selected(contains(detected, a.Name)))
	}

	var selected []string
	if err := huh.NewMultiSelect[string]().
		Title("Install the skills for which AI agents?").
		Description("Space to toggle, Enter to confirm. Use --agent to skip this question.").
		Options(options...).
		Validate(func(s []string) error {
			if len(s) == 0 {
				return fmt.Errorf("choose at least one agent")
			}
			return nil
		}).
		Value(&selected).
		Run(); err != nil {
		return nil, err
	}

	return selected, nil
}

func agentTitles(names []string) string {
	var titles []string
	for _, a := range kit.Agents() {
		if contains(names, a.Name) {
			titles = append(titles, a.Title)
		}
	}

	return joinWords(titles)
}

func agentsHelp() string {
	var sb strings.Builder
	for _, a := range kit.Agents() {
		fmt.Fprintf(&sb, "  %-12s %s\n", a.Name, a.Title)
	}

	return sb.String()
}

func bundleArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var names []string
	for _, b := range kit.Default().Bundles() {
		names = append(names, b.Name+"\t"+b.Description)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

func agentArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var names []string
	for _, a := range kit.Agents() {
		names = append(names, a.Name+"\t"+a.Title)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

func bundlesHelp() string {
	var sb strings.Builder
	for _, b := range kit.Default().Bundles() {
		fmt.Fprintf(&sb, "  %-12s %s", b.Name, b.Description)
		if b.ProjectOnly {
			sb.WriteString(" (project only)")
		}
		if len(b.Requires) > 0 {
			fmt.Fprintf(&sb, " (includes %s)", strings.Join(b.Requires, ", "))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func versionOf(cli labcli.CLI) string {
	if fields := strings.Fields(cli.Version()); len(fields) > 0 {
		return fields[0]
	}

	return "unknown"
}

func printChanges(cli labcli.CLI, changes []kit.Change) {
	for _, c := range changes {
		p := c.Path
		if file, bundle, ok := strings.Cut(p, "#"); ok {
			p = fmt.Sprintf("%s (the %s section)", file, bundle)
		}

		if c.Note != "" {
			cli.PrintAux("  %-8s %s (%s)\n", c.Action, p, c.Note)
		} else {
			cli.PrintAux("  %-8s %s\n", c.Action, p)
		}
	}
}

func countChanges(changes []kit.Change, actions ...kit.Action) int {
	var n int
	for _, c := range changes {
		for _, a := range actions {
			if c.Action == a {
				n++
			}
		}
	}

	return n
}

func joinWords(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	default:
		return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
	}
}

func union(a, b []string) []string {
	out := append([]string{}, a...)
	for _, s := range b {
		if !contains(out, s) {
			out = append(out, s)
		}
	}

	return out
}

func without(list, drop []string) []string {
	var out []string
	for _, s := range list {
		if !contains(drop, s) {
			out = append(out, s)
		}
	}

	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}

	return false
}
