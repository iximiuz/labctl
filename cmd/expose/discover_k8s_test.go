package expose

import (
	"testing"

	"github.com/iximiuz/labctl/api"
)

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
