package internal

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// ExecutionRoots 避免通过全局 cwd 隐式混用源码、外部输入和运行状态。
type ExecutionRoots struct {
	InvocationRoot string // 用户执行 ship 的原始项目目录
	SourceRoot     string // 已锁定源码快照所在的 worktree（或等于 InvocationRoot）
	StateRoot      string // .ship 等持久状态，始终相对 InvocationRoot
	RunID          string
}

// SourceSnapshot 管理一次 run 的 SourceRoot 生命周期。
type SourceSnapshot struct {
	Roots        ExecutionRoots
	Identity     ReleaseIdentity
	changedDir   bool
	previousDir  string
	worktreePath string
	mu           sync.Mutex
	closed       bool
}

// BeginSourceSnapshot 根据 identity 创建执行根目录；git-tag 模式使用 detached worktree。
func BeginSourceSnapshot(identity ReleaseIdentity, invocationRoot string) (*SourceSnapshot, error) {
	absInvocation, err := filepath.Abs(invocationRoot)
	if err != nil {
		return nil, fmt.Errorf("解析 InvocationRoot 失败: %w", err)
	}

	runID, err := newRunID()
	if err != nil {
		return nil, err
	}

	roots := ExecutionRoots{
		InvocationRoot: absInvocation,
		StateRoot:      filepath.Join(absInvocation, ".ship"),
		RunID:          runID,
		SourceRoot:     absInvocation,
	}

	snap := &SourceSnapshot{
		Roots:    roots,
		Identity: identity,
	}

	if !identity.NeedsSourceSnapshot() {
		return snap, nil
	}

	repoID := repositoryID(absInvocation)
	sourceRoot := filepath.Join(os.TempDir(), "ship", repoID, runID, "source")
	if err := os.MkdirAll(filepath.Dir(sourceRoot), 0o755); err != nil {
		return nil, fmt.Errorf("创建 worktree 父目录失败: %w", err)
	}

	if err := gitWorktreeAdd(absInvocation, sourceRoot, identity.SourceCommit); err != nil {
		return nil, err
	}
	snap.worktreePath = sourceRoot
	snap.Roots.SourceRoot = sourceRoot

	head, err := gitOutputIn(sourceRoot, "rev-parse", "HEAD")
	if err != nil {
		_ = snap.cleanupWorktree()
		return nil, fmt.Errorf("校验 worktree HEAD 失败: %w", err)
	}
	if strings.TrimSpace(head) != identity.SourceCommit {
		_ = snap.cleanupWorktree()
		return nil, fmt.Errorf(
			"worktree HEAD (%s) 与锁定 commit (%s) 不一致",
			strings.TrimSpace(head), identity.SourceCommit,
		)
	}

	return snap, nil
}

// Enter 将进程 cwd 切换到 SourceRoot（过渡实现）；Close 时恢复。
func (s *SourceSnapshot) Enter() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("source snapshot 已关闭")
	}
	if s.Roots.SourceRoot == "" || s.Roots.SourceRoot == s.Roots.InvocationRoot {
		return nil
	}
	prev, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(s.Roots.SourceRoot); err != nil {
		return fmt.Errorf("切换到 SourceRoot 失败: %w", err)
	}
	s.previousDir = prev
	s.changedDir = true
	return nil
}

// Close 恢复 cwd 并清理临时 worktree。
func (s *SourceSnapshot) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	var firstErr error
	if s.changedDir && s.previousDir != "" {
		if err := os.Chdir(s.previousDir); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("恢复 InvocationRoot cwd 失败: %w", err)
		}
	}
	if err := s.cleanupWorktree(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *SourceSnapshot) cleanupWorktree() error {
	if s.worktreePath == "" {
		return nil
	}
	cmd := exec.Command("git", "worktree", "remove", "--force", s.worktreePath)
	cmd.Dir = s.Roots.InvocationRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		PrintWarning(fmt.Sprintf(
			"清理临时 worktree 失败: %s (%v); 可手动执行: git -C %s worktree remove --force %s",
			strings.TrimSpace(string(out)), err,
			s.Roots.InvocationRoot, s.worktreePath,
		))
		return fmt.Errorf("清理 worktree 失败: %s: %w", s.worktreePath, err)
	}
	// 尽量清理 run 目录（忽略错误）。
	_ = os.RemoveAll(filepath.Dir(s.worktreePath))
	s.worktreePath = ""
	return nil
}

func gitWorktreeAdd(repo, path, commit string) error {
	cmd := exec.Command("git", "worktree", "add", "--detach", path, commit)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("创建 detached worktree 失败: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func gitOutputIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
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

// NewRunID 生成短随机 run ID，用于本地镜像 tag 与临时目录隔离。
func NewRunID() (string, error) {
	return newRunID()
}

func newRunID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("生成 run ID 失败: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func repositoryID(absPath string) string {
	sum := sha1.Sum([]byte(filepath.Clean(absPath)))
	return hex.EncodeToString(sum[:8])
}

// AbsoluteExternalPath 在创建 worktree 前把外部输入解析为 InvocationRoot 绝对路径。
func AbsoluteExternalPath(invocationRoot, path string) (string, error) {
	return ResolvePathAgainstRoot(invocationRoot, path)
}
