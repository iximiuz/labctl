package sshproxy

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
	"github.com/iximiuz/labctl/internal/ssh"
)

type Options struct {
	PlayID  string
	Machine string
	User    string
	Address string

	IDE bool
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts Options

	cmd := &cobra.Command{
		Use:   "ssh-proxy [flags] <playground-id>",
		Short: `Start SSH proxy to the playground's machine`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.PlayID = args[0]

			if opts.Address != "" && strings.Count(opts.Address, ":") != 1 {
				return fmt.Errorf("invalid address %q", opts.Address)
			}

			return labcli.WrapStatusError(RunSSHProxy(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(
		&opts.Machine,
		"machine",
		"m",
		"",
		`Target machine (default: the first machine in the playground)`,
	)
	flags.StringVar(
		&opts.User,
		"user",
		"",
		`Login user (default: the machine's default login user)`,
	)
	flags.StringVar(
		&opts.Address,
		"address",
		"",
		`Local address to map to the machine's SSHD port (default: 'localhost:<random port>')`,
	)
	flags.BoolVar(
		&opts.IDE,
		"ide",
		false,
		`Open the playground in the IDE (only VSCode is supported at the moment)`,
	)

	return cmd
}

func RunSSHProxy(ctx context.Context, cli labcli.CLI, opts *Options) error {
	p, err := cli.Client().GetPlay(ctx, opts.PlayID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if opts.Machine == "" {
		opts.Machine = p.Machines[0].Name
	} else {
		if p.GetMachine(opts.Machine) == nil {
			return fmt.Errorf("machine %q not found in the playground", opts.Machine)
		}
	}

	if opts.User == "" {
		if u := p.GetMachine(opts.Machine).DefaultUser(); u != nil {
			opts.User = u.Name
		} else {
			opts.User = "root"
		}
	}
	if !p.GetMachine(opts.Machine).HasUser(opts.User) {
		return fmt.Errorf("user %q not found in the machine %q", opts.User, opts.Machine)
	}

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:   opts.PlayID,
		Machine:  opts.Machine,
		PlaysDir: cli.Config().PlaysDir,
		SSHUser:  opts.User,
		SSHDir:   cli.Config().SSHDir,
	})
	if err != nil {
		return fmt.Errorf("couldn't start tunnel: %w", err)
	}

	var (
		localPort = portStr(opts.Address)
		localHost = hostStr(opts.Address)
		errCh     = make(chan error, 100)
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		if err := tunnel.Forward(ctx, portforward.ForwardingSpec{
			LocalPort:  localPort,
			LocalHost:  localHost,
			RemotePort: "22",
		}, errCh); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				slog.Debug("Tunnel borked", "error", err.Error())
			}
		}
	}()

	if !opts.IDE {
		cli.PrintOut("SSH proxy is running on %s\n", localPort)
		cli.PrintOut(
			"\n# Connect from the terminal:\nssh -i %s/%s ssh://%s@%s:%s\n",
			cli.Config().SSHDir, ssh.IdentityFile, opts.User, localHost, localPort,
		)

		cli.PrintOut("\n# Or add the following to your ~/.ssh/config:\n")
		cli.PrintOut("Host %s\n", opts.PlayID+"-"+opts.Machine)
		cli.PrintOut("  HostName %s\n", localHost)
		cli.PrintOut("  Port %s\n", localPort)
		cli.PrintOut("  User %s\n", opts.User)
		cli.PrintOut("  IdentityFile %s/%s\n", cli.Config().SSHDir, ssh.IdentityFile)
		cli.PrintOut("  StrictHostKeyChecking no\n")
		cli.PrintOut("  UserKnownHostsFile /dev/null\n\n")

		cli.PrintOut("# To access the playground in Visual Studio Code:\n")
		cli.PrintOut("code --folder-uri vscode-remote://ssh-remote+%s@%s:%s%s\n\n",
			opts.User, localHost, localPort, userHomeDir(opts.User))

		cli.PrintOut("\nPress Ctrl+C to stop\n")
	} else {
		cli.PrintAux("Opening the playground in the IDE...\n")

		// Hack: SSH into the playground first - otherwise, VSCode will fail to connect for some reason.
		cmd := exec.Command("ssh",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "StrictHostKeyChecking=no",
			"-o", "IdentitiesOnly=yes",
			"-o", "PreferredAuthentications=publickey",
			"-i", fmt.Sprintf("%s/%s", cli.Config().SSHDir, ssh.IdentityFile),
			fmt.Sprintf("ssh://%s@%s:%s", opts.User, localHost, localPort),
		)
		cmd.Run()

		cmd = exec.Command("code",
			"--folder-uri", fmt.Sprintf("vscode-remote://ssh-remote+%s@%s:%s%s",
				opts.User, localHost, localPort, userHomeDir(opts.User)),
		)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("couldn't open the IDE: %w", err)
		}
	}

	// Wait for ctrl+c
	<-ctx.Done()

	return nil
}

func portStr(address string) string {
	if address == "" {
		return portforward.RandomLocalPort()
	}

	if address[0] == ':' {
		return address[1:]
	}

	return strings.Split(address, ":")[1]
}

func hostStr(address string) string {
	if address == "" {
		return "127.0.0.1"
	}

	if address[0] == ':' {
		return "127.0.0.1"
	}

	return strings.Split(address, ":")[0]
}

func userHomeDir(user string) string {
	if user == "root" {
		return "/root"
	}
	return fmt.Sprintf("/home/%s", user)
}
