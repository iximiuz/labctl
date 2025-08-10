package labcli

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/docker/cli/cli/streams"
	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/config"
)

type Streams interface {
	InputStream() *streams.In
	OutputStream() *streams.Out
	AuxStream() *streams.Out // ErrorStream unless quiet else io.Discard
	ErrorStream() io.Writer
}

type CLI interface {
	Streams

	SetQuiet(bool)

	// Regular print to stdout.
	PrintOut(string, ...any)

	// Regular print to stderr.
	PrintErr(string, ...any)

	// Print to stderr unless quiet else - discard.
	PrintAux(string, ...any)

	SetConfig(*config.Config)

	Config() *config.Config

	SetClient(*api.Client)

	Client() *api.Client

	Confirm(title, affirmative, negative string) bool

	Input(title, prompt string, value *string, validate func(string) error) error

	Version() string
}

type cli struct {
	inputStream  *streams.In
	outputStream *streams.Out
	auxStream    *streams.Out
	errorStream  io.Writer

	config *config.Config
	client *api.Client

	version string
}

var _ CLI = &cli{}

func NewCLI(cin io.ReadCloser, cout io.Writer, cerr io.Writer, version string) CLI {
	return &cli{
		inputStream:  streams.NewIn(cin),
		outputStream: streams.NewOut(cout),
		auxStream:    streams.NewOut(cerr),
		errorStream:  cerr,
		version:      version,
	}
}

func (c *cli) SetConfig(cfg *config.Config) {
	c.config = cfg
}

func (c *cli) Config() *config.Config {
	return c.config
}

func (c *cli) SetClient(client *api.Client) {
	c.client = client
}

func (c *cli) Client() *api.Client {
	return c.client
}

func (c *cli) InputStream() *streams.In {
	return c.inputStream
}

func (c *cli) OutputStream() *streams.Out {
	return c.outputStream
}

func (c *cli) AuxStream() *streams.Out {
	return c.auxStream
}

func (c *cli) ErrorStream() io.Writer {
	return c.errorStream
}

func (c *cli) SetQuiet(v bool) {
	if v {
		c.auxStream = streams.NewOut(io.Discard)
	} else {
		c.auxStream = streams.NewOut(c.errorStream)
	}
}

func (c *cli) PrintOut(format string, a ...any) {
	fmt.Fprintf(c.OutputStream(), format, a...)
}

func (c *cli) PrintErr(format string, a ...any) {
	fmt.Fprintf(c.ErrorStream(), format, a...)
}

func (c *cli) PrintAux(format string, a ...any) {
	fmt.Fprintf(c.AuxStream(), format, a...)
}

func (c *cli) Confirm(title, affirmative, negative string) bool {
	var confirm bool

	if err := huh.NewConfirm().Title(title).Affirmative(affirmative).Negative(negative).Value(&confirm).Run(); err != nil {
		slog.Warn("Confirmation prompt failed", "error", err.Error())
		return false
	}

	return confirm
}

func (c *cli) Input(
	title string,
	prompt string,
	value *string,
	validate func(string) error,
) error {
	return huh.NewInput().Title(title).Prompt(prompt).Validate(validate).Value(value).Run()
}

func (c *cli) Version() string {
	return c.version
}

type StatusError struct {
	status string
	code   int
}

var _ error = StatusError{}

func NewStatusError(code int, format string, a ...any) StatusError {
	status := fmt.Sprintf(format, a...)
	if !strings.HasSuffix(status, ".") && !strings.HasSuffix(status, "!") {
		status += "."
	}
	return StatusError{
		code:   code,
		status: strings.ToUpper(status[:1]) + status[1:],
	}
}

type ExitError interface {
	ExitStatus() int
}

func WrapStatusError(err error) error {
	if err == nil {
		return nil
	}

	if errors.As(err, new(StatusError)) {
		return err
	}

	var exitErr ExitError
	if errors.As(err, &exitErr) {
		return NewStatusError(exitErr.ExitStatus(), err.Error())
	}

	return NewStatusError(1, err.Error())
}

func (e StatusError) Error() string {
	return e.status
}

func (e StatusError) Code() int {
	return e.code
}
