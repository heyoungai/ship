package cmd

import (
	"fmt"
	"ship/internal"

	"github.com/spf13/cobra"
)

var (
	pushVersion string
	pushProfile string
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "推送镜像到所有配置的仓库",
	RunE: func(cmd *cobra.Command, args []string) error {
		ver, err := internal.ResolveVersion(pushVersion)
		if err != nil {
			return err
		}
		profiles := cfg.GetProfiles(pushProfile)
		for _, p := range profiles {
			if err := doPush(ver, p); err != nil {
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

// doPush 推送单个 profile 的镜像到所有仓库
func doPush(version string, profile internal.Profile) error {
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

	if profile.Default {
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
