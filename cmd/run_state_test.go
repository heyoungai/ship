package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/heyoungai/ship/internal"
)

var errExpected = errors.New("expected failure")

func TestPrepareRunState_ResumeKeepsSucceededStages(t *testing.T) {
	stateRoot := t.TempDir()
	identity := internal.ReleaseIdentity{Version: "v1.0.0", SourceMode: internal.SourceModeStatic, SourceCommit: "commit"}
	cfg := &internal.Config{
		Build:   internal.BuildConfig{Driver: "command", Command: internal.BuildCommandConfig{Run: "make build"}},
		Publish: internal.PublishConfig{Driver: "none"},
		Deploy:  internal.DeployConfig{Driver: "none"},
	}
	recipe, err := internal.ConfigFingerprint(cfg)
	if err != nil {
		t.Fatalf("ConfigFingerprint() error = %v", err)
	}
	manifest := internal.NewReleaseManifest(identity, "resume123", "test")
	checkpoint := internal.NewRunCheckpoint("resume123", identity, recipe, []string{"default"}, "test")
	checkpoint.MarkStage("build", "default", internal.StageSucceeded, "")
	if err := internal.SaveReleaseManifest(stateRoot, manifest, false); err != nil {
		t.Fatalf("SaveReleaseManifest() error = %v", err)
	}
	if err := internal.SaveRunCheckpoint(stateRoot, checkpoint); err != nil {
		t.Fatalf("SaveRunCheckpoint() error = %v", err)
	}

	session := &releaseSession{
		Identity: identity,
		Roots:    internal.ExecutionRoots{StateRoot: stateRoot, RunID: "fresh"},
		Manifest: internal.NewReleaseManifest(identity, "fresh", "test"),
	}
	state, err := prepareRunState(session, cfg, []internal.Profile{{Default: true}}, recipe, "resume123", false)
	if err != nil {
		t.Fatalf("prepareRunState() error = %v", err)
	}
	if !state.reused || session.RunID() != "resume123" {
		t.Fatalf("resume did not reuse state: reused=%v run_id=%s", state.reused, session.RunID())
	}
	if state.checkpoint.StageStatus("build", "default") != internal.StageSucceeded {
		t.Fatalf("build status = %s", state.checkpoint.StageStatus("build", "default"))
	}
}

func TestResolveRunVersionForResumeUsesCheckpointVersion(t *testing.T) {
	dir := t.TempDir()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	identity := internal.ReleaseIdentity{Version: "v9.9.9", SourceMode: internal.SourceModeStatic, SourceCommit: "commit"}
	checkpoint := internal.NewRunCheckpoint("resume-version", identity, "sha256:recipe", []string{"default"}, "test")
	stateRoot := filepath.Join(dir, ".ship")
	if err := internal.SaveRunCheckpoint(stateRoot, checkpoint); err != nil {
		t.Fatalf("SaveRunCheckpoint() error = %v", err)
	}
	version, err := resolveRunVersionForResume("", "resume-version")
	if err != nil || version != "v9.9.9" {
		t.Fatalf("resolveRunVersionForResume() = %q, %v", version, err)
	}
	if _, err := resolveRunVersionForResume("v1.0.0", "resume-version"); err == nil {
		t.Fatal("mismatched version should fail")
	}
}

func TestRunStagePersistsFailureForResume(t *testing.T) {
	stateRoot := t.TempDir()
	identity := internal.ReleaseIdentity{Version: "v1.0.0", SourceMode: internal.SourceModeStatic, SourceCommit: "commit"}
	session := &releaseSession{
		Identity: identity,
		Roots:    internal.ExecutionRoots{StateRoot: stateRoot, RunID: "run-fail"},
		Manifest: internal.NewReleaseManifest(identity, "run-fail", "test"),
	}
	checkpoint := internal.NewRunCheckpoint("run-fail", identity, "sha256:recipe", []string{"default"}, "test")
	if err := internal.SaveReleaseManifest(stateRoot, session.Manifest, false); err != nil {
		t.Fatalf("SaveReleaseManifest() error = %v", err)
	}
	if _, err := runStage(session, checkpoint, "deploy", "", func() error { return errExpected }); err == nil {
		t.Fatal("runStage() error = nil, want failure")
	}
	loaded, err := internal.LoadRunCheckpoint(stateRoot, "run-fail")
	if err != nil {
		t.Fatalf("LoadRunCheckpoint() error = %v", err)
	}
	if loaded.StageStatus("deploy", "") != internal.StageFailed {
		t.Fatalf("deploy status = %q", loaded.StageStatus("deploy", ""))
	}
}

func TestRunStageSkipsAlreadySucceededStage(t *testing.T) {
	stateRoot := t.TempDir()
	identity := internal.ReleaseIdentity{Version: "v1.0.0", SourceMode: internal.SourceModeStatic, SourceCommit: "commit"}
	session := &releaseSession{
		Identity: identity,
		Roots:    internal.ExecutionRoots{StateRoot: stateRoot, RunID: "run-skip"},
		Manifest: internal.NewReleaseManifest(identity, "run-skip", "test"),
	}
	checkpoint := internal.NewRunCheckpoint("run-skip", identity, "sha256:recipe", []string{"default"}, "test")
	checkpoint.MarkStage("publish", "default", internal.StageSucceeded, "")
	called := false
	ran, err := runStage(session, checkpoint, "publish", "default", func() error {
		called = true
		return nil
	})
	if err != nil || ran || called {
		t.Fatalf("succeeded stage should be reused: ran=%v called=%v err=%v", ran, called, err)
	}
}

func TestValidateCheckpointCompatibilityRejectsChangedRecipe(t *testing.T) {
	identity := internal.ReleaseIdentity{Version: "v1.0.0", SourceMode: internal.SourceModeStatic, SourceCommit: "commit"}
	checkpoint := internal.NewRunCheckpoint("run-compatible", identity, "sha256:old", []string{"default"}, "test")
	manifest := internal.NewReleaseManifest(identity, "run-compatible", "test")
	err := validateCheckpointCompatibility(checkpoint, manifest, identity, "sha256:new", []internal.Profile{{Default: true}})
	if err == nil {
		t.Fatal("changed recipe should be rejected")
	}
}
