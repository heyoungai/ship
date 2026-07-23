package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteEmbeddedSkills(t *testing.T) {
	dir := t.TempDir()
	written, err := writeEmbeddedSkills(dir)
	if err != nil {
		t.Fatalf("writeEmbeddedSkills: %v", err)
	}

	need := map[string]bool{
		"SKILL.md":     false,
		"REFERENCE.md": false,
		"EXAMPLES.md":  false,
	}
	for _, name := range written {
		if _, ok := need[name]; ok {
			need[name] = true
		}
	}
	for name, ok := range need {
		if !ok {
			t.Fatalf("missing written file %s; got %v", name, written)
		}
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	got, found := ParseSkillDocVersion(data)
	if !found {
		t.Fatal("SKILL.md missing stamped version")
	}
	if got != ExpectedSkillVersion() {
		t.Fatalf("stamped version = %q, want %q", got, ExpectedSkillVersion())
	}
	if !strings.Contains(string(data), "Hard rules") {
		t.Fatal("SKILL.md body looks unexpected")
	}
}

func TestInstallSkillForce(t *testing.T) {
	root := t.TempDir()
	skillForce = true
	t.Cleanup(func() { skillForce = false })

	if err := installSkill(root); err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	// second install with force should overwrite
	if err := installSkill(root); err != nil {
		t.Fatalf("reinstall: %v", err)
	}

	skillPath := filepath.Join(root, skillTargetDir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	v, found := ParseSkillDocVersion(data)
	if !found || v != ExpectedSkillVersion() {
		t.Fatalf("version = %q found=%v", v, found)
	}
	for _, name := range []string{"REFERENCE.md", "EXAMPLES.md"} {
		if _, err := os.Stat(filepath.Join(root, skillTargetDir, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
