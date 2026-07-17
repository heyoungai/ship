package cmd

import (
	"fmt"

	"github.com/heyoungai/ship/internal"

	"github.com/spf13/cobra"
)

var (
	pushVersion string
	pushProfile string
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "发布产物",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := prepareReleaseSession(cfg, pushVersion, false)
		if err != nil {
			return err
		}
		defer session.Close()
		ver := session.Version()
		profiles, err := cfg.GetProfiles(pushProfile)
		if err != nil {
			return err
		}
		for _, p := range profiles {
			if err := executePublishProfile(cfg, ver, p, session.RunID()); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	pushCmd.Flags().StringVarP(&pushVersion, "version", "v", "", "正式 release tag（git-tag 模式下必须存在）")
	pushCmd.Flags().StringVarP(&pushProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
}

// doPush 按当前 publish.driver 执行单个 profile 的发布。
func doPush(cfg *internal.Config, version string, profile internal.Profile, runID string) error {
	_ = runID
	renderedCfg, renderedProfile, err := internal.RenderConfigForProfile(cfg, profile, version)
	if err != nil {
		return err
	}
	cfg = renderedCfg
	profile = renderedProfile

	switch cfg.Publish.Driver {
	case "registry":
		return doRegistryPush(cfg, version, profile)
	case "scp":
		return doSCPPush(cfg, profile, version)
	case "none":
		internal.PrintInfo("当前配置未启用发布阶段")
		return nil
	default:
		return fmt.Errorf("当前不支持的 publish.driver: %s", cfg.Publish.Driver)
	}
}

func doRegistryPush(cfg *internal.Config, version string, profile internal.Profile) error {
	if !cfg.Publish.Registry.Push {
		internal.PrintInfo("publish.registry.push = false，跳过 docker push")
		return nil
	}

	remoteTag := internal.ImageTag(version, profile)
	name := internal.FormatProfileName(profile)
	nameLabel := ""
	if name != "" {
		nameLabel = " " + internal.BoldStyle.Render("["+name+"]")
	}

	for _, target := range cfg.RegistryTargets(remoteTag) {
		fmt.Printf("  %s Push%s  → %s\n",
			internal.StepStyle.Render("▸"),
			nameLabel,
			target)
		internal.ProgressSub(target)

		// 本地 daemon 在 tag 阶段已拥有 registry 引用；与远端 manifest 比对以实现幂等/拒覆盖。
		skip, err := internal.EnsureRegistryTagImmutable(target, target)
		if err != nil {
			return err
		}
		if skip {
			continue
		}

		if err := internal.RunCmd(
			[]string{"docker", "push", target},
			target,
		); err != nil {
			return err
		}
	}

	// latest 仅为显式 promotion alias；deploy/rollback 永不修改 latest。
	// 此处保留既有 tag_latest_on_default_profile 行为，但不对 latest 做不可变拒绝。
	if cfg.ShouldTagLatest(profile) {
		for _, target := range cfg.RegistryTargets("latest") {
			fmt.Printf("  %s Push%s  %s\n",
				internal.StepStyle.Render("▸"),
				nameLabel,
				internal.DimStyle.Render("→ latest"))
			internal.ProgressSub(target)
			if err := internal.RunCmd(
				[]string{"docker", "push", target},
				target,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// doSCPPush 按当前 profile 渲染 scp 发布目标并上传产物。
func doSCPPush(cfg *internal.Config, profile internal.Profile, version string) error {
	ctx := internal.NewRenderContext(cfg, profile, version)
	name := internal.FormatProfileName(profile)
	nameLabel := ""
	if name != "" {
		nameLabel = " " + internal.BoldStyle.Render("["+name+"]")
	}

	local, err := ctx.RenderString(cfg.Publish.SCP.Local)
	if err != nil {
		return err
	}
	host, err := ctx.RenderString(cfg.Publish.SCP.Host)
	if err != nil {
		return err
	}
	remotePath, err := ctx.RenderString(cfg.Publish.SCP.Remote)
	if err != nil {
		return err
	}
	remote := fmt.Sprintf("%s:%s", host, remotePath)
	fmt.Printf("  %s SCP 上传%s  → %s\n",
		internal.StepStyle.Render("▸"),
		nameLabel,
		remote)
	internal.ProgressSub(local)

	return internal.RunCmd(
		[]string{"scp", local, remote},
		fmt.Sprintf("scp%s -> %s", nameLabel, remote),
	)
}
