package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunCheckpoint_SaveLoadAndInterruptRunningStage(t *testing.T) {
	stateRoot := t.TempDir()
	identity := ReleaseIdentity{Version: "v1.2.3", SourceMode: SourceModeGitTag, SourceRef: "refs/tags/v1.2.3", SourceCommit: "abc"}
	checkpoint := NewRunCheckpoint("run123", identity, "sha256:recipe", []string{"default", "brand-a"}, "test")
	checkpoint.MarkStage("build", "brand-a", StageRunning, "")
	checkpoint.SyncArtifacts(&ReleaseManifest{Artifacts: []ArtifactRecord{{Type: ArtifactTypeImage, Profile: "brand-a", Ref: "reg/app:v1.2.3-brand-a", Digest: "sha256:abc"}}})
	if err := SaveRunCheckpoint(stateRoot, checkpoint); err != nil {
		t.Fatalf("SaveRunCheckpoint() error = %v", err)
	}

	path := RunCheckpointPath(stateRoot, "run123")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("checkpoint missing at %s: %v", path, err)
	}
	loaded, err := LoadRunCheckpoint(stateRoot, "run123")
	if err != nil {
		t.Fatalf("LoadRunCheckpoint() error = %v", err)
	}
	if loaded.StageStatus("build", "brand-a") != StageRunning {
		t.Fatalf("stage status = %q", loaded.StageStatus("build", "brand-a"))
	}
	if !loaded.MarkRunningInterrupted() || loaded.StageStatus("build", "brand-a") != StageInterrupted {
		t.Fatalf("running stage was not converted to interrupted: %#v", loaded.Stages)
	}
	if err := SaveRunCheckpoint(stateRoot, loaded); err != nil {
		t.Fatalf("SaveRunCheckpoint(interrupted) error = %v", err)
	}
	if matches, err := filepath.Glob(filepath.Join(stateRoot, "runs", "run123", ".run-*.tmp")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary checkpoint files = %v, err=%v", matches, err)
	}
}

func TestConfigFingerprintAndProfileComparison(t *testing.T) {
	cfg := &Config{Schema: 2, ImageName: "app", Vars: map[string]string{"b": "2", "a": "1"}}
	first, err := ConfigFingerprint(cfg)
	if err != nil {
		t.Fatalf("ConfigFingerprint() error = %v", err)
	}
	second, err := ConfigFingerprint(cfg)
	if err != nil || first != second {
		t.Fatalf("fingerprint unstable: %q %q, err=%v", first, second, err)
	}
	if !SameProfileNames([]string{"brand-a", "default"}, []string{"default", "brand-a"}) {
		t.Fatal("profiles should compare as an unordered set")
	}
}
