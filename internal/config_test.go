package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withTempConfigDir(t *testing.T, files map[string]string, fn func()) {
	t.Helper()
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%q) error: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	for name, content := range files {
		fullPath := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("MkdirAll(%q) error: %v", fullPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile(%q) error: %v", fullPath, err)
		}
	}

	fn()
}

// ── ImageTag ────────────────────────────────────────────────────

func TestImageTag_DefaultProfile(t *testing.T) {
	p := Profile{Name: "", Default: true}
	got := ImageTag("v2.0.0", p)
	if got != "v2.0.0" {
		t.Errorf("ImageTag(default) = %q, want %q", got, "v2.0.0")
	}
}

func TestImageTag_NamedProfile(t *testing.T) {
	p := Profile{Name: "brand-a", Default: true}
	got := ImageTag("v2.0.0", p)
	if got != "v2.0.0-brand-a" {
		t.Errorf("ImageTag(brand-a) = %q, want %q", got, "v2.0.0-brand-a")
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
	if got != "" {
		t.Errorf("FormatProfileName(default) = %q, want %q", got, "")
	}
}

func TestFormatProfileName_Named(t *testing.T) {
	got := FormatProfileName(Profile{Name: "brand-a"})
	if got != "brand-a" {
		t.Errorf("FormatProfileName(brand-a) = %q, want %q", got, "brand-a")
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
	profiles, err := cfg.GetProfiles("")
	if err != nil {
		t.Fatalf("GetProfiles(no matrix) error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "" || !profiles[0].Default {
		t.Errorf("GetProfiles(no matrix) = %v, want single default", profiles)
	}
}

func TestGetProfiles_NoMatrixRejectsNamedProfile(t *testing.T) {
	cfg := &Config{}
	_, err := cfg.GetProfiles("brand-a")
	if err == nil {
		t.Fatal("GetProfiles should reject named profile when matrix is not configured")
	}
}

func TestGetProfiles_WithMatrix(t *testing.T) {
	cfg := &Config{
		Matrix: []Profile{
			{Name: "a", Default: true},
			{Name: "b"},
		},
	}
	profiles, err := cfg.GetProfiles("")
	if err != nil {
		t.Fatalf("GetProfiles(all) error: %v", err)
	}
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
	profiles, err := cfg.GetProfiles("b")
	if err != nil {
		t.Fatalf("GetProfiles(b) error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "b" {
		t.Errorf("GetProfiles(b) = %v, want [b]", profiles)
	}
}

func TestGetProfiles_FilterByMissingName(t *testing.T) {
	cfg := &Config{
		Matrix: []Profile{
			{Name: "a", Default: true},
		},
	}
	_, err := cfg.GetProfiles("missing")
	if err == nil {
		t.Fatal("GetProfiles should reject unknown profile")
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

// ── Validate ─────────────────────────────────────────────────────

func TestValidate_MissingFields(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Schema = 2
	cfg.Features.Deploy = false
	cfg.Features.Verify = false
	cfg.normalize()
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate should fail when required fields are missing")
	}
	if !strings.Contains(err.Error(), "build.docker.image") || !strings.Contains(err.Error(), "publish.registry.targets") {
		t.Fatalf("Validate error should mention missing fields, got: %v", err)
	}
}

func TestValidate_MultipleDefaultProfiles(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Schema = 2
	cfg.Build.Docker.Image = "home"
	cfg.Publish.Registry.Targets = []Registry{
		{Type: "dockerhub", Namespace: "deali", Image: "home"},
	}
	cfg.Features.Deploy = false
	cfg.Features.Verify = false
	cfg.Matrix = []Profile{
		{Name: "a", Default: true},
		{Name: "b", Default: true},
	}
	cfg.normalize()
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate should reject multiple default profiles")
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Schema = 2
	cfg.Build.Docker.Image = "home"
	cfg.Publish.Registry.Targets = []Registry{
		{Type: "dockerhub", Namespace: "deali", Image: "home"},
	}
	cfg.Features.Deploy = false
	cfg.Features.Verify = false
	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

// ── BuildxOutputArgs / ShellEscape ───────────────────────────────

func TestBuildxOutputArgs_SinglePlatform(t *testing.T) {
	args, err := BuildxOutputArgs("linux/amd64")
	if err != nil {
		t.Fatalf("BuildxOutputArgs(single) error: %v", err)
	}
	if len(args) != 1 || args[0] != "--load" {
		t.Fatalf("BuildxOutputArgs(single) = %v, want [--load]", args)
	}
}

func TestBuildxOutputArgs_MultiPlatform(t *testing.T) {
	_, err := BuildxOutputArgs("linux/amd64,linux/arm64")
	if err == nil {
		t.Fatal("BuildxOutputArgs should reject multi-platform staged build")
	}
}

func TestShellEscape(t *testing.T) {
	got := ShellEscape("a'b")
	want := `'a'"'"'b'`
	if got != want {
		t.Fatalf("ShellEscape = %q, want %q", got, want)
	}
}

// ── V2 Config ─────────────────────────────────────────────────────

func TestLoadConfig_V2DockerRegistryComposeCompatibility(t *testing.T) {
	withTempConfigDir(t, map[string]string{
		"ship.toml": `
schema = 2

[build]
driver = "docker"

[build.docker]
image = "home"
dockerfile = "./Dockerfile"
platforms = ["linux/amd64"]
env_file = "./.env.local"
local_build = "bun run build"

[publish]
driver = "registry"

[[publish.registry.targets]]
type = "private"
url = "registry.cn-hangzhou.aliyuncs.com"
namespace = "deali"
image = "home"

[deploy]
driver = "compose"

[deploy.compose]
host = "deali.cn"
path = "/home/deali/projects/home"
`,
	}, func() {
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("LoadConfig(v2 docker) error: %v", err)
		}
		if cfg.Schema != 2 {
			t.Fatalf("cfg.Schema = %d, want 2", cfg.Schema)
		}
		if cfg.Build.Driver != "docker" {
			t.Fatalf("cfg.Build.Driver = %q, want docker", cfg.Build.Driver)
		}
		if cfg.Build.Platforms != "linux/amd64" {
			t.Fatalf("cfg.Build.Platforms = %q, want linux/amd64", cfg.Build.Platforms)
		}
		if cfg.Build.Dockerfile != "./Dockerfile" {
			t.Fatalf("cfg.Build.Dockerfile = %q", cfg.Build.Dockerfile)
		}
		if cfg.Build.EnvFile != "./.env.local" {
			t.Fatalf("cfg.Build.EnvFile = %q", cfg.Build.EnvFile)
		}
		if cfg.ImageName != "home" {
			t.Fatalf("cfg.ImageName = %q, want home", cfg.ImageName)
		}
		if len(cfg.Registries) != 1 {
			t.Fatalf("len(cfg.Registries) = %d, want 1", len(cfg.Registries))
		}
		if !cfg.Deploy.Enabled {
			t.Fatal("cfg.Deploy.Enabled should be true for v2 compose config")
		}
		if cfg.Deploy.Host != "deali.cn" || cfg.Deploy.Path != "/home/deali/projects/home" {
			t.Fatalf("legacy deploy mapping mismatch: host=%q path=%q", cfg.Deploy.Host, cfg.Deploy.Path)
		}
		if cfg.Deploy.Compose.TagKey != "APP_IMAGE_TAG" {
			t.Fatalf("cfg.Deploy.Compose.TagKey = %q, want APP_IMAGE_TAG", cfg.Deploy.Compose.TagKey)
		}
	})
}

func TestLoadConfig_V2DriverInference(t *testing.T) {
	withTempConfigDir(t, map[string]string{
		"ship.toml": `
schema = 2

[build.go]
main = "./cmd/swag-cli"
output = "./build/swag-cli"

[publish.scp]
local = "./build/swag-cli"
host = "deali.cn"
remote = "/tmp/swag-cli"

[deploy.binary_install]
host = "deali.cn"
remote_install_path = "/usr/local/bin"
`,
	}, func() {
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("LoadConfig(v2 inference) error: %v", err)
		}
		if cfg.Build.Driver != "go-binary" {
			t.Fatalf("cfg.Build.Driver = %q, want go-binary", cfg.Build.Driver)
		}
		if cfg.Publish.Driver != "scp" {
			t.Fatalf("cfg.Publish.Driver = %q, want scp", cfg.Publish.Driver)
		}
		if cfg.Deploy.Driver != "binary-install" {
			t.Fatalf("cfg.Deploy.Driver = %q, want binary-install", cfg.Deploy.Driver)
		}
		if cfg.Publish.SCP.Local != "./build/swag-cli" {
			t.Fatalf("cfg.Publish.SCP.Local = %q, want ./build/swag-cli", cfg.Publish.SCP.Local)
		}
		if cfg.Deploy.BinaryInstall.RemoteTempPath != "/tmp/swag-cli" {
			t.Fatalf("cfg.Deploy.BinaryInstall.RemoteTempPath = %q, want /tmp/swag-cli", cfg.Deploy.BinaryInstall.RemoteTempPath)
		}
		if cfg.Deploy.BinaryInstall.Chmod != "+x" {
			t.Fatalf("cfg.Deploy.BinaryInstall.Chmod = %q, want +x", cfg.Deploy.BinaryInstall.Chmod)
		}
		if cfg.UsesTagStage() {
			t.Fatal("cfg.UsesTagStage() should be false for go-binary/scp")
		}
		if !cfg.UsesPublishStage() || !cfg.UsesDeployStage() {
			t.Fatal("go-binary/scp/binary-install should enable publish and deploy stages")
		}
	})
}

func TestValidate_V2GoBinarySCPBinaryInstallSuccess(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Schema = 2
	cfg.Build.Driver = "go-binary"
	cfg.Build.Go.Main = "./cmd/swag-cli"
	cfg.Build.Go.Output = "./build/swag-cli"
	cfg.Publish.Driver = "scp"
	cfg.Publish.SCP.Local = "./build/swag-cli"
	cfg.Publish.SCP.Host = "deali.cn"
	cfg.Publish.SCP.Remote = "/tmp/swag-cli"
	cfg.Deploy.Driver = "binary-install"
	cfg.Deploy.BinaryInstall.Host = "deali.cn"
	cfg.Deploy.BinaryInstall.RemoteInstallPath = "/usr/local/bin"
	cfg.Verify.Driver = "none"
	cfg.normalize()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(v2 go-binary) error: %v", err)
	}
}

func TestValidate_V2MissingPublishTargets(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Schema = 2
	cfg.Build.Driver = "docker"
	cfg.Build.Docker.Image = "home"
	cfg.Publish.Driver = "registry"
	cfg.Features.Deploy = false
	cfg.Features.Verify = false
	cfg.normalize()

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate should fail when v2 registry publish has no targets")
	}
	if !strings.Contains(err.Error(), "publish.registry.targets") {
		t.Fatalf("Validate error should mention publish.registry.targets, got: %v", err)
	}
}
