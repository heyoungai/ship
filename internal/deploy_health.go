package internal

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DeployHealthcheck 定义部署完成后的可用性探测配置。
type DeployHealthcheck struct {
	URL             string `toml:"url"`
	ExpectedStatus  int    `toml:"expected_status"`
	Attempts        int    `toml:"attempts"`
	IntervalSeconds int    `toml:"interval_seconds"`
	TimeoutSeconds  int    `toml:"timeout_seconds"`
}

// ApplyDefaults 为健康检查填充安全默认值。
func (h *DeployHealthcheck) ApplyDefaults() {
	if h.ExpectedStatus == 0 {
		h.ExpectedStatus = http.StatusOK
	}
	if h.Attempts == 0 {
		h.Attempts = 20
	}
	if h.IntervalSeconds == 0 {
		h.IntervalSeconds = 3
	}
	if h.TimeoutSeconds == 0 {
		h.TimeoutSeconds = 5
	}
}

// Enabled 返回健康检查是否启用。
func (h DeployHealthcheck) Enabled() bool {
	return strings.TrimSpace(h.URL) != ""
}

// WaitForHealthcheck 轮询健康检查地址，直到返回期望状态码或超时失败。
func WaitForHealthcheck(h DeployHealthcheck) error {
	if !h.Enabled() {
		return nil
	}

	h.ApplyDefaults()
	client := &http.Client{Timeout: time.Duration(h.TimeoutSeconds) * time.Second}
	var lastErr error

	for attempt := 1; attempt <= h.Attempts; attempt++ {
		PrintInfo(fmt.Sprintf("healthcheck attempt %d/%d: %s", attempt, h.Attempts, h.URL))
		resp, err := client.Get(h.URL)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == h.ExpectedStatus {
				PrintSuccess(fmt.Sprintf("healthcheck ready: %s status=%d", h.URL, resp.StatusCode))
				return nil
			}
			lastErr = fmt.Errorf("attempt=%d/%d status=%d expected=%d", attempt, h.Attempts, resp.StatusCode, h.ExpectedStatus)
		} else {
			lastErr = fmt.Errorf("attempt=%d/%d request error: %w", attempt, h.Attempts, err)
		}
		PrintWarning(fmt.Sprintf("healthcheck pending: %v", lastErr))

		if attempt < h.Attempts {
			time.Sleep(time.Duration(h.IntervalSeconds) * time.Second)
		}
	}

	return fmt.Errorf("健康检查失败: url=%s expected=%d attempts=%d timeout=%ds last_error=%w", h.URL, h.ExpectedStatus, h.Attempts, h.TimeoutSeconds, lastErr)
}
