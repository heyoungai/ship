package cmd

import (
	"ship/internal"
	"fmt"

	"github.com/spf13/cobra"
)

var rollbackVersion string

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "回滚到上一个成功部署的版本",
	Long:  "回滚远程部署版本。不指定版本时自动回滚到上一个成功部署的版本。",
	RunE: func(cmd *cobra.Command, args []string) error {
		return doRollback()
	},
}

func init() {
	rollbackCmd.Flags().StringVarP(&rollbackVersion, "version", "v", "", "指定回滚版本 (默认回滚到上一个成功版本)")
}

// doRollback 执行回滚操作
func doRollback() error {
	// 1. 确定目标版本
	var targetVersion string
	if rollbackVersion != "" {
		targetVersion = rollbackVersion
	} else {
		// 获取当前版本用于查找上一个
		currentVersion, err := internal.ResolveVersion("")
		if err != nil {
			fmt.Printf("%s 无法确定当前版本\n", internal.ErrorStyle.Render("❌"))
			return err
		}

		targetVersion, err = internal.GetPreviousVersion(currentVersion)
		if err != nil {
			fmt.Printf("%s %v\n", internal.ErrorStyle.Render("❌"), err)
			return err
		}
	}

	fmt.Printf("%s 回滚到版本: %s\n", internal.WarnStyle.Render("⏪"), targetVersion)

	// 2. 执行部署
	if err := doDeploy(targetVersion); err != nil {
		internal.RecordDeployment(targetVersion, "rollback", "fail", err.Error())
		return err
	}

	// 3. 记录历史
	internal.RecordDeployment(targetVersion, "rollback", "success", "")
	fmt.Printf("%s 回滚完成\n", internal.SuccessStyle.Render("✅"))
	return nil
}
