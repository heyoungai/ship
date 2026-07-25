package internal

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestDigestsCompatible_SameLayersDifferentConfig(t *testing.T) {
	local := `sha256:aaaaaaaa ["sha256:layer1","sha256:layer2"]`
	remote := "sha256:bbbbbbbb sha256:layer1,sha256:layer2"
	if digestsCompatible(local, remote) {
		t.Fatal("config changes must not be hidden by identical layers")
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

func TestRegistryIdentitiesMatch_ListDigestAndMembers(t *testing.T) {
	local := []string{"sha256:11111111"}
	remote := []string{
		"index:sha256:aaaaaaaa,sha256:bbbbbbbb",
		"sha256:11111111",
	}
	if !registryIdentitiesMatch(local, remote) {
		t.Fatal("expected local list digest to match the resolved remote index digest")
	}
}

func TestRegistryIdentitiesMatch_LocalConfigAndExpandedIndexMember(t *testing.T) {
	local := []string{`sha256:cccccccc ["sha256:l1","sha256:l2"]`}
	remote := []string{
		"index:sha256:aaaaaaaa,sha256:bbbbbbbb",
		"sha256:cccccccc sha256:l1,sha256:l2",
		"sha256:attest sha256:metadata",
	}
	if !registryIdentitiesMatch(local, remote) {
		t.Fatal("expected local config/layers to match an expanded remote index member")
	}
}

func TestRegistryIdentitiesMatch_DifferentContent(t *testing.T) {
	local := []string{`sha256:cccccccc ["sha256:l1","sha256:l2"]`}
	remote := []string{
		"sha256:11111111",
		"sha256:dddddddd sha256:l1,sha256:other",
	}
	if registryIdentitiesMatch(local, remote) {
		t.Fatal("different image content must not match")
	}
}

func TestEnsureRegistryTagImmutable_ExpandsIndexMembers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake docker executable uses a POSIX shell")
	}

	binDir := t.TempDir()
	dockerPath := filepath.Join(binDir, "docker")
	script := `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "inspect" ]; then
	case "$3" in
		*"@sha256:aaaaaaaa")
			printf '%s\n' '{"schemaVersion":2,"config":{"digest":"sha256:cccccccc"},"layers":[{"digest":"sha256:dddddddd"},{"digest":"sha256:eeeeeeee"}]}'
			exit 0
			;;
		*"@sha256:bbbbbbbb")
			printf '%s\n' '{"schemaVersion":2,"config":{"digest":"sha256:ffffffff"},"layers":[{"digest":"sha256:11111111"}]}'
			exit 0
			;;
		*)
			printf '%s\n' '{"schemaVersion":2,"manifests":[{"digest":"sha256:aaaaaaaa"},{"digest":"sha256:bbbbbbbb"}]}'
			exit 0
			;;
	esac
fi
if [ "$1" = "image" ] && [ "$2" = "inspect" ]; then
	case "$5" in
		*"RepoDigests"*) printf '%s\n' '[]' ;;
		*) printf '%s\n' 'sha256:cccccccc ["sha256:dddddddd","sha256:eeeeeeee"]' ;;
	esac
	exit 0
fi
if [ "$1" = "buildx" ]; then
	printf '%s\n' 'buildx unavailable' >&2
	exit 1
fi
exit 2
`
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	skip, err := EnsureRegistryTagImmutable(
		"registry.example.com/team/app:v0.2.3",
		"registry.example.com/team/app:v0.2.3",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Fatal("same platform image inside remote index should skip push")
	}
}

func TestImmutableTagConflictError_GuidesRetry(t *testing.T) {
	err := immutableTagConflictError(
		"registry.example.com:5000/team/app:v0.2.3",
		"sha256:aaaaaaaa",
		"sha256:bbbbbbbb",
		false,
	)
	message := err.Error()
	for _, want := range []string{
		"与本地发布内容不一致",
		"ship deploy -v v0.2.3 -y",
		"请打新 tag 后重新 ship run",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("conflict error missing %q: %s", want, message)
		}
	}
}

func TestImageTag(t *testing.T) {
	for ref, want := range map[string]string{
		"registry.example.com/team/app:v1.2.3":      "v1.2.3",
		"registry.example.com:5000/app:v2":          "v2",
		"registry.example.com/app:v3@sha256:abcdef": "v3",
		"registry.example.com/app":                  "",
	} {
		if got := imageTag(ref); got != want {
			t.Fatalf("imageTag(%q)=%q want %q", ref, got, want)
		}
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
