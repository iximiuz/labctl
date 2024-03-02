package auth

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/cliutil"
)

// type authOptions struct {
// }

func NewCommand(cli cliutil.CLI) *cobra.Command {
	// var opts options

	cmd := &cobra.Command{
		Short: "Authenticate the current machine with iximiuz Labs",
		Use:   "auth <login|logout>",
		// Args: cobra.MinimumNArgs(1),
		// RunE: func(cmd *cobra.Command, args []string) error {
		// 	// cli.SetQuiet(opts.quiet)

		// 	// return cliutil.WrapStatusError(err)

		// 	return nil
		// },
	}

	// flags := cmd.Flags()
	// flags.SetInterspersed(false) // Instead of relying on --

	// flags.BoolVarP(
	// 	&opts.quiet,
	// 	"quiet",
	// 	"q",
	// 	false,
	// 	`Suppress verbose output`,
	// )

	cmd.AddCommand(
		newLoginCommand(cli),
		newLogoutCommand(cli),
		newWhoAmICommand(cli),
	)

	return cmd
}
