package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type PlayJournalRequest struct {
	Machine string `json:"machine,omitempty"`
	Unit    string `json:"unit,omitempty"`
	Lines   int    `json:"lines,omitempty"`
	Since   string `json:"since,omitempty"`
	Until   string `json:"until,omitempty"`
	Cursor  string `json:"cursor,omitempty"`
}

type PlayJournalHandle struct {
	URL string `json:"url"`
}

// PlayJournalChunk is a single frame of streamed journal output.
type PlayJournalChunk struct {
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
	EOF      bool   `json:"eof,omitempty"`
}

// RequestPlayJournal asks the server for a conductor-terminated WebSocket URL
// that streams the requested machine's journal (journalctl --follow).
func (c *Client) RequestPlayJournal(ctx context.Context, id string, req PlayJournalRequest) (*PlayJournalHandle, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var handle PlayJournalHandle
	return &handle, c.PostInto(ctx, "/plays/"+id+"/journals", nil, nil, body, &handle)
}

var journalDialer = &websocket.Dialer{
	Proxy:            http.ProxyFromEnvironment,
	HandshakeTimeout: 30 * time.Second,
}

// StreamPlayJournal connects to the journal stream URL and forwards each chunk's
// stdout/stderr to the given writers until the stream ends or ctx is cancelled.
func (c *Client) StreamPlayJournal(
	ctx context.Context,
	streamURL string,
	origin string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	conn, _, err := journalDialer.DialContext(ctx, streamURL, http.Header{
		"Origin": {origin},
	})
	if err != nil {
		return fmt.Errorf("couldn't connect to the journal stream: %w", err)
	}
	defer conn.Close()

	// Unblock ReadMessage when the caller cancels (e.g., Ctrl-C).
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil ||
				errors.Is(err, io.EOF) ||
				websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return fmt.Errorf("error reading journal stream: %w", err)
		}

		var chunk PlayJournalChunk
		if err := json.Unmarshal(message, &chunk); err != nil {
			continue
		}

		if chunk.Stdout != "" {
			_, _ = io.WriteString(stdout, chunk.Stdout)
		}
		if chunk.Stderr != "" {
			_, _ = io.WriteString(stderr, chunk.Stderr)
		}
		if chunk.EOF {
			return nil
		}
	}
}
