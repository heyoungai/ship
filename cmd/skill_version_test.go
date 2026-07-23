package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkillDocVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		want    string
		found   bool
	}{
		{
			name: "ship semver",
			content: `---
name: ship
version: v2.6.1
description: test
---

# Body
`,
			want:  "v2.6.1",
			found: true,
		},
		{
			name: "quoted version",
			content: `---
version: "v2.6.0"
---
`,
			want:  "v2.6.0",
			found: true,
		},
		{
			name: "dev version",
			content: `---
version: dev
---
`,
			want:  "dev",
			found: true,
		},
		{
			name: "missing version",
			content: `---
name: ship
---
`,
			found: false,
		},
		{
			name:    "no frontmatter",
			content: "# Ship\n",
			found:   false,
		},
		{
			name: "empty version",
			content: `---
version: 
---
`,
			found: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, found := ParseSkillDocVersion([]byte(tc.content))
			if found != tc.found {
				t.Fatalf("found = %v, want %v", found, tc.found)
			}
			if found && got != tc.want {
				t.Fatalf("version = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStampSkillVersion(t *testing.T) {
	t.Parallel()

	in := []byte(`---
name: ship
version: 1
description: x
---

# Body
`)
	out, err := stampSkillVersion(in, "v2.6.1")
	if err != nil {
		t.Fatal(err)
	}
	got, found := ParseSkillDocVersion(out)
	if !found || got != "v2.6.1" {
		t.Fatalf("stamped version = %q found=%v, want v2.6.1", got, found)
	}
	if !strings.Contains(string(out), "name: ship") {
		t.Fatal("lost name field")
	}

	noVer := []byte(`---
name: ship
---

# Body
`)
	out2, err := stampSkillVersion(noVer, "dev")
	if err != nil {
		t.Fatal(err)
	}
	got2, found2 := ParseSkillDocVersion(out2)
	if !found2 || got2 != "dev" {
		t.Fatalf("inserted version = %q found=%v, want dev", got2, found2)
	}
}

func TestExpectedSkillVersion(t *testing.T) {
	if ExpectedSkillVersion() != normalizeSkillVersion(Version) {
		t.Fatalf("ExpectedSkillVersion = %q, want %q", ExpectedSkillVersion(), Version)
	}
}

func TestWarnIfInstalledSkillOutdated(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "ship")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	// outdated: no version → should not panic
	old := []byte("---\nname: ship\n---\n# old\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), old, 0644); err != nil {
		t.Fatal(err)
	}
	warnIfInstalledSkillOutdated(root)

	// current version → no panic
	current := []byte("---\nname: ship\nversion: " + ExpectedSkillVersion() + "\n---\n# ok\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), current, 0644); err != nil {
		t.Fatal(err)
	}
	warnIfInstalledSkillOutdated(root)

	// mismatched → no panic
	mismatch := []byte("---\nname: ship\nversion: v0.0.0\n---\n# old\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), mismatch, 0644); err != nil {
		t.Fatal(err)
	}
	warnIfInstalledSkillOutdated(root)

	// missing file → silent
	warnIfInstalledSkillOutdated(t.TempDir())
}

func TestSkillVersionOutdated(t *testing.T) {
	t.Parallel()
	if !skillVersionOutdated("v2.6.0", "v2.6.1") {
		t.Fatal("expected outdated")
	}
	if skillVersionOutdated("v2.6.1", "v2.6.1") {
		t.Fatal("same version should not be outdated")
	}
}
