package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ship/internal"
	"strings"

	"github.com/spf13/cobra"
)

var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "在当前目录初始化 ship.toml 配置文件",
	RunE: func(cmd *cobra.Command, args []string) error {
		return doInit()
	},
}

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "强制覆盖已存在的 ship.toml")
}

// doInit 在当前目录生成 ship.toml 配置文件
func doInit() error {
	const configFile = "ship.toml"

	// 检查是否已存在
	if _, err := os.Stat(configFile); err == nil && !initForce {
		confirmed, confirmErr := confirmAction("检测到已有 ship.toml，是否覆盖？")
		if confirmErr != nil {
			internal.PrintWarning("ship.toml 已存在，使用 --force 或 --yes 覆盖")
			return nil
		}
		if !confirmed {
			internal.PrintWarning("已取消初始化")
			return nil
		}
	}

	// 自动探测项目信息
	info := detectProject()

	// 生成配置内容
	content := generateConfig(info)

	// 写入 ship.toml
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入 ship.toml 失败: %w", err)
	}
	internal.PrintSuccess("已生成 ship.toml")

	// 将 .ship/ 加入 .gitignore
	ensureGitignore()

	internal.PrintInfo("请检查并修改以下探测结果：")
	internal.PrintKeyValueTable(info)
	internal.PrintInfo("ship.toml 可继续手动编辑，参考 config.example.toml")
	return nil
}

// projectInfo 存储自动探测到的项目信息
type projectInfo struct {
	ImageName     string
	LocalBuild    string
	HasDockerfile bool
	HasEnvFile    bool
	EnvFile       string
}

// detectProject 自动探测当前项目的信息
func detectProject() map[string]string {
	info := make(map[string]string)

	// 1. 推断 image name：优先 git remote，其次目录名
	if name := detectImageFromGitRemote(); name != "" {
		info["镜像名称"] = name + " (从 git remote 推断)"
	} else {
		dir := filepath.Base(mustGetwd())
		info["镜像名称"] = dir + " (从目录名推断)"
	}

	// 2. 检测 Dockerfile
	if _, err := os.Stat("Dockerfile"); err == nil {
		info["Dockerfile"] = "已检测到"
	}

	// 3. 检测本地构建命令
	if cmd := internal.DetectLocalBuild(); cmd != "" {
		info["本地构建"] = cmd
	}

	// 4. 检测 .env 文件
	for _, f := range []string{".env.local", ".env"} {
		if _, err := os.Stat(f); err == nil {
			info["环境文件"] = f
			break
		}
	}

	return info
}

// detectImageFromGitRemote 从 git remote URL 推断镜像名
func detectImageFromGitRemote() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))

	// SSH: git@github.com:user/repo.git → repo
	// HTTPS: https://github.com/user/repo.git → repo
	url = strings.TrimSuffix(url, ".git")
	parts := strings.FieldsFunc(url, func(r rune) bool {
		return r == '/' || r == ':'
	})
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// mustGetwd 获取当前工作目录，失败时返回 "."
func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ensureGitignore 将 .ship/ 添加到 .gitignore（如尚未存在）
func ensureGitignore() {
	const entry = ".ship/"
	const gitignore = ".gitignore"

	data, err := os.ReadFile(gitignore)
	if err == nil && strings.Contains(string(data), entry) {
		// 已存在，跳过
		return
	}

	// 追加到 .gitignore
	var f *os.File
	if err != nil {
		// 文件不存在，创建
		f, err = os.Create(gitignore)
		if err != nil {
			fmt.Printf("  %s 创建 .gitignore 失败: %v\n", internal.WarnStyle.Render("▸"), err)
			return
		}
	} else {
		f, err = os.OpenFile(gitignore, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("  %s 打开 .gitignore 失败: %v\n", internal.WarnStyle.Render("▸"), err)
			return
		}
	}
	defer f.Close()

	// 如果文件非空且末尾没有换行，先加一个换行
	if len(data) > 0 && data[len(data)-1] != '\n' {
		_, _ = f.WriteString("\n")
	}
	_, _ = f.WriteString("\n# ship 部署历史（本机记录，不提交）\n")
	_, _ = f.WriteString(entry + "\n")

	internal.PrintSuccess(fmt.Sprintf("已将 %s 添加到 .gitignore", entry))
}

// generateConfig 根据探测信息生成 ship.toml 内容
func generateConfig(info map[string]string) string {
	imageName := "myapp"
	if v, ok := info["镜像名称"]; ok {
		// 去掉括号里的推断说明
		if idx := strings.Index(v, " ("); idx > 0 {
			imageName = v[:idx]
		}
	}

	envFile := "./.env.local"
	if v, ok := info["环境文件"]; ok {
		envFile = "./" + v
	}

	var b strings.Builder

	b.WriteString("# ship 配置文件\n")
	b.WriteString("# 由 ship init 自动生成，请根据项目实际情况修改\n\n")

	// [build]
	b.WriteString("# ── 构建设置 ────────────────────────────────────────────────\n")
	b.WriteString("[build]\n")
	b.WriteString("platforms = \"linux/amd64\"  # 当前分阶段流程会把镜像 load 回本地，因此这里应保持单平台\n")
	b.WriteString("dockerfile = \"./Dockerfile\"\n")
	b.WriteString(fmt.Sprintf("env_file = \"%s\"\n", envFile))

	if v, ok := info["本地构建"]; ok {
		b.WriteString(fmt.Sprintf("local_build = \"%s\"\n", v))
	}
	b.WriteString("\n")

	// [[registries]]
	b.WriteString("# ── 镜像仓库 ────────────────────────────────────────────────\n")
	b.WriteString("[[registries]]\n")
	b.WriteString("type = \"private\"\n")
	b.WriteString("url = \"registry.cn-hangzhou.aliyuncs.com\"  # 改为你的仓库地址\n")
	b.WriteString("namespace = \"deali\"                        # 改为你的命名空间\n")
	b.WriteString(fmt.Sprintf("image = \"%s\"\n", imageName))
	b.WriteString("\n")

	// [deploy]
	b.WriteString("# ── 远程部署 ────────────────────────────────────────────────\n")
	b.WriteString("[deploy]\n")
	b.WriteString("enabled = false\n")
	b.WriteString("host = \"your-server\"                       # SSH Host\n")
	b.WriteString(fmt.Sprintf("path = \"/home/user/projects/%s\"  # 远程项目路径\n", imageName))
	b.WriteString("\n")
	b.WriteString("# [deploy.healthcheck]\n")
	b.WriteString("# url = \"https://example.com/api/health\"\n")
	b.WriteString("# expected_status = 200\n")
	b.WriteString("# attempts = 20\n")
	b.WriteString("# interval_seconds = 3\n")
	b.WriteString("# timeout_seconds = 5\n")
	b.WriteString("\n")

	// [matrix] 注释示例
	b.WriteString("# ── 矩阵构建（可选）────────────────────────────────────────\n")
	b.WriteString("# [[matrix]]\n")
	b.WriteString("# name = \"brand-a\"\n")
	b.WriteString("# default = true\n")
	b.WriteString("# env = { NEXT_PUBLIC_APP_BRAND = \"brand-a\" }\n")
	b.WriteString("#\n")
	b.WriteString("# [[matrix]]\n")
	b.WriteString("# name = \"brand-b\"\n")
	b.WriteString("# env = { NEXT_PUBLIC_APP_BRAND = \"brand-b\" }\n")

	return b.String()
}
