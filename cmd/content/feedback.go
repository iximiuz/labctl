package content

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/content"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

func newFeedbackCommand(cli labcli.CLI) *cobra.Command {
	var opts feedbackOptions

	cmd := &cobra.Command{
		Use:               "feedback [flags] <kind> <name>",
		Short:             "Read feedback on content you authored",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completion.ContentArgs(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.kind.Set(args[0]); err != nil {
				return labcli.WrapStatusError(err)
			}
			baseURL := ""
			if cfg := cli.Config(); cfg != nil {
				baseURL = cfg.BaseURL
			}
			name, err := feedbackContentName(opts.kind, args[1], baseURL)
			if err != nil {
				return labcli.WrapStatusError(err)
			}
			opts.name = name

			return labcli.WrapStatusError(runFeedback(cmd.Context(), cli, &opts))
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(
		&opts.output,
		"output",
		"o",
		"yaml",
		"Output format: yaml or json",
	)

	return cmd
}

type feedbackOptions struct {
	kind   content.ContentKind
	name   string
	output string
}

func feedbackContentName(kind content.ContentKind, arg, baseURL string) (string, error) {
	if strings.Contains(arg, "://") {
		u, err := url.Parse(arg)
		base, baseErr := url.Parse(baseURL)
		if err != nil || baseErr != nil || u.User != nil || !sameFeedbackOrigin(u, base) {
			return "", fmt.Errorf("invalid content URL %q", arg)
		}
		parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/"), "/")
		if len(parts) != 2 || parts[0] != kind.Plural() {
			return "", fmt.Errorf("expected a %s URL with path /%s/<name>", kind, kind.Plural())
		}
		arg = parts[1]
	}
	// Content names use lowercase ASCII letters, digits, and interior hyphens.
	// This also excludes encoded separators and traversal segments
	// left after url.Parse decodes a URL path once.
	if !validFeedbackSlug(arg) {
		return "", fmt.Errorf("invalid content name %q", arg)
	}
	return arg, nil
}

func validFeedbackSlug(name string) bool {
	if name == "" || !feedbackSlugAlnum(name[0]) || !feedbackSlugAlnum(name[len(name)-1]) {
		return false
	}
	for i := 1; i < len(name)-1; i++ {
		if !feedbackSlugAlnum(name[i]) && name[i] != '-' {
			return false
		}
	}
	return true
}

func feedbackSlugAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

func sameFeedbackOrigin(u, base *url.URL) bool {
	if u == nil || base == nil || u.Scheme != base.Scheme || u.Host == "" || base.Host == "" ||
		!strings.EqualFold(u.Hostname(), base.Hostname()) || feedbackPort(u) != feedbackPort(base) {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	if strings.EqualFold(u.Hostname(), "localhost") {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsLoopback()
}

func feedbackPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

func runFeedback(ctx context.Context, cli labcli.CLI, opts *feedbackOptions) error {
	if opts.output != "yaml" && opts.output != "json" {
		return fmt.Errorf("invalid output format %q (supported formats: yaml, json)", opts.output)
	}

	me, err := cli.Client().GetMe(ctx)
	if err != nil {
		return fmt.Errorf("couldn't verify your identity: %w", err)
	}
	if me.ID == "" {
		return fmt.Errorf("couldn't verify your identity: the server returned no user ID")
	}
	cont, err := getContent(ctx, cli, opts.kind, opts.name)
	if err != nil {
		return fmt.Errorf("couldn't get %s %q: %w", opts.kind, opts.name, err)
	}
	authored, ok := cont.(interface{ IsAuthoredBy(string) bool })
	if !ok || !authored.IsAuthoredBy(me.ID) {
		return fmt.Errorf("feedback for %s %q is available only to its authors", opts.kind, opts.name)
	}

	summary, err := cli.Client().ListContentFeedback(ctx, opts.kind, opts.name)
	if err != nil {
		return fmt.Errorf("couldn't list %s feedback: %w", opts.kind, err)
	}
	if summary.Role != "author" {
		return fmt.Errorf("feedback for %s %q is available only to its authors", opts.kind, opts.name)
	}

	threads := make([]api.FeedbackThread, 0, len(summary.Threads))
	for _, item := range summary.Threads {
		thread, err := cli.Client().GetContentFeedbackThread(ctx, item.ID)
		if err != nil {
			return fmt.Errorf("couldn't get feedback thread %s: %w", item.ID, err)
		}
		threads = append(threads, *thread)
	}
	result := struct {
		Kind    content.ContentKind  `json:"kind" yaml:"kind"`
		Name    string               `json:"name" yaml:"name"`
		Threads []api.FeedbackThread `json:"threads" yaml:"threads"`
	}{Kind: opts.kind, Name: opts.name, Threads: threads}

	if opts.output == "json" {
		enc := json.NewEncoder(cli.OutputStream())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	enc := yaml.NewEncoder(cli.OutputStream())
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(result)
}
