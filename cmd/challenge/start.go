package challenge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/briandowns/spinner"
	"github.com/gorilla/websocket"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/cmd/ssh"
	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
	issh "github.com/iximiuz/labctl/internal/ssh"
)

const startChallengeTimeout = 10 * time.Minute

type startOptions struct {
	challenge string
	machine   string
	user      string

	noOpen    bool
	noSSH     bool
	keepAlive bool

	ide bool
}

func newStartCommand(cli labcli.CLI) *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:   "start [flags] <challenge-name>",
		Short: `Solve a challenge from the comfort of your local command line`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return labcli.NewStatusError(1,
					"challenge name is required\n\nHint: Use `labctl challenge list` to see all available challenges",
				)
			}

			opts.challenge = args[0]

			return labcli.WrapStatusError(runStartChallenge(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVar(
		&opts.machine,
		"machine",
		"",
		`SSH into the machine with the given name (requires --ssh flag, default to the first machine)`,
	)
	flags.StringVarP(
		&opts.user,
		"user",
		"u",
		"",
		`SSH user (default: the machine's default login user)`,
	)

	flags.BoolVar(
		&opts.noOpen,
		"no-open",
		false,
		`Don't open the challenge in the browser`,
	)
	flags.BoolVar(
		&opts.noSSH,
		"no-ssh",
		false,
		`Don't SSH into the challenge playground immediately after it's created`,
	)
	flags.BoolVar(
		&opts.keepAlive,
		"keep-alive",
		false,
		`Keep the challenge playground alive after it's completed`,
	)
	flags.BoolVar(
		&opts.ide,
		"ide",
		false,
		`Open the challenge playground in the IDE (only VSCode is supported at the moment)`,
	)

	return cmd
}

func runStartChallenge(ctx context.Context, cli labcli.CLI, opts *startOptions) error {
	chal, err := cli.Client().StartChallenge(ctx, opts.challenge)
	if err != nil {
		return fmt.Errorf("couldn't start solving the challenge: %w", err)
	}

	if opts.machine == "" {
		opts.machine = chal.Play.Machines[0].Name
	} else {
		if chal.Play.GetMachine(opts.machine) == nil {
			return fmt.Errorf("machine %q not found in the challenge playground", opts.machine)
		}
	}

	if opts.user == "" {
		if u := chal.Play.GetMachine(opts.machine).DefaultUser(); u != nil {
			opts.user = u.Name
		} else {
			opts.user = "root"
		}
	}
	if !chal.Play.GetMachine(opts.machine).HasUser(opts.user) {
		return fmt.Errorf("user %q not found in the machine %q", opts.user, opts.machine)
	}

	if !opts.noOpen {
		cli.PrintAux("Opening %s in your browser...\n", chal.PageURL)

		if err := open.Run(chal.PageURL); err != nil {
			cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually to access the playground.\n")
		}
	}

	hconn, err := cli.Client().RequestPlayConn(ctx, chal.Play.ID)
	if err != nil {
		return fmt.Errorf("couldn't create a connection to the challenge playground: %w", err)
	}

	connCtx, playConnCh, err := startPlayConn(ctx, cli, hconn.URL)
	if err != nil {
		return fmt.Errorf("couldn't start play connection: %w", err)
	}

	if err := waitChallengeReady(connCtx, cli, chal, playConnCh); err != nil {
		return fmt.Errorf("couldn't wait for the challenge to be ready: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if opts.ide {
		go func() {
			if err := sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
				PlayID:  chal.Play.ID,
				Machine: opts.machine,
				User:    opts.user,
				IDE:     true,
			}); err != nil {
				cli.PrintErr("Error running IDE session: %v", err)
			}
		}()
	}

	var sess *issh.Session
	if !opts.noSSH {
		cli.PrintAux("SSH-ing into challenge playground (%s machine)...\n", opts.machine)

		sess, err = ssh.StartSSHSession(ctx, cli, chal.Play.ID, opts.machine, opts.user, nil)
		if err != nil {
			return fmt.Errorf("couldn't start SSH session: %w", err)
		}
	}

	go func() {
		if err := waitChallengeDone(connCtx, chal, playConnCh); err != nil {
			cli.PrintErr("Error waiting for the challenge to be completable: %v", err)
		}

		for ctx.Err() == nil {
			if chal.IsCompletable() {
				if _, err := cli.Client().CompleteChallenge(ctx, chal.Name); err != nil {
					slog.Debug("Error completing the challenge: " + err.Error())
					time.Sleep(5 * time.Second)
					continue
				}
			}

			if !opts.keepAlive {
				sess.Close()

				// Reset terminal
				cli.PrintAux("\033c\r")

				if chal.IsFailed() {
					cli.PrintAux("************************************************************************\r\n")
					cli.PrintAux("** Oops... 🙈 The challenge playground has been irrecoverably broken. **\r\n")
					cli.PrintAux("************************************************************************\r\n")
				} else {
					cli.PrintAux("**********************************\r\n")
					cli.PrintAux("** Yay! Challenge completed! 🎉 **\r\n")
					cli.PrintAux("**********************************\r\n")
				}
				cli.PrintAux("\n\nStopping the playground...\r\n")

				for ctx.Err() == nil {
					if chal, err := cli.Client().StopChallenge(ctx, chal.Name); err != nil {
						cli.PrintErr("Error stopping the challenge: %v", err)
					} else if chal.Play == nil || !chal.Play.Active {
						cli.PrintAux("Playground stopped.\r\n")
						cancel()
						break
					}

					time.Sleep(2 * time.Second)
				}
			}
			break
		}
	}()

	if sess != nil {
		if err := sess.Wait(); err != nil {
			slog.Debug("SSH session wait said: " + err.Error())
		}

		if opts.keepAlive {
			cancel()
		}
	}

	<-ctx.Done()
	return nil
}

type PlayConnMessage struct {
	Kind    string       `json:"kind"`
	Machine string       `json:"machine,omitempty"`
	Task    api.PlayTask `json:"task,omitempty"`
}

func startPlayConn(
	ctx context.Context,
	cli labcli.CLI,
	url string,
) (context.Context, chan PlayConnMessage, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, http.Header{
		"Origin": {cli.Config().WebSocketOrigin()},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't connect to play connection WebSocket: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan PlayConnMessage, 1024)

	go func() {
		defer conn.Close()
		defer close(ch)
		defer cancel()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if err == io.EOF || websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}

				cli.PrintErr("Error reading play connection message: %v", err)
				continue
			}

			var msg PlayConnMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				cli.PrintErr("Error decoding play connection message: %v", err)
				continue
			}

			ch <- msg
		}
	}()

	return ctx, ch, nil
}

func waitChallengeReady(
	ctx context.Context,
	cli labcli.CLI,
	chal *api.Challenge,
	playConnCh chan PlayConnMessage,
) error {
	s := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	s.Writer = cli.AuxStream()
	s.Prefix = fmt.Sprintf(
		"Warming up playground... Init tasks completed: %d/%d ",
		chal.CountCompletedInitTasks(), chal.CountInitTasks(),
	)
	s.Start()

	ctx, cancel := context.WithTimeout(ctx, startChallengeTimeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-playConnCh:
			if msg.Kind == "task" {
				chal.Tasks[msg.Task.Name] = msg.Task
			}
		}

		s.Prefix = fmt.Sprintf(
			"Warming up playground... Init tasks completed: %d/%d ",
			chal.CountCompletedInitTasks(), chal.CountInitTasks(),
		)

		if chal.IsInitialized() {
			s.FinalMSG = "Warming up playground... Done.\n"
			s.Stop()
			return nil
		}
	}
}

func waitChallengeDone(
	ctx context.Context,
	chal *api.Challenge,
	playConnCh chan PlayConnMessage,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-playConnCh:
			if msg.Kind == "task" {
				chal.Tasks[msg.Task.Name] = msg.Task
			}
		}

		if chal.IsCompletable() || chal.IsFailed() {
			return nil
		}
	}
}
