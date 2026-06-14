package api

import (
	"testing"
	"time"
)

func TestRetryAfterSeconds(t *testing.T) {
	now := time.Now().Unix()

	cases := []struct {
		name  string
		reset int64
		want  int
	}{
		{"absolute timestamp 30s in the future", now + 30, 30},
		{"absolute timestamp in the past clamps to 0", now - 120, 0},
		{"relative seconds-from-now", 45, 45},
		{"far-future absolute clamps to max (no overflow)", now + 10_000, maxRetryAfterSeconds},
		{"absurd value cannot overflow time.Duration", 9_000_000_000_000, maxRetryAfterSeconds},
		{"large relative value clamps to max", 999_999, maxRetryAfterSeconds},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := retryAfterSeconds(tc.reset)
			// Allow a 1s slack for the absolute cases (clock advances between
			// computing `now` here and inside the function).
			if got < tc.want-1 || got > tc.want {
				t.Fatalf("retryAfterSeconds(%d) = %d, want ~%d", tc.reset, got, tc.want)
			}
			if d := time.Duration(got) * time.Second; d < 0 {
				t.Fatalf("retryAfterSeconds(%d) produced a negative/overflowed duration %s", tc.reset, d)
			}
		})
	}
}
