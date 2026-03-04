package expose

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

// kubeServiceList represents the minimal structure of `kubectl get svc -A -o json`.
type kubeServiceList struct {
	Items []kubeService `json:"items"`
}

type kubeService struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Type  string     `json:"type"`
		Ports []kubePort `json:"ports"`
	} `json:"spec"`
}

type kubePort struct {
	NodePort int `json:"nodePort"`
}

func parseKubeSvcOutput(output []byte) ([]discoveredPort, error) {
	var svcList kubeServiceList
	if err := json.Unmarshal(output, &svcList); err != nil {
		return nil, fmt.Errorf("failed to parse kubectl output: %w", err)
	}

	var ports []discoveredPort
	seen := make(map[int]bool)
	for _, svc := range svcList.Items {
		if svc.Spec.Type != "NodePort" {
			continue
		}
		for _, p := range svc.Spec.Ports {
			if p.NodePort == 0 || seen[p.NodePort] {
				continue
			}
			seen[p.NodePort] = true
			ports = append(ports, discoveredPort{
				Port:      p.NodePort,
				Service:   svc.Metadata.Name,
				Namespace: svc.Metadata.Namespace,
			})
		}
	}

	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Port < ports[j].Port
	})
	return ports, nil
}

// kubeNodeList represents the minimal structure of `kubectl get nodes -o json`.
type kubeNodeList struct {
	Items []kubeNode `json:"items"`
}

type kubeNode struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

// pickExposeNode selects a playground machine to expose NodePorts on by
// cross-referencing kubectl node names with playground machine names.
// Falls back to the first playground machine if no match is found.
func pickExposeNode(nodesJSON []byte, playMachines []api.Machine) (string, error) {
	var nodeList kubeNodeList
	if err := json.Unmarshal(nodesJSON, &nodeList); err != nil {
		return "", fmt.Errorf("failed to parse kubectl nodes output: %w", err)
	}

	machineNames := make(map[string]bool, len(playMachines))
	for _, m := range playMachines {
		machineNames[m.Name] = true
	}

	for _, node := range nodeList.Items {
		if machineNames[node.Metadata.Name] {
			return node.Metadata.Name, nil
		}
	}

	if len(playMachines) > 0 {
		return playMachines[0].Name, nil
	}

	return "", fmt.Errorf("no machines available in playground")
}

func runK8sExpose(ctx context.Context, cli labcli.CLI, opts *portOptions) error {
	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	kubectlMachine := p.Machines[0].Name
	cli.PrintAux("Connecting to %s to run kubectl...\n", kubectlMachine)

	sess, cancel, err := connectSSH(ctx, cli, p, kubectlMachine)
	if err != nil {
		return fmt.Errorf("couldn't connect to %s: %w", kubectlMachine, err)
	}
	defer cancel()

	kubectlSvcCmd := "kubectl get svc --all-namespaces -o json"
	if opts.namespace != "" {
		kubectlSvcCmd = fmt.Sprintf("kubectl get svc -n %s -o json", opts.namespace)
		cli.PrintAux("Discovering NodePort services in namespace %q...\n", opts.namespace)
	} else {
		cli.PrintAux("Discovering NodePort services across all namespaces...\n")
	}

	svcOutput, err := sess.RunOutput(ctx, kubectlSvcCmd)
	if err != nil {
		sess.Close()
		return fmt.Errorf("couldn't run kubectl get svc on %s: %w", kubectlMachine, err)
	}

	discoveredPorts, err := parseKubeSvcOutput(svcOutput)
	if err != nil {
		sess.Close()
		return err
	}

	if len(discoveredPorts) == 0 {
		sess.Close()
		cli.PrintAux("No NodePort services found in the cluster\n")
		return nil
	}

	cli.PrintAux("Found %d NodePort service port(s)\n", len(discoveredPorts))

	exposeMachine := opts.machine
	if exposeMachine == "" {
		nodesOutput, err := sess.RunOutput(ctx, "kubectl get nodes -o json")
		sess.Close()
		if err != nil {
			return fmt.Errorf("couldn't run kubectl get nodes on %s: %w", kubectlMachine, err)
		}

		exposeMachine, err = pickExposeNode(nodesOutput, p.Machines)
		if err != nil {
			return fmt.Errorf("couldn't determine which node to expose on: %w", err)
		}
		cli.PrintAux("Selected node %s for exposing NodePorts\n", exposeMachine)
	} else {
		sess.Close()
		if p.GetMachine(exposeMachine) == nil {
			return fmt.Errorf("machine %q not found in the playground", exposeMachine)
		}
	}

	alreadyExposed, err := cli.Client().ListPorts(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't list existing exposed ports: %w", err)
	}
	exposedSet := make(map[string]bool)
	for _, port := range alreadyExposed {
		exposedSet[fmt.Sprintf("%s:%d", port.Machine, port.Number)] = true
	}

	for _, dp := range discoveredPorts {
		key := fmt.Sprintf("%s:%d", exposeMachine, dp.Port)
		if exposedSet[key] {
			cli.PrintAux("Skipping %s (already exposed)\n", dp.Label())
			continue
		}

		cli.PrintAux("Exposing %s...\n", dp.Label())

		resp, err := cli.Client().ExposePort(ctx, opts.playID, api.ExposePortRequest{
			Machine: exposeMachine,
			Number:  dp.Port,
			Access:  opts.access(),
		})
		if err != nil {
			cli.PrintErr("Warning: couldn't expose %s: %v\n", dp.Label(), err)
			continue
		}

		cli.PrintAux("HTTP port %s:%d exposed as %s\n", resp.Machine, resp.Number, resp.URL)
		cli.PrintOut("%s\n", resp.URL)
	}

	return nil
}
