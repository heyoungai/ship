package cmd

import (
	"ship/internal"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfg *internal.Config

var rootCmd = &cobra.Command{
	Use:   "ship",
	Short: "Docker 镜像构建、推送和远程部署 CLI 工具",
	Long:  "支持独立执行 build / tag / push / deploy 各阶段，也可 run 一键全流程。\n支持 ship.toml 配置文件和矩阵构建（多品牌/多变体）。",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// init 命令不需要已有配置
		if cmd.Name() == "init" {
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
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(runCmd)
}

// printHeader 打印阶段标题
func printHeader(text string) {
	fmt.Printf("\n%s\n", internal.StepStyle.Render("━━━ "+text+" ━━━"))
}

// printDone 打印完成标题
func printDone(text string) {
	fmt.Printf("\n%s\n", internal.SuccessStyle.Render("━━━ "+text+" ━━━"))
}
