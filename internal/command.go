package internal

import (
	"fmt"
	"runtime"
	"strings"
)

// ShellCommandArgs 根据当前平台返回执行 shell 命令所需的参数。
// local_build 是字符串命令，这里统一由平台对应的 shell 负责解析。
func ShellCommandArgs(command string) []string {
	if runtime.GOOS == "windows" {
		return []string{"powershell", "-NoProfile", "-Command", command}
	}
	return []string{"sh", "-c", command}
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
// 这里显式使用 --load，把镜像装载回本地 Docker，后续 tag/push 才有正确输入。
func BuildxOutputArgs(platforms string) ([]string, error) {
	if len(SplitCSV(platforms)) != 1 {
		return nil, fmt.Errorf(
			"当前 build → tag → push 流程需要把镜像加载到本地 Docker，因此仅支持单平台构建；当前 platforms=%q",
			platforms,
		)
	}
	return []string{"--load"}, nil
}
