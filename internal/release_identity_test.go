package internal

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type stubGitLookup struct {
	tagCommits map[string]string
	head       string
	ahead      int
	aheadErr   error
	dirty      bool
	dirtyErr   error
	headErr    error
}

func (s stubGitLookup) ResolveTagCommit(tag string) (string, error) {
	if s.tagCommits == nil {
		return "", errors.New("tag missing")
	}
	commit, ok := s.tagCommits[tag]
	if !ok {
		return "", errors.New("tag missing: " + tag)
	}
	return commit, nil
}

func (s stubGitLookup) HeadCommit() (string, error) {
	if s.headErr != nil {
		return "", s.headErr
	}
	return s.head, nil
}

func (s stubGitLookup) CommitsAhead(string) (int, error) {
	if s.aheadErr != nil {
		return -1, s.aheadErr
	}
	return s.ahead, nil
}

func (s stubGitLookup) IsDirty() (bool, error) {
	if s.dirtyErr != nil {
		return false, s.dirtyErr
	}
	return s.dirty, nil
}

func TestResolveReleaseIdentity_GitTagLocksCommit(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Version.Source = SourceModeGitTag

	git := stubGitLookup{
		tagCommits: map[string]string{"v1.2.3": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		head:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ahead:      12,
		dirty:      true,
	}

	got, err := resolveReleaseIdentityWithLookup(cfg, "v1.2.3", func(string) string {
		return ""
	}, func() (string, error) {
		return "v0.0.1", nil
	}, git)
	if err != nil {
		t.Fatalf("resolveReleaseIdentityWithLookup error: %v", err)
	}
	if got.Version != "v1.2.3" {
		t.Fatalf("Version = %q, want v1.2.3", got.Version)
	}
	if got.SourceMode != SourceModeGitTag {
		t.Fatalf("SourceMode = %q, want %q", got.SourceMode, SourceModeGitTag)
	}
	if got.SourceRef != "refs/tags/v1.2.3" {
		t.Fatalf("SourceRef = %q, want refs/tags/v1.2.3", got.SourceRef)
	}
	if got.SourceCommit != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("SourceCommit = %q", got.SourceCommit)
	}
	if got.AheadBy != 12 {
		t.Fatalf("AheadBy = %d, want 12", got.AheadBy)
	}
	if !got.Dirty {
		t.Fatal("Dirty should be true")
	}
}

func TestResolveReleaseIdentity_MissingTagFails(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Version.Source = SourceModeGitTag
	cfg.Version.Fallback = "error"

	_, err := resolveReleaseIdentityWithLookup(cfg, "v9.9.9", func(string) string {
		return ""
	}, func() (string, error) {
		return "v1.0.0", nil
	}, stubGitLookup{tagCommits: map[string]string{}})
	if err == nil {
		t.Fatal("expected missing tag error")
	}
	if !strings.Contains(err.Error(), "v9.9.9") {
		t.Fatalf("error should mention version, got: %v", err)
	}
}

func TestResolveReleaseIdentity_DescribeThenLock(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	got, err := resolveReleaseIdentityWithLookup(cfg, "", func(string) string {
		return ""
	}, func() (string, error) {
		return "v2.0.0", nil
	}, stubGitLookup{
		tagCommits: map[string]string{"v2.0.0": "cccccccccccccccccccccccccccccccccccccccc"},
		head:       "cccccccccccccccccccccccccccccccccccccccc",
		ahead:      0,
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.Version != "v2.0.0" || got.SourceCommit != "cccccccccccccccccccccccccccccccccccccccc" {
		t.Fatalf("unexpected identity: %+v", got)
	}
}

func TestResolveReleaseIdentity_OverrideEnvMustBeRealTag(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	_, err := resolveReleaseIdentityWithLookup(cfg, "", func(key string) string {
		if key == "SHIP_VERSION" {
			return "not-a-tag"
		}
		return ""
	}, func() (string, error) {
		return "v1.0.0", nil
	}, stubGitLookup{tagCommits: map[string]string{"v1.0.0": "aaa"}})
	if err == nil {
		t.Fatal("expected override env tag validation failure")
	}
}

func TestResolveReleaseIdentity_FallbackDevUsesCurrent(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Version.Fallback = "dev"

	got, err := resolveReleaseIdentityWithLookup(cfg, "", func(string) string {
		return ""
	}, func() (string, error) {
		return "", errors.New("no tags")
	}, stubGitLookup{head: "dddddddddddddddddddddddddddddddddddddddd"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.SourceMode != SourceModeCurrent {
		t.Fatalf("SourceMode = %q, want current", got.SourceMode)
	}
	if got.Version != "dev" {
		t.Fatalf("Version = %q, want dev", got.Version)
	}
	if got.SourceCommit != "dddddddddddddddddddddddddddddddddddddddd" {
		t.Fatalf("SourceCommit = %q", got.SourceCommit)
	}
	if got.NeedsSourceSnapshot() {
		t.Fatal("current mode should not need source snapshot")
	}
}

func TestResolveReleaseIdentity_StaticSource(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Version.Source = SourceModeStatic
	cfg.Version.Static = "v5.0.0"

	got, err := resolveReleaseIdentityWithLookup(cfg, "", func(string) string {
		return ""
	}, func() (string, error) {
		return "", errors.New("should not call")
	}, stubGitLookup{head: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.SourceMode != SourceModeStatic {
		t.Fatalf("SourceMode = %q", got.SourceMode)
	}
	if got.NeedsSourceSnapshot() {
		t.Fatal("static mode should not need snapshot")
	}
}

func TestTagRef(t *testing.T) {
	if got := tagRef("v1.0.0"); got != "refs/tags/v1.0.0" {
		t.Fatalf("tagRef = %q", got)
	}
	if got := tagRef("refs/tags/v1.0.0"); got != "refs/tags/v1.0.0" {
		t.Fatalf("tagRef idempotent = %q", got)
	}
}

func TestResolvePathAgainstRoot(t *testing.T) {
	root := t.TempDir()
	got, err := ResolvePathAgainstRoot(root, ".env.local")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".env.local")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
