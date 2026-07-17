package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadConfig 从当前工作目录加载配置。
func LoadConfig(imageName string) (*Config, error) {
	return LoadConfigFrom(".", imageName)
}

// LoadConfigFrom 从指定目录加载 ship.toml（两阶段配置的 SourceRoot recipe 使用此入口）。
func LoadConfigFrom(dir, imageName string) (*Config, error) {
	cfg := &Config{ImageName: imageName}
	cfg.applyDefaults()

	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = "."
	}
	tomlPath := filepath.Join(dir, "ship.toml")
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("未找到 %s，请先运行 ship init 生成配置文件，或确认目标 tag 包含 ship.toml", tomlPath)
	}
	if err := decodeConfigFile(tomlPath, cfg); err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", tomlPath, err)
	}

	if v := os.Getenv("PLATFORMS"); v != "" {
		cfg.Build.Docker.Platforms = SplitCSV(v)
	}
	if v := os.Getenv("DOCKERFILE"); v != "" {
		cfg.Build.Docker.Dockerfile = v
	}
	if v := os.Getenv("IMAGE_NAME"); v != "" {
		cfg.Build.Docker.Image = v
		cfg.ImageName = v
	}
	if v := os.Getenv("REMOTE_HOST"); v != "" {
		cfg.Deploy.Compose.Host = v
	}
	if v := os.Getenv("REMOTE_PROJECT_PATH"); v != "" {
		cfg.Deploy.Compose.Path = v
	}
	if v := os.Getenv("ENV_FILE"); v != "" {
		cfg.Build.Docker.EnvFile = v
	}

	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) normalize() {
	c.Deploy.Healthcheck.ApplyDefaults()
	c.normalizeV2()
}

func (c *Config) normalizeV2() {
	switch {
	case c.Build.Go.Main != "" || c.Build.Go.Output != "":
		c.Build.Driver = "go-binary"
	case c.Build.Command.Run != "":
		c.Build.Driver = "command"
	case c.Build.Driver == "":
		c.Build.Driver = "docker"
	}

	if len(c.Build.Docker.Platforms) == 0 && c.Build.Platforms != "" {
		c.Build.Docker.Platforms = SplitCSV(c.Build.Platforms)
	}
	if len(c.Build.Docker.Platforms) == 0 {
		c.Build.Docker.Platforms = []string{"linux/amd64"}
	}
	if c.Build.Docker.Context == "" {
		c.Build.Docker.Context = "."
	}
	if c.Build.Docker.Dockerfile == "" {
		c.Build.Docker.Dockerfile = c.Build.Dockerfile
	}
	if c.Build.Docker.Dockerfile == "" {
		c.Build.Docker.Dockerfile = "./Dockerfile"
	}
	if c.Build.Docker.EnvFile == "" {
		c.Build.Docker.EnvFile = c.Build.EnvFile
	}
	if c.Build.Docker.EnvFile == "" {
		c.Build.Docker.EnvFile = "./.env.local"
	}
	if c.Build.Docker.LocalBuild == "" {
		c.Build.Docker.LocalBuild = c.Build.LocalBuild
	}

	switch {
	case c.Publish.SCP.Local != "" || c.Publish.SCP.Host != "" || c.Publish.SCP.Remote != "":
		c.Publish.Driver = "scp"
	case len(c.Publish.Registry.Targets) > 0:
		c.Publish.Driver = "registry"
	case c.Publish.Driver == "":
		c.Publish.Driver = "none"
	}

	if c.Deploy.Compose.Host == "" {
		c.Deploy.Compose.Host = c.Deploy.Host
	}
	if c.Deploy.Compose.Path == "" {
		c.Deploy.Compose.Path = c.Deploy.Path
	}
	if c.Deploy.Host == "" {
		c.Deploy.Host = c.Deploy.Compose.Host
	}
	if c.Deploy.Path == "" {
		c.Deploy.Path = c.Deploy.Compose.Path
	}
	if c.Deploy.Compose.EnvFile == "" {
		c.Deploy.Compose.EnvFile = ".env"
	}
	if c.Deploy.Compose.TagKey == "" {
		c.Deploy.Compose.TagKey = "APP_IMAGE_TAG"
	}
	if c.Deploy.Compose.Pin == "" {
		c.Deploy.Compose.Pin = "digest"
	}
	if c.Deploy.Compose.DigestKey == "" {
		c.Deploy.Compose.DigestKey = "APP_IMAGE_DIGEST"
	}
	if c.Deploy.Compose.Up == "" {
		c.Deploy.Compose.Up = "docker compose up -d"
	}

	switch {
	case c.Deploy.BinaryInstall.Host != "" || c.Deploy.BinaryInstall.RemoteInstallPath != "":
		c.Deploy.Driver = "binary-install"
	case c.Deploy.SSH.Host != "" || len(c.Deploy.SSH.Commands) > 0:
		c.Deploy.Driver = "ssh"
	case c.Deploy.Compose.Host != "" || c.Deploy.Compose.Path != "" || c.Deploy.Enabled:
		c.Deploy.Driver = "compose"
	case c.Deploy.Driver == "":
		c.Deploy.Driver = "none"
	}

	switch {
	case c.Verify.Command.Run != "":
		c.Verify.Driver = "command"
	case c.Verify.SSH.Host != "" || c.Verify.SSH.Command != "":
		c.Verify.Driver = "ssh"
	case c.Verify.HTTP.URL != "":
		c.Verify.Driver = "http"
	case c.Verify.Driver == "":
		c.Verify.Driver = "none"
	}

	if c.Build.Driver == "docker" {
		c.Build.Platforms = strings.Join(c.Build.Docker.Platforms, ",")
		c.Build.Dockerfile = c.Build.Docker.Dockerfile
		c.Build.EnvFile = c.Build.Docker.EnvFile
		c.Build.LocalBuild = c.Build.Docker.LocalBuild
		if c.Build.Docker.Image != "" {
			c.ImageName = c.Build.Docker.Image
		}
	}

	if c.ImageName == "" && len(c.Publish.Registry.Targets) > 0 {
		c.ImageName = c.Publish.Registry.Targets[0].Image
	}
	if c.Build.Driver == "docker" && c.Build.Docker.Image == "" {
		c.Build.Docker.Image = c.ImageName
	}
	if c.Build.Driver == "go-binary" && c.Build.Go.ExecutableName == "" && c.Build.Go.Output != "" {
		c.Build.Go.ExecutableName = filepath.Base(c.Build.Go.Output)
	}

	if c.Publish.Driver == "scp" && c.Publish.SCP.Local == "" && c.Build.Driver == "go-binary" {
		c.Publish.SCP.Local = c.Build.Go.Output
	}
	if c.Publish.Driver == "scp" && c.Publish.SCP.Host == "" && c.Deploy.BinaryInstall.Host != "" {
		c.Publish.SCP.Host = c.Deploy.BinaryInstall.Host
	}
	if c.Deploy.Driver == "binary-install" && c.Deploy.BinaryInstall.Host == "" && c.Publish.SCP.Host != "" {
		c.Deploy.BinaryInstall.Host = c.Publish.SCP.Host
	}
	if c.Deploy.Driver == "binary-install" && c.Deploy.BinaryInstall.RemoteTempPath == "" && c.Publish.SCP.Remote != "" {
		c.Deploy.BinaryInstall.RemoteTempPath = c.Publish.SCP.Remote
	}
	if c.Deploy.Driver == "binary-install" && c.Deploy.BinaryInstall.Chmod == "" {
		c.Deploy.BinaryInstall.Chmod = "+x"
	}

	c.Registries = append([]Registry(nil), c.Publish.Registry.Targets...)
	c.Deploy.Enabled = c.Features.Deploy && c.Deploy.Driver != "none"
}
