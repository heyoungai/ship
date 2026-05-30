package internal

import (
	"errors"
	"testing"
)

// TestResolveVersion_CommandArgumentWins 验证显式命令参数优先级最高。
func TestResolveVersion_CommandArgumentWins(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	got, err := resolveVersionWithLookup(cfg, "v9.9.9", func(string) string {
		return "ignored"
	}, func() (string, error) {
		return "v1.0.0", nil
	})
	if err != nil {
		t.Fatalf("resolveVersionWithLookup(command arg) error: %v", err)
	}
	if got != "v9.9.9" {
		t.Fatalf("resolveVersionWithLookup(command arg) = %q, want %q", got, "v9.9.9")
	}
}

// TestResolveVersion_OverrideEnvWins 验证 override_env 可覆盖自动来源。
func TestResolveVersion_OverrideEnvWins(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Version.Source = "git-tag"

	got, err := resolveVersionWithLookup(cfg, "", func(key string) string {
		if key == "SHIP_VERSION" {
			return "v2.3.4"
		}
		return ""
	}, func() (string, error) {
		return "v1.0.0", nil
	})
	if err != nil {
		t.Fatalf("resolveVersionWithLookup(override env) error: %v", err)
	}
	if got != "v2.3.4" {
		t.Fatalf("resolveVersionWithLookup(override env) = %q, want %q", got, "v2.3.4")
	}
}

// TestResolveVersion_SourceEnv 验证 version.source=env 会从 override_env 读取版本。
func TestResolveVersion_SourceEnv(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Version.Source = "env"

	got, err := resolveVersionWithLookup(cfg, "", func(key string) string {
		if key == "SHIP_VERSION" {
			return "v3.0.0"
		}
		return ""
	}, func() (string, error) {
		return "", errors.New("should not call latestTag")
	})
	if err != nil {
		t.Fatalf("resolveVersionWithLookup(source env) error: %v", err)
	}
	if got != "v3.0.0" {
		t.Fatalf("resolveVersionWithLookup(source env) = %q, want %q", got, "v3.0.0")
	}
}

// TestResolveVersion_SourceStatic 验证 version.source=static 使用静态版本。
func TestResolveVersion_SourceStatic(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Version.Source = "static"
	cfg.Version.Static = "v5.0.0"

	got, err := resolveVersionWithLookup(cfg, "", func(string) string {
		return ""
	}, func() (string, error) {
		return "", errors.New("should not call latestTag")
	})
	if err != nil {
		t.Fatalf("resolveVersionWithLookup(source static) error: %v", err)
	}
	if got != "v5.0.0" {
		t.Fatalf("resolveVersionWithLookup(source static) = %q, want %q", got, "v5.0.0")
	}
}

// TestResolveVersion_FallbackDev 验证主来源失败时可回退到 dev。
func TestResolveVersion_FallbackDev(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Version.Source = "git-tag"
	cfg.Version.Fallback = "dev"

	got, err := resolveVersionWithLookup(cfg, "", func(string) string {
		return ""
	}, func() (string, error) {
		return "", errors.New("git tag missing")
	})
	if err != nil {
		t.Fatalf("resolveVersionWithLookup(fallback dev) error: %v", err)
	}
	if got != "dev" {
		t.Fatalf("resolveVersionWithLookup(fallback dev) = %q, want %q", got, "dev")
	}
}

// TestResolveVersion_FallbackStatic 验证主来源失败时可回退到静态版本。
func TestResolveVersion_FallbackStatic(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Version.Source = "git-tag"
	cfg.Version.Fallback = "static"
	cfg.Version.Static = "v7.0.0"

	got, err := resolveVersionWithLookup(cfg, "", func(string) string {
		return ""
	}, func() (string, error) {
		return "", errors.New("git tag missing")
	})
	if err != nil {
		t.Fatalf("resolveVersionWithLookup(fallback static) error: %v", err)
	}
	if got != "v7.0.0" {
		t.Fatalf("resolveVersionWithLookup(fallback static) = %q, want %q", got, "v7.0.0")
	}
}

// TestResolveVersion_FallbackError 验证 fallback=error 时返回原始错误。
func TestResolveVersion_FallbackError(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Version.Source = "git-tag"
	cfg.Version.Fallback = "error"

	_, err := resolveVersionWithLookup(cfg, "", func(string) string {
		return ""
	}, func() (string, error) {
		return "", errors.New("git tag missing")
	})
	if err == nil {
		t.Fatal("resolveVersionWithLookup(fallback error) should return error")
	}
	if err.Error() != "git tag missing" {
		t.Fatalf("resolveVersionWithLookup(fallback error) = %v, want git tag missing", err)
	}
}
