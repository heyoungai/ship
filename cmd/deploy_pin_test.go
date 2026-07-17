package cmd

import (
	"testing"

	"github.com/heyoungai/ship/internal"
)

func TestComposeEnvUpdates_DigestPin(t *testing.T) {
	cfg := &internal.Config{}
	cfg.Deploy.Compose.TagKey = "APP_IMAGE_TAG"
	cfg.Deploy.Compose.Pin = "digest"
	cfg.Deploy.Compose.DigestKey = "APP_IMAGE_DIGEST"
	cfg.Deploy.Compose.ImageKey = "APP_IMAGE"

	session := &releaseSession{
		Manifest: &internal.ReleaseManifest{
			Artifacts: []internal.ArtifactRecord{{
				Type:    internal.ArtifactTypeImage,
				Profile: "default",
				Ref:     "reg.example.com/ns/app:v1.2.3",
				Digest:  "sha256:abc123",
			}},
		},
	}

	updates, err := composeEnvUpdates(cfg, "v1.2.3", internal.Profile{Default: true}, session)
	if err != nil {
		t.Fatal(err)
	}
	if updates["APP_IMAGE_TAG"] != "v1.2.3" {
		t.Fatalf("tag=%q", updates["APP_IMAGE_TAG"])
	}
	if updates["APP_IMAGE_DIGEST"] != "sha256:abc123" {
		t.Fatalf("digest=%q", updates["APP_IMAGE_DIGEST"])
	}
	if updates["APP_IMAGE"] != "reg.example.com/ns/app@sha256:abc123" {
		t.Fatalf("image=%q", updates["APP_IMAGE"])
	}
}

func TestComposeEnvUpdates_TagPin(t *testing.T) {
	cfg := &internal.Config{}
	cfg.Deploy.Compose.TagKey = "APP_IMAGE_TAG"
	cfg.Deploy.Compose.Pin = "tag"
	cfg.Deploy.Compose.DigestKey = "APP_IMAGE_DIGEST"

	session := &releaseSession{
		Manifest: &internal.ReleaseManifest{
			Artifacts: []internal.ArtifactRecord{{
				Type:   internal.ArtifactTypeImage,
				Ref:    "reg/app:v1",
				Digest: "sha256:abc",
			}},
		},
	}
	updates, err := composeEnvUpdates(cfg, "v1", internal.Profile{}, session)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates["APP_IMAGE_TAG"] != "v1" {
		t.Fatalf("%v", updates)
	}
}
