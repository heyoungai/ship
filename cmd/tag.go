package cmd

import (
	"fmt"

	"github.com/heyoungai/ship/internal"

	"github.com/spf13/cobra"
)

var (
	tagVersion string
	tagProfile string
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "给镜像打 tag（仅 Docker/registry）",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := prepareReleaseSession(cfg, tagVersion, false)
		if err != nil {
			return err
		}
		defer session.Close()
		ver := session.Version()
		profiles, err := cfg.GetProfiles(tagProfile)
		if err != nil {
			return err
		}
		for _, p := range profiles {
			// 独立 tag：消费兼容本地 tag（由独立 build 的 alias 或历史行为产生）。
			if err := doTag(cfg, ver, p, ""); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	tagCmd.Flags().StringVarP(&tagVersion, "version", "v", "", "正式 release tag（git-tag 模式下必须存在）")
	tagCmd.Flags().StringVarP(&tagProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
}

// doTag 给单个 profile 的镜像打 tag。
//
// 这里显式先渲染一轮配置，确保 publish.registry.targets[*].url / namespace / image
// 和 build.docker.image 等字段与 build/push/deploy 阶段保持一致，而不是直接使用原始 TOML 文本。
func doTag(cfg *internal.Config, version string, profile internal.Profile, runID string) error {
	renderedCfg, renderedProfile, err := internal.RenderConfigForProfile(cfg, profile, version)
	if err != nil {
		return err
	}
	cfg = renderedCfg
	profile = renderedProfile

	if !cfg.UsesTagStage() {
		internal.PrintInfo("当前配置不需要独立 tag 阶段")
		return nil
	}

	localTag := cfg.BuildSourceTagForRun(runID, profile)
	remoteTag := internal.ImageTag(version, profile)
	name := internal.FormatProfileName(profile)
	nameLabel := ""
	if name != "" {
		nameLabel = " " + internal.BoldStyle.Render("["+name+"]")
	}

	for _, target := range cfg.RegistryTargets(remoteTag) {
		fmt.Printf("  %s Tag%s  %s → %s\n",
			internal.StepStyle.Render("▸"),
			nameLabel,
			cfg.ImageRef(localTag), target)
		internal.ProgressSub(target)
		if err := internal.RunCmd(
			[]string{"docker", "tag", cfg.ImageRef(localTag), target},
			target,
		); err != nil {
			return err
		}
	}

	if cfg.ShouldTagLatest(profile) {
		for _, target := range cfg.RegistryTargets("latest") {
			fmt.Printf("  %s Tag%s  %s\n",
				internal.StepStyle.Render("▸"),
				nameLabel,
				internal.DimStyle.Render("→ latest"))
			internal.ProgressSub(target)
			if err := internal.RunCmd(
				[]string{"docker", "tag", cfg.ImageRef(localTag), target},
				target,
			); err != nil {
				return err
			}
		}
	}

	return nil
}
