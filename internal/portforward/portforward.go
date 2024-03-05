package portforward

import (
	"fmt"
	"math/rand"
	"strings"
)

type ForwardingSpec struct {
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

func ParseLocal(s string) (ForwardingSpec, error) {
	var cfg ForwardingSpec

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

func RandomLocalPort() string {
	return fmt.Sprintf("%d", 40000+rand.Intn(20000))
}
