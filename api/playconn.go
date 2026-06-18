package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/gorilla/websocket"
)

// ErrPlayTasksFailed is returned by WaitTasks when one or more of the
// playground's (non-helper) tasks ended up in the failed state.
var ErrPlayTasksFailed = errors.New("one or more playground tasks failed")

type PlayConnMessage struct {
	Kind    string      `json:"kind"`
	Machine string      `json:"machine,omitempty"`
	Task    PlayTask    `json:"task,omitempty"`
	Status  *PlayStatus `json:"status,omitempty"`
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

	closeOnce sync.Once
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

var (
	playConnDialer = &websocket.Dialer{
		Proxy: http.ProxyFromEnvironment,

		// Smaller timeout than in the default dialer, but we'll do more attempts.
		HandshakeTimeout: 30 * time.Second,
	}
)

func (pc *PlayConn) Start() error {
	hconn, err := pc.client.RequestPlayConn(pc.ctx, pc.play.ID)
	if err != nil {
		return fmt.Errorf("couldn't create a connection to the challenge playground: %w", err)
	}

	// Retry connection with exponential backoff
	var conn *websocket.Conn
	maxRetries := 10
	baseDelay := 500 * time.Millisecond
	maxDelay := 5 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 500ms, 1s, 2s, 4s, 5s, 5s, 5s, 5s, 5s, 5s
			delay := max(maxDelay, baseDelay*time.Duration(1<<uint(attempt-1)))
			slog.Debug("Retrying WebSocket connection", "attempt", attempt+1, "delay", delay)

			select {
			case <-pc.ctx.Done():
				return fmt.Errorf("context cancelled while retrying WebSocket connection: %w", pc.ctx.Err())
			case <-time.After(delay):
			}
		}

		conn, _, err = playConnDialer.DialContext(pc.ctx, hconn.URL, http.Header{
			"Origin": {pc.origin},
		})
		if err == nil {
			break // Success!
		}

		slog.Debug("WebSocket connection attempt failed", "attempt", attempt+1, "error", err)

		// Don't retry on context cancellation/timeout
		if pc.ctx.Err() != nil {
			return fmt.Errorf("couldn't connect to play connection WebSocket: %w", err)
		}

		// If this is the last attempt, return the error
		if attempt == maxRetries-1 {
			return fmt.Errorf("couldn't connect to play connection WebSocket after %d attempts: %w", maxRetries, err)
		}
	}

	pc.conn = conn

	pc.msgCh = make(chan PlayConnMessage, 1024)
	pc.errCh = make(chan error, 1)

	go func() {
		// The reader is the sole sender on msgCh/errCh, so it owns closing them -
		// doing it here (and nowhere else) makes the close race-free even when
		// Close() is called concurrently from another goroutine. cancel() wakes
		// any waiters that were blocked on the channels.
		defer func() {
			pc.cancel()
			close(pc.msgCh)
			close(pc.errCh)
		}()

		sendErr := func(err error) {
			select {
			case pc.errCh <- err:
			case <-pc.ctx.Done():
			}
		}

		for pc.ctx.Err() == nil {
			_, message, err := pc.conn.ReadMessage()
			if err != nil {
				if err == io.EOF || websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return // terminal error
				}
				if websocket.IsUnexpectedCloseError(err) {
					sendErr(fmt.Errorf("play connection WebSocket closed unexpectedly: %w", err))
					return // terminal error
				}

				sendErr(fmt.Errorf("error reading play connection message: %w", err))
				continue // non-terminal error
			}

			var msg PlayConnMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				sendErr(fmt.Errorf("error decoding play connection message: %w", err))
				continue // non-terminal error
			}

			select {
			case pc.msgCh <- msg:
			case <-pc.ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Close stops the play connection. It cancels the connection context and closes
// the underlying websocket, which makes the reader goroutine exit and close the
// message channels. It is idempotent and safe to call concurrently with the
// reader (it never closes the channels itself).
func (pc *PlayConn) Close() {
	pc.closeOnce.Do(func() {
		pc.cancel()
		pc.conn.Close()
	})
}

// applyMessage folds an incoming play-connection message into the local play
// snapshot (task statuses and the latest machine/play status).
func (pc *PlayConn) applyMessage(msg PlayConnMessage) {
	switch msg.Kind {
	case "task":
		pc.play.Tasks[msg.Task.Name] = msg.Task
	case "status":
		if msg.Status != nil {
			pc.play.Status = msg.Status
		}
	}
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
			pc.applyMessage(msg)
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

// WaitMachinesRunning blocks until every machine of the playground has reached
// the RUNNING state, driven by live status updates received over the play
// connection. A non-positive timeout means wait indefinitely (until the
// connection's context is cancelled).
func (pc *PlayConn) WaitMachinesRunning(timeout time.Duration, s *spinner.Spinner) error {
	prefix := func() string {
		return fmt.Sprintf(
			"Waiting for machines to start... Running: %d/%d ",
			pc.play.CountRunningMachines(), pc.play.CountMachines(),
		)
	}

	if s != nil {
		s.Prefix = prefix()
		s.Start()
		defer s.Stop()
	}

	if pc.play.AllMachinesRunning() {
		if s != nil {
			s.FinalMSG = "Waiting for machines to start... Done.\n"
		}
		return nil
	}

	ctx := pc.ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(pc.ctx, timeout)
		defer cancel()
	}

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-pc.errCh:
			slog.Warn("Play connection error", "error", err.Error())

		case msg := <-pc.msgCh:
			pc.applyMessage(msg)
		}

		if s != nil {
			s.Prefix = prefix()
		}

		if pc.play.AllMachinesRunning() {
			if s != nil {
				s.FinalMSG = "Waiting for machines to start... Done.\n"
			}
			return nil
		}
	}

	return ctx.Err()
}

// WaitMachinesReady blocks until every machine of the playground is ready - i.e.
// actually reachable (its sshd accepts connections), as reported by the
// conductor's "Ready" condition. This is a stronger guarantee than merely being
// RUNNING, which doesn't imply the machine can yet be SSH-ed into. A non-positive
// timeout means wait indefinitely (until the connection's context is cancelled).
func (pc *PlayConn) WaitMachinesReady(timeout time.Duration, s *spinner.Spinner) error {
	prefix := func() string {
		return fmt.Sprintf(
			"Waiting for machines to become ready... Ready: %d/%d ",
			pc.play.CountReadyMachines(), pc.play.CountMachines(),
		)
	}

	if s != nil {
		s.Prefix = prefix()
		s.Start()
		defer s.Stop()
	}

	if pc.play.AllMachinesReady() {
		if s != nil {
			s.FinalMSG = "Waiting for machines to become ready... Done.\n"
		}
		return nil
	}

	ctx := pc.ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(pc.ctx, timeout)
		defer cancel()
	}

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-pc.errCh:
			slog.Warn("Play connection error", "error", err.Error())

		case msg := <-pc.msgCh:
			pc.applyMessage(msg)
		}

		if s != nil {
			s.Prefix = prefix()
		}

		if pc.play.AllMachinesReady() {
			if s != nil {
				s.FinalMSG = "Waiting for machines to become ready... Done.\n"
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
			pc.applyMessage(msg)
		}

		if pc.play.IsCompletable() || pc.play.IsFailed() {
			return nil
		}
	}

	return pc.ctx.Err()
}

// WaitTasks blocks until the playground's tasks reach a terminal state, driven
// by live task updates received over the play connection.
//
// When initOnly is true, it returns successfully as soon as all init tasks have
// completed; otherwise it waits for all non-helper tasks to complete. In either
// mode it returns ErrPlayTasksFailed as soon as a non-helper task fails, unless
// the wait condition has already been satisfied.
//
// A non-positive timeout means wait indefinitely (until the connection's context
// is cancelled).
func (pc *PlayConn) WaitTasks(timeout time.Duration, initOnly bool, s *spinner.Spinner) error {
	prefix := func() string {
		if initOnly {
			return fmt.Sprintf(
				"Waiting for init tasks to complete: %d/%d ",
				pc.play.CountCompletedInitTasks(), pc.play.CountInitTasks(),
			)
		}
		return fmt.Sprintf(
			"Waiting for tasks to complete: %d/%d ",
			pc.play.CountCompletedTasks(), pc.play.CountTasks(),
		)
	}

	if s != nil {
		s.Prefix = prefix()
		s.Start()
		defer s.Stop()
	}

	done := func() (bool, error) {
		if initOnly {
			if pc.play.IsInitialized() {
				return true, nil
			}
		} else if pc.play.IsCompletable() {
			return true, nil
		}
		if pc.play.HasFailedTask() {
			return true, ErrPlayTasksFailed
		}
		return false, nil
	}

	if ok, err := done(); ok {
		return err
	}

	ctx := pc.ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(pc.ctx, timeout)
		defer cancel()
	}

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-pc.errCh:
			slog.Warn("Play connection error", "error", err.Error())

		case msg := <-pc.msgCh:
			pc.applyMessage(msg)
		}

		if s != nil {
			s.Prefix = prefix()
		}

		if ok, err := done(); ok {
			return err
		}
	}

	return ctx.Err()
}
