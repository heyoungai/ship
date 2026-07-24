package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/heyoungai/ship/internal"

	"github.com/spf13/cobra"
)

var (
	pushVersion       string
	pushProfile       string
	pushPromoteLatest bool
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

		// 独立 push：必须已有 build 写入的 release manifest。
		existing, err := internal.RequireReleaseManifest(session.StateRoot(), ver)
		if err != nil {
			return err
		}
		session.Manifest = existing
		internal.PrintInfo(fmt.Sprintf("using release manifest run_id=%s artifacts=%d", existing.RunID, len(existing.Artifacts)))

		profiles, err := cfg.GetProfiles(pushProfile)
		if err != nil {
			return err
		}
		for _, p := range profiles {
			if err := executePublishProfileWithOptions(cfg, ver, p, session.RunID(), session, pushPromoteLatest); err != nil {
				return err
			}
		}
		if err := session.saveManifest(true); err != nil {
			return fmt.Errorf("保存 release manifest 失败: %w", err)
		}
		internal.PrintInfo(fmt.Sprintf("release indexed: %s", internal.ReleaseIndexPath(session.StateRoot(), ver)))
		return nil
	},
}

func init() {
	pushCmd.Flags().StringVarP(&pushVersion, "version", "v", "", "正式 release tag（git-tag 模式下必须存在）")
	pushCmd.Flags().StringVarP(&pushProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
	pushCmd.Flags().BoolVar(&pushPromoteLatest, "promote-latest", false, "显式将 default profile 推送到 :latest")
}

// doPush 按当前 publish.driver 执行单个 profile 的发布。
func doPush(cfg *internal.Config, version string, profile internal.Profile, runID string, session *releaseSession) error {
	return doPushWithOptions(cfg, version, profile, runID, session, pushPromoteLatest)
}

func doPushWithOptions(cfg *internal.Config, version string, profile internal.Profile, runID string, session *releaseSession, promoteLatest bool) error {
	renderedCfg, renderedProfile, err := internal.RenderConfigForProfile(cfg, profile, version)
	if err != nil {
		return err
	}
	cfg = renderedCfg
	profile = renderedProfile

	switch cfg.Publish.Driver {
	case "registry":
		return doRegistryPush(cfg, version, profile, runID, session, promoteLatest)
	case "scp":
		return doSCPPush(cfg, profile, version, session)
	case "none":
		internal.PrintInfo("当前配置未启用发布阶段")
		return nil
	default:
		return fmt.Errorf("当前不支持的 publish.driver: %s", cfg.Publish.Driver)
	}
}

func doRegistryPush(cfg *internal.Config, version string, profile internal.Profile, runID string, session *releaseSession, promoteLatest bool) error {
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

	// 优先使用 manifest 中的本地引用；否则回退到本次 runID / 兼容 tag。
	localRef := resolveLocalImageRef(cfg, profile, runID, session)

	for _, target := range cfg.RegistryTargets(remoteTag) {
		fmt.Printf("  %s Push%s  → %s\n",
			internal.StepStyle.Render("▸"),
			nameLabel,
			target)
		internal.ProgressSub(target)

		// 若本地引用与目标不同，先确保已 tag。
		if localRef != "" && localRef != target {
			if err := internal.RunCmd(
				[]string{"docker", "tag", localRef, target},
				fmt.Sprintf("tag %s → %s", localRef, target),
			); err != nil {
				return err
			}
		}

		skip, err := internal.EnsureRegistryTagImmutable(target, target)
		if err != nil {
			return err
		}
		if !skip {
			if err := internal.RunCmd(
				[]string{"docker", "push", target},
				target,
			); err != nil {
				return err
			}
		}

		digest := ""
		if d, exists, err := internal.ResolveRegistryPinDigest(target); err != nil {
			internal.PrintWarning(fmt.Sprintf(
				"获取 registry pin digest 失败 (%s): %v；manifest 不写入 digest（deploy 将按 tag，避免把本地 config digest 误记为 pin）",
				target, err,
			))
		} else if exists && d != "" {
			digest = d
		} else if exists {
			internal.PrintWarning(fmt.Sprintf(
				"远端 %s 为 manifest list/index 且无法解析 index digest；manifest 不写入 pin digest（请升级 buildx 或改用 pin=tag）",
				target,
			))
		} else {
			internal.PrintWarning(fmt.Sprintf(
				"远端尚未可读到 digest (%s)；manifest 不写入 pin digest，不回退本地 config digest",
				target,
			))
		}
		if session != nil {
			platform := cfg.Build.Platforms
			session.recordImageArtifact(profile, platform, localRef, target, digest)
		}
	}

	// latest：配置 tag_latest_on_default_profile 仍兼容；--promote-latest 可显式打开。
	if promoteLatest || cfg.ShouldTagLatest(profile) {
		for _, target := range cfg.RegistryTargets("latest") {
			fmt.Printf("  %s Push%s  %s\n",
				internal.StepStyle.Render("▸"),
				nameLabel,
				internal.DimStyle.Render("→ latest"))
			internal.ProgressSub(target)
			src := cfg.RegistryTargets(remoteTag)
			if len(src) > 0 {
				_ = internal.RunCmd(
					[]string{"docker", "tag", src[0], target},
					"tag → latest",
				)
			}
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

func resolveLocalImageRef(cfg *internal.Config, profile internal.Profile, runID string, session *releaseSession) string {
	pname := internal.FormatProfileName(profile)
	if pname == "" {
		pname = "default"
	}
	if session != nil && session.Manifest != nil {
		for _, a := range session.Manifest.Artifacts {
			if a.Type != internal.ArtifactTypeImage {
				continue
			}
			ap := a.Profile
			if ap == "" {
				ap = "default"
			}
			if ap == pname && strings.TrimSpace(a.LocalRef) != "" {
				return a.LocalRef
			}
		}
	}
	if runID != "" {
		return cfg.ImageRef(cfg.BuildSourceTagForRun(runID, profile))
	}
	return cfg.ImageRef(cfg.BuildSourceTag(profile))
}

// doSCPPush 按当前 profile 渲染 scp 发布目标并上传产物。
func doSCPPush(cfg *internal.Config, profile internal.Profile, version string, session *releaseSession) error {
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
	// 优先使用 manifest 中已持久化的产物路径（worktree 清理后仍可用）。
	if persisted := resolvePersistedBinaryPath(profile, session); persisted != "" {
		local = persisted
	}
	if _, err := os.Stat(local); err != nil {
		return fmt.Errorf("SCP 本地产物不存在: %s；请先 ship build（产物会持久化到 .ship/artifacts）: %w", local, err)
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

	if err := internal.RunCmd(
		[]string{"scp", local, remote},
		fmt.Sprintf("scp%s -> %s", nameLabel, remote),
	); err != nil {
		return err
	}
	if session != nil && session.Manifest != nil {
		pname := name
		if pname == "" {
			pname = "default"
		}
		digest := ""
		for _, a := range session.Manifest.Artifacts {
			if a.Type == internal.ArtifactTypeBinary {
				ap := a.Profile
				if ap == "" {
					ap = "default"
				}
				if ap == pname && a.Digest != "" {
					digest = a.Digest
					break
				}
			}
		}
		session.Manifest.UpsertArtifact(internal.ArtifactRecord{
			Type:     internal.ArtifactTypeBinary,
			Profile:  pname,
			LocalRef: local,
			Ref:      remote,
			Digest:   digest,
		})
	}
	return nil
}

func resolvePersistedBinaryPath(profile internal.Profile, session *releaseSession) string {
	if session == nil || session.Manifest == nil {
		return ""
	}
	want := internal.FormatProfileName(profile)
	if want == "" {
		want = "default"
	}
	for _, a := range session.Manifest.Artifacts {
		if a.Type != internal.ArtifactTypeBinary {
			continue
		}
		ap := a.Profile
		if ap == "" {
			ap = "default"
		}
		if ap == want && strings.TrimSpace(a.LocalRef) != "" {
			return a.LocalRef
		}
	}
	return ""
}
