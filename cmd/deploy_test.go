package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestComposeRemotePath 验证远端相对路径会落到 deploy.path 下，绝对路径保持不变。
func TestComposeRemotePath(t *testing.T) {
	if got := composeRemotePath("/srv/app", ".env"); got != "/srv/app/.env" {
		t.Fatalf("composeRemotePath(relative) = %q, want /srv/app/.env", got)
	}
	if got := composeRemotePath("/srv/app", "/etc/myapp/.env"); got != "/etc/myapp/.env" {
		t.Fatalf("composeRemotePath(absolute) = %q, want /etc/myapp/.env", got)
	}
}

// TestComposeRemoteFilePath 验证未显式配置 remote_file 时会继承本地文件名。
func TestComposeRemoteFilePath(t *testing.T) {
	got := composeRemoteFilePath("/srv/app", "", filepath.Join("deploy", "docker-compose.prod.yml"))
	if got != "/srv/app/docker-compose.prod.yml" {
		t.Fatalf("composeRemoteFilePath(default) = %q, want /srv/app/docker-compose.prod.yml", got)
	}
	got = composeRemoteFilePath("/srv/app", "compose.yaml", filepath.Join("deploy", "docker-compose.prod.yml"))
	if got != "/srv/app/compose.yaml" {
		t.Fatalf("composeRemoteFilePath(explicit) = %q, want /srv/app/compose.yaml", got)
	}
}

// TestValidateComposeLocalSource 验证 compose 上传源文件缺失或为目录时会明确报错。
func TestValidateComposeLocalSource(t *testing.T) {
	tempDir := t.TempDir()
	missingPath := filepath.Join(tempDir, "missing.yml")
	if err := validateComposeLocalSource(missingPath, "deploy.compose.local_file"); err == nil || !strings.Contains(err.Error(), "本地文件不存在") {
		t.Fatalf("validateComposeLocalSource(missing) error = %v", err)
	}

	dirPath := filepath.Join(tempDir, "dir")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatalf("Mkdir error: %v", err)
	}
	if err := validateComposeLocalSource(dirPath, "deploy.compose.local_file"); err == nil || !strings.Contains(err.Error(), "当前是目录") {
		t.Fatalf("validateComposeLocalSource(dir) error = %v", err)
	}

	filePath := filepath.Join(tempDir, "compose.yml")
	if err := os.WriteFile(filePath, []byte("services:\n  app:\n    image: demo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if err := validateComposeLocalSource(filePath, "deploy.compose.local_file"); err != nil {
		t.Fatalf("validateComposeLocalSource(file) error: %v", err)
	}
}

// TestUniqueRemoteDirs 验证远端目录集合会去重，避免重复 mkdir -p。
func TestUniqueRemoteDirs(t *testing.T) {
	dirs := uniqueRemoteDirs("/srv/app", "/srv/app/.env", "/srv/app/compose.yaml")
	if len(dirs) != 1 || dirs[0] != "/srv/app" {
		t.Fatalf("uniqueRemoteDirs = %v, want [/srv/app]", dirs)
	}

	dirs = uniqueRemoteDirs("/srv/app", "/srv/app/envs/prod.env", "/srv/app/compose/compose.yaml")
	if len(dirs) != 3 {
		t.Fatalf("uniqueRemoteDirs nested = %v, want 3 dirs", dirs)
	}
}
