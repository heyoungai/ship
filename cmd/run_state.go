package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/heyoungai/ship/internal"
)

type runState struct {
	checkpoint *internal.RunCheckpoint
	manifest   *internal.ReleaseManifest
	reused     bool
}

// resolveRunVersionForResume 让 --resume 不依赖当前最新 git tag；checkpoint 是
// 已经锁定过的 release identity，-v 若存在仅作为一致性保护。
func resolveRunVersionForResume(version, resumeID string) (string, error) {
	if strings.TrimSpace(resumeID) == "" {
		return version, nil
	}
	invocationRoot, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取 InvocationRoot 失败: %w", err)
	}
	checkpoint, err := internal.LoadRunCheckpoint(filepath.Join(invocationRoot, ".ship"), resumeID)
	if err != nil {
		return "", fmt.Errorf("读取 resume run %s 失败: %w", resumeID, err)
	}
	if strings.TrimSpace(version) != "" && strings.TrimSpace(version) != checkpoint.Identity.Version {
		return "", fmt.Errorf("--version %s 与 resume run %s 的版本 %s 不一致", version, resumeID, checkpoint.Identity.Version)
	}
	return checkpoint.Identity.Version, nil
}

func profileNames(profiles []internal.Profile) []string {
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		name := internal.FormatProfileName(profile)
		if name == "" {
			name = "default"
		}
		names = append(names, name)
	}
	return names
}

// prepareRunState 创建新的 checkpoint，或加载一个用户显式指定/安全自动发现的 checkpoint。
func prepareRunState(session *releaseSession, cfg *internal.Config, profiles []internal.Profile, recipeDigest, resumeID string, restart bool) (*runState, error) {
	if session == nil {
		return nil, fmt.Errorf("release session 为空")
	}
	if strings.TrimSpace(resumeID) != "" && restart {
		return nil, fmt.Errorf("--resume 与 --restart 不能同时使用")
	}

	if strings.TrimSpace(resumeID) != "" {
		checkpoint, err := internal.LoadRunCheckpoint(session.StateRoot(), resumeID)
		if err != nil {
			return nil, fmt.Errorf("读取 resume run %s 失败: %w", resumeID, err)
		}
		manifest, err := loadCheckpointManifest(session, checkpoint)
		if err != nil {
			return nil, err
		}
		if err := validateCheckpointCompatibility(checkpoint, manifest, session.Identity, recipeDigest, profiles); err != nil {
			return nil, fmt.Errorf("不能恢复 run %s: %w", checkpoint.RunID, err)
		}
		if checkpoint.MarkRunningInterrupted() {
			if err := internal.SaveRunCheckpoint(session.StateRoot(), checkpoint); err != nil {
				return nil, err
			}
		}
		if err := validatePublishedCheckpoint(cfg, checkpoint, manifest, profiles); err != nil {
			return nil, fmt.Errorf("不能恢复 run %s: %w", checkpoint.RunID, err)
		}
		session.setRunID(checkpoint.RunID)
		session.Manifest = manifest
		return &runState{checkpoint: checkpoint, manifest: manifest, reused: true}, nil
	}

	if !restart {
		candidates, err := findAutoReusableRuns(session, cfg, profiles, recipeDigest)
		if err != nil {
			return nil, err
		}
		switch len(candidates) {
		case 1:
			candidate := candidates[0]
			session.setRunID(candidate.checkpoint.RunID)
			session.Manifest = candidate.manifest
			internal.PrintInfo(fmt.Sprintf("reusing verified run: %s", candidate.checkpoint.RunID))
			return candidate, nil
		case 0:
			// 新建会话，继续向下创建 checkpoint。
		default:
			ids := make([]string, 0, len(candidates))
			for _, candidate := range candidates {
				ids = append(ids, candidate.checkpoint.RunID)
			}
			return nil, fmt.Errorf("发现多个可复用 run（%s）；请显式选择：ship run --resume <run-id>", strings.Join(ids, ", "))
		}
	}

	checkpoint := internal.NewRunCheckpoint(session.RunID(), session.Identity, recipeDigest, profileNames(profiles), Version)
	checkpoint.SyncArtifacts(session.Manifest)
	if err := session.saveManifest(false); err != nil {
		return nil, fmt.Errorf("初始化 run manifest 失败: %w", err)
	}
	if err := internal.SaveRunCheckpoint(session.StateRoot(), checkpoint); err != nil {
		return nil, fmt.Errorf("创建 run checkpoint 失败: %w", err)
	}
	return &runState{checkpoint: checkpoint, manifest: session.Manifest}, nil
}

func loadCheckpointManifest(session *releaseSession, checkpoint *internal.RunCheckpoint) (*internal.ReleaseManifest, error) {
	if checkpoint == nil {
		return nil, fmt.Errorf("run checkpoint 为空")
	}
	manifest, err := internal.LoadReleaseManifestFile(internal.RunManifestPath(session.StateRoot(), checkpoint.RunID))
	if err != nil {
		return nil, fmt.Errorf("读取 run %s 的 release manifest 失败: %w", checkpoint.RunID, err)
	}
	return manifest, nil
}

func validateCheckpointCompatibility(checkpoint *internal.RunCheckpoint, manifest *internal.ReleaseManifest, identity internal.ReleaseIdentity, recipeDigest string, profiles []internal.Profile) error {
	if checkpoint == nil || manifest == nil {
		return fmt.Errorf("checkpoint 或 release manifest 缺失")
	}
	if checkpoint.RunID == "" || checkpoint.RunID != manifest.RunID {
		return fmt.Errorf("checkpoint 与 release manifest 的 run_id 不一致")
	}
	if checkpoint.Identity.Version != identity.Version || checkpoint.Identity.SourceCommit != identity.SourceCommit || checkpoint.Identity.SourceRef != identity.SourceRef || checkpoint.Identity.SourceMode != identity.SourceMode {
		return fmt.Errorf("release identity 已变化")
	}
	if checkpoint.RecipeDigest != recipeDigest {
		return fmt.Errorf("release recipe 已变化")
	}
	if !internal.SameProfileNames(checkpoint.Profiles, profileNames(profiles)) {
		return fmt.Errorf("profile 集合已变化")
	}
	if manifest.Version != identity.Version || manifest.Source.Commit != identity.SourceCommit || manifest.Source.Ref != identity.SourceRef || manifest.Source.Mode != identity.SourceMode {
		return fmt.Errorf("release manifest 的源码身份不匹配")
	}
	return nil
}

func findAutoReusableRuns(session *releaseSession, cfg *internal.Config, profiles []internal.Profile, recipeDigest string) ([]*runState, error) {
	if cfg == nil || cfg.Build.Driver != "docker" || cfg.Publish.Driver != "registry" {
		return nil, nil
	}
	checkpoints, err := internal.FindRunCheckpoints(session.StateRoot())
	if err != nil {
		return nil, err
	}
	candidates := make([]*runState, 0, 1)
	for _, checkpoint := range checkpoints {
		if checkpoint.MarkRunningInterrupted() {
			if err := internal.SaveRunCheckpoint(session.StateRoot(), checkpoint); err != nil {
				return nil, err
			}
		}
		if !allProfileStagesSucceeded(checkpoint, "publish", profiles) {
			continue
		}
		manifest, err := loadCheckpointManifest(session, checkpoint)
		if err != nil {
			continue
		}
		if err := validateCheckpointCompatibility(checkpoint, manifest, session.Identity, recipeDigest, profiles); err != nil {
			continue
		}
		if err := validatePublishedCheckpoint(cfg, checkpoint, manifest, profiles); err != nil {
			internal.PrintWarning(fmt.Sprintf("run %s 不能安全复用：%v", checkpoint.RunID, err))
			continue
		}
		candidates = append(candidates, &runState{checkpoint: checkpoint, manifest: manifest, reused: true})
	}
	return candidates, nil
}

func validatePublishedCheckpoint(cfg *internal.Config, checkpoint *internal.RunCheckpoint, manifest *internal.ReleaseManifest, profiles []internal.Profile) error {
	if checkpoint == nil || manifest == nil {
		return fmt.Errorf("checkpoint 或 manifest 缺失")
	}
	if !hasAnySucceededPublish(checkpoint, profiles) {
		return nil
	}
	if cfg == nil || cfg.Build.Driver != "docker" || cfg.Publish.Driver != "registry" {
		return nil
	}
	for _, profile := range profiles {
		name := internal.FormatProfileName(profile)
		if name == "" {
			name = "default"
		}
		if checkpoint.StageStatus("publish", name) != internal.StageSucceeded {
			continue
		}
		for _, target := range cfg.RegistryTargets(internal.ImageTag(checkpoint.Identity.Version, profile)) {
			artifact, ok := imageArtifactForRef(manifest, name, target)
			if !ok || !internal.IsPinableDigest(artifact.Digest) {
				return fmt.Errorf("profile %s 缺少可验证的已发布镜像 %s", name, target)
			}
			var remoteDigest string
			var exists bool
			err := internal.RetryNetwork(cfg.Retry, fmt.Sprintf("验证可复用镜像 %s", target), func() error {
				var inspectErr error
				remoteDigest, exists, inspectErr = internal.ResolveRegistryPinDigest(target)
				return inspectErr
			})
			if err != nil || !exists || !internal.IsPinableDigest(remoteDigest) || !internal.DigestsMatch(artifact.Digest, remoteDigest) {
				if err != nil {
					return fmt.Errorf("无法验证 %s: %w", target, err)
				}
				return fmt.Errorf("远端镜像 %s 与 manifest digest 不一致或不可验证", target)
			}
		}
	}
	return nil
}

func hasAnySucceededPublish(checkpoint *internal.RunCheckpoint, profiles []internal.Profile) bool {
	for _, profile := range profiles {
		if checkpoint.StageStatus("publish", internal.FormatProfileName(profile)) == internal.StageSucceeded {
			return true
		}
	}
	return false
}

func allProfileStagesSucceeded(checkpoint *internal.RunCheckpoint, stage string, profiles []internal.Profile) bool {
	if checkpoint == nil {
		return false
	}
	for _, profile := range profiles {
		if checkpoint.StageStatus(stage, internal.FormatProfileName(profile)) != internal.StageSucceeded {
			return false
		}
	}
	return len(profiles) > 0
}

func imageArtifactForRef(manifest *internal.ReleaseManifest, profile, ref string) (internal.ArtifactRecord, bool) {
	for _, artifact := range manifest.Artifacts {
		artifactProfile := artifact.Profile
		if artifactProfile == "" {
			artifactProfile = "default"
		}
		if artifact.Type == internal.ArtifactTypeImage && artifactProfile == profile && artifact.Ref == ref {
			return artifact, true
		}
	}
	return internal.ArtifactRecord{}, false
}

func saveRunCheckpoint(session *releaseSession, checkpoint *internal.RunCheckpoint) error {
	if session == nil || checkpoint == nil {
		return nil
	}
	checkpoint.SyncArtifacts(session.Manifest)
	return internal.SaveRunCheckpoint(session.StateRoot(), checkpoint)
}

func runStage(session *releaseSession, checkpoint *internal.RunCheckpoint, stage, profile string, fn func() error) (bool, error) {
	if checkpoint.StageStatus(stage, profile) == internal.StageSucceeded {
		internal.PrintInfo(fmt.Sprintf("reuse %s", internal.StageKey(stage, profile)))
		return false, nil
	}
	checkpoint.MarkStage(stage, profile, internal.StageRunning, "")
	if err := saveRunCheckpoint(session, checkpoint); err != nil {
		return false, err
	}
	if err := fn(); err != nil {
		_ = session.saveManifest(false)
		checkpoint.MarkStage(stage, profile, internal.StageFailed, err.Error())
		if saveErr := saveRunCheckpoint(session, checkpoint); saveErr != nil {
			return false, fmt.Errorf("%w；保存失败 checkpoint 失败: %v", err, saveErr)
		}
		return false, err
	}
	if err := session.saveManifest(false); err != nil {
		return false, fmt.Errorf("保存 run manifest 失败: %w", err)
	}
	checkpoint.MarkStage(stage, profile, internal.StageSucceeded, "")
	if err := saveRunCheckpoint(session, checkpoint); err != nil {
		return false, err
	}
	return true, nil
}

func printRunFailure(session *releaseSession, checkpoint *internal.RunCheckpoint, err error) {
	if session == nil || checkpoint == nil || err == nil {
		return
	}
	failedAt := "unknown"
	for key, stage := range checkpoint.Stages {
		if stage.Status == internal.StageFailed {
			failedAt = key
			break
		}
	}
	internal.PrintWarning(fmt.Sprintf("run %s failed at %s: %v", checkpoint.RunID, failedAt, err))
	internal.PrintInfo(fmt.Sprintf("继续本次发布：ship run --resume %s", checkpoint.RunID))
	if session.Manifest != nil && session.Manifest.HasPublishedImage() {
		internal.PrintInfo(fmt.Sprintf("只部署已发布产物：ship deploy -v %s -y", session.Version()))
	}
}
