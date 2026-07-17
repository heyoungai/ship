package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/heyoungai/ship/internal"
)

// releaseSession 绑定一次命令执行的 identity、recipe 与执行根目录。
type releaseSession struct {
	Identity internal.ReleaseIdentity
	Roots    internal.ExecutionRoots
	Snapshot *internal.SourceSnapshot
	Config   *internal.Config
	Manifest *internal.ReleaseManifest
}

// prepareReleaseSession 解析 ReleaseIdentity，并在需要时创建 SourceRoot worktree。
// withSnapshot 时会从 SourceRoot 重载 ship.toml 作为 release recipe（两阶段配置）。
// 调用方必须 defer session.Close()。
func prepareReleaseSession(bootstrap *internal.Config, versionFlag string, withSnapshot bool) (*releaseSession, error) {
	identity, err := internal.ResolveReleaseIdentity(bootstrap, versionFlag)
	if err != nil {
		return nil, err
	}
	internal.PrintReleaseIdentity(identity)

	invocationRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("获取 InvocationRoot 失败: %w", err)
	}

	session := &releaseSession{
		Identity: identity,
		Config:   bootstrap,
	}
	if !withSnapshot {
		runID, err := newStandaloneRunID()
		if err != nil {
			return nil, err
		}
		session.Roots = internal.ExecutionRoots{
			InvocationRoot: invocationRoot,
			SourceRoot:     invocationRoot,
			StateRoot:      filepath.Join(invocationRoot, ".ship"),
			RunID:          runID,
		}
		internal.SetStateRoot(session.Roots.StateRoot)
		session.Manifest = internal.NewReleaseManifest(identity, runID, Version)
		return session, nil
	}

	snap, err := internal.BeginSourceSnapshot(identity, invocationRoot)
	if err != nil {
		return nil, err
	}
	session.Snapshot = snap
	session.Roots = snap.Roots
	internal.SetStateRoot(session.Roots.StateRoot)

	if err := snap.Enter(); err != nil {
		internal.ClearStateRoot()
		_ = snap.Close()
		return nil, err
	}

	// 两阶段配置：从 SourceRoot 重载 recipe，不再重新解析 version/commit。
	if identity.NeedsSourceSnapshot() {
		recipe, err := internal.LoadConfigFrom(session.Roots.SourceRoot, os.Getenv("IMAGE_NAME"))
		if err != nil {
			_ = session.Close()
			return nil, fmt.Errorf("从 SourceRoot 加载 release recipe 失败: %w", err)
		}
		session.Config = recipe
		cfg = recipe // 保持全局 cfg 与 recipe 一致，供后续 pipeline 使用
		internal.PrintInfo(fmt.Sprintf("release recipe loaded from SourceRoot (%s)", session.Roots.SourceRoot))
	}

	session.Manifest = internal.NewReleaseManifest(identity, session.Roots.RunID, Version)
	return session, nil
}

func (s *releaseSession) Close() error {
	if s == nil {
		return nil
	}
	internal.ClearStateRoot()
	if s.Snapshot == nil {
		return nil
	}
	err := s.Snapshot.Close()
	s.Snapshot = nil
	return err
}

func (s *releaseSession) Version() string {
	return s.Identity.Version
}

func (s *releaseSession) RunID() string {
	return s.Roots.RunID
}

func (s *releaseSession) InvocationRoot() string {
	return s.Roots.InvocationRoot
}

func (s *releaseSession) SourceRoot() string {
	return s.Roots.SourceRoot
}

func (s *releaseSession) StateRoot() string {
	return s.Roots.StateRoot
}

// saveManifest 将当前 manifest 落盘；indexRelease 为 true 时同时更新 releases/<version>.json。
func (s *releaseSession) saveManifest(indexRelease bool) error {
	if s == nil || s.Manifest == nil {
		return nil
	}
	return internal.SaveReleaseManifest(s.StateRoot(), s.Manifest, indexRelease)
}

// recordImageArtifact 记录 docker 镜像产物（build/tag/push 阶段渐进补全）。
func (s *releaseSession) recordImageArtifact(profile internal.Profile, platform, localRef, remoteRef, digest string) {
	if s == nil || s.Manifest == nil {
		return
	}
	name := internal.FormatProfileName(profile)
	if name == "" {
		name = "default"
	}
	s.Manifest.UpsertArtifact(internal.ArtifactRecord{
		Type:     internal.ArtifactTypeImage,
		Profile:  name,
		Platform: platform,
		LocalRef: localRef,
		Ref:      remoteRef,
		Digest:   digest,
	})
}

// resolveExternalEnvFile 将 CLI / 配置中的 env 文件解析为 InvocationRoot 绝对路径。
func resolveExternalEnvFile(session *releaseSession, cfg *internal.Config, cliEnvFile string) (string, error) {
	path := strings.TrimSpace(cliEnvFile)
	if path == "" && cfg != nil {
		path = strings.TrimSpace(cfg.Build.EnvFile)
	}
	if path == "" {
		return "", nil
	}
	if strings.Contains(path, "{{") {
		return path, nil
	}
	return internal.AbsoluteExternalPath(session.InvocationRoot(), path)
}

func newStandaloneRunID() (string, error) {
	return internal.NewRunID()
}

// resolveComposeLocalPaths 将 compose 本地上传源解析为正确根目录的绝对路径。
// local_file（版本化物料）→ SourceRoot；local_env_file（外部输入）→ InvocationRoot。
func resolveComposeLocalPaths(session *releaseSession, localFile, localEnvFile string) (string, string, error) {
	var err error
	sourceRoot := session.InvocationRoot()
	if session.SourceRoot() != "" {
		sourceRoot = session.SourceRoot()
	}
	if localFile, err = absIfLiteral(sourceRoot, localFile); err != nil {
		return "", "", err
	}
	if localEnvFile, err = absIfLiteral(session.InvocationRoot(), localEnvFile); err != nil {
		return "", "", err
	}
	return localFile, localEnvFile, nil
}

func absIfLiteral(root, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "{{") {
		return path, nil
	}
	return internal.AbsoluteExternalPath(root, path)
}
