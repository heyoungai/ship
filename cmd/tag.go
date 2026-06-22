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
		ver, err := internal.ResolveVersion(cfg, tagVersion)
		if err != nil {
			return err
		}
		profiles, err := cfg.GetProfiles(tagProfile)
		if err != nil {
			return err
		}
		for _, p := range profiles {
			if err := doTag(ver, p); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	tagCmd.Flags().StringVarP(&tagVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
	tagCmd.Flags().StringVarP(&tagProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
}

// doTag 给单个 profile 的镜像打 tag
func doTag(version string, profile internal.Profile) error {
	if !cfg.UsesTagStage() {
		internal.PrintInfo("当前配置不需要独立 tag 阶段")
		return nil
	}

	localTag := cfg.BuildSourceTag(profile)
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
