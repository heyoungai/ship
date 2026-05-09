package cmd

import (
	"fmt"
	"ship/internal"

	"github.com/spf13/cobra"
)

var deployVersion string

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "远程部署：更新版本号并重启容器",
	RunE: func(cmd *cobra.Command, args []string) error {
		ver, err := internal.ResolveVersion(deployVersion)
		if err != nil {
			return err
		}
		if err := doDeploy(ver); err != nil {
			internal.RecordDeployment(ver, "deploy", "fail", err.Error())
			return err
		}
		internal.RecordDeployment(ver, "deploy", "success", "")
		return nil
	},
}

func init() {
	deployCmd.Flags().StringVarP(&deployVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
}

// doDeploy 远程部署：更新版本号并重启容器
func doDeploy(version string) error {
	fmt.Printf("  %s 远程部署  %s\n",
		internal.StepStyle.Render("▸"),
		internal.BoldStyle.Render("version="+version))

	sedCmd := fmt.Sprintf(
		"sed -i 's/^APP_IMAGE_TAG=.*/APP_IMAGE_TAG=%s/' %s/.env",
		version, cfg.Deploy.Path,
	)
	if err := internal.RunCmd(
		[]string{"ssh", cfg.Deploy.Host, sedCmd},
		fmt.Sprintf("ssh %s: 更新 .env", cfg.Deploy.Host),
	); err != nil {
		return err
	}

	restartCmd := fmt.Sprintf("cd %s && docker compose up -d", cfg.Deploy.Path)
	if err := internal.RunCmd(
		[]string{"ssh", cfg.Deploy.Host, restartCmd},
		fmt.Sprintf("ssh %s: docker compose up", cfg.Deploy.Host),
	); err != nil {
		return err
	}

	fmt.Printf("  %s 远程部署完成\n", internal.SuccessStyle.Render("✔"))
	return nil
}
