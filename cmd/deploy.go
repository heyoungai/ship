package cmd

import (
	"fmt"
	"path/filepath"
	"ship/internal"
	"strings"

	"github.com/spf13/cobra"
)

var deployVersion string

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "远程部署：更新版本号并重启容器",
	RunE: func(cmd *cobra.Command, args []string) error {
		ver, err := internal.ResolveVersion(cfg, deployVersion)
		if err != nil {
			return err
		}
		profile := cfg.DefaultProfile()
		if err := executeDeployStage(ver, profile); err != nil {
			return recordDeploymentResult(err, ver, "deploy", "fail", err.Error())
		}
		if err := internal.ExecuteVerify(cfg, profile, ver); err != nil {
			return recordDeploymentResult(err, ver, "deploy", "fail", err.Error())
		}
		return recordDeploymentResult(nil, ver, "deploy", "success", "")
	},
}

func init() {
	deployCmd.Flags().StringVarP(&deployVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
}

// doDeploy 按当前 deploy.driver 执行部署。
func doDeploy(version string, profile internal.Profile) error {
	switch cfg.Deploy.Driver {
	case "compose":
		if err := doComposeDeploy(version, profile); err != nil {
			return err
		}
	case "binary-install":
		if err := doBinaryInstallDeploy(profile, version); err != nil {
			return err
		}
	case "ssh":
		if err := doSSHDeploy(profile, version); err != nil {
			return err
		}
	case "none":
		internal.PrintInfo("当前配置未启用部署阶段")
		return nil
	default:
		return fmt.Errorf("当前不支持的 deploy.driver: %s", cfg.Deploy.Driver)
	}
	return nil
}

// doComposeDeploy 执行 compose driver 的远程部署，并渲染 tag_key/up/path 等字段。
func doComposeDeploy(version string, profile internal.Profile) error {
	ctx := internal.NewRenderContext(cfg, profile, version)
	host, err := ctx.RenderString(cfg.Deploy.Compose.Host)
	if err != nil {
		return err
	}
	deployPath, err := ctx.RenderString(cfg.Deploy.Compose.Path)
	if err != nil {
		return err
	}
	tagKey, err := ctx.RenderString(cfg.Deploy.Compose.TagKey)
	if err != nil {
		return err
	}
	up, err := ctx.RenderString(cfg.Deploy.Compose.Up)
	if err != nil {
		return err
	}
	envFile, err := ctx.RenderString(cfg.Deploy.Compose.EnvFile)
	if err != nil {
		return err
	}
	deployPath = strings.TrimRight(deployPath, "/")
	if !strings.HasPrefix(envFile, "/") {
		envFile = deployPath + "/" + strings.TrimLeft(envFile, "/")
	}
	tagLine := fmt.Sprintf("%s=%s", tagKey, version)
	updateEnvCmd := fmt.Sprintf(
		"set -e; env_file=%s; tmp_file=\"${env_file}.ship.tmp\"; if [ -f \"$env_file\" ]; then grep -v '^%s=' \"$env_file\" > \"$tmp_file\"; else : > \"$tmp_file\"; fi; printf '%%s\\n' %s >> \"$tmp_file\"; mv \"$tmp_file\" \"$env_file\"",
		internal.ShellEscape(envFile),
		tagKey,
		internal.ShellEscape(tagLine),
	)
	internal.ProgressSub(fmt.Sprintf("ssh %s: 更新 .env", host))
	if err := internal.RunCmd(
		[]string{"ssh", host, updateEnvCmd},
		fmt.Sprintf("ssh %s: 更新 .env", host),
	); err != nil {
		return err
	}

	restartCmd := fmt.Sprintf("set -e; cd %s && %s", internal.ShellEscape(deployPath), up)
	internal.ProgressSub(fmt.Sprintf("ssh %s: docker compose up", host))
	if err := internal.RunCmd(
		[]string{"ssh", host, restartCmd},
		fmt.Sprintf("ssh %s: docker compose up", host),
	); err != nil {
		return err
	}

	return nil
}

// doBinaryInstallDeploy 执行 binary-install driver 的远程安装逻辑。
func doBinaryInstallDeploy(profile internal.Profile, version string) error {
	ctx := internal.NewRenderContext(cfg, profile, version)
	renderedLocal, err := ctx.RenderString(cfg.Publish.SCP.Local)
	if err != nil {
		return err
	}
	host, err := ctx.RenderString(cfg.Deploy.BinaryInstall.Host)
	if err != nil {
		return err
	}
	remoteTempPath, err := ctx.RenderString(cfg.Deploy.BinaryInstall.RemoteTempPath)
	if err != nil {
		return err
	}
	remoteInstallPath, err := ctx.RenderString(cfg.Deploy.BinaryInstall.RemoteInstallPath)
	if err != nil {
		return err
	}
	chmod, err := ctx.RenderString(cfg.Deploy.BinaryInstall.Chmod)
	if err != nil {
		return err
	}
	artifactName := filepath.Base(renderedLocal)
	sudoPrefix := ""
	if cfg.Deploy.BinaryInstall.SudoNoPasswd {
		sudoPrefix = "sudo -n "
	}
	installPathLooksLikeDir := "0"
	if strings.HasSuffix(remoteInstallPath, "/") {
		installPathLooksLikeDir = "1"
	}
	remoteCmd := fmt.Sprintf(
		"set -e; artifact_name=%s; src_base=%s; install_base=%s; src=\"$src_base\"; if [ -d \"$src_base\" ]; then src=\"${src_base%%/}/$artifact_name\"; fi; if [ ! -f \"$src\" ]; then echo \"uploaded artifact not found: $src\" >&2; exit 1; fi; target=\"$install_base\"; if [ -d \"$install_base\" ] || [ %s = 1 ]; then target=\"${install_base%%/}/$artifact_name\"; fi; %smkdir -p \"$(dirname \"$target\")\"; %scp \"$src\" \"$target\"; %schmod %s \"$target\"",
		internal.ShellEscape(artifactName),
		internal.ShellEscape(remoteTempPath),
		internal.ShellEscape(remoteInstallPath),
		installPathLooksLikeDir,
		sudoPrefix,
		sudoPrefix,
		sudoPrefix,
		internal.ShellEscape(chmod),
	)

	args := []string{"ssh"}
	if cfg.Deploy.BinaryInstall.UseSSHTTY {
		args = append(args, "-t")
	}
	args = append(args, host, remoteCmd)

	internal.ProgressSub(fmt.Sprintf("ssh %s: 安装二进制", host))
	return internal.RunCmd(args, fmt.Sprintf("ssh %s: install binary", host))
}

// doSSHDeploy 执行 ssh driver 的远程命令列表。
func doSSHDeploy(profile internal.Profile, version string) error {
	ctx := internal.NewRenderContext(cfg, profile, version)
	host, err := ctx.RenderString(cfg.Deploy.SSH.Host)
	if err != nil {
		return err
	}
	commands, err := ctx.RenderSlice(cfg.Deploy.SSH.Commands)
	if err != nil {
		return err
	}
	for _, command := range commands {
		internal.ProgressSub(fmt.Sprintf("ssh %s", host))
		if err := internal.RunCmd(
			[]string{"ssh", host, command},
			fmt.Sprintf("ssh %s", host),
		); err != nil {
			return err
		}
	}
	return nil
}
