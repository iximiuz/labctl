package retry

import (
	"context"
	"errors"
	"time"
)

type unrecoverableError struct {
	error
}

func (e unrecoverableError) Unwrap() error {
	return e.error
}

// Unrecoverable marks an error as permanent: UntilSuccess stops retrying
// and returns the original error immediately.
func Unrecoverable(err error) error {
	if err == nil {
		return nil
	}
	return unrecoverableError{err}
}

func UntilSuccess(ctx context.Context, do func() error, maxAttempts int, delay time.Duration) error {
	var err error
	for i := 0; i < maxAttempts; i++ {
		err = do()
		if err == nil {
			return nil
		}

		var unrec unrecoverableError
		if errors.As(err, &unrec) {
			return unrec.error
		}

		select {
		case <-ctx.Done():
			return err

		case <-time.After(delay):
		}
	}
	return err
}
