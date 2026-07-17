package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersistFileArtifact(t *testing.T) {
	state := t.TempDir()
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "app.bin")
	if err := os.WriteFile(src, []byte("hello-artifact"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest, digest, err := PersistFileArtifact(state, "run1", "default", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(digest, "sha256:") {
		t.Fatalf("digest=%q", digest)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello-artifact" {
		t.Fatalf("content=%q", data)
	}
	wantPath := filepath.Join(state, "artifacts", "run1", "default", "app.bin")
	if dest != wantPath {
		t.Fatalf("dest=%q want=%q", dest, wantPath)
	}
}

func TestImageRepoFromRef(t *testing.T) {
	cases := map[string]string{
		"registry.example.com/ns/app:v1.2.3":        "registry.example.com/ns/app",
		"registry.example.com:5000/ns/app:v1":       "registry.example.com:5000/ns/app",
		"registry.example.com/ns/app@sha256:abc":    "registry.example.com/ns/app",
		"ns/app:latest":                            "ns/app",
	}
	for in, want := range cases {
		if got := ImageRepoFromRef(in); got != want {
			t.Fatalf("ImageRepoFromRef(%q)=%q want %q", in, got, want)
		}
	}
}

func TestImageDigestRef(t *testing.T) {
	got := ImageDigestRef("reg/ns/app:v1.0.0", "sha256:dead")
	if got != "reg/ns/app@sha256:dead" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveComposePin(t *testing.T) {
	pin, degraded := ResolveComposePin("digest", "sha256:abc")
	if pin != "digest" || degraded {
		t.Fatalf("pin=%s degraded=%v", pin, degraded)
	}
	pin, degraded = ResolveComposePin("digest", "")
	if pin != "tag" || !degraded {
		t.Fatalf("expected degrade to tag, got pin=%s degraded=%v", pin, degraded)
	}
	pin, degraded = ResolveComposePin("tag", "sha256:abc")
	if pin != "tag" || degraded {
		t.Fatalf("pin=%s degraded=%v", pin, degraded)
	}
}
