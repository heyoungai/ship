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
		ver, err := internal.ResolveVersion(cfg, pushVersion)
		if err != nil {
			return err
		}
		profiles, err := cfg.GetProfiles(pushProfile)
		if err != nil {
			return err
		}
		for _, p := range profiles {
			if err := executePublishProfile(ver, p); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	pushCmd.Flags().StringVarP(&pushVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
	pushCmd.Flags().StringVarP(&pushProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
}

// doPush 按当前 publish.driver 执行单个 profile 的发布。
func doPush(version string, profile internal.Profile) error {
	switch cfg.Publish.Driver {
	case "registry":
		return doRegistryPush(version, profile)
	case "scp":
		return doSCPPush(profile, version)
	case "none":
		internal.PrintInfo("当前配置未启用发布阶段")
		return nil
	default:
		return fmt.Errorf("当前不支持的 publish.driver: %s", cfg.Publish.Driver)
	}
}

func doRegistryPush(version string, profile internal.Profile) error {
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
		if err := internal.RunCmd(
			[]string{"docker", "push", target},
			target,
		); err != nil {
			return err
		}
	}

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
func doSCPPush(profile internal.Profile, version string) error {
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
