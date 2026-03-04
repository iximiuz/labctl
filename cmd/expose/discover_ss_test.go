package expose

import (
	"testing"
)

func TestParseSSOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{
			name:     "empty output",
			input:    "",
			expected: []int{},
		},
		{
			name: "header only",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
`,
			expected: []int{},
		},
		{
			name: "single IPv4 port on wildcard address",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     0.0.0.0:30001       0.0.0.0:*          users:(("kube-proxy",pid=1234,fd=5))
`,
			expected: []int{30001},
		},
		{
			name: "multiple IPv4 ports",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     0.0.0.0:30001       0.0.0.0:*          users:(("kube-proxy",pid=1234,fd=5))
LISTEN 0       128     0.0.0.0:30002       0.0.0.0:*          users:(("kube-proxy",pid=1234,fd=7))
LISTEN 0       128     0.0.0.0:8080        0.0.0.0:*          users:(("nginx",pid=5678,fd=6))
`,
			expected: []int{8080, 30001, 30002},
		},
		{
			name: "IPv6 ports on wildcard",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     [::]:30001          [::]:*             users:(("kube-proxy",pid=1234,fd=8))
LISTEN 0       128     [::]:30002          [::]:*             users:(("kube-proxy",pid=1234,fd=9))
`,
			expected: []int{30001, 30002},
		},
		{
			name: "mixed IPv4 and IPv6 deduplication",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     0.0.0.0:30001       0.0.0.0:*          users:(("kube-proxy",pid=1234,fd=5))
LISTEN 0       128     [::]:30001          [::]:*             users:(("kube-proxy",pid=1234,fd=8))
LISTEN 0       128     0.0.0.0:30002       0.0.0.0:*          users:(("kube-proxy",pid=1234,fd=7))
LISTEN 0       128     [::]:30002          [::]:*             users:(("kube-proxy",pid=1234,fd=9))
`,
			expected: []int{30001, 30002},
		},
		{
			name: "skips port 22",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     0.0.0.0:22          0.0.0.0:*          users:(("sshd",pid=100,fd=3))
LISTEN 0       128     0.0.0.0:30001       0.0.0.0:*          users:(("kube-proxy",pid=1234,fd=5))
`,
			expected: []int{30001},
		},
		{
			name: "skips port 53 DNS",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     127.0.0.53:53       0.0.0.0:*          users:(("systemd-resolve",pid=850,fd=14))
LISTEN 0       128     0.0.0.0:8080        0.0.0.0:*          users:(("nginx",pid=5678,fd=6))
`,
			expected: []int{8080},
		},
		{
			name: "skips localhost-only binding",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     127.0.0.1:8443      0.0.0.0:*          users:(("apiserver",pid=2000,fd=3))
`,
			expected: []int{},
		},
		{
			name: "skips IPv6 loopback binding",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     [::1]:9090          [::]:*             users:(("prometheus",pid=4000,fd=5))
`,
			expected: []int{},
		},
		{
			name: "keeps port that has both loopback and wildcard binding",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     127.0.0.1:8080      0.0.0.0:*          users:(("nginx",pid=5678,fd=6))
LISTEN 0       128     0.0.0.0:8080        0.0.0.0:*          users:(("nginx",pid=5678,fd=7))
`,
			expected: []int{8080},
		},
		{
			name: "wildcard binding",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     *:80                *:*                users:(("nginx",pid=3000,fd=6))
`,
			expected: []int{80},
		},
		{
			name: "skips systemd-only process",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       4096    0.0.0.0:50061       0.0.0.0:*          users:(("systemd",pid=1,fd=42))
`,
			expected: []int{},
		},
		{
			name: "skips examiner process",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       4096    *:40059             *:*                users:(("examiner",pid=922,fd=4))
`,
			expected: []int{},
		},
		{
			name: "skips systemd-resolve process",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       4096    127.0.0.54:53       0.0.0.0:*          users:(("systemd-resolve",pid=850,fd=16))
`,
			expected: []int{},
		},
		{
			name: "realistic docker playground - filters system noise",
			input: `State                    Recv-Q                    Send-Q                                       Local Address:Port                                        Peer Address:Port                   Process
LISTEN                   0                         4096                                               0.0.0.0:22                                               0.0.0.0:*                       users:(("sshd",pid=936,fd=3),("systemd",pid=1,fd=63))
LISTEN                   0                         4096                                         127.0.0.53%lo:53                                               0.0.0.0:*                       users:(("systemd-resolve",pid=850,fd=14))
LISTEN                   0                         4096                                               0.0.0.0:50061                                            0.0.0.0:*                       users:(("systemd",pid=1,fd=42))
LISTEN                   0                         4096                                            127.0.0.54:53                                               0.0.0.0:*                       users:(("systemd-resolve",pid=850,fd=16))
LISTEN                   0                         4096                                                  [::]:22                                                  [::]:*                       users:(("sshd",pid=936,fd=4),("systemd",pid=1,fd=66))
LISTEN                   0                         4096                                                     *:40059                                                  *:*                       users:(("examiner",pid=922,fd=4))
`,
			expected: []int{},
		},
		{
			name: "realistic docker playground with user services",
			input: `State                    Recv-Q                    Send-Q                                       Local Address:Port                                        Peer Address:Port                   Process
LISTEN                   0                         4096                                               0.0.0.0:22                                               0.0.0.0:*                       users:(("sshd",pid=936,fd=3),("systemd",pid=1,fd=63))
LISTEN                   0                         4096                                         127.0.0.53%lo:53                                               0.0.0.0:*                       users:(("systemd-resolve",pid=850,fd=14))
LISTEN                   0                         4096                                               0.0.0.0:50061                                            0.0.0.0:*                       users:(("systemd",pid=1,fd=42))
LISTEN                   0                         4096                                               0.0.0.0:8080                                             0.0.0.0:*                       users:(("nginx",pid=1234,fd=6))
LISTEN                   0                         4096                                               0.0.0.0:3000                                             0.0.0.0:*                       users:(("grafana",pid=2345,fd=8))
LISTEN                   0                         4096                                                     *:40059                                                  *:*                       users:(("examiner",pid=922,fd=4))
`,
			expected: []int{3000, 8080},
		},
		{
			name: "realistic k8s control-plane output - filters system, keeps NodePorts",
			input: `State   Recv-Q  Send-Q   Local Address:Port   Peer Address:Port  Process
LISTEN  0       4096     0.0.0.0:30080        0.0.0.0:*          users:(("kube-proxy",pid=745,fd=13))
LISTEN  0       4096     0.0.0.0:30443        0.0.0.0:*          users:(("kube-proxy",pid=745,fd=15))
LISTEN  0       4096     127.0.0.1:10248      0.0.0.0:*          users:(("kubelet",pid=567,fd=22))
LISTEN  0       4096     127.0.0.1:10249      0.0.0.0:*          users:(("kube-proxy",pid=745,fd=11))
LISTEN  0       128      0.0.0.0:22           0.0.0.0:*          users:(("sshd",pid=1,fd=3))
LISTEN  0       4096     127.0.0.1:2379       0.0.0.0:*          users:(("etcd",pid=423,fd=7))
LISTEN  0       4096     0.0.0.0:6443         0.0.0.0:*          users:(("kube-apiserver",pid=456,fd=7))
LISTEN  0       4096     [::]:30080           [::]:*             users:(("kube-proxy",pid=745,fd=14))
LISTEN  0       4096     [::]:30443           [::]:*             users:(("kube-proxy",pid=745,fd=16))
`,
			expected: []int{6443, 30080, 30443},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports, err := parseSSOutput(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(ports) == 0 && len(tt.expected) == 0 {
				return
			}
			if len(ports) != len(tt.expected) {
				t.Fatalf("expected %d ports %v, got %d ports %v", len(tt.expected), tt.expected, len(ports), ports)
			}
			for i, p := range ports {
				if p != tt.expected[i] {
					t.Errorf("port[%d]: expected %d, got %d", i, tt.expected[i], p)
				}
			}
		})
	}
}

func TestExtractHostPort(t *testing.T) {
	tests := []struct {
		addr         string
		expectedHost string
		expectedPort int
		hasError     bool
	}{
		{"0.0.0.0:80", "0.0.0.0", 80, false},
		{"127.0.0.1:8443", "127.0.0.1", 8443, false},
		{"127.0.0.53%lo:53", "127.0.0.53%lo", 53, false},
		{"127.0.0.54:53", "127.0.0.54", 53, false},
		{"*:30001", "*", 30001, false},
		{"[::]:30001", "[::]", 30001, false},
		{"[::1]:8080", "[::1]", 8080, false},
		{"noport", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			host, port, err := extractHostPort(tt.addr)
			if tt.hasError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != tt.expectedHost {
				t.Errorf("host: expected %q, got %q", tt.expectedHost, host)
			}
			if port != tt.expectedPort {
				t.Errorf("port: expected %d, got %d", tt.expectedPort, port)
			}
		})
	}
}

func TestSSEntryIsLoopback(t *testing.T) {
	tests := []struct {
		host     string
		expected bool
	}{
		{"0.0.0.0", false},
		{"*", false},
		{"[::]", false},
		{"127.0.0.1", true},
		{"127.0.0.53%lo", true},
		{"127.0.0.54", true},
		{"[::1]", true},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			e := ssEntry{host: tt.host}
			if got := e.isLoopback(); got != tt.expected {
				t.Errorf("isLoopback(%q) = %v, want %v", tt.host, got, tt.expected)
			}
		})
	}
}

func TestSSEntryHasOnlySkippedProcesses(t *testing.T) {
	tests := []struct {
		name      string
		processes string
		expected  bool
	}{
		{"nginx", `users:(("nginx",pid=1234,fd=6))`, false},
		{"kube-proxy", `users:(("kube-proxy",pid=745,fd=13))`, false},
		{"systemd only", `users:(("systemd",pid=1,fd=42))`, true},
		{"examiner only", `users:(("examiner",pid=922,fd=4))`, true},
		{"systemd-resolve only", `users:(("systemd-resolve",pid=850,fd=14))`, true},
		{"sshd and systemd", `users:(("sshd",pid=936,fd=3),("systemd",pid=1,fd=63))`, false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := ssEntry{processes: tt.processes}
			if got := e.hasOnlySkippedProcesses(); got != tt.expected {
				t.Errorf("hasOnlySkippedProcesses(%q) = %v, want %v", tt.processes, got, tt.expected)
			}
		})
	}
}
