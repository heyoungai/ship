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

func TestDigestsMatch_PinInIndexMembers(t *testing.T) {
	recorded := "sha256:aaaaaaaa"
	remote := "index:sha256:bbbbbbbb,sha256:aaaaaaaa"
	if !DigestsMatch(recorded, remote) {
		t.Fatal("expected pin digest to match index member")
	}
	if DigestsMatch("sha256:cccccccc", remote) {
		t.Fatal("expected mismatch")
	}
}

func TestIsPinableDigest(t *testing.T) {
	if !IsPinableDigest("sha256:abc123") {
		t.Fatal("expected pinable")
	}
	if IsPinableDigest("index:sha256:a,sha256:b") {
		t.Fatal("index aggregate must not be pinable")
	}
	if IsPinableDigest("sha256:cfg [\"sha256:l1\"]") {
		t.Fatal("local config fingerprint must not be pinable")
	}
	if PinDigestToken("index:sha256:a,sha256:b") != "" {
		t.Fatal("PinDigestToken must reject index aggregate")
	}
	if PinDigestToken("sha256:deadbeef more") != "sha256:deadbeef" {
		t.Fatalf("PinDigestToken=%q", PinDigestToken("sha256:deadbeef more"))
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

func TestParseManifestDigest_Index(t *testing.T) {
	raw := []byte(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.index.v1+json",
		"manifests": [
			{"digest": "sha256:aaa", "platform": {"architecture": "amd64", "os": "linux"}},
			{"digest": "sha256:bbb", "platform": {"architecture": "arm64", "os": "linux"}}
		]
	}`)
	got, err := parseManifestDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "index:") || !strings.Contains(got, "sha256:aaa") {
		t.Fatalf("unexpected index fingerprint: %q", got)
	}
	if IsPinableDigest(got) || PinDigestToken(got) != "" {
		t.Fatalf("index fingerprint must not be pinable: %q", got)
	}
}

func TestParseManifestDigest_NotFoundShape(t *testing.T) {
	_, err := parseManifestDigest([]byte(`{"foo":1}`))
	if err == nil {
		t.Fatal("expected error for unusable manifest")
	}
}
