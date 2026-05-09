package cmd

import (
	"fmt"
	"ship/internal"
	"strings"

	"github.com/spf13/cobra"
)

var (
	runVersion    string
	runEnvFile    string
	runSkipDeploy bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "执行完整流程: build → tag → push → deploy",
	RunE: func(cmd *cobra.Command, args []string) error {
		ver, err := internal.ResolveVersion(runVersion)
		if err != nil {
			return err
		}

		profiles := cfg.GetProfiles("")
		profileNames := make([]string, len(profiles))
		for i, p := range profiles {
			profileNames[i] = internal.FormatProfileName(p)
		}

		totalSteps := 4
		if runSkipDeploy || !cfg.Deploy.Enabled {
			totalSteps = 3
		}

		fmt.Printf("\n  %s\n",
			internal.BoldStyle.Render(fmt.Sprintf(
				"🚀 ship run  version=%s  profiles=%s",
				ver, strings.Join(profileNames, ", "),
			)))

		internal.ProgressInit(totalSteps)

		// 1. Build
		internal.ProgressStep(1, "构建镜像")
		for _, p := range profiles {
			if err := doBuild(p, runEnvFile); err != nil {
				return err
			}
		}

		// 2. Tag
		internal.ProgressStep(2, "打 Tag")
		for _, p := range profiles {
			if err := doTag(ver, p); err != nil {
				return err
			}
		}

		// 3. Push
		internal.ProgressStep(3, "推送镜像")
		for _, p := range profiles {
			if err := doPush(ver, p); err != nil {
				return err
			}
		}

		// 4. Deploy
		if !runSkipDeploy && cfg.Deploy.Enabled {
			internal.ProgressStep(4, "远程部署")
			if err := doDeploy(ver); err != nil {
				internal.RecordDeployment(ver, "deploy", "fail", err.Error())
				return err
			}
			internal.RecordDeployment(ver, "deploy", "success", "")
		} else if runSkipDeploy {
			fmt.Printf("  %s 已跳过远程部署\n", internal.WarnStyle.Render("⏭"))
		}

		fmt.Printf("\n  %s\n", internal.SuccessTagStyle.Render("✔ 所有任务已完成"))
		return nil
	},
}

func init() {
	runCmd.Flags().StringVarP(&runVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
	runCmd.Flags().StringVar(&runEnvFile, "env-file", "", ".env 文件路径 (默认使用配置)")
	runCmd.Flags().BoolVar(&runSkipDeploy, "skip-deploy", false, "跳过远程部署步骤")
}
