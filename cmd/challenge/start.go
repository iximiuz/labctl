package challenge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
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
		Use:     "start [flags] <challenge-url|challenge-name>",
		Short:   `Solve a challenge from the comfort of your local command line`,
		Aliases: []string{"solve"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return labcli.NewStatusError(1,
					"challenge name is required\n\nHint: Use `labctl challenge catalog` to see all available challenges",
				)
			}

			opts.challenge = args[0]
			if strings.HasPrefix(opts.challenge, "https://") {
				parts := strings.Split(strings.Trim(opts.challenge, "/"), "/")
				opts.challenge = parts[len(parts)-1]
			}

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

type challengeEvent string

const (
	EventChallengeReady       challengeEvent = "challenge-ready"
	EventChallengeCompletable challengeEvent = "challenge-completable"
	EventChallengeCompleted   challengeEvent = "challenge-completed"
	EventChallengeFailed      challengeEvent = "challenge-failed"
	EventSSHConnEnded         challengeEvent = "ssh-conn-ended"
	EventWSConnFailed         challengeEvent = "ws-conn-failed"
)

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

	playConn := newPlayConn(ctx, cli, hconn.URL)
	if err := playConn.Start(); err != nil {
		return fmt.Errorf("couldn't start play connection: %w", err)
	}

	eventCh := make(chan challengeEvent, 100)

	go func() {
		if err := playConn.WaitChallengeReady(chal); err != nil {
			eventCh <- EventWSConnFailed
			return
		}
		eventCh <- EventChallengeReady

		if err := playConn.WaitChallengeDone(chal); err != nil {
			eventCh <- EventWSConnFailed
			return
		}

		if chal.IsCompletable() {
			eventCh <- EventChallengeCompletable
		} else {
			eventCh <- EventChallengeFailed
		}
	}()

	if opts.ide {
		go func() {
			cli.PrintAux("Opening local IDE...\n")

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

	for {
		select {
		case <-ctx.Done():
			return nil

		case ev := <-eventCh:
			switch ev {
			case EventChallengeReady:
				if !opts.noSSH {
					cli.PrintAux("SSH-ing into challenge playground (%s machine)...\n", opts.machine)

					sess, err = ssh.StartSSHSession(ctx, cli, chal.Play.ID, opts.machine, opts.user, nil)
					if err != nil {
						return fmt.Errorf("couldn't start SSH session: %w", err) // critical error
					}

					go func() {
						if err := sess.Wait(); err != nil {
							slog.Debug("SSH session said: " + err.Error())
						}
						eventCh <- EventSSHConnEnded
					}()
				}

			case EventChallengeCompletable:
				if _, err := cli.Client().CompleteChallenge(ctx, chal.Name); err != nil {
					slog.Debug("Error completing the challenge: " + err.Error())
					go func() {
						time.Sleep(5 * time.Second) // retry in 5 seconds without blocking the event loop
						eventCh <- EventChallengeCompletable
					}()
				} else {
					// cli.PrintAux("\033c\r") // Reset terminal
					cli.PrintAux("\r\n\r\n")
					cli.PrintAux("**********************************\r\n")
					cli.PrintAux("** Yay! Challenge completed! 🎉 **\r\n")
					cli.PrintAux("**********************************\r\n")

					if opts.keepAlive {
						cli.PrintAux("\r\n\r\n")
						cli.PrintAux("The challenge playground will be kept alive.\r\n")
						cli.PrintAux("Press ENTER to continue...\r\n")
					}

					eventCh <- EventChallengeCompleted
				}

			case EventChallengeCompleted, EventChallengeFailed:
				if chal.IsFailed() {
					// cli.PrintAux("\033c\r") // Reset terminal
					cli.PrintAux("\r\n\r\n")
					cli.PrintAux("************************************************************************\r\n")
					cli.PrintAux("** Oops... 🙈 The challenge playground has been irrecoverably broken. **\r\n")
					cli.PrintAux("************************************************************************\r\n")
				}

				if !opts.keepAlive {
					if sess != nil {
						sess.Close()
						sess = nil
						eventCh <- EventSSHConnEnded
					}

					cli.PrintAux("\r\n\r\nStopping the playground...\r\n")

					if chal, err := cli.Client().StopChallenge(ctx, chal.Name); err != nil {
						cli.PrintErr("Error stopping the challenge: %v", err)
					} else if chal.Play == nil || !chal.Play.Active {
						cli.PrintAux("Playground stopped.\r\n")
					}
				}

			case EventWSConnFailed:
				return fmt.Errorf("play connection WebSocket closed unexpectedly")

			case EventSSHConnEnded:
				cli.PrintAux("\r\n")
				return nil
			}
		}
	}
}

type PlayConnMessage struct {
	Kind    string       `json:"kind"`
	Machine string       `json:"machine,omitempty"`
	Task    api.PlayTask `json:"task,omitempty"`
}

type PlayConn struct {
	ctx    context.Context
	cancel context.CancelFunc

	cli labcli.CLI

	url  string
	conn *websocket.Conn

	msgCh chan PlayConnMessage
	errCh chan error
}

func newPlayConn(
	ctx context.Context,
	cli labcli.CLI,
	url string,
) *PlayConn {
	ctx, cancel := context.WithCancel(ctx)

	return &PlayConn{
		ctx:    ctx,
		cancel: cancel,
		cli:    cli,
		url:    url,
	}
}

func (p *PlayConn) Start() error {
	conn, _, err := websocket.DefaultDialer.DialContext(p.ctx, p.url, http.Header{
		"Origin": {p.cli.Config().WebSocketOrigin()},
	})
	if err != nil {
		return fmt.Errorf("couldn't connect to play connection WebSocket: %w", err)
	}
	p.conn = conn

	p.msgCh = make(chan PlayConnMessage, 1024)
	p.errCh = make(chan error, 1)

	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if err == io.EOF || websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}
				if websocket.IsUnexpectedCloseError(err) {
					p.cli.PrintErr("Play connection WebSocket closed unexpectedly: %v", err)
					p.errCh <- err
					return
				}

				p.cli.PrintErr("Error reading play connection message: %v", err)
				continue
			}

			var msg PlayConnMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				p.cli.PrintErr("Error decoding play connection message: %v", err)
				continue
			}

			p.msgCh <- msg
		}
	}()

	return nil
}

func (p *PlayConn) Close() {
	p.cancel()
	p.conn.Close()
	close(p.msgCh)
	close(p.errCh)
}

func (p *PlayConn) WaitChallengeReady(chal *api.Challenge) error {
	s := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	s.Writer = p.cli.AuxStream()
	s.Prefix = fmt.Sprintf(
		"Warming up playground... Init tasks completed: %d/%d ",
		chal.CountCompletedInitTasks(), chal.CountInitTasks(),
	)
	s.Start()

	ctx, cancel := context.WithTimeout(p.ctx, startChallengeTimeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-p.errCh:
			return err

		case msg := <-p.msgCh:
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

func (p *PlayConn) WaitChallengeDone(chal *api.Challenge) error {
	for {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()

		case err := <-p.errCh:
			return err

		case msg := <-p.msgCh:
			if msg.Kind == "task" {
				chal.Tasks[msg.Task.Name] = msg.Task
			}
		}

		if chal.IsCompletable() || chal.IsFailed() {
			return nil
		}
	}
}
