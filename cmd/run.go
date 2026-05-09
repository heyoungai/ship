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
	runProfile    string
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

		profiles, err := cfg.GetProfiles(runProfile)
		if err != nil {
			return err
		}
		profileNames := make([]string, len(profiles))
		for i, p := range profiles {
			profileNames[i] = internal.FormatProfileName(p)
			if profileNames[i] == "" {
				profileNames[i] = "default"
			}
		}

		totalSteps := 4
		if runSkipDeploy || !cfg.Deploy.Enabled {
			totalSteps = 3
		}
		steps := []string{"构建镜像", "打 Tag", "推送镜像"}
		if !runSkipDeploy && cfg.Deploy.Enabled {
			steps = append(steps, "远程部署")
		}

		internal.PrintBanner(fmt.Sprintf(
			"ship run  version=%s  profiles=%s",
			ver, strings.Join(profileNames, ", "),
		))
		internal.SetProgressTotal(totalSteps)
		internal.PrintRunSummary(
			ver,
			strings.Join(profileNames, ", "),
			runEnvFile,
			totalSteps,
			!runSkipDeploy && cfg.Deploy.Enabled,
		)
		internal.PrintInfo(fmt.Sprintf("plan=%s", strings.Join(steps, " → ")))

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
				return recordDeploymentResult(err, ver, "deploy", "fail", err.Error())
			}
			if err := recordDeploymentResult(nil, ver, "deploy", "success", ""); err != nil {
				return err
			}
		} else if runSkipDeploy {
			internal.PrintWarning("已跳过远程部署")
		}

		internal.PrintSuccess("所有任务已完成")
		return nil
	},
}

func init() {
	runCmd.Flags().StringVarP(&runVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
	runCmd.Flags().StringVar(&runEnvFile, "env-file", "", ".env 文件路径 (默认使用配置)")
	runCmd.Flags().StringVarP(&runProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
	runCmd.Flags().BoolVar(&runSkipDeploy, "skip-deploy", false, "跳过远程部署步骤")
}
