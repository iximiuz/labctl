package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestUntilSuccessRetriesRegularErrors(t *testing.T) {
	attempts := 0
	err := UntilSuccess(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("transient")
		}
		return nil
	}, 5, time.Millisecond)

	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got: %d", attempts)
	}
}

func TestUntilSuccessStopsOnUnrecoverable(t *testing.T) {
	permanent := errors.New("permanent")

	attempts := 0
	err := UntilSuccess(context.Background(), func() error {
		attempts++
		return Unrecoverable(fmt.Errorf("wrapped: %w", permanent))
	}, 5, time.Millisecond)

	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got: %d", attempts)
	}
	if !errors.Is(err, permanent) {
		t.Fatalf("expected the original error chain, got: %v", err)
	}
	if err.Error() != "wrapped: permanent" {
		t.Fatalf("unexpected error text: %q", err.Error())
	}
}

func TestUnrecoverableNil(t *testing.T) {
	if Unrecoverable(nil) != nil {
		t.Fatal("Unrecoverable(nil) must be nil")
	}
}
