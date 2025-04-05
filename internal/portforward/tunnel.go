package portforward

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iximiuz/wsmux/pkg/client"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/retry"
	"github.com/iximiuz/labctl/internal/ssh"
)

const (
	conductorSessionCookieName = ".ixcondsess"
)

type TunnelOptions struct {
	PlayID          string
	Machine         string
	PlaysDir        string
	SSHUser         string
	SSHIdentityFile string
}

type Tunnel struct {
	url   string
	token string
}

func StartTunnel(ctx context.Context, client *api.Client, opts TunnelOptions) (*Tunnel, error) {
	uniq := opts.PlayID + "-" + opts.Machine
	if opts.SSHUser != "" {
		uniq += "-" + opts.SSHUser
	}
	tunnelFile := filepath.Join(opts.PlaysDir, uniq, "tunnel.json")
	if t, err := loadTunnel(tunnelFile); err == nil {
		return t, nil
	}

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

	resp, err := client.StartTunnel(ctx, opts.PlayID, api.StartTunnelRequest{
		Machine:          opts.Machine,
		Access:           api.PortAccessPrivate,
		GenerateLoginURL: true,
		SSHUser:          opts.SSHUser,
		SSHPubKey:        sshPubKey,
	})
	if err != nil {
		return nil, fmt.Errorf("client.StartTunnel(): %w", err)
	}

	var token string
	if err := retry.UntilSuccess(ctx, func() error {
		token, err = authenticate(ctx, resp.LoginURL, conductorSessionCookieName)
		return err
	}, 10, 1*time.Second); err != nil {
		return nil, fmt.Errorf("authenticate(): %w", err)
	}

	t := &Tunnel{
		url:   resp.URL,
		token: token,
	}

	if err := saveTunnel(tunnelFile, t); err != nil {
		slog.Warn("Couldn't save tunnel info to file", "error", err.Error())
	}

	return t, nil
}

func (t *Tunnel) Forward(ctx context.Context, spec ForwardingSpec, errCh chan error) error {
	wsUrl := "wss://" + strings.Split(t.url, "://")[1]

	wsmux := client.NewClient(ctx, spec.LocalAddr(), spec.RemoteAddr(), wsUrl, errCh)
	wsmux.SetHeader("Cookie", conductorSessionCookieName+"="+t.token)

	return wsmux.ListenAndServe()
}

func (t *Tunnel) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"url":   t.url,
		"token": t.token,
	})
}

func (t *Tunnel) UnmarshalJSON(data []byte) error {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	t.url = m["url"]
	t.token = m["token"]
	return nil
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

func loadTunnel(file string) (*Tunnel, error) {
	bytes, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var t Tunnel
	if err := json.Unmarshal(bytes, &t); err != nil {
		return nil, err
	}

	return &t, nil
}

func saveTunnel(file string, t *Tunnel) error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}

	bytes, err := json.Marshal(t)
	if err != nil {
		return err
	}

	return os.WriteFile(file, bytes, 0644)
}
