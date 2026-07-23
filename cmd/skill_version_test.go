package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestParseSkillDocVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		want    int
		found   bool
	}{
		{
			name: "with version",
			content: `---
name: ship
version: 3
description: test
---

# Body
`,
			want:  3,
			found: true,
		},
		{
			name: "quoted version",
			content: `---
version: "2"
---
`,
			want:  2,
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
			name: "invalid version",
			content: `---
version: abc
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
				t.Fatalf("version = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEmbeddedSkillVersion(t *testing.T) {
	v, err := EmbeddedSkillVersion()
	if err != nil {
		t.Fatalf("EmbeddedSkillVersion: %v", err)
	}
	if v < 1 {
		t.Fatalf("embedded skill version = %d, want >= 1", v)
	}
}

func TestWarnIfInstalledSkillOutdated(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "ship")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	expected, err := EmbeddedSkillVersion()
	if err != nil {
		t.Fatal(err)
	}

	// outdated: no version → should not panic
	old := []byte("---\nname: ship\n---\n# old\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), old, 0644); err != nil {
		t.Fatal(err)
	}
	warnIfInstalledSkillOutdated(root)

	// current version → no panic
	current := []byte(fmtFrontmatter(expected))
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), current, 0644); err != nil {
		t.Fatal(err)
	}
	warnIfInstalledSkillOutdated(root)

	// missing file → silent
	warnIfInstalledSkillOutdated(t.TempDir())
}

func fmtFrontmatter(version int) string {
	return "---\nname: ship\nversion: " + strconv.Itoa(version) + "\n---\n# ok\n"
}
