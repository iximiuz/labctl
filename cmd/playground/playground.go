package playground

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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
		newRestartCommand(cli),
		newDestroyCommand(cli),
		newPersistCommand(cli),
		newLifetimeCommand(cli),
		newRegionCommand(cli),
		newMachinesCommand(cli),
		newMachineCommand(cli),
		newCreateCommand(cli),
		newManifestCommand(cli),
		newUpdateCommand(cli),
		newRemoveCommand(cli),
		newTasksCommand(cli),
		newWaitCommand(cli),
		newStatusCommand(cli),
		newOpenCommand(cli),
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

// findPlayByTitle looks up the play whose title starts with the given prefix.
// It fails if no play matches or if the prefix is ambiguous.
func findPlayByTitle(plays []*api.Play, title string) (*api.Play, error) {
	var matches []*api.Play
	for _, p := range plays {
		if p.Title != "" && strings.HasPrefix(p.Title, title) {
			matches = append(matches, p)
		}
	}

	if len(matches) == 0 {
		return nil, errors.New("could not find a play with the given title")
	}

	if len(matches) > 1 {
		return nil, errors.New("ambiguous title, please use the full title of a play or a longer prefix")
	}

	return matches[0], nil
}
