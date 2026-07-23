package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/heyoungai/ship/internal"
)

// skillInstallRelPath 是 ship skill 安装到项目内的相对路径。
const skillInstallRelPath = ".claude/skills/ship/SKILL.md"

// ExpectedSkillVersion 返回当前 ship 二进制版本，作为已安装 skill 应对齐的版本。
func ExpectedSkillVersion() string {
	return normalizeSkillVersion(Version)
}

// ParseSkillDocVersion 从 SKILL.md 内容解析 YAML frontmatter 中的 version（字符串，通常为 ship 版本如 v2.6.1）。
// 无 frontmatter、无 version 字段或值为空时返回 found=false。
func ParseSkillDocVersion(content []byte) (version string, found bool) {
	trimmed := strings.TrimSpace(string(content))
	if !strings.HasPrefix(trimmed, "---") {
		return "", false
	}

	body := strings.TrimPrefix(trimmed, "---")
	endIdx := strings.Index(body, "\n---")
	if endIdx < 0 {
		return "", false
	}
	frontmatter := body[:endIdx]

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "version" {
			continue
		}
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if val == "" {
			return "", false
		}
		return normalizeSkillVersion(val), true
	}
	return "", false
}

// stampSkillVersion 将 frontmatter 中的 version 设为 ship 当前版本；若无 version 字段则插入。
func stampSkillVersion(content []byte, version string) ([]byte, error) {
	version = normalizeSkillVersion(version)
	if version == "" {
		return nil, fmt.Errorf("ship version 为空，无法写入 skill")
	}

	trimmed := strings.TrimSpace(string(content))
	if !strings.HasPrefix(trimmed, "---") {
		return nil, fmt.Errorf("SKILL.md 缺少 YAML frontmatter")
	}

	body := strings.TrimPrefix(trimmed, "---")
	endIdx := strings.Index(body, "\n---")
	if endIdx < 0 {
		return nil, fmt.Errorf("SKILL.md frontmatter 未正确结束")
	}

	frontmatter := body[:endIdx]
	rest := body[endIdx:] // starts with \n---

	lines := strings.Split(frontmatter, "\n")
	replaced := false
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}
		key, _, ok := strings.Cut(trimmedLine, ":")
		if !ok || strings.TrimSpace(key) != "version" {
			continue
		}
		// 保留原缩进
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + "version: " + version
		replaced = true
		break
	}
	if !replaced {
		// 插到 frontmatter 首行之后（跳过可能的空首行）
		insertAt := 0
		if len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
			insertAt = 1
		}
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:insertAt]...)
		newLines = append(newLines, "version: "+version)
		newLines = append(newLines, lines[insertAt:]...)
		lines = newLines
	}

	out := "---" + strings.Join(lines, "\n") + rest
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out), nil
}

func normalizeSkillVersion(v string) string {
	return strings.TrimSpace(v)
}

// skillVersionOutdated 在已安装 version 与当前 ship 版本不一致时视为过时。
func skillVersionOutdated(installed, expected string) bool {
	return normalizeSkillVersion(installed) != normalizeSkillVersion(expected)
}

// warnIfInstalledSkillOutdated 若 InvocationRoot 下已安装 skill 且版本与当前 ship 不一致，则警告。
// 未安装 skill 时不提示（并非所有项目使用 agent skill）。
func warnIfInstalledSkillOutdated(invocationRoot string) {
	expected := ExpectedSkillVersion()
	if expected == "" {
		return
	}

	path := filepath.Join(invocationRoot, skillInstallRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	installed, found := ParseSkillDocVersion(data)
	switch {
	case !found:
		internal.PrintWarning(fmt.Sprintf(
			"检测到过时的 ship skill（%s 无 version），请运行: ship skill -f（当前 ship=%s）",
			skillInstallRelPath, expected,
		))
	case skillVersionOutdated(installed, expected):
		internal.PrintWarning(fmt.Sprintf(
			"检测到过时的 ship skill（%s ≠ ship %s），请运行: ship skill -f",
			installed, expected,
		))
	}
}
