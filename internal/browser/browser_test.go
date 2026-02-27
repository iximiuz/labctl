package browser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectWSL(t *testing.T) {
	tests := []struct {
		name        string
		procVersion string
		want        bool
	}{
		{
			name:        "WSL2 standard",
			procVersion: "Linux version 5.15.167.4-microsoft-standard-WSL2 (root@1234) (gcc 12.3.0) #1 SMP",
			want:        true,
		},
		{
			name:        "WSL1 with Microsoft",
			procVersion: "Linux version 4.4.0-19041-Microsoft (Microsoft@Microsoft.com)",
			want:        true,
		},
		{
			name:        "lowercase microsoft",
			procVersion: "Linux version 5.10.0-microsoft-standard",
			want:        true,
		},
		{
			name:        "contains WSL only",
			procVersion: "Linux version 5.15.0-WSL2-custom (builder@host)",
			want:        true,
		},
		{
			name:        "normal Linux kernel",
			procVersion: "Linux version 6.1.0-18-amd64 (debian-kernel@lists.debian.org) (gcc-12 (Debian 12.2.0-14) 12.2.0)",
			want:        false,
		},
		{
			name:        "empty string",
			procVersion: "",
			want:        false,
		},
		{
			name:        "macOS-like string",
			procVersion: "Darwin Kernel Version 23.4.0: Fri Mar 15 00:12:49 PDT 2024",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectWSL(tt.procVersion)
			assert.Equal(t, tt.want, got)
		})
	}
}

type mockPrinter struct {
	buf strings.Builder
}

func (m *mockPrinter) PrintAux(format string, a ...any) {
	fmt.Fprintf(&m.buf, format, a...)
}

func TestOpenWithFallbackMessage_Success(t *testing.T) {
	// We can't easily test actual browser opening, but we can verify
	// that the "Opening..." message is always printed.
	p := &mockPrinter{}
	url := "https://example.com/test"

	// This will attempt to open a real browser, which will likely fail in CI.
	// The important thing is the output format.
	OpenWithFallbackMessage(p, url)

	output := p.buf.String()
	require.Contains(t, output, "Opening https://example.com/test in your browser...")
}

func TestOpenWithFallbackMessage_FallbackContainsURL(t *testing.T) {
	p := &mockPrinter{}
	url := "https://example.com/auth/sessions/abc123"

	OpenWithFallbackMessage(p, url)

	output := p.buf.String()

	// Regardless of whether browser opened, the URL must appear at least once
	// (in the "Opening..." line). If it failed, it appears again in the fallback.
	assert.Contains(t, output, url)

	// If the browser failed to open (likely in CI/testing), verify the fallback format.
	if strings.Contains(output, "Could not open browser automatically") {
		assert.Contains(t, output, "Please open this URL in your browser:")
		// The URL should appear on its own indented line for easy copying.
		assert.Contains(t, output, "  "+url)
	}
}
