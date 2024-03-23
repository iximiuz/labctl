package content

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/iximiuz/labctl/internal/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

var _ pflag.Value = (*content.ContentKind)(nil)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "content <create|list|pull|files|sync|rm> <content-name> [flags]",
		Aliases: []string{"c", "contents"},
		Short:   "Authoring and managing content (challenge, tutorial, course, etc.)",
	}

	cmd.AddCommand(
		newCreateCommand(cli),
		newListCommand(cli),
		newPullCommand(cli),
		newPushCommand(cli),
		newRemoveCommand(cli),
	)

	return cmd
}

type dirOptions struct {
	dir string
}

func (o *dirOptions) AddDirFlag(fs *pflag.FlagSet) {
	fs.StringVarP(&o.dir, "dir", "d", "", "Local directory with content files (default: $CWD/<content-name>)")
}

func (o *dirOptions) ContentDir(name string) (string, error) {
	dir := o.dir
	if dir == "" {
		dir = name
	}

	if abs, err := filepath.Abs(dir); err != nil {
		return "", fmt.Errorf("couldn't get the absolute path of %s: %w", dir, err)
	} else {
		return abs, nil
	}
}
