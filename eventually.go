package bdd

import (
	"fmt"
	"time"
)

const assertionPollInterval = 10 * time.Millisecond

func parseAssertionDuration(value, kind string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid %s %q: expected a positive Go duration", kind, value)
	}
	return duration, nil
}

func waitForAssertion(timeoutValue, description string, assertion func() error) error {
	timeout, err := parseAssertionDuration(timeoutValue, "assertion timeout")
	if err != nil {
		return err
	}

	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		lastErr = assertion()
		if lastErr == nil {
			return nil
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("%s did not become true within %s: %w", description, timeout, lastErr)
		}
		delay := assertionPollInterval
		if remaining < delay {
			delay = remaining
		}
		timer := time.NewTimer(delay)
		<-timer.C
	}
}

func assertForDuration(durationValue, description string, assertion func() error) error {
	duration, err := parseAssertionDuration(durationValue, "assertion duration")
	if err != nil {
		return err
	}

	deadline := time.Now().Add(duration)
	for {
		if err := assertion(); err != nil {
			return fmt.Errorf("%s did not remain true for %s: %w", description, duration, err)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil
		}
		delay := assertionPollInterval
		if remaining < delay {
			delay = remaining
		}
		timer := time.NewTimer(delay)
		<-timer.C
	}
}
