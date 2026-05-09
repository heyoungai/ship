package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// ── ImageTag ────────────────────────────────────────────────────

func TestImageTag_DefaultProfile(t *testing.T) {
	p := Profile{Name: "", Default: true}
	got := ImageTag("v2.0.0", p)
	if got != "v2.0.0" {
		t.Errorf("ImageTag(default) = %q, want %q", got, "v2.0.0")
	}
}

func TestImageTag_NamedProfile(t *testing.T) {
	p := Profile{Name: "linglu", Default: true}
	got := ImageTag("v2.0.0", p)
	if got != "v2.0.0-linglu" {
		t.Errorf("ImageTag(linglu) = %q, want %q", got, "v2.0.0-linglu")
	}
}

// ── RegistryTargets ─────────────────────────────────────────────

func TestRegistryTargets_Private(t *testing.T) {
	cfg := &Config{
		ImageName: "home",
		Registries: []Registry{
			{Type: "private", URL: "registry.cn-hangzhou.aliyuncs.com", Namespace: "deali", Image: "home"},
		},
	}
	targets := cfg.RegistryTargets("v1.0.0")
	want := "registry.cn-hangzhou.aliyuncs.com/deali/home:v1.0.0"
	if len(targets) != 1 || targets[0] != want {
		t.Errorf("RegistryTargets(private) = %v, want [%s]", targets, want)
	}
}

func TestRegistryTargets_Dockerhub(t *testing.T) {
	cfg := &Config{
		ImageName: "home",
		Registries: []Registry{
			{Type: "dockerhub", Namespace: "deali", Image: "home"},
		},
	}
	targets := cfg.RegistryTargets("latest")
	want := "deali/home:latest"
	if len(targets) != 1 || targets[0] != want {
		t.Errorf("RegistryTargets(dockerhub) = %v, want [%s]", targets, want)
	}
}

func TestRegistryTargets_Multiple(t *testing.T) {
	cfg := &Config{
		ImageName: "app",
		Registries: []Registry{
			{Type: "dockerhub", Namespace: "user", Image: "app"},
			{Type: "private", URL: "reg.example.com", Namespace: "ns", Image: "app"},
		},
	}
	targets := cfg.RegistryTargets("v1.0.0")
	if len(targets) != 2 {
		t.Fatalf("RegistryTargets(multiple) got %d targets, want 2", len(targets))
	}
	if targets[0] != "user/app:v1.0.0" {
		t.Errorf("target[0] = %q, want %q", targets[0], "user/app:v1.0.0")
	}
	if targets[1] != "reg.example.com/ns/app:v1.0.0" {
		t.Errorf("target[1] = %q, want %q", targets[1], "reg.example.com/ns/app:v1.0.0")
	}
}

// ── ImageRef ────────────────────────────────────────────────────

func TestImageRef(t *testing.T) {
	cfg := &Config{ImageName: "home"}
	got := cfg.ImageRef("latest")
	if got != "home:latest" {
		t.Errorf("ImageRef = %q, want %q", got, "home:latest")
	}
}

// ── MergeEnv ────────────────────────────────────────────────────

func TestMergeEnv_Merge(t *testing.T) {
	base := map[string]string{"A": "1", "B": "2"}
	override := map[string]string{"B": "99", "C": "3"}
	got := MergeEnv(base, override)

	if got["A"] != "1" || got["C"] != "3" {
		t.Errorf("MergeEnv missing keys: %v", got)
	}
	if got["B"] != "99" {
		t.Errorf("MergeEnv should override B, got %q", got["B"])
	}
}

func TestMergeEnv_Empty(t *testing.T) {
	got := MergeEnv(nil, nil)
	if len(got) != 0 {
		t.Errorf("MergeEnv(nil, nil) = %v, want empty", got)
	}
}

// ── EnvToSlice ──────────────────────────────────────────────────

func TestEnvToSlice(t *testing.T) {
	m := map[string]string{"FOO": "bar", "BAZ": "qux"}
	s := EnvToSlice(m)
	if len(s) != 2 {
		t.Fatalf("EnvToSlice got %d items, want 2", len(s))
	}
	// 顺序不确定，检查包含即可
	has := func(target string) bool {
		for _, v := range s {
			if v == target {
				return true
			}
		}
		return false
	}
	if !has("FOO=bar") || !has("BAZ=qux") {
		t.Errorf("EnvToSlice = %v, want [FOO=bar BAZ=qux]", s)
	}
}

// ── FormatProfileName ───────────────────────────────────────────

func TestFormatProfileName_Default(t *testing.T) {
	got := FormatProfileName(Profile{Name: "", Default: true})
	if got != "(default)" {
		t.Errorf("FormatProfileName(default) = %q, want %q", got, "(default)")
	}
}

func TestFormatProfileName_Named(t *testing.T) {
	got := FormatProfileName(Profile{Name: "linglu"})
	if got != "linglu" {
		t.Errorf("FormatProfileName(linglu) = %q, want %q", got, "linglu")
	}
}

// ── StringSliceContains ─────────────────────────────────────────

func TestStringSliceContains_Found(t *testing.T) {
	if !StringSliceContains([]string{"a", "b", "c"}, "b") {
		t.Error("StringSliceContains should find 'b'")
	}
}

func TestStringSliceContains_CaseInsensitive(t *testing.T) {
	if !StringSliceContains([]string{"Foo", "Bar"}, "foo") {
		t.Error("StringSliceContains should be case-insensitive")
	}
}

func TestStringSliceContains_NotFound(t *testing.T) {
	if StringSliceContains([]string{"a", "b"}, "z") {
		t.Error("StringSliceContains should not find 'z'")
	}
}

// ── GetProfiles ─────────────────────────────────────────────────

func TestGetProfiles_NoMatrix(t *testing.T) {
	cfg := &Config{}
	profiles := cfg.GetProfiles("")
	if len(profiles) != 1 || profiles[0].Name != "" || !profiles[0].Default {
		t.Errorf("GetProfiles(no matrix) = %v, want single default", profiles)
	}
}

func TestGetProfiles_WithMatrix(t *testing.T) {
	cfg := &Config{
		Matrix: []Profile{
			{Name: "a", Default: true},
			{Name: "b"},
		},
	}
	profiles := cfg.GetProfiles("")
	if len(profiles) != 2 {
		t.Errorf("GetProfiles(all) got %d, want 2", len(profiles))
	}
}

func TestGetProfiles_FilterByName(t *testing.T) {
	cfg := &Config{
		Matrix: []Profile{
			{Name: "a", Default: true},
			{Name: "b"},
		},
	}
	profiles := cfg.GetProfiles("b")
	if len(profiles) != 1 || profiles[0].Name != "b" {
		t.Errorf("GetProfiles(b) = %v, want [b]", profiles)
	}
}

// ── DefaultProfile ──────────────────────────────────────────────

func TestDefaultProfile_NoMatrix(t *testing.T) {
	cfg := &Config{}
	p := cfg.DefaultProfile()
	if p.Name != "" || !p.Default {
		t.Errorf("DefaultProfile(no matrix) = %+v", p)
	}
}

func TestDefaultProfile_WithMarked(t *testing.T) {
	cfg := &Config{
		Matrix: []Profile{
			{Name: "a"},
			{Name: "b", Default: true},
		},
	}
	p := cfg.DefaultProfile()
	if p.Name != "b" {
		t.Errorf("DefaultProfile = %q, want %q", p.Name, "b")
	}
}

// ── LoadBuildArgs ───────────────────────────────────────────────

func TestLoadBuildArgs_EmptyPath(t *testing.T) {
	args := LoadBuildArgs("")
	if args != nil {
		t.Errorf("LoadBuildArgs('') = %v, want nil", args)
	}
}

func TestLoadBuildArgs_NoFile(t *testing.T) {
	args := LoadBuildArgs("/nonexistent/.env")
	if args != nil {
		t.Errorf("LoadBuildArgs(missing) = %v, want nil", args)
	}
}

func TestLoadBuildArgs_ValidFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("FOO=bar\nBAZ=qux\n"), 0644)

	args := LoadBuildArgs(envFile)
	if len(args) != 4 {
		t.Fatalf("LoadBuildArgs got %d args, want 4", len(args))
	}
	// 结果顺序不确定，检查包含即可
	hasFoo := false
	hasBaz := false
	for i := 0; i < len(args)-1; i += 2 {
		if args[i] == "--build-arg" {
			switch args[i+1] {
			case "FOO=bar":
				hasFoo = true
			case "BAZ=qux":
				hasBaz = true
			}
		}
	}
	if !hasFoo || !hasBaz {
		t.Errorf("LoadBuildArgs missing expected args: %v", args)
	}
}

// ── validate ────────────────────────────────────────────────────
// 注意：validate() 调用 os.Exit(1)，无法直接测试。
// 通过 LoadConfig 集成测试间接覆盖（需要 ship.toml 文件）。
// 这里测试不涉及 os.Exit 的纯逻辑。
