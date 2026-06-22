package cmd

import (
	"os"
	"github.com/heyoungai/ship/internal"

	"github.com/spf13/cobra"
)

var cfg *internal.Config

var rootCmd = &cobra.Command{
	Use:          "ship",
	Short:        "Docker 镜像构建、推送和远程部署 CLI 工具",
	Long:         "支持独立执行 build / tag / push / deploy 各阶段，也可 run 一键全流程。\n支持 ship.toml 配置文件和矩阵构建（多品牌/多变体）。",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 不需要配置的命令
		switch cmd.Name() {
		case "init", "version", "current", "history", "skill", "help", "completion":
			return nil
		}
		loadedCfg, err := internal.LoadConfig(os.Getenv("IMAGE_NAME"))
		if err != nil {
			return err
		}
		cfg = loadedCfg
		return nil
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
	rootCmd.AddCommand(skillCmd)
}
