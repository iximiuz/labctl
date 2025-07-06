package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

const example = `  # Get user information
  labctl api /auth/me

  # Start a tutorial
  echo '{"started": true}' | labctl api /tutorials/NAME --method PATCH --input -`

type apiOptions struct {
	method string
	input  string
	silent bool
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts apiOptions

	cmd := &cobra.Command{
		Use:     "api <path>",
		Short:   "Send requests to iximiuz Labs API",
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runAPIRequest(cmd.Context(), cli, args[0], &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(
		&opts.method,
		"method",
		"X",
		"GET",
		"HTTP method for the request",
	)
	flags.StringVar(
		&opts.input,
		"input",
		"",
		`File to use as body for the HTTP request (use "-" to read from standard input)`,
	)
	flags.BoolVarP(
		&opts.silent,
		"silent",
		"s",
		false,
		"Do not print the response body",
	)

	return cmd
}

func runAPIRequest(ctx context.Context, cli labcli.CLI, path string, opts *apiOptions) error {
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Handle method and input logic
	method := strings.ToUpper(opts.method)
	var body io.Reader

	// If input is specified, read from file or stdin
	if opts.input != "" {
		if opts.input == "-" {
			// Read from stdin
			body = cli.InputStream()
		} else {
			// Read from file
			file, err := os.Open(opts.input)
			if err != nil {
				return fmt.Errorf("failed to open input file: %w", err)
			}
			defer file.Close()
			body = file
		}

		// If method is GET but input is provided, change to POST
		if method == "GET" {
			method = "POST"
		}
	}

	// Make the HTTP request
	var resp *http.Response
	var err error

	switch method {
	case "GET":
		resp, err = cli.Client().Get(ctx, path, nil, nil)
	case "POST":
		resp, err = cli.Client().Post(ctx, path, nil, nil, body)
	case "PUT":
		resp, err = cli.Client().Put(ctx, path, nil, nil, body)
	case "PATCH":
		resp, err = cli.Client().Patch(ctx, path, nil, nil, body)
	case "DELETE":
		resp, err = cli.Client().Delete(ctx, path, nil, nil)
	default:
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	if err != nil {
		return fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	// Check if response is successful and return error if not
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Copy response body to stdout unless silent flag is set
	if !opts.silent {
		_, err = io.Copy(cli.OutputStream(), resp.Body)
		if err != nil {
			return fmt.Errorf("failed to write response: %w", err)
		}
	}

	return nil
}
