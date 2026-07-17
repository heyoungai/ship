package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBeginSourceSnapshot_GitTagWorktree(t *testing.T) {
	repo := initTestRepo(t)
	tagCommit := gitOutputMust(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "tag", "v1.0.0")

	// Advance HEAD past the tag.
	writeFile(t, filepath.Join(repo, "after.txt"), "after tag\n")
	runGit(t, repo, "add", "after.txt")
	runGit(t, repo, "commit", "-m", "after")

	identity := ReleaseIdentity{
		Version:      "v1.0.0",
		SourceMode:   SourceModeGitTag,
		SourceRef:    "refs/tags/v1.0.0",
		SourceCommit: tagCommit,
	}

	snap, err := BeginSourceSnapshot(identity, repo)
	if err != nil {
		t.Fatalf("BeginSourceSnapshot: %v", err)
	}
	defer func() {
		if err := snap.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	if snap.Roots.SourceRoot == snap.Roots.InvocationRoot {
		t.Fatal("expected detached SourceRoot different from InvocationRoot")
	}
	if _, err := os.Stat(filepath.Join(snap.Roots.SourceRoot, "after.txt")); !os.IsNotExist(err) {
		t.Fatalf("worktree should not contain post-tag file, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(snap.Roots.SourceRoot, "README.md")); err != nil {
		t.Fatalf("worktree missing tagged file: %v", err)
	}
	head := gitOutputMust(t, snap.Roots.SourceRoot, "rev-parse", "HEAD")
	if head != tagCommit {
		t.Fatalf("worktree HEAD = %s, want %s", head, tagCommit)
	}
}

func TestBeginSourceSnapshot_CurrentNoWorktree(t *testing.T) {
	repo := initTestRepo(t)
	identity := ReleaseIdentity{
		Version:    "dev",
		SourceMode: SourceModeCurrent,
	}
	snap, err := BeginSourceSnapshot(identity, repo)
	if err != nil {
		t.Fatalf("BeginSourceSnapshot: %v", err)
	}
	defer snap.Close()
	if snap.Roots.SourceRoot != snap.Roots.InvocationRoot {
		t.Fatalf("current mode SourceRoot = %s, want %s", snap.Roots.SourceRoot, snap.Roots.InvocationRoot)
	}
	if snap.Roots.RunID == "" {
		t.Fatal("RunID should be set")
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "ship@test.local")
	runGit(t, dir, "config", "user.name", "ship-test")
	writeFile(t, filepath.Join(dir, "README.md"), "hello\n")
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s (%v)", args, out, err)
	}
}

func gitOutputMust(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitOutputIn(dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveTagCommit_AnnotatedAndLightweight(t *testing.T) {
	repo := initTestRepo(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	lightCommit := gitOutputMust(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "tag", "v-light")

	writeFile(t, filepath.Join(repo, "ann.txt"), "ann\n")
	runGit(t, repo, "add", "ann.txt")
	runGit(t, repo, "commit", "-m", "annotated base")
	annCommit := gitOutputMust(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "tag", "-a", "v-ann", "-m", "annotated")

	// branch with same name as a tag should not win over refs/tags/
	runGit(t, repo, "branch", "v-light")

	gotLight, err := resolveTagCommit("v-light")
	if err != nil {
		t.Fatal(err)
	}
	if gotLight != lightCommit {
		t.Fatalf("lightweight tag commit = %s, want %s", gotLight, lightCommit)
	}

	gotAnn, err := resolveTagCommit("v-ann")
	if err != nil {
		t.Fatal(err)
	}
	if gotAnn != annCommit {
		t.Fatalf("annotated tag commit = %s, want %s", gotAnn, annCommit)
	}
}
