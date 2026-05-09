package cmd

import (
	"ship/internal"
	"fmt"

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

		fmt.Printf("%s\n",
			internal.BoldStyle.Render(fmt.Sprintf(
				"🚀 开始完整流程 (version=%s, profiles=%s)",
				ver, joinStr(profileNames, ", "),
			)))

		// 1. Build
		printHeader("构建镜像")
		for _, p := range profiles {
			if err := doBuild(p, runEnvFile); err != nil {
				return err
			}
		}

		// 2. Tag
		printHeader("打 Tag")
		for _, p := range profiles {
			if err := doTag(ver, p); err != nil {
				return err
			}
		}

		// 3. Push
		printHeader("推送镜像")
		for _, p := range profiles {
			if err := doPush(ver, p); err != nil {
				return err
			}
		}

		// 4. Deploy
		if !runSkipDeploy && cfg.Deploy.Enabled {
			printHeader("远程部署")
			if err := doDeploy(ver); err != nil {
				return err
			}
		} else if runSkipDeploy {
			fmt.Printf("%s 已跳过远程部署\n", internal.WarnStyle.Render("⏭️"))
		}

		printDone("🎉 所有任务已完成！")
		return nil
	},
}

func init() {
	runCmd.Flags().StringVarP(&runVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
	runCmd.Flags().StringVar(&runEnvFile, "env-file", "", ".env 文件路径 (默认使用配置)")
	runCmd.Flags().BoolVar(&runSkipDeploy, "skip-deploy", false, "跳过远程部署步骤")
}

// joinStr 连接字符串切片
func joinStr(slice []string, sep string) string {
	if len(slice) == 0 {
		return ""
	}
	result := slice[0]
	for _, s := range slice[1:] {
		result += sep + s
	}
	return result
}
