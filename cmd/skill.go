package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/heyoungai/ship/internal"
	"github.com/spf13/cobra"
)

//go:embed skills/*
var skillFS embed.FS

var skillForce bool

const skillTargetDir = ".claude/skills/ship"

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "安装 ship agent skill 到当前项目的 .claude/skills/ship/",
	Long:  "将嵌入的 skill 目录（SKILL.md、REFERENCE.md、EXAMPLES.md）安装到 .claude/skills/ship/，供 Claude Code 等 AI agent 使用。",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := installSkill(".")
		if err == errSkillInstallCancelled {
			return nil
		}
		return err
	},
}

func init() {
	skillCmd.Flags().BoolVarP(&skillForce, "force", "f", false, "强制覆盖已存在的 skill 文件")
}

// installSkill 将嵌入的 skills/* 写入 targetRoot 下的 .claude/skills/ship/。
// 仅对 SKILL.md 盖戳当前 ship 版本。
func installSkill(targetRoot string) error {
	targetDir := filepath.Join(targetRoot, skillTargetDir)

	if err := confirmSkillOverwrite(targetDir); err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目录 %s 失败: %w", targetDir, err)
	}

	written, err := writeEmbeddedSkills(targetDir)
	if err != nil {
		return err
	}
	if len(written) == 0 {
		return fmt.Errorf("嵌入的 skill 目录为空")
	}

	internal.PrintSuccess(fmt.Sprintf("已安装 ship agent skill → %s (%s)", targetDir, strings.Join(written, ", ")))
	internal.PrintInfo(fmt.Sprintf("skill version = %s（对齐 ship %s）", ExpectedSkillVersion(), Version))
	internal.PrintInfo("Claude Code 等 agent 现在可以使用 ship 工具进行构建和部署了")
	return nil
}

func confirmSkillOverwrite(targetDir string) error {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("检查目录 %s 失败: %w", targetDir, err)
	}
	if len(entries) == 0 {
		return nil
	}
	if skillForce {
		return nil
	}
	confirmed, confirmErr := confirmAction("检测到已有 ship skill，是否覆盖整个目录？")
	if confirmErr != nil {
		internal.PrintWarning("ship skill 已存在，使用 --force 或 --yes 覆盖")
		return errSkillInstallCancelled
	}
	if !confirmed {
		internal.PrintWarning("已取消安装")
		return errSkillInstallCancelled
	}
	return nil
}

// errSkillInstallCancelled 表示用户取消或非交互环境未确认。
var errSkillInstallCancelled = fmt.Errorf("skill install cancelled")

func writeEmbeddedSkills(targetDir string) ([]string, error) {
	var written []string
	err := fs.WalkDir(skillFS, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// embed.FS 路径始终使用正斜杠
		rel := strings.TrimPrefix(path, "skills/")
		if rel == path || rel == "" {
			return nil
		}
		data, err := skillFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("读取嵌入文件 %s 失败: %w", path, err)
		}
		if strings.EqualFold(filepath.Base(rel), "SKILL.md") {
			data, err = stampSkillVersion(data, ExpectedSkillVersion())
			if err != nil {
				return fmt.Errorf("写入 skill version 失败: %w", err)
			}
		}
		outPath := filepath.Join(targetDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", outPath, err)
		}
		written = append(written, filepath.Base(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return written, nil
}
