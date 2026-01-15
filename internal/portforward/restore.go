package portforward

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

// RestoreSavedForwards starts port forwarding for all saved port forwards in the background.
// It returns a channel that will receive the result (nil on success, error on failure).
// The caller can choose to wait on the channel or let it run in the background.
func RestoreSavedForwards(
	ctx context.Context,
	client *api.Client,
	playID string,
	out labcli.Outputer,
) (<-chan error, error) {
	forwards, err := client.ListPortForwards(ctx, playID)
	if err != nil {
		return nil, fmt.Errorf("couldn't list port forwards: %w", err)
	}

	resultCh := make(chan error, 1)

	if len(forwards) == 0 {
		out.PrintAux("No saved port forwards to restore.\n")
		resultCh <- nil
		return resultCh, nil
	}

	out.PrintAux("Restoring %d port forward(s)...\n", len(forwards))

	// Group forwards by machine
	machineForwards := make(map[string][]ForwardingSpec)
	for _, pf := range forwards {
		spec := PortForwardToSpec(pf)
		machineForwards[pf.Machine] = append(machineForwards[pf.Machine], spec)
	}

	tunnelErrCh := make(chan error, 100)

	// Log tunnel errors in background
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-tunnelErrCh:
				out.PrintErr("Tunnel error: %v", err)
			}
		}
	}()

	// Start all port forwards in background
	go func() {
		var g errgroup.Group

		for machine, specs := range machineForwards {
			g.Go(func() error {
				tunnel, err := StartTunnel(ctx, client, TunnelOptions{
					PlayID:  playID,
					Machine: machine,
				})
				if err != nil {
					return fmt.Errorf("couldn't start tunnel for machine %s: %w", machine, err)
				}

				var innerG errgroup.Group
				for _, spec := range specs {
					innerG.Go(func() error {
						out.PrintAux("Forwarding %s -> %s (machine: %s)\n", spec.LocalAddr(), spec.RemoteAddr(), machine)
						return tunnel.Forward(ctx, spec, tunnelErrCh)
					})
				}

				return innerG.Wait()
			})
		}

		resultCh <- g.Wait()
	}()

	return resultCh, nil
}
