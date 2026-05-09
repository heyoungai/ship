package cmd

import (
	"fmt"
	"ship/internal"
	"strings"

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
			return recordDeploymentResult(err, ver, "deploy", "fail", err.Error())
		}
		return recordDeploymentResult(nil, ver, "deploy", "success", "")
	},
}

func init() {
	deployCmd.Flags().StringVarP(&deployVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
}

// doDeploy 远程部署：更新版本号并重启容器
func doDeploy(version string) error {
	deployPath := strings.TrimRight(cfg.Deploy.Path, "/")
	envFile := deployPath + "/.env"
	tagLine := "APP_IMAGE_TAG=" + version
	updateEnvCmd := fmt.Sprintf(
		"set -e; env_file=%s; tmp_file=\"${env_file}.ship.tmp\"; if [ -f \"$env_file\" ]; then grep -v '^APP_IMAGE_TAG=' \"$env_file\" > \"$tmp_file\"; else : > \"$tmp_file\"; fi; printf '%%s\\n' %s >> \"$tmp_file\"; mv \"$tmp_file\" \"$env_file\"",
		internal.ShellEscape(envFile),
		internal.ShellEscape(tagLine),
	)
	internal.ProgressSub(fmt.Sprintf("ssh %s: 更新 .env", cfg.Deploy.Host))
	if err := internal.RunCmd(
		[]string{"ssh", cfg.Deploy.Host, updateEnvCmd},
		fmt.Sprintf("ssh %s: 更新 .env", cfg.Deploy.Host),
	); err != nil {
		return err
	}

	restartCmd := fmt.Sprintf("set -e; cd %s && docker compose up -d", internal.ShellEscape(deployPath))
	internal.ProgressSub(fmt.Sprintf("ssh %s: docker compose up", cfg.Deploy.Host))
	if err := internal.RunCmd(
		[]string{"ssh", cfg.Deploy.Host, restartCmd},
		fmt.Sprintf("ssh %s: docker compose up", cfg.Deploy.Host),
	); err != nil {
		return err
	}

	if cfg.Deploy.Healthcheck.Enabled() {
		internal.ProgressSub(fmt.Sprintf("healthcheck %s", cfg.Deploy.Healthcheck.URL))
		if err := internal.WaitForHealthcheck(cfg.Deploy.Healthcheck); err != nil {
			return err
		}
		fmt.Printf("  %s 健康检查通过\n", internal.SuccessStyle.Render("✔"))
	}

	return nil
}
