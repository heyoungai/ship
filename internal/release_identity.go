package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Source mode constants for ReleaseIdentity.
const (
	SourceModeGitTag  = "git-tag"
	SourceModeCurrent = "current"
	SourceModeStatic  = "static"
	SourceModeEnv     = "env"
)

// ReleaseIdentity 在计划阶段一次性解析完成，后续阶段只消费结果。
type ReleaseIdentity struct {
	Version      string // 对外版本，例如 v1.2.3
	SourceMode   string // git-tag | current | static | env
	SourceRef    string // refs/tags/v1.2.3；current 模式可为空
	SourceCommit string // 完整 commit SHA
	HeadCommit   string // 调用时 HEAD commit，用于展示 delta
	AheadBy      int    // HEAD 相对 SourceCommit 超前的提交数；未知为 -1
	Dirty        bool   // InvocationRoot 工作区是否有未提交修改
}

// ResolveReleaseIdentity 解析版本字符串，并在 git-tag 模式下锁定 tag 对应的 commit。
func ResolveReleaseIdentity(cfg *Config, version string) (ReleaseIdentity, error) {
	return resolveReleaseIdentityWithLookup(cfg, version, os.Getenv, GetLatestTag, defaultGitLookup{})
}

type gitLookup interface {
	ResolveTagCommit(tag string) (string, error)
	HeadCommit() (string, error)
	CommitsAhead(commit string) (int, error)
	IsDirty() (bool, error)
}

type defaultGitLookup struct{}

func (defaultGitLookup) ResolveTagCommit(tag string) (string, error) {
	return resolveTagCommit(tag)
}

func (defaultGitLookup) HeadCommit() (string, error) {
	return gitOutput("rev-parse", "HEAD")
}

func (defaultGitLookup) CommitsAhead(commit string) (int, error) {
	out, err := gitOutput("rev-list", "--count", commit+"..HEAD")
	if err != nil {
		return -1, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return -1, fmt.Errorf("解析 ahead 计数失败: %w", err)
	}
	return n, nil
}

func (defaultGitLookup) IsDirty() (bool, error) {
	out, err := gitOutput("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func resolveReleaseIdentityWithLookup(
	cfg *Config,
	version string,
	getenv func(string) string,
	latestTag func() (string, error),
	git gitLookup,
) (ReleaseIdentity, error) {
	resolved, err := resolveVersionWithLookup(cfg, version, getenv, latestTag)
	if err != nil {
		return ReleaseIdentity{}, err
	}

	source := ""
	fallback := ""
	if cfg != nil {
		source = cfg.Version.Source
		fallback = cfg.Version.Fallback
	}
	if source == "" {
		source = SourceModeGitTag
	}

	identity := ReleaseIdentity{
		Version:    resolved,
		SourceMode: source,
		AheadBy:    -1,
	}

	head, headErr := git.HeadCommit()
	if headErr == nil {
		identity.HeadCommit = head
	}
	if dirty, dirtyErr := git.IsDirty(); dirtyErr == nil {
		identity.Dirty = dirty
	}

	switch source {
	case SourceModeGitTag, "":
		// fallback 降级为 dev 时，视为显式开发构建，不绑定正式 tag。
		if resolved == "dev" && (fallback == "dev" || strings.TrimSpace(version) == "") {
			identity.SourceMode = SourceModeCurrent
			if identity.HeadCommit != "" {
				identity.SourceCommit = identity.HeadCommit
			}
			return identity, nil
		}
		commit, err := git.ResolveTagCommit(resolved)
		if err != nil {
			return ReleaseIdentity{}, fmt.Errorf("git-tag 模式无法锁定版本 %s: %w", resolved, err)
		}
		identity.SourceMode = SourceModeGitTag
		identity.SourceRef = tagRef(resolved)
		identity.SourceCommit = commit
		if ahead, aheadErr := git.CommitsAhead(commit); aheadErr == nil {
			identity.AheadBy = ahead
		}
		return identity, nil
	case SourceModeEnv:
		identity.SourceMode = SourceModeEnv
		if identity.HeadCommit != "" {
			identity.SourceCommit = identity.HeadCommit
		}
		return identity, nil
	case SourceModeStatic:
		identity.SourceMode = SourceModeStatic
		if identity.HeadCommit != "" {
			identity.SourceCommit = identity.HeadCommit
		}
		return identity, nil
	default:
		return ReleaseIdentity{}, fmt.Errorf("不支持的 version.source: %s", source)
	}
}

// PrintReleaseIdentity 输出 version/ref/commit 与 HEAD delta。
func PrintReleaseIdentity(identity ReleaseIdentity) {
	shortSource := shortSHA(identity.SourceCommit)
	shortHead := shortSHA(identity.HeadCommit)

	PrintInfo(fmt.Sprintf("release: %s", identity.Version))
	if identity.SourceRef != "" {
		PrintInfo(fmt.Sprintf("source:  %s @ %s", identity.SourceRef, shortSource))
	} else if identity.SourceCommit != "" {
		PrintInfo(fmt.Sprintf("source:  %s @ %s", identity.SourceMode, shortSource))
	} else {
		PrintInfo(fmt.Sprintf("source:  %s", identity.SourceMode))
	}

	if identity.HeadCommit != "" {
		delta := "same as source"
		if identity.SourceCommit != "" && identity.HeadCommit != identity.SourceCommit {
			if identity.AheadBy > 0 {
				delta = fmt.Sprintf("ahead by %d commits; ignored", identity.AheadBy)
			} else {
				delta = "differs from source; ignored"
			}
		}
		PrintInfo(fmt.Sprintf("current: HEAD @ %s (%s)", shortHead, delta))
	}

	if identity.AheadBy > 0 && identity.SourceMode == SourceModeGitTag {
		PrintWarning(fmt.Sprintf(
			"HEAD 领先 %s %d 个提交；本次构建使用 tag 源码，忽略当前工作区后续提交",
			identity.Version, identity.AheadBy,
		))
	}
	if identity.Dirty && identity.SourceMode == SourceModeGitTag {
		PrintWarning("InvocationRoot 工作区有未提交修改；git-tag 构建不会包含这些修改")
	}
}

// NeedsSourceSnapshot 返回是否需要创建 detached worktree。
func (r ReleaseIdentity) NeedsSourceSnapshot() bool {
	return r.SourceMode == SourceModeGitTag && strings.TrimSpace(r.SourceCommit) != ""
}

func tagRef(version string) string {
	version = strings.TrimSpace(version)
	if strings.HasPrefix(version, "refs/tags/") {
		return version
	}
	return "refs/tags/" + version
}

func resolveTagCommit(tag string) (string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", fmt.Errorf("tag 不能为空")
	}
	ref := tagRef(tag)
	// 使用 ^{commit} 同时支持 annotated / lightweight tag，并强制走 refs/tags/ 避免与分支同名歧义。
	out, err := gitOutput("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("本地不存在 tag %s（完整引用 %s）；请确认已 fetch 对应 tag", tag, ref)
	}
	return strings.TrimSpace(out), nil
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if stderr != "" {
			return "", fmt.Errorf("git %s 失败: %s", strings.Join(args, " "), stderr)
		}
		return "", fmt.Errorf("git %s 失败: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func shortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// ResolvePathAgainstRoot 将相对路径解析为相对 root 的绝对路径；已是绝对路径则原样 Clean。
func ResolvePathAgainstRoot(root, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if strings.TrimSpace(root) == "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	return filepath.Clean(filepath.Join(root, path)), nil
}
