package portforward

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"

	"github.com/iximiuz/wsmux/pkg/client"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/ssh"
)

const (
	conductorSessionCookieName = ".ixcondsess"
)

type TunnelOptions struct {
	PlayID          string
	Machine         string
	SSHUser         string
	SSHIdentityFile string

	// Out, when set, reports progress while the tunnel is being established.
	// A cold-booting machine can take minutes, and silence for that long is
	// indistinguishable from a hang.
	Out labcli.Outputer
}

type Tunnel struct {
	url   string
	token string
}

const (
	// tunnelStartTimeout bounds the wait for a tunnel. Most machines are ready
	// within seconds, but a cold-booting one can take up to ~20 minutes, and
	// giving up on it early is what makes `labctl ssh` feel broken.
	tunnelStartTimeout = 25 * time.Minute

	// authenticateTimeout bounds the token exchange that follows. The tunnel
	// already exists at that point, so anything beyond a transient blip here is
	// a real failure, not a slow boot.
	authenticateTimeout = 1 * time.Minute

	// The server holds the request while the machine boots, so progress has to
	// be reported on a timer rather than per attempt - otherwise the first sign
	// of life would come a full server-side wait in.
	progressAfter = 10 * time.Second
	progressEvery = 30 * time.Second
)

func StartTunnel(ctx context.Context, client *api.Client, opts TunnelOptions) (*Tunnel, error) {
	var (
		sshPubKey string
		err       error
	)
	if opts.SSHIdentityFile != "" {
		sshPubKey, err = ssh.ReadPublicKey(opts.SSHIdentityFile)
		if err != nil {
			return nil, fmt.Errorf("ssh.ReadPublicKey(): %w", err)
		}
	}

	stopProgress := opts.reportProgress(ctx)

	// The tunnel endpoint itself absorbs the wait: it holds the request while
	// the machine comes up and only answers 503 once its own budget runs out,
	// so the common case (a machine ready within seconds) costs exactly one
	// request and no control-plane traffic. Only the slow-boot tail retries,
	// which keeps a 20-minute boot at tens of requests rather than the hundreds
	// a fixed interval would cost.
	resp, err := backoff.Retry(ctx, func() (*api.StartTunnelResponse, error) {
		resp, err := client.StartTunnel(ctx, opts.PlayID, api.StartTunnelRequest{
			Machine:          opts.Machine,
			Access:           api.PortAccessPrivate,
			GenerateLoginURL: true,
			SSHUser:          opts.SSHUser,
			SSHPubKey:        sshPubKey,
		})
		if err != nil && !retryableTunnelError(err) {
			return nil, backoff.Permanent(err)
		}

		return resp, err
	}, retryOptions(tunnelStartTimeout)...)

	if waited := stopProgress(); waited && err == nil {
		opts.Out.PrintAux("Machine is ready.\n")
	}
	if err != nil {
		return nil, fmt.Errorf("client.StartTunnel(): %w", err)
	}

	token, err := backoff.Retry(ctx, func() (string, error) {
		return authenticate(ctx, resp.LoginURL, conductorSessionCookieName)
	}, retryOptions(authenticateTimeout)...)
	if err != nil {
		return nil, fmt.Errorf("authenticate(): %w", err)
	}

	return &Tunnel{
		url:   resp.URL,
		token: token,
	}, nil
}

// reportProgress starts printing "still waiting" lines once the tunnel has been
// pending for progressAfter, and keeps them coming every progressEvery. The
// returned stop function ends the reporting and says whether anything was
// printed, so the caller can close the sequence off. Reporting on a timer (not
// per attempt) is what keeps the feedback honest: the server holds each request
// for up to a minute, so attempts are far too coarse to show liveness.
func (o TunnelOptions) reportProgress(ctx context.Context) (stop func() (printed bool)) {
	if o.Out == nil {
		return func() bool { return false }
	}

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	printed := false

	go func() {
		defer close(done)

		t0 := time.Now()
		timer := time.NewTimer(progressAfter)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-timer.C:
				o.Out.PrintAux("Waiting for %s to become ready... (%s)\n",
					o.machineLabel(), time.Since(t0).Round(time.Second))
				printed = true
				timer.Reset(progressEvery)
			}
		}
	}()

	return func() bool {
		cancel()
		<-done // the goroutine owns `printed` until it returns
		return printed
	}
}

// retryableTunnelError reports whether a failed tunnel start is worth another
// attempt. Only "not yet" answers are: the machine is still coming up, the hop
// in front of it timed out, or we're being rate limited. Everything else - a
// play that's gone, a machine whose agents have had their full budget and aren't
// coming up, a rejected request - is the server's final answer, and the client
// must stop on it. The api package classifies these once, but flattens them
// back to plain errors on the way out, so the decision has to be re-made here.
func retryableTunnelError(err error) bool {
	var retryAfter *backoff.RetryAfterError

	return errors.Is(err, api.ErrServiceUnavailable) ||
		errors.Is(err, api.ErrGatewayTimeout) ||
		errors.Is(err, api.ErrRateLimitExceeded) ||
		errors.As(err, &retryAfter)
}

// retryOptions is the single retry policy behind tunnel setup. The server
// already spends most of a minute waiting before it answers, so these intervals
// are only the gap on top of that - keeping them short costs nothing and adds
// no idle time to a boot that's about to finish.
func retryOptions(maxElapsed time.Duration) []backoff.RetryOption {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Second
	b.MaxInterval = 5 * time.Second
	b.RandomizationFactor = 0.3

	return []backoff.RetryOption{
		backoff.WithBackOff(b),
		backoff.WithMaxElapsedTime(maxElapsed),
	}
}

// machineLabel names the machine being tunneled into for progress messages. An
// empty name means the server picks the play's first machine, which the client
// can't name yet without asking for it.
func (o TunnelOptions) machineLabel() string {
	if o.Machine == "" {
		return "the machine"
	}
	return "machine " + o.Machine
}

func (t *Tunnel) Forward(ctx context.Context, spec ForwardingSpec, errCh chan error) error {
	wsUrl := "wss://" + strings.Split(t.url, "://")[1]
	cookie := conductorSessionCookieName + "=" + t.token

	if spec.Kind == "remote" {
		c := client.NewReverseClient(ctx, spec.RemoteAddr(), spec.LocalAddr(), wsUrl, errCh)
		c.SetHeader("Cookie", cookie)
		return c.ListenAndServe()
	}

	c := t.newLocalClient(ctx, spec, errCh)
	return c.ListenAndServe()
}

func (t *Tunnel) newLocalClient(ctx context.Context, spec ForwardingSpec, errCh chan error) *client.Client {
	wsUrl := "wss://" + strings.Split(t.url, "://")[1]

	c := client.NewClient(ctx, spec.LocalAddr(), spec.RemoteAddr(), wsUrl, errCh)
	c.SetHeader("Cookie", conductorSessionCookieName+"="+t.token)

	return c
}

// ListenAndForward binds the local side of a "local" forwarding spec
// synchronously and only then starts serving in the background. Unlike
// StartForwarding, a failure to bind the local port is reported immediately
// instead of surfacing later as connection-refused dials against a port
// nobody listens on. spec.LocalPort may be "0" to let the kernel pick a
// guaranteed-free port - the actually bound address is returned. The done
// channel receives the terminal result (nil or error) when forwarding stops.
func (t *Tunnel) ListenAndForward(ctx context.Context, spec ForwardingSpec) (net.Addr, <-chan error, error) {
	if spec.Kind == "remote" {
		return nil, nil, fmt.Errorf("ListenAndForward supports only local forwarding specs")
	}

	errCh := make(chan error, 100)
	c := t.newLocalClient(ctx, spec, errCh)

	if err := c.Listen(); err != nil {
		return nil, nil, err
	}

	doneCh := make(chan error, 1)

	go func() {
		doneCh <- c.Serve()
		close(doneCh)
	}()

	go drainForwardingErrors(ctx, errCh)

	return c.Addr(), doneCh, nil
}

// StartForwarding starts port forwarding in the background and logs transient
// errors. It returns a channel that receives the final result (nil or error)
// when the forwarding stops.
func (t *Tunnel) StartForwarding(ctx context.Context, spec ForwardingSpec) <-chan error {
	errCh := make(chan error, 100)
	doneCh := make(chan error, 1)

	go func() {
		doneCh <- t.Forward(ctx, spec, errCh)
		close(doneCh)
	}()

	go drainForwardingErrors(ctx, errCh)

	return doneCh
}

// drainForwardingErrors logs per-connection forwarding errors until the
// context is done or the channel is closed.
func drainForwardingErrors(ctx context.Context, errCh <-chan error) {
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil {
				slog.Debug("Tunnel forwarding error", "error", err.Error())
			}
		}
	}
}

func authenticate(ctx context.Context, url string, name string) (string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()

	for _, cookie := range httpResp.Cookies() {
		if cookie.Name == name {
			return cookie.Value, nil
		}
	}

	return "", fmt.Errorf("session cookie not found: %s", name)
}
