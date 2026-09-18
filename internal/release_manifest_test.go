package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndFindReleaseManifest(t *testing.T) {
	state := t.TempDir()
	m := NewReleaseManifest(ReleaseIdentity{
		Version:      "v1.2.3",
		SourceMode:   SourceModeGitTag,
		SourceRef:    "refs/tags/v1.2.3",
		SourceCommit: "abc123",
	}, "run001", "dev")
	m.UpsertArtifact(ArtifactRecord{
		Type:     ArtifactTypeImage,
		Profile:  "default",
		Platform: "linux/amd64",
		LocalRef: "home:ship-build-run001-default",
		Ref:      "registry.example.com/ns/home:v1.2.3",
		Digest:   "sha256:deadbeef",
	})

	if err := SaveReleaseManifest(state, m, true); err != nil {
		t.Fatal(err)
	}

	runPath := RunManifestPath(state, "run001")
	if _, err := os.Stat(runPath); err != nil {
		t.Fatalf("run manifest missing: %v", err)
	}
	idxPath := ReleaseIndexPath(state, "v1.2.3")
	if _, err := os.Stat(idxPath); err != nil {
		t.Fatalf("release index missing: %v", err)
	}

	found, err := FindReleaseManifest(state, "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if found.RunID != "run001" || found.PrimaryImageDigest() != "sha256:deadbeef" {
		t.Fatalf("unexpected manifest: %+v", found)
	}
}

func TestFindReleaseManifest_Missing(t *testing.T) {
	state := t.TempDir()
	_, err := FindReleaseManifest(state, "v9.9.9")
	if err == nil {
		t.Fatal("expected missing manifest error")
	}
}

func TestSelectImageArtifact_PrefersPublishedDigestOverLocalBuild(t *testing.T) {
	m := NewReleaseManifest(ReleaseIdentity{Version: "v1"}, "r1", "")
	m.UpsertArtifact(ArtifactRecord{
		Type:     ArtifactTypeImage,
		Profile:  "default",
		Platform: "linux/amd64",
		LocalRef: "olive-gateway:ship-build-r1-default",
	})
	m.UpsertArtifact(ArtifactRecord{
		Type:     ArtifactTypeImage,
		Profile:  "default",
		Platform: "linux/amd64",
		Ref:      "sgccr.example.com/ns/olive-gateway:v1",
		LocalRef: "olive-gateway:ship-build-r1-default",
		Digest:   "sha256:8bcffc481b30031e565f64f6c94b0fe8c06aa91cde754280e897df1a4b1ab737",
	})

	got := m.SelectImageArtifact(Profile{})
	if got.Digest != "sha256:8bcffc481b30031e565f64f6c94b0fe8c06aa91cde754280e897df1a4b1ab737" {
		t.Fatalf("digest=%q ref=%q", got.Digest, got.Ref)
	}
	if got.Ref != "sgccr.example.com/ns/olive-gateway:v1" {
		t.Fatalf("ref=%q", got.Ref)
	}
	if m.PrimaryImageDigest() != got.Digest {
		t.Fatalf("PrimaryImageDigest=%q", m.PrimaryImageDigest())
	}
}

func TestSelectImageArtifact_KeepsMatchingProfile(t *testing.T) {
	m := NewReleaseManifest(ReleaseIdentity{Version: "v1"}, "r1", "")
	m.UpsertArtifact(ArtifactRecord{
		Type:     ArtifactTypeImage,
		Profile:  "brand-a",
		LocalRef: "app:local-a",
	})
	m.UpsertArtifact(ArtifactRecord{
		Type:    ArtifactTypeImage,
		Profile: "brand-a",
		Ref:     "reg/app:v1-brand-a",
		Digest:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	m.UpsertArtifact(ArtifactRecord{
		Type:    ArtifactTypeImage,
		Profile: "brand-b",
		Ref:     "reg/app:v1-brand-b",
		Digest:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})

	got := m.SelectImageArtifact(Profile{Name: "brand-a"})
	if got.Ref != "reg/app:v1-brand-a" {
		t.Fatalf("ref=%q digest=%q", got.Ref, got.Digest)
	}
}

func TestUpsertArtifact_MergesDigest(t *testing.T) {
	m := NewReleaseManifest(ReleaseIdentity{Version: "v1"}, "r1", "")
	m.UpsertArtifact(ArtifactRecord{
		Type:     ArtifactTypeImage,
		Profile:  "default",
		LocalRef: "img:local",
		Ref:      "reg/img:v1",
	})
	m.UpsertArtifact(ArtifactRecord{
		Type:    ArtifactTypeImage,
		Profile: "default",
		Ref:     "reg/img:v1",
		Digest:  "sha256:aaa",
	})
	if len(m.Artifacts) != 1 {
		t.Fatalf("len=%d", len(m.Artifacts))
	}
	if m.Artifacts[0].LocalRef != "img:local" || m.Artifacts[0].Digest != "sha256:aaa" {
		t.Fatalf("%+v", m.Artifacts[0])
	}
}

func TestReleaseIndexPath_Sanitizes(t *testing.T) {
	p := ReleaseIndexPath("/tmp/.ship", "v1.2.3/rc")
	if filepath.Base(p) != "v1.2.3_rc.json" {
		t.Fatalf("got %s", p)
	}
}
