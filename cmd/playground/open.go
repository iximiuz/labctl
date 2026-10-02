package playground

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/browser"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

type openOptions struct {
	playID string

	quiet bool
}

func newOpenCommand(cli labcli.CLI) *cobra.Command {
	var opts openOptions

	cmd := &cobra.Command{
		Use:               "open [flags] <playground-id|title>",
		Short:             `Open a playground run (running or stopped) in a browser and print its URL`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.NonDestroyedPlaysAndTitles(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.playID = args[0]

			return labcli.WrapStatusError(runOpenPlayground(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Only print the playground URL`,
	)

	return cmd
}

func runOpenPlayground(ctx context.Context, cli labcli.CLI, opts *openOptions) error {
	play, err := resolvePlay(ctx, cli, opts.playID)
	if err != nil {
		return err
	}

	if play.PageURL == "" {
		return fmt.Errorf("playground %s has no page URL", play.ID)
	}

	// Prints a copy-friendly warning with the URL if the browser can't be opened.
	browser.OpenWithFallbackMessage(cli, play.PageURL)

	cli.PrintOut("%s\n", play.PageURL)

	return nil
}

// resolvePlay fetches a play by its ID or, if the identifier doesn't look like
// a play ID, by a (prefix of its) title. Title lookup covers both running and
// stopped playgrounds but ignores destroyed ones.
func resolvePlay(ctx context.Context, cli labcli.CLI, idOrTitle string) (*api.Play, error) {
	if api.LooksLikePlayID(idOrTitle) {
		play, err := cli.Client().GetPlay(ctx, idOrTitle)
		if err != nil {
			return nil, fmt.Errorf("couldn't get playground: %w", err)
		}
		return play, nil
	}

	cli.PrintAux("Searching for a playground with title: %s\n", idOrTitle)

	plays, err := cli.Client().ListAllPlays(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get a list of playgrounds: %w", err)
	}

	var candidates []*api.Play
	for _, p := range plays {
		if !p.StateIs(api.StateDestroyed) {
			candidates = append(candidates, p)
		}
	}

	return findPlayByTitle(candidates, idOrTitle)
}
