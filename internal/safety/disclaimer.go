package safety

import (
	"github.com/iximiuz/labctl/internal/labcli"
)

func ShowSafetyDisclaimer(cli labcli.CLI) (bool, error) {
	cli.PrintAux("\n!!! THIRD-PARTY PLAYGROUND SAFETY WARNING !!!\n\n")
	cli.PrintAux("You are about to start a playground created by another user.\n")
	cli.PrintAux("Please follow these guidelines to ensure your security:\n\n")
	cli.PrintAux("  • Do not process or store any confidential or sensitive data\n")
	cli.PrintAux("  • Never enter passwords or API keys of sensitive accounts\n")
	cli.PrintAux("\n")
	cli.PrintAux("The platform is not responsible for any damage resulting from\n")
	cli.PrintAux("the use of third-party playgrounds.\n\n")

	if cli.Confirm(
		"Do you acknowledge the risks and agree to continue?",
		"Yes", "No",
	) {
		return true, nil
	}

	return false, labcli.NewStatusError(0, "See you later!")
}
