package content

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type createOptions struct {
	kind ContentKind
	name string

	dir string

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

	cli.PrintAux("Creating a new %s...\n", opts.kind)

	// if _, err := os.Stat(opts.dir); err == nil {
	// 	return fmt.Errorf("directory %s already exists - aborting to avoid overwriting existing files", opts.dir)
	// }

	// 			if cwd, err := os.Getwd(); err != nil {
	// 				return labcli.WrapStatusError(fmt.Errorf("couldn't get the current working directory: %w", err))
	// 			} else {
	// 				opts.dir = cwd
	// 			}
	// 		if opts.dir == "" {
	// 			if cwd, err := os.Getwd(); err != nil {
	// 				return labcli.WrapStatusError(fmt.Errorf("couldn't get the current working directory: %w", err))
	// 			} else {
	// 				opts.dir = cwd
	// 			}
	// 		}
	// 		if absDir, err := filepath.Abs(opts.dir); err != nil {
	// 			return labcli.WrapStatusError(fmt.Errorf("couldn't get the absolute path of %s: %w", opts.dir, err))
	// 		} else {
	// 			opts.dir = absDir
	// 		}

	// if err := os.MkdirAll(opts.dir, 0755); err != nil {
	// 	return fmt.Errorf("couldn't create directory %s: %w", opts.dir, err)
	// }

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
	ch, err := cli.Client().CreateChallenge(ctx, api.CreateChallengeRequest{
		Name: opts.name,
	})
	if err != nil {
		return fmt.Errorf("couldn't create challenge: %w", err)
	}

	cli.PrintAux("Created a new challenge %s\n", ch.PageURL)
	if err := open.Run(ch.PageURL); err != nil {
		cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually.\n")
	}

	if err := cli.Client().PutMarkdown(ctx, api.PutMarkdownRequest{
		Kind: "challenge",
		Name: ch.Name,
		Content: `---
title: Sample Challenge 444
description: |
  This is a sample challenge.

kind: challenge
playground: docker

createdAt: 2024-01-01
updatedAt: 2024-02-09

difficulty: medium

categories:
  - containers

tagz:
  - containerd
  - ctr
  - docker

tasks:
  init_run_container_labs_are_fun:
    init: true
    run: |
      docker run -q -d --name labs-are-fun busybox sleep 999999
---
# Sample Challenge

This is a sample challenge. You can edit this file in ... .`,
	}); err != nil {
		return fmt.Errorf("couldn't create a sample markdown file: %w", err)
	}

	return labcli.NewStatusError(0, "Happy authoring..")
}

func createTutorial(ctx context.Context, cli labcli.CLI, opts *createOptions) error {
	return nil
}

func createCourse(ctx context.Context, cli labcli.CLI, opts *createOptions) error {
	ch, err := cli.Client().CreateCourse(ctx, api.CreateCourseRequest{
		Name:    opts.name,
		Variant: api.CourseVariantModular,
	})
	if err != nil {
		return fmt.Errorf("couldn't create course: %w", err)
	}

	cli.PrintAux("Created a new course %s\n", ch.PageURL)
	if err := open.Run(ch.PageURL); err != nil {
		cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually.\n")
	}

	return labcli.NewStatusError(0, "Happy authoring..")
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
