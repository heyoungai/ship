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
	runResume        string
	runRestart       bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "执行完整流程: build → tag → push → deploy",
	RunE: func(cmd *cobra.Command, args []string) error {
		versionFlag, err := resolveRunVersionForResume(runVersion, runResume)
		if err != nil {
			return err
		}
		session, err := prepareReleaseSession(cfg, versionFlag, true)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := session.Close(); closeErr != nil {
				internal.PrintWarning(fmt.Sprintf("release session cleanup: %v", closeErr))
			}
		}()
		warnIfInstalledSkillOutdated(session.InvocationRoot())

		activeCfg := session.Config
		if activeCfg == nil {
			activeCfg = cfg
		}
		if err := applyDockerPullFlag(cmd, activeCfg); err != nil {
			return err
		}

		ver := session.Version()
		envFile, err := resolveExternalEnvFile(session, activeCfg, runEnvFile)
		if err != nil {
			return err
		}

		profiles, err := activeCfg.GetProfiles(runProfile)
		if err != nil {
			return err
		}
		recipeDigest, err := internal.ConfigFingerprint(activeCfg)
		if err != nil {
			return err
		}
		state, err := prepareRunState(session, activeCfg, profiles, recipeDigest, runResume, runRestart)
		if err != nil {
			return err
		}
		checkpoint := state.checkpoint
		plan, err := internal.CompileReleasePlan(activeCfg, session.Identity, session.Roots, internal.PlanOptions{
			ProfileFilter: runProfile,
			SkipDeploy:    runSkipDeploy,
			EnvFile:       envFile,
		})
		if err != nil {
			return err
		}
		internal.PrintReleasePlan(plan)

		shouldTag := activeCfg.UsesTagStage()
		shouldPublish := activeCfg.UsesPublishStage()
		shouldDeploy := !runSkipDeploy && activeCfg.UsesDeployStage()
		shouldVerify := !runSkipDeploy && activeCfg.UsesVerifyStage()

		internal.SetProgressTotal(len(plan.Stages))
		currentStep := 1

		internal.ProgressStep(currentStep, buildStepTitleFor(activeCfg))
		for _, p := range profiles {
			if _, err := runStage(session, checkpoint, "build", internal.FormatProfileName(p), func() error {
				return executeBuildProfile(activeCfg, ver, p, envFile, session.RunID(), session)
			}); err != nil {
				printRunFailure(session, checkpoint, err)
				return err
			}
		}
		if err := session.saveManifest(false); err != nil {
			printRunFailure(session, checkpoint, err)
			return fmt.Errorf("保存 build manifest 失败: %w", err)
		}
		if err := saveRunCheckpoint(session, checkpoint); err != nil {
			return err
		}
		currentStep++

		if shouldTag {
			internal.ProgressStep(currentStep, "打 Tag")
			for _, p := range profiles {
				if _, err := runStage(session, checkpoint, "tag", internal.FormatProfileName(p), func() error {
					return doTag(activeCfg, ver, p, session.RunID())
				}); err != nil {
					printRunFailure(session, checkpoint, err)
					return err
				}
			}
			currentStep++
		}

		if shouldPublish {
			internal.ProgressStep(currentStep, publishStepTitleFor(activeCfg))
			for _, p := range profiles {
				if _, err := runStage(session, checkpoint, "publish", internal.FormatProfileName(p), func() error {
					return executePublishProfileWithOptions(activeCfg, ver, p, session.RunID(), session, runPromoteLatest)
				}); err != nil {
					printRunFailure(session, checkpoint, err)
					return err
				}
			}
			if err := session.saveManifest(true); err != nil {
				printRunFailure(session, checkpoint, err)
				return fmt.Errorf("保存 release manifest 失败: %w", err)
			}
			if err := saveRunCheckpoint(session, checkpoint); err != nil {
				return err
			}
			internal.PrintInfo(fmt.Sprintf("release indexed: %s", internal.ReleaseIndexPath(session.StateRoot(), ver)))
			currentStep++
		} else {
			_ = session.saveManifest(false)
		}

		deployProfile := selectDeployProfile(activeCfg, profiles)
		meta := historyMetaFromSession(session)
		deployRan := false
		verifyRan := false
		if shouldDeploy {
			internal.ProgressStep(currentStep, deployStepTitleFor(activeCfg))
			var deployErr error
			deployRan, deployErr = runStage(session, checkpoint, "deploy", "", func() error {
				return executeDeployStage(activeCfg, ver, deployProfile, session)
			})
			if deployErr != nil {
				resultErr := recordDeploymentResult(deployErr, ver, "deploy", "fail", deployErr.Error(), meta)
				printRunFailure(session, checkpoint, resultErr)
				return resultErr
			}
			currentStep++
		} else if runSkipDeploy {
			internal.PrintWarning("已跳过远程部署")
		}

		if shouldVerify {
			internal.ProgressStep(currentStep, verifyStepTitleFor(activeCfg))
			var verifyErr error
			verifyRan, verifyErr = runStage(session, checkpoint, "verify", "", func() error {
				return internal.ExecuteVerify(activeCfg, deployProfile, ver)
			})
			if verifyErr != nil {
				resultErr := recordDeploymentResult(verifyErr, ver, "deploy", "fail", verifyErr.Error(), meta)
				printRunFailure(session, checkpoint, resultErr)
				return resultErr
			}
		}

		if shouldDeploy && (deployRan || verifyRan) {
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
	runCmd.Flags().StringVar(&runResume, "resume", "", "从指定失败 run ID 的首个未完成阶段继续")
	runCmd.Flags().BoolVar(&runRestart, "restart", false, "禁用已验证 run 的自动复用，强制从头开始")
	registerDockerPullFlag(runCmd)
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
