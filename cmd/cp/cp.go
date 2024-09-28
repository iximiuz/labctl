package cp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/internal/labcli"
)

const example = `  # Copy a file from local machine to playground
  labctl cp ./some/file 65e78a64366c2b0cf9ddc34c:~/some/file

  # Copy a directory from local machine to playground
  labctl cp -r ./some/dir 65e78a64366c2b0cf9ddc34c:~/some/dir

  # Copy a file from the playground to local machine
  labctl cp 65e78a64366c2b0cf9ddc34c:~/some/file ./some/file

  # Copy a directory from the playground to local machine
  labctl cp 65e78a64366c2b0cf9ddc34c:~/some/dir ./some/dir
`

type Direction string

const (
	DirectionLocalToRemote Direction = "local-to-remote"
	DirectionRemoteToLocal Direction = "remote-to-local"
)

type options struct {
	machine string
	user    string

	playID     string
	localPath  string
	remotePath string
	recursive  bool

	direction Direction
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:     "cp [flags] <playground-id>:<source-path> <destination-path>\n  labctl cp [flags] <source-path> <playground-id>:<destination-path>",
		Short:   `Copy files to and from the target playground`,
		Example: example,
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.Count(args[0]+args[1], ":") != 1 {
				return fmt.Errorf("exactly one argument must be a colon-separated <playground-id>:<path> pair")
			}

			if strings.Contains(args[0], ":") {
				opts.direction = DirectionRemoteToLocal

				parts := strings.Split(args[0], ":")
				opts.playID = parts[0]
				opts.remotePath = parts[1]
				opts.localPath = args[1]
			} else {
				opts.direction = DirectionLocalToRemote

				parts := strings.Split(args[1], ":")
				opts.playID = parts[0]
				opts.remotePath = parts[1]
				opts.localPath = args[0]
			}

			return labcli.WrapStatusError(runCopy(cmd.Context(), cli, &opts))
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
	flags.StringVarP(
		&opts.user,
		"user",
		"u",
		"",
		`SSH user (default: the machine's default login user)`,
	)
	flags.BoolVarP(
		&opts.recursive,
		"recursive",
		"r",
		false,
		`Copy directories recursively`,
	)

	return cmd
}

func runCopy(ctx context.Context, cli labcli.CLI, opts *options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	return sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
		PlayID:  opts.playID,
		Machine: opts.machine,
		User:    opts.user,
		Quiet:   true,
		WithProxy: func(ctx context.Context, info *sshproxy.SSHProxyInfo) error {
			args := []string{
				"-i", info.IdentityFile,
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-P", info.ProxyPort,
				"-C", // compress
			}

			if opts.recursive {
				args = append(args, "-r")
			}

			if opts.direction == DirectionLocalToRemote {
				args = append(args,
					opts.localPath,
					fmt.Sprintf("%s@%s:%s", info.User, info.ProxyHost, opts.remotePath),
				)
			} else {
				args = append(args,
					fmt.Sprintf("%s@%s:%s", info.User, info.ProxyHost, opts.remotePath),
					opts.localPath,
				)
			}

			cmd := exec.CommandContext(ctx, "scp", args...)
			cmd.Stdout = cli.OutputStream()
			cmd.Stderr = cli.ErrorStream()

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("copy command failed %s: %w", cmd.String(), err)
			}

			cli.PrintAux("Done!\n")
			return nil
		},
	})
}
