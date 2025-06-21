package playground

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "playground <list|start|stop> [playground-name]",
		Aliases: []string{"p", "playgrounds"},
		Short:   "List, start and stop playgrounds",
	}

	cmd.AddCommand(
		newListCommand(cli),
		newCatalogCommand(cli),
		newStartCommand(cli),
		newStopCommand(cli),
		newMachinesCommand(cli),
		newCreateCommand(cli),
		newManifestCommand(cli),
		newUpdateCommand(cli),
		newRemoveCommand(cli),
	)

	return cmd
}

func readManifestFile(filePath string) (*api.PlaygroundManifest, error) {
	var rawManifest []byte
	var err error

	if filePath == "-" {
		rawManifest, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read manifest from stdin: %w", err)
		}
	} else {
		rawManifest, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read manifest file: %w", err)
		}
	}

	var manifest api.PlaygroundManifest
	if err := yaml.Unmarshal(rawManifest, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest file: %w", err)
	}

	if manifest.Kind != "playground" {
		return nil, fmt.Errorf("invalid manifest kind: %s (expected 'playground')", manifest.Kind)
	}

	return &manifest, nil
}
