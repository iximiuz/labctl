package portforward

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iximiuz/wsmux/pkg/client"

	"github.com/iximiuz/labctl/internal/api"
)

const (
	conductorSessionCookieName = ".ixcondsess"
)

type TunnelOptions struct {
	PlayID  string
	Machine string
}

type Tunnel struct {
	url    string
	cookie string
}

func StartTunnel(ctx context.Context, client *api.Client, opts TunnelOptions) (*Tunnel, error) {
	resp, err := client.StartTunnel(ctx, opts.PlayID, api.StartTunnelRequest{
		Machine:          opts.Machine,
		Access:           api.PortAccessPrivate,
		GenerateLoginURL: true,
	})
	if err != nil {
		return nil, fmt.Errorf("client.StartTunnel(): %w", err)
	}

	var cookie string
	if err := retry(ctx, func() error {
		cookie, err = authenticate(ctx, resp.LoginURL, conductorSessionCookieName)
		return err
	}, 10, 1000); err != nil {
		return nil, fmt.Errorf("authenticate(): %w", err)
	}

	return &Tunnel{
		url:    resp.URL,
		cookie: cookie,
	}, nil
}

func (t *Tunnel) Forward(ctx context.Context, spec ForwardingSpec, errCh chan error) error {
	wsUrl := "wss://" + strings.Split(t.url, "://")[1]

	wsmux := client.NewClient(ctx, spec.LocalAddr(), spec.RemoteAddr(), wsUrl, errCh)
	wsmux.SetHeader("Cookie", conductorSessionCookieName+"="+t.cookie)

	return wsmux.ListenAndServe()
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

func retry(ctx context.Context, f func() error, retries int, delay int) error {
	var err error
	for i := 0; i < retries; i++ {
		err = f()
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return err

		case <-time.After(time.Duration(delay) * time.Millisecond):
		}
	}
	return err
}
