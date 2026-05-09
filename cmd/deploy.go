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
		ver, err := internal.ResolveVersion(deployVersion)
		if err != nil {
			return err
		}
		if err := doDeploy(ver); err != nil {
			return recordDeploymentResult(err, ver, "deploy", "fail", err.Error())
		}
		return recordDeploymentResult(nil, ver, "deploy", "success", "")
	},
}

func init() {
	deployCmd.Flags().StringVarP(&deployVersion, "version", "v", "", "版本号 (默认取最新 git tag)")
}

// doDeploy 按当前 deploy.driver 执行部署。
func doDeploy(version string) error {
	switch cfg.Deploy.Driver {
	case "compose":
		if err := doComposeDeploy(version); err != nil {
			return err
		}
	case "binary-install":
		if err := doBinaryInstallDeploy(); err != nil {
			return err
		}
	case "ssh":
		if err := doSSHDeploy(); err != nil {
			return err
		}
	case "none":
		internal.PrintInfo("当前配置未启用部署阶段")
		return nil
	default:
		return fmt.Errorf("当前不支持的 deploy.driver: %s", cfg.Deploy.Driver)
	}

	if cfg.Deploy.Healthcheck.Enabled() {
		internal.ProgressSub(fmt.Sprintf("healthcheck %s", cfg.Deploy.Healthcheck.URL))
		if err := internal.WaitForHealthcheck(cfg.Deploy.Healthcheck); err != nil {
			return err
		}
		internal.PrintSuccess("健康检查通过")
	}

	return nil
}

func doComposeDeploy(version string) error {
	deployPath := strings.TrimRight(cfg.Deploy.Path, "/")
	envFile := cfg.Deploy.Compose.EnvFile
	if !strings.HasPrefix(envFile, "/") {
		envFile = deployPath + "/" + strings.TrimLeft(envFile, "/")
	}
	tagLine := fmt.Sprintf("%s=%s", cfg.Deploy.Compose.TagKey, version)
	updateEnvCmd := fmt.Sprintf(
		"set -e; env_file=%s; tmp_file=\"${env_file}.ship.tmp\"; if [ -f \"$env_file\" ]; then grep -v '^%s=' \"$env_file\" > \"$tmp_file\"; else : > \"$tmp_file\"; fi; printf '%%s\\n' %s >> \"$tmp_file\"; mv \"$tmp_file\" \"$env_file\"",
		internal.ShellEscape(envFile),
		cfg.Deploy.Compose.TagKey,
		internal.ShellEscape(tagLine),
	)
	internal.ProgressSub(fmt.Sprintf("ssh %s: 更新 .env", cfg.Deploy.Host))
	if err := internal.RunCmd(
		[]string{"ssh", cfg.Deploy.Host, updateEnvCmd},
		fmt.Sprintf("ssh %s: 更新 .env", cfg.Deploy.Host),
	); err != nil {
		return err
	}

	restartCmd := fmt.Sprintf("set -e; cd %s && %s", internal.ShellEscape(deployPath), cfg.Deploy.Compose.Up)
	internal.ProgressSub(fmt.Sprintf("ssh %s: docker compose up", cfg.Deploy.Host))
	if err := internal.RunCmd(
		[]string{"ssh", cfg.Deploy.Host, restartCmd},
		fmt.Sprintf("ssh %s: docker compose up", cfg.Deploy.Host),
	); err != nil {
		return err
	}

	return nil
}

func doBinaryInstallDeploy() error {
	artifactName := filepath.Base(cfg.Publish.SCP.Local)
	sudoPrefix := ""
	if cfg.Deploy.BinaryInstall.SudoNoPasswd {
		sudoPrefix = "sudo -n "
	}
	installPathLooksLikeDir := "0"
	if strings.HasSuffix(cfg.Deploy.BinaryInstall.RemoteInstallPath, "/") {
		installPathLooksLikeDir = "1"
	}
	remoteCmd := fmt.Sprintf(
		"set -e; artifact_name=%s; src_base=%s; install_base=%s; src=\"$src_base\"; if [ -d \"$src_base\" ]; then src=\"${src_base%%/}/$artifact_name\"; fi; if [ ! -f \"$src\" ]; then echo \"uploaded artifact not found: $src\" >&2; exit 1; fi; target=\"$install_base\"; if [ -d \"$install_base\" ] || [ %s = 1 ]; then target=\"${install_base%%/}/$artifact_name\"; fi; %smkdir -p \"$(dirname \"$target\")\"; %scp \"$src\" \"$target\"; %schmod %s \"$target\"",
		internal.ShellEscape(artifactName),
		internal.ShellEscape(cfg.Deploy.BinaryInstall.RemoteTempPath),
		internal.ShellEscape(cfg.Deploy.BinaryInstall.RemoteInstallPath),
		installPathLooksLikeDir,
		sudoPrefix,
		sudoPrefix,
		sudoPrefix,
		internal.ShellEscape(cfg.Deploy.BinaryInstall.Chmod),
	)

	args := []string{"ssh"}
	if cfg.Deploy.BinaryInstall.UseSSHTTY {
		args = append(args, "-t")
	}
	args = append(args, cfg.Deploy.BinaryInstall.Host, remoteCmd)

	internal.ProgressSub(fmt.Sprintf("ssh %s: 安装二进制", cfg.Deploy.BinaryInstall.Host))
	return internal.RunCmd(args, fmt.Sprintf("ssh %s: install binary", cfg.Deploy.BinaryInstall.Host))
}

func doSSHDeploy() error {
	for _, command := range cfg.Deploy.SSH.Commands {
		internal.ProgressSub(fmt.Sprintf("ssh %s", cfg.Deploy.SSH.Host))
		if err := internal.RunCmd(
			[]string{"ssh", cfg.Deploy.SSH.Host, command},
			fmt.Sprintf("ssh %s", cfg.Deploy.SSH.Host),
		); err != nil {
			return err
		}
	}
	return nil
}
