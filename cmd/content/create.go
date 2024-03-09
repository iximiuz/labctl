package content

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type ContentKind string

const (
	KindChallenge ContentKind = "challenge"
	KindTutorial  ContentKind = "tutorial"
	KindCourse    ContentKind = "course"
)

func (k *ContentKind) UnmarshalText(text []byte) error {
	switch string(text) {
	case "challenge":
		*k = KindChallenge
	case "tutorial":
		*k = KindTutorial
	case "course":
		*k = KindCourse
	default:
		return fmt.Errorf("unknown content kind: %s", text)
	}

	return nil
}

type createOptions struct {
	kind ContentKind
	name string

	dir string

	noSample bool
}

func newCreateCommand(cli labcli.CLI) *cobra.Command {
	var opts createOptions

	cmd := &cobra.Command{
		Use:   "create [flags] <challenge|tutorial|course> <name>",
		Short: "Create a new piece of content (visible only to the author)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.kind.UnmarshalText([]byte(args[0])); err != nil {
				return labcli.WrapStatusError(err)
			}
			opts.name = args[1]

			if opts.dir == "" {
				if cwd, err := os.Getwd(); err != nil {
					return labcli.WrapStatusError(fmt.Errorf("couldn't get the current working directory: %w", err))
				} else {
					opts.dir = filepath.Join(cwd, opts.name)
				}
			}

			return labcli.WrapStatusError(runCreateContent(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(
		&opts.noSample,
		"no-sample",
		false,
		`Don't create a sample piece of content`,
	)
	flags.StringVar(
		&opts.dir,
		"dir",
		"",
		`Local directory to create the content in (default: current working directory)`,
	)

	return cmd
}

func runCreateContent(ctx context.Context, cli labcli.CLI, opts *createOptions) error {
	cli.PrintAux("Checking if the current user has an author profile...\n")

	hasAuthor, err := hasAuthorProfile(ctx, cli)
	if err != nil {
		return err
	}

	if !hasAuthor {
		if err := maybeCreateAuthorProfile(ctx, cli); err != nil {
			return err
		}
	}

	cli.PrintAux("Creating a new %s in %s...\n", opts.kind, opts.dir)

	if _, err := os.Stat(opts.dir); err == nil {
		return fmt.Errorf("directory %s already exists - aborting to avoid overwriting existing files", opts.dir)
	}

	if err := os.MkdirAll(opts.dir, 0755); err != nil {
		return fmt.Errorf("couldn't create directory %s: %w", opts.dir, err)
	}

	switch opts.kind {
	case KindChallenge:
		if err := createChallenge(ctx, cli, opts); err != nil {
			return err
		}

	case KindTutorial:
		if err := createTutorial(ctx, cli, opts); err != nil {
			return err
		}

	case KindCourse:
		if err := createCourse(ctx, cli, opts); err != nil {
			return err
		}
	}

	return nil
}

func createChallenge(ctx context.Context, cli labcli.CLI, opts *createOptions) error {
	// ch, err := cli.Client().CreateChallenge(ctx, labcli.CreateChallengeRequest{
	// 	Name: opts.name,
	// })
	// if err != nil {
	// 	return fmt.Errorf("couldn't create challenge: %w", err)
	// }

	return nil
}

func createTutorial(ctx context.Context, cli labcli.CLI, opts *createOptions) error {
	return nil
}

func createCourse(ctx context.Context, cli labcli.CLI, opts *createOptions) error {
	return nil
}

func hasAuthorProfile(ctx context.Context, cli labcli.CLI) (bool, error) {
	me, err := cli.Client().GetMe(ctx)
	if err != nil {
		return false, fmt.Errorf("couldn't get the current user: %w", err)
	}

	authors, err := cli.Client().ListAuthors(ctx, api.ListAuthorsFilter{
		UserID: []string{me.ID},
	})
	if err != nil {
		return false, fmt.Errorf("couldn't list author profiles: %w", err)
	}

	return len(authors) > 0, nil
}

func maybeCreateAuthorProfile(ctx context.Context, cli labcli.CLI) error {
	confirm := true
	if err := huh.NewConfirm().
		Title("You don't have an author profile yet. Would you like to create one now?").
		Affirmative("Yes!").
		Negative("No.").
		Value(&confirm).
		Run(); err != nil {
		return err
	}

	if !confirm {
		return labcli.NewStatusError(0, "See you later!")
	}

	cli.PrintAux("Creating an author profile...\n")

	displayName := "John Doe"
	if err := huh.NewInput().
		Title("Please enter your full name:").
		Prompt("?").
		Validate(func(v string) error {
			if v == "" {
				return fmt.Errorf("display name cannot be empty")
			}
			if !strings.Contains(v, " ") {
				return fmt.Errorf("display name must contain at least two words")
			}
			if len(v) < 5 {
				return fmt.Errorf("display name is too short")
			}
			if len(v) > 24 {
				return fmt.Errorf("display name is too long")
			}
			return nil
		}).
		Value(&displayName).
		Run(); err != nil {
		return err
	}

	var profileURL string
	if err := huh.NewInput().
		Title("Please enter your X, LinkedIn, or other public profile URL:").
		Prompt("?").
		Validate(func(v string) error {
			parsed, err := url.Parse(v)
			if err != nil {
				return fmt.Errorf("invalid URL: %w", err)
			}
			if parsed.Host == "" {
				return fmt.Errorf("invalid URL: hostname is required")
			}
			return nil
		}).
		Value(&profileURL).
		Run(); err != nil {
		return err
	}

	if _, err := cli.Client().CreateAuthor(ctx, api.CreateAuthorRequest{
		Author: api.Author{
			DisplayName:        displayName,
			ExternalProfileURL: profileURL,
		},
	}); err != nil {
		return fmt.Errorf("couldn't create an author profile: %w", err)
	}

	return nil
}
