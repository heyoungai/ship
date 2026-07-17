package cmd

import (
	"fmt"

	"github.com/heyoungai/ship/internal"

	"github.com/spf13/cobra"
)

var (
	runVersion       string
	runEnvFile       string
	runProfile       string
	runSkipDeploy    bool
	runPromoteLatest bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "执行完整流程: build → tag → push → deploy",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := prepareReleaseSession(cfg, runVersion, true)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := session.Close(); closeErr != nil {
				internal.PrintWarning(fmt.Sprintf("release session cleanup: %v", closeErr))
			}
		}()

		activeCfg := session.Config
		if activeCfg == nil {
			activeCfg = cfg
		}

		ver := session.Version()
		envFile, err := resolveExternalEnvFile(session, activeCfg, runEnvFile)
		if err != nil {
			return err
		}

		plan, err := internal.CompileReleasePlan(activeCfg, session.Identity, session.Roots, internal.PlanOptions{
			ProfileFilter: runProfile,
			SkipDeploy:    runSkipDeploy,
			EnvFile:       envFile,
		})
		if err != nil {
			return err
		}
		internal.PrintReleasePlan(plan)

		profiles, err := activeCfg.GetProfiles(runProfile)
		if err != nil {
			return err
		}

		shouldTag := activeCfg.UsesTagStage()
		shouldPublish := activeCfg.UsesPublishStage()
		shouldDeploy := !runSkipDeploy && activeCfg.UsesDeployStage()
		shouldVerify := !runSkipDeploy && activeCfg.UsesVerifyStage()

		internal.SetProgressTotal(len(plan.Stages))
		currentStep := 1

		internal.ProgressStep(currentStep, buildStepTitleFor(activeCfg))
		for _, p := range profiles {
			if err := executeBuildProfile(activeCfg, ver, p, envFile, session.RunID(), session); err != nil {
				return err
			}
		}
		_ = session.saveManifest(false)
		currentStep++

		if shouldTag {
			internal.ProgressStep(currentStep, "打 Tag")
			for _, p := range profiles {
				if err := doTag(activeCfg, ver, p, session.RunID()); err != nil {
					return err
				}
			}
			currentStep++
		}

		if shouldPublish {
			internal.ProgressStep(currentStep, publishStepTitleFor(activeCfg))
			for _, p := range profiles {
				if err := executePublishProfileWithOptions(activeCfg, ver, p, session.RunID(), session, runPromoteLatest); err != nil {
					return err
				}
			}
			if err := session.saveManifest(true); err != nil {
				return fmt.Errorf("保存 release manifest 失败: %w", err)
			}
			internal.PrintInfo(fmt.Sprintf("release indexed: %s", internal.ReleaseIndexPath(session.StateRoot(), ver)))
			currentStep++
		} else {
			_ = session.saveManifest(false)
		}

		deployProfile := selectDeployProfile(activeCfg, profiles)
		meta := historyMetaFromSession(session)
		if shouldDeploy {
			internal.ProgressStep(currentStep, deployStepTitleFor(activeCfg))
			if err := executeDeployStage(activeCfg, ver, deployProfile, session); err != nil {
				return recordDeploymentResult(err, ver, "deploy", "fail", err.Error(), meta)
			}
			currentStep++
		} else if runSkipDeploy {
			internal.PrintWarning("已跳过远程部署")
		}

		if shouldVerify {
			internal.ProgressStep(currentStep, verifyStepTitleFor(activeCfg))
			if err := internal.ExecuteVerify(activeCfg, deployProfile, ver); err != nil {
				return recordDeploymentResult(err, ver, "deploy", "fail", err.Error(), meta)
			}
		}

		if shouldDeploy {
			if err := recordDeploymentResult(nil, ver, "deploy", "success", "", meta); err != nil {
				return err
			}
		}

		internal.PrintSuccess("所有任务已完成")
		return nil
	},
}

func init() {
	runCmd.Flags().StringVarP(&runVersion, "version", "v", "", "正式 release tag（git-tag 模式下必须存在）")
	runCmd.Flags().StringVar(&runEnvFile, "env-file", "", ".env 文件路径 (默认使用配置；相对 InvocationRoot)")
	runCmd.Flags().StringVarP(&runProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
	runCmd.Flags().BoolVar(&runSkipDeploy, "skip-deploy", false, "跳过远程部署步骤")
	runCmd.Flags().BoolVar(&runPromoteLatest, "promote-latest", false, "显式将 default profile 推送到 :latest")
}

func buildStepTitle() string {
	return buildStepTitleFor(cfg)
}

func buildStepTitleFor(c *internal.Config) string {
	if c == nil {
		return "构建"
	}
	switch c.Build.Driver {
	case "go-binary":
		return "构建二进制"
	case "command":
		return "执行构建命令"
	default:
		return "构建镜像"
	}
}

func publishStepTitleFor(c *internal.Config) string {
	if c == nil {
		return "发布"
	}
	switch c.Publish.Driver {
	case "scp":
		return "上传产物"
	default:
		return "推送镜像"
	}
}

func deployStepTitleFor(c *internal.Config) string {
	if c == nil {
		return "远程部署"
	}
	switch c.Deploy.Driver {
	case "binary-install":
		return "安装二进制"
	case "ssh":
		return "执行远程命令"
	default:
		return "远程部署"
	}
}

func verifyStepTitleFor(c *internal.Config) string {
	if c == nil {
		return "健康检查"
	}
	switch c.Verify.Driver {
	case "ssh":
		return "SSH 校验"
	case "command":
		return "本地校验"
	default:
		return "健康检查"
	}
}

func historyMetaFromSession(session *releaseSession) internal.HistoryMeta {
	meta := internal.HistoryMeta{}
	if session == nil {
		return meta
	}
	meta.Commit = session.Identity.SourceCommit
	meta.RunID = session.RunID()
	if session.Manifest != nil {
		meta.Digest = session.Manifest.PrimaryImageDigest()
	}
	return meta
}
