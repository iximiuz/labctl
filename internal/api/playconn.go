package api

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
)

type PlayConnMessage struct {
	Kind    string   `json:"kind"`
	Machine string   `json:"machine,omitempty"`
	Task    PlayTask `json:"task,omitempty"`
}

type PlayConn struct {
	ctx    context.Context
	cancel context.CancelFunc

	play *Play

	client *Client
	origin string

	conn *websocket.Conn

	msgCh chan PlayConnMessage
	errCh chan error
}

func NewPlayConn(
	ctx context.Context,
	play *Play,
	client *Client,
	origin string,
) *PlayConn {
	ctx, cancel := context.WithCancel(ctx)

	return &PlayConn{
		ctx:    ctx,
		cancel: cancel,
		play:   play,
		client: client,
		origin: origin,
	}
}

func (pc *PlayConn) Start() error {
	hconn, err := pc.client.RequestPlayConn(pc.ctx, pc.play.ID)
	if err != nil {
		return fmt.Errorf("couldn't create a connection to the challenge playground: %w", err)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(pc.ctx, hconn.URL, http.Header{
		"Origin": {pc.origin},
	})
	if err != nil {
		return fmt.Errorf("couldn't connect to play connection WebSocket: %w", err)
	}
	pc.conn = conn

	pc.msgCh = make(chan PlayConnMessage, 1024)
	pc.errCh = make(chan error, 1)

	go func() {
		defer pc.Close()

		for pc.ctx.Err() == nil {
			_, message, err := pc.conn.ReadMessage()
			if err != nil {
				if err == io.EOF || websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return // terminal error
				}
				if websocket.IsUnexpectedCloseError(err) {
					pc.errCh <- fmt.Errorf("play connection WebSocket closed unexpectedly: %w", err)
					return // terminal error
				}

				pc.errCh <- fmt.Errorf("error reading play connection message: %w", err)
				continue // non-terminal error
			}

			var msg PlayConnMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				pc.errCh <- fmt.Errorf("error decoding play connection message: %w", err)
				continue // non-terminal error
			}

			pc.msgCh <- msg
		}
	}()

	return nil
}

func (pc *PlayConn) Close() {
	pc.cancel()
	pc.conn.Close()
	close(pc.msgCh)
	close(pc.errCh)
}

func (pc *PlayConn) WaitPlayReady(timeout time.Duration, s *spinner.Spinner) error {
	if s != nil {
		s.Prefix = fmt.Sprintf(
			"Warming up playground... Init tasks completed: %d/%d ",
			pc.play.CountCompletedInitTasks(), pc.play.CountInitTasks(),
		)
		s.Start()
		defer s.Stop()
	}

	ctx, cancel := context.WithTimeout(pc.ctx, timeout)
	defer cancel()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-pc.errCh:
			slog.Warn("Play connection error", "error", err.Error())

		case msg := <-pc.msgCh:
			if msg.Kind == "task" {
				pc.play.Tasks[msg.Task.Name] = msg.Task
			}
		}

		if s != nil {
			s.Prefix = fmt.Sprintf(
				"Warming up playground... Init tasks completed: %d/%d ",
				pc.play.CountCompletedInitTasks(), pc.play.CountInitTasks(),
			)
		}

		if pc.play.IsInitialized() {
			if s != nil {
				s.FinalMSG = "Warming up playground... Done.\n"
			}
			return nil
		}
	}

	return ctx.Err()
}

func (pc *PlayConn) WaitDone() error {
	for pc.ctx.Err() == nil {
		select {
		case <-pc.ctx.Done():
			return pc.ctx.Err()

		case err := <-pc.errCh:
			slog.Warn("Play connection error", "error", err.Error())

		case msg := <-pc.msgCh:
			if msg.Kind == "task" {
				pc.play.Tasks[msg.Task.Name] = msg.Task
			}
		}

		if pc.play.IsCompletable() || pc.play.IsFailed() {
			return nil
		}
	}

	return pc.ctx.Err()
}
