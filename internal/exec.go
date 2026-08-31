package internal

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// CommandError 保留外部命令的退出错误与最近输出，供网络重试识别暂态错误。
// Error 保持简短，避免调用方把已实时打印的日志再输出一遍。
type CommandError struct {
	Err    error
	Output string
}

func (e *CommandError) Error() string {
	if e == nil || e.Err == nil {
		return "command failed"
	}
	return e.Err.Error()
}

func (e *CommandError) Unwrap() error { return e.Err }

// RunCmd 执行外部命令，实时输出 stdout/stderr，失败时返回错误。
func RunCmd(args []string, label string) error {
	return runCmd(args, label, "", nil)
}

// RunCmdWithEnv 执行外部命令，支持注入额外环境变量。
func RunCmdWithEnv(args []string, label string, env map[string]string) error {
	return runCmd(args, label, "", env)
}

// RunCmdWithOptions 执行外部命令，支持工作目录和额外环境变量。
func RunCmdWithOptions(args []string, label, cwd string, env map[string]string) error {
	return runCmd(args, label, cwd, env)
}

func runCmd(args []string, label, cwd string, env map[string]string) error {
	fmt.Printf("  %s %s\n", DimStyle.Render("$"), DimStyle.Render(strings.Join(args, " ")))
	fmt.Printf("  %s %s\n", InfoStyle.Render("·"), label)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = os.Environ()
	var output tailBuffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &output)
	cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	if cwd != "" {
		cmd.Dir = cwd
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("  %s %s  %v\n", ErrorStyle.Render("✖"), label, err)
		return &CommandError{Err: err, Output: output.String()}
	}

	fmt.Printf("  %s %s\n", SuccessStyle.Render("✔"), label)
	return nil
}

// tailBuffer 有界地保存最近命令输出，避免 Docker 构建等大日志无限占用内存。
type tailBuffer struct {
	buf bytes.Buffer
}

const maxCommandOutputForError = 32 * 1024

func (b *tailBuffer) Write(p []byte) (int, error) {
	if len(p) >= maxCommandOutputForError {
		b.buf.Reset()
		_, _ = b.buf.Write(p[len(p)-maxCommandOutputForError:])
		return len(p), nil
	}
	if overflow := b.buf.Len() + len(p) - maxCommandOutputForError; overflow > 0 {
		remaining := append([]byte(nil), b.buf.Bytes()[overflow:]...)
		b.buf.Reset()
		_, _ = b.buf.Write(remaining)
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}

func (b *tailBuffer) String() string { return b.buf.String() }

// GetLatestTag 获取最新的 git tag
func GetLatestTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取 git tag 失败: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveVersion 按命令参数、环境变量和 v2 配置解析版本号。
func ResolveVersion(cfg *Config, version string) (string, error) {
	return resolveVersionWithLookup(cfg, version, os.Getenv, GetLatestTag)
}

// resolveVersionWithLookup 注入环境读取和 git tag 查询，便于测试版本决策逻辑。
func resolveVersionWithLookup(cfg *Config, version string, getenv func(string) string, latestTag func() (string, error)) (string, error) {
	if strings.TrimSpace(version) != "" {
		return version, nil
	}

	if cfg == nil {
		return latestTag()
	}

	overrideEnv := strings.TrimSpace(cfg.Version.OverrideEnv)
	if overrideEnv != "" {
		if override := strings.TrimSpace(getenv(overrideEnv)); override != "" {
			return override, nil
		}
	}

	resolved, err := resolveVersionFromSource(cfg, overrideEnv, getenv, latestTag)
	if err == nil && strings.TrimSpace(resolved) != "" {
		return resolved, nil
	}

	return resolveVersionFallback(cfg, err)
}

// resolveVersionFromSource 根据 version.source 读取版本来源。
func resolveVersionFromSource(cfg *Config, overrideEnv string, getenv func(string) string, latestTag func() (string, error)) (string, error) {
	switch cfg.Version.Source {
	case "", "git-tag":
		return latestTag()
	case "env":
		if overrideEnv == "" {
			return "", errors.New("version.override_env 不能为空")
		}
		resolved := strings.TrimSpace(getenv(overrideEnv))
		if resolved == "" {
			return "", fmt.Errorf("环境变量 %s 未设置", overrideEnv)
		}
		return resolved, nil
	case "static":
		resolved := strings.TrimSpace(cfg.Version.Static)
		if resolved == "" {
			return "", errors.New("version.static 不能为空")
		}
		return resolved, nil
	default:
		return "", fmt.Errorf("不支持的 version.source: %s", cfg.Version.Source)
	}
}

// resolveVersionFallback 在主版本来源失败时应用 fallback 策略。
func resolveVersionFallback(cfg *Config, sourceErr error) (string, error) {
	switch cfg.Version.Fallback {
	case "", "error":
		if sourceErr == nil {
			return "", errors.New("无法解析版本")
		}
		return "", sourceErr
	case "dev":
		return "dev", nil
	case "static":
		resolved := strings.TrimSpace(cfg.Version.Static)
		if resolved == "" {
			if sourceErr == nil {
				return "", errors.New("version.static 不能为空")
			}
			return "", fmt.Errorf("%w; version.static 不能为空", sourceErr)
		}
		return resolved, nil
	default:
		if sourceErr == nil {
			return "", fmt.Errorf("不支持的 version.fallback: %s", cfg.Version.Fallback)
		}
		return "", fmt.Errorf("%w; 不支持的 version.fallback: %s", sourceErr, cfg.Version.Fallback)
	}
}
