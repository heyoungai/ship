package internal

import (
	"fmt"
	"runtime"
	"strings"
)

// ShellCommandArgs 根据当前平台返回执行 shell 命令所需的参数。
// local_build 是字符串命令，这里统一由平台对应的 shell 负责解析。
func ShellCommandArgs(command string) []string {
	args, _ := ShellCommandArgsWithMode("auto", command)
	return args
}

// ShellCommandArgsWithMode 根据显式 shell 模式返回执行命令所需的参数。
func ShellCommandArgsWithMode(shell, command string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "", "auto":
		if runtime.GOOS == "windows" {
			return []string{"powershell", "-NoProfile", "-Command", command}, nil
		}
		return []string{"sh", "-c", command}, nil
	case "powershell", "pwsh":
		return []string{"powershell", "-NoProfile", "-Command", command}, nil
	case "sh", "bash":
		return []string{"sh", "-c", command}, nil
	default:
		return nil, fmt.Errorf("不支持的 shell: %s", shell)
	}
}

// ShellEscape 将任意字符串安全包裹为远端 POSIX shell 单个参数。
func ShellEscape(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// SplitCSV 将逗号分隔配置切成去空白后的字符串切片。
func SplitCSV(input string) []string {
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// BuildxOutputArgs 返回当前分阶段构建流程所需的 buildx 输出参数。
// 当前 build → tag → push 流程依赖本地 Docker 镜像作为阶段输入，因此必须启用 --load。
func BuildxOutputArgs(platforms string, load bool) ([]string, error) {
	if !load {
		return nil, fmt.Errorf(
			"当前 build → tag → push 分阶段流程需要把镜像加载到本地 Docker，因此 build.docker.load 必须为 true",
		)
	}
	if len(SplitCSV(platforms)) != 1 {
		return nil, fmt.Errorf(
			"当前 build → tag → push 流程需要把镜像加载到本地 Docker，因此仅支持单平台构建；当前 platforms=%q",
			platforms,
		)
	}
	return []string{"--load"}, nil
}

// BuildxPullArgs 返回 buildx --pull 相关参数。
// pull=true 时不追加参数，保持 buildx 默认行为；pull=false 时显式传 --pull=false，
// 以便本地已有基础镜像时跳过 registry HEAD/校验（可省开头耗时并避开 mirror 429）。
func BuildxPullArgs(pull bool) []string {
	if pull {
		return nil
	}
	return []string{"--pull=false"}
}
