package cmd

import (
	"fmt"
	"github.com/heyoungai/ship/internal"
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
		ver, err := internal.ResolveVersion(cfg, runVersion)
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

		buildStep := buildStepTitle()
		publishStep := publishStepTitle()
		deployStep := deployStepTitle()
		verifyStep := verifyStepTitle()
		shouldTag := cfg.UsesTagStage()
		shouldPublish := cfg.UsesPublishStage()
		shouldDeploy := !runSkipDeploy && cfg.UsesDeployStage()
		shouldVerify := !runSkipDeploy && cfg.UsesVerifyStage()

		steps := []string{buildStep}
		if shouldTag {
			steps = append(steps, "打 Tag")
		}
		if shouldPublish {
			steps = append(steps, publishStep)
		}
		if shouldDeploy {
			steps = append(steps, deployStep)
		}
		if shouldVerify {
			steps = append(steps, verifyStep)
		}
		totalSteps := len(steps)

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
			shouldDeploy,
		)
		internal.PrintInfo(fmt.Sprintf("plan=%s", strings.Join(steps, " → ")))

		currentStep := 1
		internal.ProgressStep(currentStep, buildStep)
		for _, p := range profiles {
			if err := executeBuildProfile(ver, p, runEnvFile); err != nil {
				return err
			}
		}
		currentStep++

		if shouldTag {
			internal.ProgressStep(currentStep, "打 Tag")
			for _, p := range profiles {
				if err := doTag(ver, p); err != nil {
					return err
				}
			}
			currentStep++
		}

		if shouldPublish {
			internal.ProgressStep(currentStep, publishStep)
			for _, p := range profiles {
				if err := executePublishProfile(ver, p); err != nil {
					return err
				}
			}
			currentStep++
		}

		deployProfile := selectDeployProfile(profiles)
		if shouldDeploy {
			internal.ProgressStep(currentStep, deployStep)
			if err := executeDeployStage(ver, deployProfile); err != nil {
				return recordDeploymentResult(err, ver, "deploy", "fail", err.Error())
			}
			currentStep++
		} else if runSkipDeploy {
			internal.PrintWarning("已跳过远程部署")
		}

		if shouldVerify {
			internal.ProgressStep(currentStep, verifyStep)
			if err := internal.ExecuteVerify(cfg, deployProfile, ver); err != nil {
				return recordDeploymentResult(err, ver, "deploy", "fail", err.Error())
			}
		}

		if shouldDeploy {
			if err := recordDeploymentResult(nil, ver, "deploy", "success", ""); err != nil {
				return err
			}
		}

		internal.PrintSuccess("所有任务已完成")
		return nil
	},
}

// verifyStepTitle 返回 verify 阶段的展示标题。
func verifyStepTitle() string {
	switch cfg.Verify.Driver {
	case "ssh":
		return "SSH 校验"
	case "command":
		return "本地校验"
	default:
		return "健康检查"
	}
}

func init() {
	runCmd.Flags().StringVarP(&runVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
	runCmd.Flags().StringVar(&runEnvFile, "env-file", "", ".env 文件路径 (默认使用配置)")
	runCmd.Flags().StringVarP(&runProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
	runCmd.Flags().BoolVar(&runSkipDeploy, "skip-deploy", false, "跳过远程部署步骤")
}

func buildStepTitle() string {
	switch cfg.Build.Driver {
	case "go-binary":
		return "构建二进制"
	case "command":
		return "执行构建命令"
	default:
		return "构建镜像"
	}
}

func publishStepTitle() string {
	switch cfg.Publish.Driver {
	case "scp":
		return "上传产物"
	default:
		return "推送镜像"
	}
}

func deployStepTitle() string {
	switch cfg.Deploy.Driver {
	case "binary-install":
		return "安装二进制"
	case "ssh":
		return "执行远程命令"
	default:
		return "远程部署"
	}
}
