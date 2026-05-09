package cmd

import (
	"ship/internal"
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示当前 git tag 版本号",
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
