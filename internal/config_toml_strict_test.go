package internal

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDecodeConfigFile_UnknownKeyErrors(t *testing.T) {
	withTempConfigDir(t, map[string]string{
		"ship.toml": `
schema = 2

[build]
driver = "docker"

[build.docker]
image = "home"

[deploy.compose]
extra_files = ["deploy/nginx/nginx.conf"]
`,
	}, func() {
		_, err := LoadConfig("")
		if err == nil {
			t.Fatal("LoadConfig should fail when ship.toml contains unknown keys")
		}
		if !strings.Contains(err.Error(), "deploy.compose.extra_files") {
			t.Fatalf("error should mention deploy.compose.extra_files, got: %v", err)
		}
		if !strings.Contains(err.Error(), "未识别的配置项") {
			t.Fatalf("error should mention unknown keys, got: %v", err)
		}
	})
}

func TestDecodeConfigFile_UnknownKeyWarnStillLoads(t *testing.T) {
	withTempConfigDir(t, map[string]string{
		"ship.toml": `
schema = 2

[config]
unknown_keys = "warn"

[build]
driver = "docker"

[build.docker]
image = "home"
dockerfile = "./Dockerfile"
platforms = ["linux/amd64"]

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
extra_files = ["deploy/nginx/nginx.conf"]
`,
		"deploy/compose.yml": "services:\n  app:\n    image: home:latest\n",
	}, func() {
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("LoadConfig(warn) error: %v", err)
		}
		if cfg.ImageName != "home" {
			t.Fatalf("cfg.ImageName = %q, want home", cfg.ImageName)
		}
	})
}

func TestDecodeConfigFile_UnknownKeyIgnoreStillLoads(t *testing.T) {
	withTempConfigDir(t, map[string]string{
		"ship.toml": `
schema = 2

[config]
unknown_keys = "ignore"

[build]
driver = "docker"

[build.docker]
image = "home"
dockerfile = "./Dockerfile"
platforms = ["linux/amd64"]

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

[build.probe]
image = "probe"
`,
		"deploy/compose.yml": "services:\n  app:\n    image: home:latest\n",
	}, func() {
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("LoadConfig(ignore) error: %v", err)
		}
		if cfg.ImageName != "home" {
			t.Fatalf("cfg.ImageName = %q, want home", cfg.ImageName)
		}
	})
}

func TestValidate_UnknownKeysModeRejectsInvalidValue(t *testing.T) {
	cfg := &Config{Schema: 2}
	cfg.Runtime.UnknownKeys = "maybe"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate should reject invalid config.unknown_keys")
	}
	if !strings.Contains(err.Error(), "config.unknown_keys") {
		t.Fatalf("Validate error should mention config.unknown_keys, got: %v", err)
	}
}

func TestCollectUnknownConfigKeys_SortsPaths(t *testing.T) {
	cfg := &Config{}
	meta, err := toml.Decode(`
schema = 2
zebra = true

[build.docker]
image = "home"
alpha = true
`, cfg)
	if err != nil {
		t.Fatalf("toml.Decode error: %v", err)
	}

	keys := collectUnknownConfigKeys(meta)
	if len(keys) != 2 {
		t.Fatalf("collectUnknownConfigKeys() = %#v, want 2 keys", keys)
	}
	if keys[0] != "build.docker.alpha" || keys[1] != "zebra" {
		t.Fatalf("collectUnknownConfigKeys() = %#v, want sorted build.docker.alpha/zebra", keys)
	}
}
