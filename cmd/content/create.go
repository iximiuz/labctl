package content

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

type createOptions struct {
	kind content.ContentKind
	name string

	dirOptions

	// noSample bool
}

func newCreateCommand(cli labcli.CLI) *cobra.Command {
	var opts createOptions

	cmd := &cobra.Command{
		Use:   "create [flags] <challenge|tutorial|course> <name>",
		Short: "Create a new piece of content (visible only to the author)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.kind.Set(args[0]); err != nil {
				return labcli.WrapStatusError(err)
			}
			opts.name = args[1]

			return labcli.WrapStatusError(runCreateContent(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	// flags.BoolVar(
	// 	&opts.noSample,
	// 	"no-sample",
	// 	false,
	// 	`Don't create a sample piece of content`,
	// )

	opts.AddDirFlag(flags)

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

	cli.PrintAux("Creating a new %s...\n", opts.kind)

	var cont content.Content

	switch opts.kind {
	case content.KindChallenge:
		cont, err = createChallenge(ctx, cli, opts)

	case content.KindTutorial:
		cont, err = createTutorial(ctx, cli, opts)

	case content.KindCourse:
		cont, err = createCourse(ctx, cli, opts)
	}

	if err != nil {
		return err
	}

	cli.PrintAux("Created a new %s %s\n", cont.GetKind(), cont.GetPageURL())
	if err := open.Run(cont.GetPageURL()); err != nil {
		cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually.\n")
	}

	dir, err := opts.ContentDir(cont)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dir); err == nil {
		cli.PrintErr("WARNING: Directory %s already exists and not empty.\n", dir)
		cli.PrintErr("Skipping pulling the sample content files to avoid\noverwriting existing local files\n.")
		cli.PrintErr("Use `labctl pull %s %s --dir <some-other-dir>` to\npull the sample content files manually.\n")
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("couldn't create directory %s: %w", dir, err)
	}

	files, err := cli.Client().ListContentFiles(ctx, cont.GetKind(), cont.GetName())
	if err != nil {
		return fmt.Errorf("couldn't list content files: %w", err)
	}

	for _, file := range files {
		// if err := cli.Client().DownloadContentFile(ctx, file, dir); err != nil {
		// 	return fmt.Errorf("couldn't download content file %s: %w", file, err)
		// }
		fmt.Printf("Downloading %s\n", file)
	}

	cli.PrintAux("Happy authoring!\n")
	return nil
}

func createChallenge(ctx context.Context, cli labcli.CLI, opts *createOptions) (content.Content, error) {
	ch, err := cli.Client().CreateChallenge(ctx, api.CreateChallengeRequest{
		Name: opts.name,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create challenge: %w", err)
	}

	return ch, nil
}

func createTutorial(ctx context.Context, cli labcli.CLI, opts *createOptions) (content.Content, error) {
	t, err := cli.Client().CreateTutorial(ctx, api.CreateTutorialRequest{
		Name: opts.name,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create tutorial: %w", err)
	}

	return t, nil
}

func createCourse(ctx context.Context, cli labcli.CLI, opts *createOptions) (content.Content, error) {
	c, err := cli.Client().CreateCourse(ctx, api.CreateCourseRequest{
		Name:    opts.name,
		Variant: api.CourseVariantModular,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create course: %w", err)
	}

	return c, nil
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
	if !cli.Confirm(
		"You don't have an author profile yet. Would you like to create one now?",
		"Yes", "No",
	) {
		return labcli.NewStatusError(0, "See you later!")
	}

	cli.PrintAux("Creating an author profile...\n")

	displayName := "John Doe"
	if err := cli.Input("Please enter your full name:", "?", &displayName, func(v string) error {
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
	}); err != nil {
		return err
	}

	var profileURL string
	if err := cli.Input("Please enter your X, LinkedIn, or other public profile URL:", "?", &profileURL, func(v string) error {
		parsed, err := url.Parse(v)
		if err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		if parsed.Host == "" {
			return fmt.Errorf("invalid URL: hostname is required")
		}
		return nil
	}); err != nil {
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
