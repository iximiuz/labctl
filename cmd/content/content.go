package content

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "content <create|pull|files|sync|rm> <content-name> [flags]",
		Aliases: []string{"c", "contents"},
		Short:   "Authoring and managing content (challenge, tutorial, course, etc.)",
	}

	cmd.AddCommand(
		newCreateCommand(cli),
	)

	return cmd
}
