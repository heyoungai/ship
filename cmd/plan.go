package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/heyoungai/ship/internal"

	"github.com/spf13/cobra"
)

var (
	planVersion    string
	planProfile    string
	planEnvFile    string
	planJSON       bool
	planSkipDeploy bool
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "展示本次 release 的执行计划（不执行副作用）",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := prepareReleaseSession(cfg, planVersion, true)
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
		envFile, err := resolveExternalEnvFile(session, activeCfg, planEnvFile)
		if err != nil {
			return err
		}

		plan, err := internal.CompileReleasePlan(activeCfg, session.Identity, session.Roots, internal.PlanOptions{
			ProfileFilter: planProfile,
			SkipDeploy:    planSkipDeploy,
			EnvFile:       envFile,
		})
		if err != nil {
			return err
		}

		if planJSON {
			data, err := internal.ReleasePlanJSON(plan)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}
		internal.PrintReleasePlan(plan)
		return nil
	},
}

func init() {
	planCmd.Flags().StringVarP(&planVersion, "version", "v", "", "正式 release tag（git-tag 模式下必须存在）")
	planCmd.Flags().StringVarP(&planProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
	planCmd.Flags().StringVar(&planEnvFile, "env-file", "", ".env 文件路径")
	planCmd.Flags().BoolVar(&planJSON, "json", false, "以 JSON 输出计划")
	planCmd.Flags().BoolVar(&planSkipDeploy, "skip-deploy", false, "计划中跳过部署阶段")
}

var (
	doctorVersion string
	doctorProfile string
	doctorEnvFile string
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "检查 release 运行条件（source snapshot / 配置 / 外部输入）",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := prepareReleaseSession(cfg, doctorVersion, true)
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
		envFile, err := resolveExternalEnvFile(session, activeCfg, doctorEnvFile)
		if err != nil {
			return err
		}

		plan, err := internal.CompileReleasePlan(activeCfg, session.Identity, session.Roots, internal.PlanOptions{
			ProfileFilter: doctorProfile,
			EnvFile:       envFile,
		})
		if err != nil {
			return err
		}

		internal.PrintBanner("ship doctor")
		internal.PrintReleasePlan(plan)

		var failures []string

		recipePath := filepath.Join(session.SourceRoot(), "ship.toml")
		if _, err := os.Stat(recipePath); err != nil {
			failures = append(failures, fmt.Sprintf("SourceRoot 缺少 ship.toml: %s", recipePath))
		} else {
			internal.PrintSuccess("SourceRoot ship.toml 可读")
		}

		if session.Identity.SourceMode == internal.SourceModeGitTag && session.Identity.SourceCommit == "" {
			failures = append(failures, "git-tag 模式未锁定 SourceCommit")
		} else {
			internal.PrintSuccess(fmt.Sprintf("source commit locked: %s", shortOrDash(session.Identity.SourceCommit)))
		}

		if envFile != "" && !strings.Contains(envFile, "{{") {
			if _, err := os.Stat(envFile); err != nil {
				failures = append(failures, fmt.Sprintf("外部 env 文件不存在: %s", envFile))
			} else {
				internal.PrintSuccess(fmt.Sprintf("env file ok: %s", envFile))
			}
		}

		if activeCfg.Build.Driver == "docker" {
			if _, err := exec.LookPath("docker"); err != nil {
				failures = append(failures, "docker 不在 PATH 中")
			} else if out, err := exec.Command("docker", "info").CombinedOutput(); err != nil {
				failures = append(failures, fmt.Sprintf("docker 不可用: %s", strings.TrimSpace(string(out))))
			} else {
				internal.PrintSuccess("docker 可用")
			}
		}

		if len(failures) > 0 {
			for _, f := range failures {
				internal.PrintWarning(f)
			}
			return fmt.Errorf("doctor 发现 %d 个问题", len(failures))
		}
		internal.PrintSuccess("doctor 检查通过")
		return nil
	},
}

func init() {
	doctorCmd.Flags().StringVarP(&doctorVersion, "version", "v", "", "正式 release tag（git-tag 模式下必须存在）")
	doctorCmd.Flags().StringVarP(&doctorProfile, "profile", "p", "", "指定 profile 名称")
	doctorCmd.Flags().StringVar(&doctorEnvFile, "env-file", "", ".env 文件路径")
}

func shortOrDash(s string) string {
	if s == "" {
		return "-"
	}
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
