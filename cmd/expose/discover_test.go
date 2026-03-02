package expose

import (
	"testing"

	"github.com/iximiuz/labctl/api"
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
			name: "single IPv4 port",
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
			name: "IPv6 ports",
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
			name: "localhost binding",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     127.0.0.1:8443      0.0.0.0:*          users:(("apiserver",pid=2000,fd=3))
`,
			expected: []int{8443},
		},
		{
			name: "wildcard binding",
			input: `State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN 0       128     *:80                *:*                users:(("nginx",pid=3000,fd=6))
`,
			expected: []int{80},
		},
		{
			name: "realistic multi-service output",
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
			expected: []int{2379, 6443, 10248, 10249, 30080, 30443},
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

func TestParseKubeSvcOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []discoveredPort
	}{
		{
			name:     "empty items",
			input:    `{"items":[]}`,
			expected: nil,
		},
		{
			name: "no NodePort services",
			input: `{
				"items": [{
					"metadata": {"name": "kubernetes", "namespace": "default"},
					"spec": {"type": "ClusterIP", "ports": [{"port": 443}]}
				}]
			}`,
			expected: nil,
		},
		{
			name: "single NodePort service",
			input: `{
				"items": [{
					"metadata": {"name": "my-app", "namespace": "default"},
					"spec": {
						"type": "NodePort",
						"ports": [{"port": 80, "targetPort": 8080, "nodePort": 30080}]
					}
				}]
			}`,
			expected: []discoveredPort{
				{Port: 30080, Service: "my-app", Namespace: "default"},
			},
		},
		{
			name: "multiple NodePort services across namespaces",
			input: `{
				"items": [
					{
						"metadata": {"name": "kubernetes", "namespace": "default"},
						"spec": {"type": "ClusterIP", "ports": [{"port": 443}]}
					},
					{
						"metadata": {"name": "frontend", "namespace": "app"},
						"spec": {
							"type": "NodePort",
							"ports": [{"port": 80, "nodePort": 30080}]
						}
					},
					{
						"metadata": {"name": "dashboard", "namespace": "monitoring"},
						"spec": {
							"type": "NodePort",
							"ports": [{"port": 443, "nodePort": 30443}]
						}
					}
				]
			}`,
			expected: []discoveredPort{
				{Port: 30080, Service: "frontend", Namespace: "app"},
				{Port: 30443, Service: "dashboard", Namespace: "monitoring"},
			},
		},
		{
			name: "service with multiple ports",
			input: `{
				"items": [{
					"metadata": {"name": "multi-port-svc", "namespace": "default"},
					"spec": {
						"type": "NodePort",
						"ports": [
							{"name": "http", "port": 80, "nodePort": 30080},
							{"name": "https", "port": 443, "nodePort": 30443}
						]
					}
				}]
			}`,
			expected: []discoveredPort{
				{Port: 30080, Service: "multi-port-svc", Namespace: "default"},
				{Port: 30443, Service: "multi-port-svc", Namespace: "default"},
			},
		},
		{
			name: "duplicate NodePort across services is deduplicated",
			input: `{
				"items": [
					{
						"metadata": {"name": "svc-a", "namespace": "ns1"},
						"spec": {"type": "NodePort", "ports": [{"nodePort": 30080}]}
					},
					{
						"metadata": {"name": "svc-b", "namespace": "ns2"},
						"spec": {"type": "NodePort", "ports": [{"nodePort": 30080}]}
					}
				]
			}`,
			expected: []discoveredPort{
				{Port: 30080, Service: "svc-a", Namespace: "ns1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports, err := parseKubeSvcOutput([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(ports) == 0 && len(tt.expected) == 0 {
				return
			}
			if len(ports) != len(tt.expected) {
				t.Fatalf("expected %d ports, got %d: %+v", len(tt.expected), len(ports), ports)
			}
			for i, p := range ports {
				if p.Port != tt.expected[i].Port {
					t.Errorf("port[%d].Port: expected %d, got %d", i, tt.expected[i].Port, p.Port)
				}
				if p.Service != tt.expected[i].Service {
					t.Errorf("port[%d].Service: expected %q, got %q", i, tt.expected[i].Service, p.Service)
				}
				if p.Namespace != tt.expected[i].Namespace {
					t.Errorf("port[%d].Namespace: expected %q, got %q", i, tt.expected[i].Namespace, p.Namespace)
				}
			}
		})
	}
}

func TestParseKubeSvcOutput_InvalidJSON(t *testing.T) {
	_, err := parseKubeSvcOutput([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestPickExposeNode(t *testing.T) {
	tests := []struct {
		name        string
		nodesJSON   string
		machines    []api.Machine
		expected    string
		expectError bool
	}{
		{
			name: "node matches playground machine",
			nodesJSON: `{
				"items": [
					{"metadata": {"name": "control-plane"}},
					{"metadata": {"name": "node-01"}},
					{"metadata": {"name": "node-02"}}
				]
			}`,
			machines: []api.Machine{
				{Name: "control-plane"},
				{Name: "node-01"},
				{Name: "node-02"},
			},
			expected: "control-plane",
		},
		{
			name: "first matching node is returned",
			nodesJSON: `{
				"items": [
					{"metadata": {"name": "worker-1"}},
					{"metadata": {"name": "worker-2"}}
				]
			}`,
			machines: []api.Machine{
				{Name: "master"},
				{Name: "worker-2"},
				{Name: "worker-1"},
			},
			expected: "worker-1",
		},
		{
			name: "no match falls back to first machine",
			nodesJSON: `{
				"items": [
					{"metadata": {"name": "k8s-node-1"}},
					{"metadata": {"name": "k8s-node-2"}}
				]
			}`,
			machines: []api.Machine{
				{Name: "vm-1"},
				{Name: "vm-2"},
			},
			expected: "vm-1",
		},
		{
			name:        "no machines available",
			nodesJSON:   `{"items": [{"metadata": {"name": "node-1"}}]}`,
			machines:    []api.Machine{},
			expectError: true,
		},
		{
			name:        "invalid JSON",
			nodesJSON:   "not json",
			machines:    []api.Machine{{Name: "vm-1"}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pickExposeNode([]byte(tt.nodesJSON), tt.machines)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractPortFromAddr(t *testing.T) {
	tests := []struct {
		addr     string
		expected int
		hasError bool
	}{
		{"0.0.0.0:80", 80, false},
		{"127.0.0.1:8443", 8443, false},
		{"*:30001", 30001, false},
		{"[::]:30001", 30001, false},
		{"[::1]:8080", 8080, false},
		{"noport", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			port, err := extractPortFromAddr(tt.addr)
			if tt.hasError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if port != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, port)
			}
		})
	}
}

func TestDiscoveredPortLabel(t *testing.T) {
	dp := discoveredPort{Port: 30080, Service: "frontend", Namespace: "app"}
	if got := dp.Label(); got != "app/frontend (NodePort 30080)" {
		t.Errorf("unexpected label: %q", got)
	}

	dp2 := discoveredPort{Port: 8080}
	if got := dp2.Label(); got != "8080" {
		t.Errorf("unexpected label: %q", got)
	}
}
