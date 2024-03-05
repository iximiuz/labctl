package retry

import (
	"context"
	"time"
)

func UntilSuccess(ctx context.Context, do func() error, maxAttempts int, delay time.Duration) error {
	var err error
	for i := 0; i < maxAttempts; i++ {
		err = do()
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return err

		case <-time.After(delay):
		}
	}
	return err
}
