package cmd

import (
	"ship/internal"
	"fmt"

	"github.com/spf13/cobra"
)

// Version ship 工具自身版本号，编译时通过 -ldflags 注入
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示 ship 工具版本号",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ship %s\n", Version)
	},
}

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "显示当前项目的 git tag 版本号",
	RunE: func(cmd *cobra.Command, args []string) error {
		tag, err := internal.GetLatestTag()
		if err != nil {
			fmt.Printf("%s 没有找到 git tag\n", internal.ErrorStyle.Render("❌"))
			return err
		}
		fmt.Printf("%s 当前版本: %s\n", internal.SuccessStyle.Render("✅"), tag)
		return nil
	},
}
