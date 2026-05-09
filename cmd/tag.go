package cmd

import (
	"fmt"
	"ship/internal"

	"github.com/spf13/cobra"
)

var (
	tagVersion string
	tagProfile string
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "给镜像打 tag",
	RunE: func(cmd *cobra.Command, args []string) error {
		ver, err := internal.ResolveVersion(tagVersion)
		if err != nil {
			return err
		}
		profiles := cfg.GetProfiles(tagProfile)
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
	localTag := internal.ImageTag("latest", profile)
	remoteTag := internal.ImageTag(version, profile)
	name := internal.FormatProfileName(profile)
	nameLabel := ""
	if name != "" {
		nameLabel = " " + internal.BoldStyle.Render("["+name+"]")
	}

	// 打版本 tag（如 v2.1.0）
	for _, target := range cfg.RegistryTargets(remoteTag) {
		fmt.Printf("  %s%s  %s → %s\n",
			internal.StepStyle.Render("▸"),
			nameLabel,
			cfg.ImageRef(localTag), target)
		if err := internal.RunCmd(
			[]string{"docker", "tag", cfg.ImageRef(localTag), target},
			fmt.Sprintf("打 tag%s", nameLabel),
		); err != nil {
			return err
		}
		fmt.Printf("  %s %s\n", internal.SuccessStyle.Render("✔"), target)
	}

	// 默认 profile 额外打 :latest tag
	if profile.Default {
		for _, target := range cfg.RegistryTargets("latest") {
			fmt.Printf("  %s%s  %s\n",
				internal.StepStyle.Render("▸"),
				nameLabel,
				internal.DimStyle.Render("→ latest"))
			if err := internal.RunCmd(
				[]string{"docker", "tag", cfg.ImageRef(localTag), target},
				fmt.Sprintf("打 latest tag%s", nameLabel),
			); err != nil {
				return err
			}
			fmt.Printf("  %s %s\n", internal.SuccessStyle.Render("✔"), target)
		}
	}

	return nil
}
