package content

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

type createOptions struct {
	kind content.ContentKind
	name string

	DirOptions
}

func newCreateCommand(cli labcli.CLI) *cobra.Command {
	var opts createOptions

	cmd := &cobra.Command{
		Use:   "create [flags] <challenge|tutorial|skill-path|course|training> <name>",
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

	opts.AddDirFlag(flags, "Local directory with content files (default: $CWD/<content-name>)")

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

	case content.KindCourse:
		cont, err = createCourse(ctx, cli, opts)

	case content.KindTutorial:
		cont, err = createTutorial(ctx, cli, opts)

	case content.KindRoadmap:
		cont, err = createRoadmap(ctx, cli, opts)

	case content.KindSkillPath:
		cont, err = createSkillPath(ctx, cli, opts)

	case content.KindTraining:
		cont, err = createTraining(ctx, cli, opts)
	}

	if err != nil {
		return err
	}

	cli.PrintAux("Created a new %s %s\n", cont.GetKind(), cont.GetPageURL())
	if err := open.Run(cont.GetPageURL()); err != nil {
		cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually.\n")
	}

	dir, err := opts.ContentDir(cont.GetName())
	if err != nil {
		return err
	}

	if _, err := os.Stat(dir); err == nil {
		cli.PrintErr("WARNING: Directory %s already exists and not empty.\n", dir)
		cli.PrintErr("Skipping pulling sample content to avoid overwriting existing local files.\n")
		cli.PrintErr("Use `labctl pull %s %s --dir <some-other-dir>`\nto pull the sample content files manually.\n", cont.GetKind(), cont.GetName())
		return nil
	}

	cli.PrintAux("Preparing the working directory %s...\n", dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("couldn't create directory %s: %w", dir, err)
	}

	files, err := cli.Client().ListContentFiles(ctx, cont.GetKind(), cont.GetName())
	if err != nil {
		return fmt.Errorf("couldn't list content files: %w", err)
	}

	cli.PrintAux("Pulling the sample content files...\n")
	for _, file := range files {
		cli.PrintAux("Downloading %s\n", file)

		if err := cli.Client().DownloadContentFile(
			ctx,
			cont.GetKind(),
			cont.GetName(),
			file,
			filepath.Join(dir, file),
		); err != nil {
			return fmt.Errorf("couldn't download content file %s: %w", file, err)
		}
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

func createRoadmap(ctx context.Context, cli labcli.CLI, opts *createOptions) (content.Content, error) {
	r, err := cli.Client().CreateRoadmap(ctx, api.CreateRoadmapRequest{
		Name: opts.name,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create roadmap: %w", err)
	}

	return r, nil
}

func createSkillPath(ctx context.Context, cli labcli.CLI, opts *createOptions) (content.Content, error) {
	sp, err := cli.Client().CreateSkillPath(ctx, api.CreateSkillPathRequest{
		Name: opts.name,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create skill path: %w", err)
	}

	return sp, nil
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

func createTraining(ctx context.Context, cli labcli.CLI, opts *createOptions) (content.Content, error) {
	t, err := cli.Client().CreateTraining(ctx, api.CreateTrainingRequest{
		Name: opts.name,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create training: %w", err)
	}

	return t, nil
}

func hasAuthorProfile(ctx context.Context, cli labcli.CLI) (bool, error) {
	if _, err := cli.Client().GetAuthor(ctx); err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return false, nil // no author profile found (HTTP 404)
		}
		return false, fmt.Errorf("couldn't get the current author profile: %w", err)
	}

	return true, nil
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
