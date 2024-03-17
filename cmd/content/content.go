package content

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "content <create|list|pull|files|sync|rm> <content-name> [flags]",
		Aliases: []string{"c", "contents"},
		Short:   "Authoring and managing content (challenge, tutorial, course, etc.)",
	}

	cmd.AddCommand(
		newCreateCommand(cli),
		newListCommand(cli),
		newSyncCommand(cli),
		newRemoveCommand(cli),
	)

	return cmd
}

type ContentKind string

var _ pflag.Value = (*ContentKind)(nil)

const (
	KindChallenge ContentKind = "challenge"
	KindTutorial  ContentKind = "tutorial"
	KindCourse    ContentKind = "course"
)

func (k *ContentKind) Set(v string) error {
	switch string(v) {
	case string(KindChallenge):
		*k = KindChallenge
	case string(KindTutorial):
		*k = KindTutorial
	case string(KindCourse):
		*k = KindCourse
	default:
		return fmt.Errorf("unknown content kind: %s", v)
	}

	return nil
}

func (k *ContentKind) String() string {
	return string(*k)
}

func (k *ContentKind) Type() string {
	return "content-kind"
}
