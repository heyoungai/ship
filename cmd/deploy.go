package cmd

import (
	"ship/internal"
	"fmt"

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
		return doDeploy(ver)
	},
}

func init() {
	deployCmd.Flags().StringVarP(&deployVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
}

// doDeploy 远程部署：更新版本号并重启容器
func doDeploy(version string) error {
	fmt.Printf("%s 开始远程部署 version=%s...\n",
		internal.StepStyle.Render("🛰️"), version)

	sedCmd := fmt.Sprintf(
		"sed -i 's/^APP_IMAGE_TAG=.*/APP_IMAGE_TAG=%s/' %s/.env",
		version, cfg.Deploy.Path,
	)
	if err := internal.RunCmd(
		[]string{"ssh", cfg.Deploy.Host, sedCmd},
		"更新远程 .env 文件...",
	); err != nil {
		return err
	}

	restartCmd := fmt.Sprintf("cd %s && docker compose up -d", cfg.Deploy.Path)
	if err := internal.RunCmd(
		[]string{"ssh", cfg.Deploy.Host, restartCmd},
		"重启远程容器...",
	); err != nil {
		return err
	}

	fmt.Printf("%s 远程部署完成\n", internal.SuccessStyle.Render("✅"))
	return nil
}
