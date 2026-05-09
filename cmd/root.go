package cmd

import (
	"fmt"
	"os"
	"ship/internal"

	"github.com/spf13/cobra"
)

var cfg *internal.Config

var rootCmd = &cobra.Command{
	Use:   "ship",
	Short: "Docker 镜像构建、推送和远程部署 CLI 工具",
	Long:  "支持独立执行 build / tag / push / deploy 各阶段，也可 run 一键全流程。\n支持 ship.toml 配置文件和矩阵构建（多品牌/多变体）。",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 不需要配置的命令
		switch cmd.Name() {
		case "init", "version", "current", "history", "help", "completion":
			return
		}
		cfg = internal.LoadConfig(os.Getenv("IMAGE_NAME"))
	},
}

// Execute 执行根命令
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(currentCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(historyCmd)
}

// printHeader 打印阶段标题（轻量样式：▸ text）
func printHeader(text string) {
	fmt.Printf("\n  %s %s\n", internal.HeaderStyle.Render("▸"), internal.HeaderStyle.Render(text))
}

// printStep 打印带编号的阶段标题（如 [1/4] 构建镜像）
func printStep(total, current int, text string) {
	num := internal.StepNumStyle.Render(fmt.Sprintf("[%d/%d]", current, total))
	fmt.Printf("\n  %s %s\n", internal.HeaderStyle.Render("▸"), num+" "+internal.HeaderStyle.Render(text))
}

// printDone 打印完成标题（✔ text）
func printDone(text string) {
	fmt.Printf("\n  %s %s\n", internal.SuccessTagStyle.Render("✔"), internal.SuccessTagStyle.Render(text))
}
