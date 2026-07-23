package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/heyoungai/ship/internal"
	"github.com/spf13/cobra"
)

//go:embed skills/SKILL.md
var skillFS embed.FS

var skillForce bool

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "安装 ship agent skill 到当前项目的 .claude/skills/ship/",
	Long:  "将 ship 工具的 SKILL.md 安装到 .claude/skills/ship/SKILL.md，供 Claude Code 等 AI agent 使用。",
	RunE: func(cmd *cobra.Command, args []string) error {
		return installSkill()
	},
}

func init() {
	skillCmd.Flags().BoolVarP(&skillForce, "force", "f", false, "强制覆盖已存在的 SKILL.md")
}

// installSkill 将嵌入的 SKILL.md 写入目标路径
func installSkill() error {
	const targetDir = ".claude/skills/ship"
	const targetFile = "SKILL.md"

	targetPath := filepath.Join(targetDir, targetFile)

	// 检查目标文件是否已存在
	if _, err := os.Stat(targetPath); err == nil && !skillForce {
		confirmed, confirmErr := confirmAction("检测到已有 SKILL.md，是否覆盖？")
		if confirmErr != nil {
			internal.PrintWarning("SKILL.md 已存在，使用 --force 或 --yes 覆盖")
			return nil
		}
		if !confirmed {
			internal.PrintWarning("已取消安装")
			return nil
		}
	}

	// 读取嵌入的 SKILL.md
	data, err := skillFS.ReadFile("skills/SKILL.md")
	if err != nil {
		return fmt.Errorf("读取嵌入的 SKILL.md 失败: %w", err)
	}

	// 创建目标目录
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目录 %s 失败: %w", targetDir, err)
	}

	// 写入文件
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("写入 SKILL.md 失败: %w", err)
	}

	internal.PrintSuccess(fmt.Sprintf("已安装 ship agent skill → %s", targetPath))
	if v, err := EmbeddedSkillVersion(); err == nil {
		internal.PrintInfo(fmt.Sprintf("skill version = %d", v))
	}
	internal.PrintInfo("Claude Code 等 agent 现在可以使用 ship 工具进行构建和部署了")
	return nil
}
