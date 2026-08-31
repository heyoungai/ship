package internal

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// retrySleep 可在测试中替换，避免真实等待。
var retrySleep = time.Sleep

// RetryNetwork 仅在错误看起来是暂态网络故障时重试 fn。
func RetryNetwork(cfg RetryConfig, label string, fn func() error) error {
	// 配置加载会校验这些值；这里仍为内部调用与测试中的零值配置保留一次执行语义。
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}
	if cfg.InitialDelaySeconds < 0 {
		cfg.InitialDelaySeconds = 0
	}
	if cfg.MaxDelaySeconds < cfg.InitialDelaySeconds {
		cfg.MaxDelaySeconds = cfg.InitialDelaySeconds
	}
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		if attempt == cfg.MaxAttempts || !IsTransientNetworkError(err) {
			return err
		}

		delay := retryDelay(cfg, attempt)
		PrintWarning(fmt.Sprintf("%s 失败（第 %d/%d 次）：%v；%s 后重试", label, attempt, cfg.MaxAttempts, err, delay))
		retrySleep(delay)
	}
	return nil
}

// RunNetworkCmd 在保留实时 stdout/stderr 的同时，为网络命令套上重试。
func RunNetworkCmd(cfg RetryConfig, args []string, label string) error {
	return RetryNetwork(cfg, label, func() error {
		return RunCmd(args, label)
	})
}

func retryDelay(cfg RetryConfig, failedAttempt int) time.Duration {
	delay := time.Duration(cfg.InitialDelaySeconds) * time.Second
	max := time.Duration(cfg.MaxDelaySeconds) * time.Second
	for i := 1; i < failedAttempt && delay < max; i++ {
		delay *= 2
	}
	if delay > max {
		return max
	}
	return delay
}

// IsTransientNetworkError 有意保持保守：无法辨认为网络抖动的失败不重试，
// 避免重复执行具副作用的远端命令。
func IsTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	var commandErr *CommandError
	if errors.As(err, &commandErr) {
		message += "\n" + strings.ToLower(commandErr.Output)
	}
	for _, marker := range []string{
		"timeout", "timed out", "i/o timeout", "connection reset", "connection refused",
		"connection aborted", "connection closed", "network is unreachable", "no route to host",
		"temporary failure", "temporary error", "tls handshake", "unexpected eof", "http 429",
		"too many requests", "service unavailable", "bad gateway", "gateway timeout", " 502", " 503", " 504",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
