package cmd

import (
	"strings"
	"testing"
)

// TestGenerateConfigOutputsV2Schema 验证 init 生成的配置与当前 v2 解析器契合。
func TestGenerateConfigOutputsV2Schema(t *testing.T) {
	config := generateConfig(map[string]string{
		"镜像名称": "demo-app (从目录名推断)",
		"环境文件": ".env",
		"本地构建": "pnpm run build",
	})

	checks := []string{
		"schema = 2",
		"[project]",
		"name = \"demo-app\"",
		"[build]",
		"driver = \"docker\"",
		"[build.docker]",
		"image = \"demo-app\"",
		"env_file = \"./.env\"",
		"[publish]",
		"[[publish.registry.targets]]",
		"[deploy]",
		"driver = \"none\"",
		"[verify]",
		"local_build = \"pnpm run build\"",
	}

	for _, want := range checks {
		if !strings.Contains(config, want) {
			t.Fatalf("generateConfig() missing %q\nconfig:\n%s", want, config)
		}
	}

	legacyFields := []string{
		"[[registries]]",
		"enabled = false",
	}

	for _, legacy := range legacyFields {
		if strings.Contains(config, legacy) {
			t.Fatalf("generateConfig() should not contain legacy field %q\nconfig:\n%s", legacy, config)
		}
	}
}
