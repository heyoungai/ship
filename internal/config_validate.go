package internal

import (
	"errors"
	"fmt"
	"strings"
)

// Validate 校验必填字段，一次性返回所有缺失项。
func (c *Config) Validate() error {
	var missing []string

	defaultProfiles := 0
	for _, profile := range c.Matrix {
		if profile.Default {
			defaultProfiles++
		}
	}
	if defaultProfiles > 1 {
		missing = append(missing, "matrix.default 只能有一个 profile 为 true")
	}

	if c.Schema != 2 {
		missing = append(missing, "schema = 2")
	} else {
		c.validateV2(&missing)
	}

	if len(missing) > 0 {
		var b strings.Builder
		b.WriteString("配置缺失或无效：")
		for _, item := range missing {
			b.WriteString("\n- ")
			b.WriteString(item)
		}
		b.WriteString("\n参考 config.example.toml 创建或修正 ship.toml")
		return errors.New(b.String())
	}
	return nil
}

func (c *Config) validateV2(missing *[]string) {
	if c.Version.Source != "" && !StringSliceContains([]string{"git-tag", "env", "static"}, c.Version.Source) {
		*missing = append(*missing, "version.source 仅支持 git-tag | env | static")
	}
	if c.Version.Fallback != "" && !StringSliceContains([]string{"error", "dev", "static"}, c.Version.Fallback) {
		*missing = append(*missing, "version.fallback 仅支持 error | dev | static")
	}
	if c.Version.Fallback == "static" && c.Version.Static == "" {
		*missing = append(*missing, "version.static (当 version.fallback = static 时必填)")
	}

	switch c.Build.Driver {
	case "docker":
		if c.Build.Docker.Image == "" && c.ImageName == "" {
			*missing = append(*missing, "build.docker.image (或 publish.registry.targets[].image)")
		}
		if c.Build.Docker.Dockerfile == "" {
			*missing = append(*missing, "build.docker.dockerfile")
		}
		if len(c.Build.Docker.Platforms) == 0 {
			*missing = append(*missing, "build.docker.platforms")
		}
		if !c.Build.Docker.Load {
			*missing = append(*missing, "build.docker.load 当前分阶段流程中必须为 true")
		}
		if c.Build.Docker.DisableBuildkit {
			*missing = append(*missing, "build.docker.disable_buildkit 当前 build.driver = docker 时暂不支持")
		}
	case "go-binary":
		if c.Build.Go.Main == "" {
			*missing = append(*missing, "build.go.main")
		}
		if c.Build.Go.Output == "" {
			*missing = append(*missing, "build.go.output")
		}
	case "command":
		if c.Build.Command.Run == "" {
			*missing = append(*missing, "build.command.run")
		}
	default:
		*missing = append(*missing, "build.driver 仅支持 docker | go-binary | command")
	}

	if c.Features.Publish {
		switch c.Publish.Driver {
		case "registry":
			if len(c.Publish.Registry.Targets) == 0 {
				*missing = append(*missing, "publish.registry.targets")
			} else {
				c.validateRegistryTargets(missing)
			}
		case "scp":
			if c.Publish.SCP.Local == "" {
				*missing = append(*missing, "publish.scp.local")
			}
			if c.Publish.SCP.Host == "" {
				*missing = append(*missing, "publish.scp.host")
			}
			if c.Publish.SCP.Remote == "" {
				*missing = append(*missing, "publish.scp.remote")
			}
		case "none":
		default:
			*missing = append(*missing, "publish.driver 仅支持 registry | scp | none")
		}
	}

	if c.Features.Deploy {
		switch c.Deploy.Driver {
		case "compose":
			if strings.TrimSpace(c.Deploy.Compose.Host) == "" {
				*missing = append(*missing, "deploy.compose.host")
			}
			if strings.TrimSpace(c.Deploy.Compose.Path) == "" {
				*missing = append(*missing, "deploy.compose.path")
			}
			if strings.TrimSpace(c.Deploy.Compose.EnvFile) == "" {
				*missing = append(*missing, "deploy.compose.env_file")
			}
			if strings.TrimSpace(c.Deploy.Compose.TagKey) == "" {
				*missing = append(*missing, "deploy.compose.tag_key")
			}
			if strings.TrimSpace(c.Deploy.Compose.Up) == "" {
				*missing = append(*missing, "deploy.compose.up")
			}
			if strings.TrimSpace(c.Deploy.Compose.RemoteFile) != "" && strings.TrimSpace(c.Deploy.Compose.LocalFile) == "" {
				*missing = append(*missing, "deploy.compose.local_file")
			}
		case "binary-install":
			if c.Deploy.BinaryInstall.Host == "" {
				*missing = append(*missing, "deploy.binary_install.host")
			}
			if c.Deploy.BinaryInstall.RemoteTempPath == "" {
				*missing = append(*missing, "deploy.binary_install.remote_temp_path")
			}
			if c.Deploy.BinaryInstall.RemoteInstallPath == "" {
				*missing = append(*missing, "deploy.binary_install.remote_install_path")
			}
		case "ssh":
			if c.Deploy.SSH.Host == "" {
				*missing = append(*missing, "deploy.ssh.host")
			}
			if len(c.Deploy.SSH.Commands) == 0 {
				*missing = append(*missing, "deploy.ssh.commands")
			}
		case "none":
		default:
			*missing = append(*missing, "deploy.driver 仅支持 compose | binary-install | ssh | none")
		}
	}

	if c.Features.Verify {
		switch c.Verify.Driver {
		case "http":
			if c.Verify.HTTP.URL == "" {
				*missing = append(*missing, "verify.http.url")
			}
		case "ssh":
			if c.Verify.SSH.Host == "" {
				*missing = append(*missing, "verify.ssh.host")
			}
			if c.Verify.SSH.Command == "" {
				*missing = append(*missing, "verify.ssh.command")
			}
		case "command":
			if c.Verify.Command.Run == "" {
				*missing = append(*missing, "verify.command.run")
			}
		case "none":
		default:
			*missing = append(*missing, "verify.driver 仅支持 http | ssh | command | none")
		}
	}
}

func (c *Config) validateRegistryTargets(missing *[]string) {
	for index, target := range c.Publish.Registry.Targets {
		prefix := fmt.Sprintf("publish.registry.targets[%d]", index)
		switch target.Type {
		case "dockerhub", "private":
		case "":
			*missing = append(*missing, prefix+".type")
		default:
			*missing = append(*missing, prefix+".type 仅支持 dockerhub | private")
		}
		if target.Type == "private" && target.URL == "" {
			*missing = append(*missing, prefix+".url")
		}
		if target.Namespace == "" {
			*missing = append(*missing, prefix+".namespace")
		}
		if target.Image == "" {
			*missing = append(*missing, prefix+".image")
		}
	}
}
