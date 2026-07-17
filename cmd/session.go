package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/heyoungai/ship/internal"
)

// releaseSession 绑定一次命令执行的 identity 与执行根目录。
type releaseSession struct {
	Identity internal.ReleaseIdentity
	Roots    internal.ExecutionRoots
	Snapshot *internal.SourceSnapshot
}

// prepareReleaseSession 解析 ReleaseIdentity，并在需要时创建 SourceRoot worktree。
// 调用方必须 defer session.Close()。
func prepareReleaseSession(cfg *internal.Config, versionFlag string, withSnapshot bool) (*releaseSession, error) {
	identity, err := internal.ResolveReleaseIdentity(cfg, versionFlag)
	if err != nil {
		return nil, err
	}
	internal.PrintReleaseIdentity(identity)

	invocationRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("获取 InvocationRoot 失败: %w", err)
	}

	session := &releaseSession{Identity: identity}
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

// resolveExternalEnvFile 将 CLI / 配置中的 env 文件解析为 InvocationRoot 绝对路径。
func resolveExternalEnvFile(session *releaseSession, cfg *internal.Config, cliEnvFile string) (string, error) {
	path := strings.TrimSpace(cliEnvFile)
	if path == "" && cfg != nil {
		path = strings.TrimSpace(cfg.Build.EnvFile)
	}
	if path == "" {
		return "", nil
	}
	// 模板变量在 build 阶段再渲染；此处仅处理字面相对路径。
	if strings.Contains(path, "{{") {
		return path, nil
	}
	return internal.AbsoluteExternalPath(session.InvocationRoot(), path)
}

func newStandaloneRunID() (string, error) {
	return internal.NewRunID()
}

// resolveComposeLocalPaths 将 compose 本地上传源解析为 InvocationRoot 绝对路径。
func resolveComposeLocalPaths(session *releaseSession, localFile, localEnvFile string) (string, string, error) {
	var err error
	if localFile, err = absIfLiteral(session.InvocationRoot(), localFile); err != nil {
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
