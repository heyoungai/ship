package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/heyoungai/ship/internal"
)

// skillInstallRelPath 是 ship skill 安装到项目内的相对路径。
const skillInstallRelPath = ".claude/skills/ship/SKILL.md"

var (
	embeddedSkillVersionOnce sync.Once
	embeddedSkillVersionVal  int
	embeddedSkillVersionErr  error
)

// EmbeddedSkillVersion 返回二进制内嵌 SKILL.md 的 frontmatter version。
func EmbeddedSkillVersion() (int, error) {
	embeddedSkillVersionOnce.Do(func() {
		data, err := skillFS.ReadFile("skills/SKILL.md")
		if err != nil {
			embeddedSkillVersionErr = fmt.Errorf("读取嵌入的 SKILL.md 失败: %w", err)
			return
		}
		v, found := ParseSkillDocVersion(data)
		if !found {
			embeddedSkillVersionErr = fmt.Errorf("嵌入的 SKILL.md 缺少 frontmatter version")
			return
		}
		if v <= 0 {
			embeddedSkillVersionErr = fmt.Errorf("嵌入的 SKILL.md version 无效: %d", v)
			return
		}
		embeddedSkillVersionVal = v
	})
	return embeddedSkillVersionVal, embeddedSkillVersionErr
}

// ParseSkillDocVersion 从 SKILL.md 内容解析 YAML frontmatter 中的 version（整数）。
// 无 frontmatter、无 version 字段或无法解析时返回 found=false。
func ParseSkillDocVersion(content []byte) (version int, found bool) {
	text := string(content)
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "---") {
		return 0, false
	}

	body := strings.TrimPrefix(trimmed, "---")
	// 允许 ---\n 后紧跟内容；找结束分隔符
	endIdx := strings.Index(body, "\n---")
	if endIdx < 0 {
		return 0, false
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
		n, err := strconv.Atoi(val)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// warnIfInstalledSkillOutdated 若 InvocationRoot 下已安装 skill 且版本落后于内嵌 skill，则警告。
// 未安装 skill 时不提示（并非所有项目使用 agent skill）。
func warnIfInstalledSkillOutdated(invocationRoot string) {
	expected, err := EmbeddedSkillVersion()
	if err != nil {
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
			"检测到过时的 ship skill（%s 无 version），请运行: ship skill -f（当前 skill version=%d）",
			skillInstallRelPath, expected,
		))
	case installed < expected:
		internal.PrintWarning(fmt.Sprintf(
			"检测到过时的 ship skill（version %d < %d），请运行: ship skill -f",
			installed, expected,
		))
	}
}
