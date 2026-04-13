package ide

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
	"github.com/iximiuz/labctl/internal/retry"
)

const (
	ideVSCode   = "code"
	ideCursor   = "cursor"
	ideWindsurf = "windsurf"
)

type repoSpec struct {
	url      string
	cloneDir string // empty if not specified
}

func parseRepoSpec(spec string) (repoSpec, error) {
	lastColon := strings.LastIndex(spec, ":")
	if lastColon == -1 {
		return repoSpec{url: spec}, nil
	}

	if strings.Contains(spec, "://") {
		// HTTPS-style URL: https://host[:port]/owner/repo[:cloneDir]
		schemeEnd := strings.Index(spec, "://") + 3
		if lastColon < schemeEnd {
			return repoSpec{url: spec}, nil
		}

		// Check if the last colon is a port separator (between host and first path slash)
		afterScheme := spec[schemeEnd:]
		firstSlash := strings.Index(afterScheme, "/")
		if firstSlash < 0 {
			firstSlash = len(afterScheme)
		}
		if lastColon < schemeEnd+firstSlash {
			return repoSpec{url: spec}, nil
		}

		dir := spec[lastColon+1:]
		if dir == "" {
			return repoSpec{}, fmt.Errorf("empty clone directory in repo spec %q", spec)
		}
		return repoSpec{url: spec[:lastColon], cloneDir: dir}, nil
	}

	// SSH-style URL: git@host:owner/repo[:cloneDir]
	firstColon := strings.Index(spec, ":")
	if firstColon == lastColon {
		return repoSpec{url: spec}, nil
	}

	dir := spec[lastColon+1:]
	if dir == "" {
		return repoSpec{}, fmt.Errorf("empty clone directory in repo spec %q", spec)
	}
	return repoSpec{url: spec[:lastColon], cloneDir: dir}, nil
}

// cloneTarget returns the absolute path where the repo should be cloned.
func (r repoSpec) cloneTarget(baseDir string) string {
	if r.cloneDir != "" {
		if filepath.IsAbs(r.cloneDir) {
			return r.cloneDir
		}
		return filepath.Join(baseDir, r.cloneDir)
	}
	return filepath.Join(baseDir, repoBaseName(r.url))
}

type options struct {
	ide     string
	playID  string
	machine string
	user    string
	workDir string
	repos   []string

	forwardAgent bool
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "ide <code|cursor|windsurf> <playground-id>",
		Short: `Open a playground in a local IDE`,
		Long: `Start an SSH proxy to the playground and open it in the specified IDE.

Optionally clone one or more Git repositories into the playground before opening.

Example:

# 1. Start a new Go(lang) development playground
PLAY_ID=$(labctl playground start golang)

# 2. Open the playground in VSCode and clone the repository into the working directory
labctl ide code $PLAY_ID --workdir projects --repo https://github.com/foo/bar
`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: ideCompletion(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ide = args[0]
			opts.playID = args[1]

			switch opts.ide {
			case ideVSCode, ideCursor, ideWindsurf:
			default:
				return fmt.Errorf("unsupported IDE %q (supported: %q, %q, %q)", opts.ide, ideVSCode, ideCursor, ideWindsurf)
			}

			return labcli.WrapStatusError(runIDE(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(
		&opts.machine,
		"machine",
		"m",
		"",
		`Target machine (default: the first machine in the playground)`,
	)
	flags.StringVar(
		&opts.user,
		"user",
		"",
		`Login user (default: the machine's default login user)`,
	)
	flags.StringVarP(
		&opts.workDir,
		"workdir",
		"w",
		"",
		`Working directory to open in the IDE (default: user's home directory if no repos are specified or the first repo's clone directory)`,
	)
	flags.StringArrayVarP(
		&opts.repos,
		"repo",
		"r",
		nil,
		`Git repository to clone (can be repeated). Format: <url>[:<path>]
  e.g. https://github.com/user/repo
       git@github.com:user/repo
       https://github.com/user/repo:path/to/dir`,
	)
	flags.BoolVar(
		&opts.forwardAgent,
		"forward-agent",
		false,
		`INSECURE: Forward the SSH agent to the playground VM to clone repo(s) (use at your own risk)`,
	)

	return cmd
}

func ideCompletion(cli labcli.CLI) completion.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return []string{
				ideVSCode + "\tVisual Studio Code",
				ideCursor + "\tCursor",
				ideWindsurf + "\tWindsurf",
			}, cobra.ShellCompDirectiveNoFileComp
		case 1:
			return completion.ActivePlays(cli)(cmd, nil, toComplete)
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

func runIDE(ctx context.Context, cli labcli.CLI, opts *options) error {
	cli.PrintAux("Starting SSH proxy to the playground...\n")

	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if opts.machine, err = p.ResolveMachine(opts.machine); err != nil {
		return err
	}
	if opts.user, err = p.ResolveUser(opts.machine, opts.user); err != nil {
		return err
	}

	var (
		localHost  = "127.0.0.1"
		localPort  = portforward.RandomLocalPort()
		remotePort = "22"
	)

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:          p.ID,
		Machine:         opts.machine,
		SSHUser:         opts.user,
		SSHIdentityFile: cli.Config().SSHIdentityFile,
	})
	if err != nil {
		return fmt.Errorf("couldn't start tunnel: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		status := tunnel.StartForwarding(ctx, portforward.ForwardingSpec{
			LocalPort:  localPort,
			LocalHost:  localHost,
			RemotePort: remotePort,
		})
		if err := <-status; err != nil {
			slog.Debug("Tunnel forwarding exited with error", "error", err.Error())
		}
	}()

	cli.PrintAux("Waiting for the SSH connection to be ready...\n")

	if err := retry.UntilSuccess(ctx, func() error {
		return runRemoteCommand(ctx, cli.Config().SSHIdentityFile, opts.user, localHost, localPort, "true")
	}, 30, 1*time.Second); err != nil {
		return fmt.Errorf("couldn't establish SSH connection: %w", err)
	}

	cli.PrintAux("SSH connection established.\n")

	homeDir := userHomeDir(opts.user)

	// Parse repo specs.
	var repos []repoSpec
	for _, raw := range opts.repos {
		r, err := parseRepoSpec(raw)
		if err != nil {
			return err
		}
		repos = append(repos, r)
	}

	// Resolve the working directory.
	workDir := opts.workDir
	switch {
	case workDir != "":
		if !filepath.IsAbs(workDir) {
			workDir = filepath.Join(homeDir, workDir)
		}
	case len(repos) == 1:
		// Default to the single repo's clone folder.
		workDir = repos[0].cloneTarget(homeDir)
	default:
		workDir = homeDir
	}

	// Normalize remote Linux paths on Windows.
	if workDir != "" && !filepath.IsAbs(workDir) {
		workDir = path.Join(homeDir, workDir)
	}

	// Workaround: SSH into the playground first - otherwise, the IDE may fail to connect.
	warmup := exec.CommandContext(ctx, "ssh",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=no",
		"-o", "IdentitiesOnly=yes",
		"-o", "PreferredAuthentications=publickey",
		"-i", cli.Config().SSHIdentityFile,
		"-p", localPort,
		fmt.Sprintf("%s@%s", opts.user, localHost),
		"true",
	)
	warmup.Run()

	if workDir != homeDir {
		if err := runRemoteCommand(
			ctx, cli.Config().SSHIdentityFile, opts.user, localHost, localPort,
			fmt.Sprintf("mkdir -p %s", workDir),
		); err != nil {
			return fmt.Errorf("couldn't create working directory: %w", err)
		}
	}

	if len(repos) > 0 {
		if err := cloneRepos(ctx, cli, opts, repos, localHost, localPort, homeDir); err != nil {
			return err
		}
	}

	folderURI := fmt.Sprintf("vscode-remote://ssh-remote+%s@%s:%s%s",
		opts.user, localHost, localPort, workDir)

	cli.PrintAux("Opening %s...\n", opts.ide)
	cli.PrintAux("Running: %s --folder-uri %s\n", opts.ide, folderURI)

	codeCmd := exec.CommandContext(ctx, opts.ide, "--folder-uri", folderURI)
	if runtime.GOOS == "windows" {
		codeCmd = exec.CommandContext(ctx, "cmd", "/C", opts.ide, "--folder-uri", folderURI)
	}
	codeCmd.Stdout = os.Stdout
	codeCmd.Stderr = os.Stderr
	codeCmd.Stdin = os.Stdin
	if err := codeCmd.Start(); err != nil {
		return fmt.Errorf("couldn't open %s: %w", opts.ide, err)
	}
	cli.PrintAux("%s launched.\n", strings.Title(opts.ide))

	cli.PrintAux("\n# If the IDE fails to connect, add the following to your ~/.ssh/config:\n")
	cli.PrintAux("Host localhost 127.0.0.1 ::1\n")
	cli.PrintAux("  IdentityFile %s\n", cli.Config().SSHIdentityFile)
	cli.PrintAux("  AddKeysToAgent yes\n")
	if runtime.GOOS == "darwin" {
		cli.PrintAux("  UseKeychain yes\n")
	}
	cli.PrintAux("  StrictHostKeyChecking no\n")
	cli.PrintAux("  UserKnownHostsFile /dev/null\n")

	cli.PrintAux("\nIDE is connected. Press Ctrl+C to stop the SSH proxy.\n")

	<-ctx.Done()

	return nil
}

func cloneRepos(ctx context.Context, cli labcli.CLI, opts *options, repos []repoSpec, localHost, localPort, baseDir string) error {
	cli.PrintAux("Cloning %d repo(s)...\n", len(repos))

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, repo := range repos {
		wg.Add(1)
		go func(r repoSpec) {
			defer wg.Done()

			target := r.cloneTarget(baseDir)
			cli.PrintAux("  Cloning %s into %s...\n", r.url, target)

			cloneCmd := fmt.Sprintf(
				"mkdir -p %s && GIT_SSH_COMMAND='ssh -o StrictHostKeyChecking=no' git clone %s %s",
				filepath.Dir(target), r.url, target)

			var err error
			if opts.forwardAgent {
				err = runRemoteCommandWithAgent(
					ctx, cli.Config().SSHIdentityFile, opts.user, localHost, localPort, cloneCmd)
			} else {
				err = runRemoteCommand(
					ctx, cli.Config().SSHIdentityFile, opts.user, localHost, localPort, cloneCmd)
			}
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("couldn't clone %s: %w", r.url, err))
				mu.Unlock()
				cli.PrintErr("  Failed to clone %s: %v\n", r.url, err)
				return
			}

			cli.PrintAux("  Cloned %s.\n", r.url)
		}(repo)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("%d repo(s) failed to clone", len(errs))
	}

	cli.PrintAux("All repositories cloned successfully.\n")
	return nil
}

func runRemoteCommand(ctx context.Context, identityFile, user, host, port, command string) error {
	return doRunRemoteCommand(ctx, identityFile, user, host, port, command, false)
}

func runRemoteCommandWithAgent(ctx context.Context, identityFile, user, host, port, command string) error {
	return doRunRemoteCommand(ctx, identityFile, user, host, port, command, true)
}

func doRunRemoteCommand(ctx context.Context, identityFile, user, host, port, command string, forwardAgent bool) error {
	args := []string{
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=no",
		"-o", "IdentitiesOnly=yes",
		"-o", "PreferredAuthentications=publickey",
		"-i", identityFile,
		"-p", port,
	}
	if forwardAgent {
		args = append(args, "-o", "ForwardAgent=yes")
	}
	args = append(args, fmt.Sprintf("%s@%s", user, host), command)

	cmd := exec.CommandContext(ctx, "ssh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func repoBaseName(repo string) string {
	name := strings.TrimSuffix(repo, ".git")
	parts := strings.Split(name, "/")
	name = parts[len(parts)-1]
	// Handle SSH-style URLs like git@github.com:user/repo
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

func userHomeDir(user string) string {
	if user == "root" {
		return "/root"
	}
	return fmt.Sprintf("/home/%s", user)
}
