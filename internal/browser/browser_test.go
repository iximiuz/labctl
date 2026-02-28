package browser

import (
	"errors"
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

// stubOpen replaces openFunc for the duration of a test and restores it on cleanup.
func stubOpen(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := openFunc
	openFunc = fn
	t.Cleanup(func() { openFunc = orig })
}

func TestOpenWithFallbackMessage_Success(t *testing.T) {
	stubOpen(t, func(string) error { return nil })

	p := &mockPrinter{}
	url := "https://example.com/test"

	OpenWithFallbackMessage(p, url)

	output := p.buf.String()
	require.Contains(t, output, "Opening https://example.com/test in your browser...")
	assert.NotContains(t, output, "Could not open browser automatically")
}

func TestOpenWithFallbackMessage_FallbackContainsURL(t *testing.T) {
	stubOpen(t, func(string) error { return errors.New("no browser") })

	p := &mockPrinter{}
	url := "https://example.com/auth/sessions/abc123"

	OpenWithFallbackMessage(p, url)

	output := p.buf.String()

	assert.Contains(t, output, "Opening "+url+" in your browser...")
	assert.Contains(t, output, "Could not open browser automatically")
	assert.Contains(t, output, "Please open this URL in your browser:")
	// The URL should appear on its own indented line for easy copying.
	assert.Contains(t, output, "  "+url)
}
