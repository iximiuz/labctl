package portforward

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/iximiuz/labctl/api"
)

type ForwardingSpec struct {
	Kind       string // "local" or "remote"
	LocalHost  string // Defaults to "127.0.0.1" if not specified
	LocalPort  string // Empty if a random port is to be used
	RemoteHost string // Defaults to ""
	RemotePort string // Required, no default
}

func (f ForwardingSpec) LocalAddr() string {
	return f.LocalHost + ":" + f.LocalPort
}

func (f ForwardingSpec) RemoteAddr() string {
	return f.RemoteHost + ":" + f.RemotePort
}

// ParseRemote parses a -R port forwarding spec, following SSH-like semantics
// where the BIND address (on the playground) comes first and the TARGET address
// (on the labctl side) comes second. Accepted forms:
//
//	REMOTE_PORT:LOCAL_PORT                         # bind 0.0.0.0:REMOTE_PORT on playground,
//	                                               # dial 127.0.0.1:LOCAL_PORT on labctl
//	REMOTE_PORT:LOCAL_HOST:LOCAL_PORT              # same, but dial LOCAL_HOST:LOCAL_PORT
//	REMOTE_HOST:REMOTE_PORT:LOCAL_PORT             # bind REMOTE_HOST:REMOTE_PORT
//	REMOTE_HOST:REMOTE_PORT:LOCAL_HOST:LOCAL_PORT  # most explicit form
func ParseRemote(s string) (ForwardingSpec, error) {
	var cfg ForwardingSpec

	cfg.Kind = "remote"

	parts := strings.Split(s, ":")

	switch len(parts) {
	case 2: // REMOTE_PORT:LOCAL_PORT
		cfg.RemoteHost = "0.0.0.0"
		cfg.RemotePort = parts[0]
		cfg.LocalHost = "127.0.0.1"
		cfg.LocalPort = parts[1]
	case 3: // REMOTE_PORT:LOCAL_HOST:LOCAL_PORT  or  REMOTE_HOST:REMOTE_PORT:LOCAL_PORT
		if _, err := strconv.Atoi(parts[1]); err == nil {
			// Second part is a port, so REMOTE_HOST:REMOTE_PORT:LOCAL_PORT
			cfg.RemoteHost = parts[0]
			cfg.RemotePort = parts[1]
			cfg.LocalHost = "127.0.0.1"
			cfg.LocalPort = parts[2]
		} else {
			// Second part is a host, so REMOTE_PORT:LOCAL_HOST:LOCAL_PORT
			cfg.RemoteHost = "0.0.0.0"
			cfg.RemotePort = parts[0]
			cfg.LocalHost = parts[1]
			cfg.LocalPort = parts[2]
		}
	case 4: // REMOTE_HOST:REMOTE_PORT:LOCAL_HOST:LOCAL_PORT
		cfg.RemoteHost = parts[0]
		cfg.RemotePort = parts[1]
		cfg.LocalHost = parts[2]
		cfg.LocalPort = parts[3]
	default:
		return cfg, fmt.Errorf("invalid forwarding configuration format")
	}

	if cfg.LocalPort == "" || cfg.RemotePort == "" {
		return cfg, fmt.Errorf("both remote (bind) and local (target) ports are required for -R")
	}

	return cfg, nil
}

func ParseLocal(s string) (ForwardingSpec, error) {
	var cfg ForwardingSpec

	cfg.Kind = "local" // Local port forwarding

	parts := strings.Split(s, ":")

	switch len(parts) {
	case 1: // REMOTE_PORT
		cfg.RemoteHost = ""
		cfg.RemotePort = parts[0]
		cfg.LocalHost = "127.0.0.1"
	case 2: // REMOTE_HOST:REMOTE_PORT or LOCAL_PORT:REMOTE_PORT
		if _, err := fmt.Sscanf(parts[0], "%d", new(int)); err == nil {
			// It's a port number
			cfg.LocalPort = parts[0]
			cfg.RemotePort = parts[1]
			cfg.RemoteHost = ""
			cfg.LocalHost = "127.0.0.1"
		} else {
			// It's a host
			cfg.RemoteHost = parts[0]
			cfg.RemotePort = parts[1]
			cfg.LocalHost = "127.0.0.1"
		}
	case 3: // LOCAL_PORT:REMOTE_HOST:REMOTE_PORT or LOCAL_HOST:LOCAL_PORT:REMOTE_PORT
		if _, err := fmt.Sscanf(parts[1], "%d", new(int)); err == nil {
			// Second part is a port, so it's LOCAL_HOST:LOCAL_PORT:REMOTE_PORT
			cfg.LocalHost = parts[0]
			cfg.LocalPort = parts[1]
			cfg.RemotePort = parts[2]
			cfg.RemoteHost = ""
		} else {
			// Second part is not a port, so it's LOCAL_PORT:REMOTE_HOST:REMOTE_PORT
			cfg.LocalPort = parts[0]
			cfg.RemoteHost = parts[1]
			cfg.RemotePort = parts[2]
			cfg.LocalHost = "127.0.0.1"
		}
	case 4: // LOCAL_HOST:LOCAL_PORT:REMOTE_HOST:REMOTE_PORT
		cfg.LocalHost = parts[0]
		cfg.LocalPort = parts[1]
		cfg.RemoteHost = parts[2]
		cfg.RemotePort = parts[3]
	default:
		return cfg, fmt.Errorf("invalid forwarding configuration format")
	}

	if cfg.LocalPort == "" {
		// Should use 0 to avoid conflicts but wsmux won't report back the actual port.
		// Hence, we use a poor man's random port instead.
		cfg.LocalPort = RandomLocalPort()
	}

	return cfg, nil
}

func (f ForwardingSpec) ToPortForward(machine string) (*api.PortForward, error) {
	pf := api.PortForward{
		Kind:       f.Kind,
		Machine:    machine,
		LocalHost:  f.LocalHost,
		RemoteHost: f.RemoteHost,
	}

	if f.LocalPort != "" {
		if port, err := strconv.Atoi(f.LocalPort); err == nil {
			pf.LocalPort = port
		}
	}
	if f.RemotePort != "" {
		if port, err := strconv.Atoi(f.RemotePort); err == nil {
			pf.RemotePort = port
		}
	}

	return &pf, nil
}

// PortForwardToSpec converts an API PortForward to a ForwardingSpec.
func PortForwardToSpec(pf *api.PortForward) ForwardingSpec {
	spec := ForwardingSpec{
		Kind:      pf.Kind,
		LocalHost: "127.0.0.1",
	}

	if pf.LocalHost != "" {
		spec.LocalHost = pf.LocalHost
	}
	if pf.LocalPort > 0 {
		spec.LocalPort = strconv.Itoa(pf.LocalPort)
	} else {
		// Use a random port if not specified
		spec.LocalPort = RandomLocalPort()
	}
	if pf.RemoteHost != "" {
		spec.RemoteHost = pf.RemoteHost
	}
	if pf.RemotePort > 0 {
		spec.RemotePort = strconv.Itoa(pf.RemotePort)
	}

	return spec
}

func RandomLocalPort() string {
	return fmt.Sprintf("%d", 40000+rand.Intn(20000))
}
