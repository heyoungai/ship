package cmd

import (
	"fmt"

	"github.com/heyoungai/ship/internal"

	"github.com/spf13/cobra"
)

var rollbackVersion string

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "回滚到上一个成功部署的版本",
	Long:  "回滚远程部署版本。不指定版本时自动回滚到上一个成功部署的版本。",
	RunE: func(cmd *cobra.Command, args []string) error {
		return doRollback(cfg)
	},
}

func init() {
	rollbackCmd.Flags().StringVarP(&rollbackVersion, "version", "v", "", "指定回滚版本 (默认回滚到上一个成功版本)")
}

// doRollback 执行回滚操作
func doRollback(cfg *internal.Config) error {
	if cfg.Deploy.Driver != "compose" {
		return fmt.Errorf("rollback 当前仅支持 deploy.driver = compose，当前为 %s", cfg.Deploy.Driver)
	}

	var targetVersion string
	if rollbackVersion != "" {
		targetVersion = rollbackVersion
	} else {
		currentVersion, err := internal.ResolveVersion(cfg, "")
		if err != nil {
			fmt.Printf("  %s 无法确定当前版本\n", internal.ErrorStyle.Render("✖"))
			return err
		}

		targetVersion, err = internal.GetPreviousVersion(currentVersion)
		if err != nil {
			fmt.Printf("  %s %v\n", internal.ErrorStyle.Render("✖"), err)
			return err
		}
	}

	fmt.Printf("  %s 回滚到版本: %s\n",
		internal.WarnStyle.Render("▸"),
		internal.BoldStyle.Render(targetVersion))
	confirmed, err := confirmAction(fmt.Sprintf("确认回滚到 %s 吗？", targetVersion))
	if err != nil {
		return err
	}
	if !confirmed {
		internal.PrintWarning("已取消回滚")
		return nil
	}

	session, err := prepareReleaseSession(cfg, targetVersion, false)
	if err != nil {
		return err
	}
	defer session.Close()

	manifest, err := internal.RequireReleaseManifest(session.StateRoot(), session.Version())
	if err != nil {
		return fmt.Errorf("回滚需要已发布的 release manifest: %w", err)
	}
	session.Manifest = manifest
	meta := historyMetaFromSession(session)

	profile := cfg.DefaultProfile()
	if err := executeDeployStage(cfg, session.Version(), profile, session); err != nil {
		return recordDeploymentResult(err, session.Version(), "rollback", "fail", err.Error(), meta)
	}
	if err := internal.ExecuteVerify(cfg, profile, session.Version()); err != nil {
		return recordDeploymentResult(err, session.Version(), "rollback", "fail", err.Error(), meta)
	}

	if err := recordDeploymentResult(nil, session.Version(), "rollback", "success", "", meta); err != nil {
		return err
	}
	internal.PrintSuccess("回滚完成")
	return nil
}
