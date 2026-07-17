package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 2 验收（计划文档验收项的自动化子集）：
// 1. plan 与 run 共用 CompileReleasePlan（stages/identity 一致）
// 2. manifest 落盘 runs/ + releases/ 索引
// 3. 无 manifest 时 RequireReleaseManifest 失败
// 4. history 可记录 commit/digest/run_id
// 5. LoadConfigFrom(SourceRoot) 读取 tag 内 recipe，与 InvocationRoot 不同
// 6. git-tag worktree 源码来自 tag，不含 HEAD 后续提交

func TestPhase2_PlanMatchesRunStages(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Build.Driver = "docker"
	cfg.Build.Docker.Image = "demo"
	cfg.ImageName = "demo"
	cfg.Features.Publish = true
	cfg.Features.Deploy = true
	cfg.Publish.Driver = "registry"
	cfg.Publish.Registry.Push = true
	cfg.Deploy.Driver = "compose"
	cfg.Deploy.Compose.Host = "host"
	cfg.Deploy.Compose.Path = "/app"
	cfg.Registries = []Registry{{Type: "private", URL: "reg.example.com", Namespace: "ns", Image: "demo"}}

	identity := ReleaseIdentity{
		Version:      "v2.0.0",
		SourceMode:   SourceModeGitTag,
		SourceRef:    "refs/tags/v2.0.0",
		SourceCommit: "deadbeef",
	}
	roots := ExecutionRoots{
		RunID:          "accept01",
		InvocationRoot: "/inv",
		SourceRoot:     "/src",
		StateRoot:      "/inv/.ship",
	}

	planRun, err := CompileReleasePlan(cfg, identity, roots, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	planPreview, err := CompileReleasePlan(cfg, identity, roots, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Join(planRun.Stages, ",") != strings.Join(planPreview.Stages, ",") {
		t.Fatalf("plan/run stages diverge: run=%v preview=%v", planRun.Stages, planPreview.Stages)
	}
	if planRun.Identity.Version != planPreview.Identity.Version ||
		planRun.Identity.SourceCommit != planPreview.Identity.SourceCommit {
		t.Fatal("plan/run identity diverge")
	}
	want := []string{"build", "tag", "publish", "deploy"}
	if len(planRun.Stages) < len(want) {
		t.Fatalf("stages=%v", planRun.Stages)
	}
	for i, s := range want {
		if planRun.Stages[i] != s {
			t.Fatalf("stage[%d]=%s want %s (full=%v)", i, planRun.Stages[i], s, planRun.Stages)
		}
	}
}

func TestPhase2_ManifestRunsAndReleasesIndex(t *testing.T) {
	state := t.TempDir()
	m := NewReleaseManifest(ReleaseIdentity{
		Version:      "v2.1.0",
		SourceMode:   SourceModeGitTag,
		SourceRef:    "refs/tags/v2.1.0",
		SourceCommit: "abc123def",
	}, "run-accept", "test")
	m.UpsertArtifact(ArtifactRecord{
		Type:     ArtifactTypeImage,
		Profile:  "default",
		Platform: "linux/amd64",
		LocalRef: "demo:ship-build-run-accept-default",
		Ref:      "reg.example.com/ns/demo:v2.1.0",
		Digest:   "sha256:cafe",
	})
	if err := SaveReleaseManifest(state, m, true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(RunManifestPath(state, "run-accept")); err != nil {
		t.Fatalf("runs manifest missing: %v", err)
	}
	if _, err := os.Stat(ReleaseIndexPath(state, "v2.1.0")); err != nil {
		t.Fatalf("releases index missing: %v", err)
	}

	found, err := FindReleaseManifest(state, "v2.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if found.RunID != "run-accept" || found.PrimaryImageDigest() != "sha256:cafe" {
		t.Fatalf("%+v", found)
	}
	if !found.HasPublishedImage() {
		t.Fatal("expected published image")
	}
}

func TestPhase2_PushDeployFailWithoutManifest(t *testing.T) {
	state := t.TempDir()
	_, err := RequireReleaseManifest(state, "v9.9.9")
	if err == nil {
		t.Fatal("expected missing manifest error for push/deploy")
	}
	if !strings.Contains(err.Error(), "manifest") && !strings.Contains(err.Error(), "ship run") {
		t.Fatalf("error should guide user to ship run/build+push, got: %v", err)
	}
}

func TestPhase2_HistoryRecordsCommitDigestRunID(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	SetStateRoot(filepath.Join(dir, ".ship"))
	defer ClearStateRoot()

	if err := RecordDeploymentWithMeta("v2.2.0", "deploy", "success", "", HistoryMeta{
		Commit: "abcdef0123456789",
		Digest: "sha256:abc",
		RunID:  "run-hist",
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no history")
	}
	last := entries[len(entries)-1]
	if last.Commit != "abcdef0123456789" || last.Digest != "sha256:abc" || last.RunID != "run-hist" {
		t.Fatalf("history meta missing: %+v", last)
	}
	rendered := FormatHistory(entries, 0)
	if !strings.Contains(rendered, "abcdef0") {
		t.Fatalf("FormatHistory should show short commit, got: %s", rendered)
	}
}

func TestPhase2_TwoPhaseConfigLoadsTagRecipe(t *testing.T) {
	repo := initTestRepo(t)

	// Tag 时刻的 recipe：image = tagged-app
	writeShipTOML(t, repo, "tagged-app")
	runGit(t, repo, "add", "ship.toml")
	runGit(t, repo, "commit", "-m", "add ship.toml")
	tagCommit := gitOutputMust(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "tag", "v2.3.0")

	// HEAD 之后改成另一份 recipe
	writeShipTOML(t, repo, "head-app")
	runGit(t, repo, "add", "ship.toml")
	runGit(t, repo, "commit", "-m", "change recipe")

	identity := ReleaseIdentity{
		Version:      "v2.3.0",
		SourceMode:   SourceModeGitTag,
		SourceRef:    "refs/tags/v2.3.0",
		SourceCommit: tagCommit,
	}
	snap, err := BeginSourceSnapshot(identity, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	invCfg, err := LoadConfigFrom(repo, "")
	if err != nil {
		t.Fatal(err)
	}
	srcCfg, err := LoadConfigFrom(snap.Roots.SourceRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	if invCfg.Build.Docker.Image != "head-app" {
		t.Fatalf("InvocationRoot recipe = %q, want head-app", invCfg.Build.Docker.Image)
	}
	if srcCfg.Build.Docker.Image != "tagged-app" {
		t.Fatalf("SourceRoot recipe = %q, want tagged-app (two-phase)", srcCfg.Build.Docker.Image)
	}
}

func TestPhase2_WorktreeIgnoresPostTagCommits(t *testing.T) {
	repo := initTestRepo(t)
	writeFile(t, filepath.Join(repo, "app.txt"), "from-tag\n")
	runGit(t, repo, "add", "app.txt")
	runGit(t, repo, "commit", "-m", "tag content")
	tagCommit := gitOutputMust(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "tag", "v2.4.0")

	writeFile(t, filepath.Join(repo, "app.txt"), "from-head\n")
	writeFile(t, filepath.Join(repo, "dirty-only.txt"), "uncommitted\n")
	runGit(t, repo, "add", "app.txt")
	runGit(t, repo, "commit", "-m", "after tag")

	snap, err := BeginSourceSnapshot(ReleaseIdentity{
		Version:      "v2.4.0",
		SourceMode:   SourceModeGitTag,
		SourceRef:    "refs/tags/v2.4.0",
		SourceCommit: tagCommit,
	}, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	got, err := os.ReadFile(filepath.Join(snap.Roots.SourceRoot, "app.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "from-tag" {
		t.Fatalf("SourceRoot app.txt = %q, want from-tag", string(got))
	}
	if _, err := os.Stat(filepath.Join(snap.Roots.SourceRoot, "dirty-only.txt")); !os.IsNotExist(err) {
		t.Fatal("uncommitted file must not appear in SourceRoot")
	}
}

func writeShipTOML(t *testing.T, dir, image string) {
	t.Helper()
	content := `schema = 2

[project]
name = "demo"

[features]
publish = true
deploy = false

[build]
driver = "docker"

[build.docker]
image = "` + image + `"
platforms = ["linux/amd64"]
dockerfile = "./Dockerfile"

[publish]
driver = "none"

[deploy]
driver = "none"
`
	writeFile(t, filepath.Join(dir, "ship.toml"), content)
}
