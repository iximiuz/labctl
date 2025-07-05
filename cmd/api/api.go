package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

type apiOptions struct {
	// Future options can be added here
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts apiOptions

	cmd := &cobra.Command{
		Use:   "api <path>",
		Short: "Send requests to iximiuz Labs API",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runAPIRequest(cmd.Context(), cli, args[0], &opts))
		},
	}

	// Future flags can be added here
	// flags := cmd.Flags()

	return cmd
}

func runAPIRequest(ctx context.Context, cli labcli.CLI, path string, opts *apiOptions) error {
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	resp, err := cli.Client().Get(ctx, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	// Copy response body to stdout
	_, err = io.Copy(cli.OutputStream(), resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	// Check if response is successful and return error if not
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return nil
}
