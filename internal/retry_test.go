package internal

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestRetryNetwork_RetriesTransientErrorWithExponentialDelay(t *testing.T) {
	originalSleep := retrySleep
	defer func() { retrySleep = originalSleep }()
	var delays []time.Duration
	retrySleep = func(delay time.Duration) { delays = append(delays, delay) }

	attempts := 0
	err := RetryNetwork(RetryConfig{MaxAttempts: 3, InitialDelaySeconds: 2, MaxDelaySeconds: 20}, "push", func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("connection reset by peer")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RetryNetwork() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if want := []time.Duration{2 * time.Second, 4 * time.Second}; !reflect.DeepEqual(delays, want) {
		t.Fatalf("delays = %v, want %v", delays, want)
	}
}

func TestRetryNetwork_DoesNotRetryLogicalError(t *testing.T) {
	originalSleep := retrySleep
	defer func() { retrySleep = originalSleep }()
	retrySleep = func(time.Duration) { t.Fatal("logical error should not sleep") }

	want := errors.New("permission denied")
	attempts := 0
	err := RetryNetwork(RetryConfig{MaxAttempts: 3, InitialDelaySeconds: 1, MaxDelaySeconds: 2}, "deploy", func() error {
		attempts++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("RetryNetwork() error = %v, want %v", err, want)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestIsTransientNetworkErrorReadsCommandOutput(t *testing.T) {
	err := &CommandError{Err: errors.New("exit status 1"), Output: "received HTTP 429 from registry"}
	if !IsTransientNetworkError(err) {
		t.Fatal("expected CommandError output to be recognized as transient")
	}
}

func TestRetryNetwork_StopsAfterMaxAttempts(t *testing.T) {
	originalSleep := retrySleep
	defer func() { retrySleep = originalSleep }()
	retrySleep = func(time.Duration) {}
	attempts := 0
	err := RetryNetwork(RetryConfig{MaxAttempts: 2, InitialDelaySeconds: 1, MaxDelaySeconds: 1}, "push", func() error {
		attempts++
		return errors.New("i/o timeout")
	})
	if err == nil || attempts != 2 {
		t.Fatalf("RetryNetwork() err=%v attempts=%d, want final error after 2 attempts", err, attempts)
	}
}

func TestRetryConfigDefaultsAndValidation(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	if cfg.Retry.MaxAttempts != 3 || cfg.Retry.InitialDelaySeconds != 2 || cfg.Retry.MaxDelaySeconds != 20 {
		t.Fatalf("retry defaults = %#v", cfg.Retry)
	}
	cfg.Retry = RetryConfig{MaxAttempts: 0, InitialDelaySeconds: 3, MaxDelaySeconds: 2}
	var missing []string
	cfg.validateRuntimeOptions(&missing)
	if len(missing) != 2 {
		t.Fatalf("validation messages = %v, want two retry errors", missing)
	}
}

func TestRetryConfigLoadsFromTOML(t *testing.T) {
	withTempConfigDir(t, map[string]string{
		"ship.toml": `
schema = 2

[retry]
max_attempts = 5
initial_delay_seconds = 1
max_delay_seconds = 8

[build]
driver = "command"

[build.command]
run = "true"

[publish]
driver = "none"

[deploy]
driver = "none"
`,
	}, func() {
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if got := cfg.Retry; got.MaxAttempts != 5 || got.InitialDelaySeconds != 1 || got.MaxDelaySeconds != 8 {
			t.Fatalf("retry config = %#v", got)
		}
	})
}
