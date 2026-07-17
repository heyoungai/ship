package internal

import (
	"strings"
	"testing"
)

func TestDigestsCompatible_SameConfig(t *testing.T) {
	local := "sha256:abc123 [\"sha256:layer1\",\"sha256:layer2\"]"
	remote := "sha256:abc123 sha256:layer1,sha256:layer2"
	if !digestsCompatible(local, remote) {
		t.Fatal("expected compatible digests")
	}
}

func TestDigestsCompatible_Different(t *testing.T) {
	local := "sha256:aaa [\"sha256:l1\"]"
	remote := "sha256:bbb sha256:l2"
	if digestsCompatible(local, remote) {
		t.Fatal("expected incompatible digests")
	}
}

func TestParseManifestDigest_ConfigAndLayers(t *testing.T) {
	raw := []byte(`{
		"schemaVersion": 2,
		"config": {"digest": "sha256:cfg"},
		"layers": [{"digest": "sha256:l1"}, {"digest": "sha256:l2"}]
	}`)
	got, err := parseManifestDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "sha256:cfg") || !strings.Contains(got, "sha256:l1") {
		t.Fatalf("unexpected digest fingerprint: %q", got)
	}
}

func TestParseManifestDigest_NotFoundShape(t *testing.T) {
	_, err := parseManifestDigest([]byte(`{"foo":1}`))
	if err == nil {
		t.Fatal("expected error for unusable manifest")
	}
}
