package expose

import (
	"testing"
)

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
