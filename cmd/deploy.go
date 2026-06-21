package cmd

import (
	"fmt"
	"os"
	"path"
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
		return fmt.Errorf("渲染 deploy.compose.host 失败: %w", err)
	}
	deployPath, err := ctx.RenderString(cfg.Deploy.Compose.Path)
	if err != nil {
		return fmt.Errorf("渲染 deploy.compose.path 失败: %w", err)
	}
	tagKey, err := ctx.RenderString(cfg.Deploy.Compose.TagKey)
	if err != nil {
		return fmt.Errorf("渲染 deploy.compose.tag_key 失败: %w", err)
	}
	up, err := ctx.RenderString(cfg.Deploy.Compose.Up)
	if err != nil {
		return fmt.Errorf("渲染 deploy.compose.up 失败: %w", err)
	}
	envFile, err := ctx.RenderString(cfg.Deploy.Compose.EnvFile)
	if err != nil {
		return fmt.Errorf("渲染 deploy.compose.env_file 失败: %w", err)
	}

	// 当 env_file 不是默认的 ".env" 且 up 命令中未显式包含 --env-file 时，
	// 自动注入 --env-file 参数，确保 docker compose 读取正确的环境变量文件。
	if envFile != ".env" && !strings.Contains(up, "--env-file") {
		up = fmt.Sprintf("docker compose --env-file ./%s up -d", envFile)
	}
	localFile, err := renderOptionalComposeValue(ctx, cfg.Deploy.Compose.LocalFile, "deploy.compose.local_file")
	if err != nil {
		return err
	}
	remoteFile, err := renderOptionalComposeValue(ctx, cfg.Deploy.Compose.RemoteFile, "deploy.compose.remote_file")
	if err != nil {
		return err
	}
	localEnvFile, err := renderOptionalComposeValue(ctx, cfg.Deploy.Compose.LocalEnvFile, "deploy.compose.local_env_file")
	if err != nil {
		return err
	}
	if err := validateRenderedComposeConfig(host, deployPath, envFile, tagKey, up, localFile, remoteFile, localEnvFile); err != nil {
		return err
	}
	deployPath = strings.TrimRight(deployPath, "/")
	remoteEnvFile := composeRemotePath(deployPath, envFile)
	remoteComposeFile := composeRemoteFilePath(deployPath, remoteFile, localFile)
	internal.PrintInfo(fmt.Sprintf("compose deploy: host=%s path=%s env_file=%s compose_file=%s tag_key=%s version=%s", host, deployPath, remoteEnvFile, remoteComposeFile, tagKey, version))
	if err := ensureRemoteComposePaths(host, deployPath, remoteEnvFile, remoteComposeFile); err != nil {
		return err
	}
	if err := uploadComposeArtifact(host, localFile, remoteComposeFile, "compose file"); err != nil {
		return err
	}
	if err := uploadComposeArtifact(host, localEnvFile, remoteEnvFile, ".env file"); err != nil {
		return err
	}
	tagLine := fmt.Sprintf("%s=%s", tagKey, version)
	updateEnvCmd := fmt.Sprintf(
		"set -e; env_file=%s; tmp_file=\"${env_file}.ship.tmp\"; if [ -f \"$env_file\" ]; then grep -v '^%s=' \"$env_file\" > \"$tmp_file\"; else : > \"$tmp_file\"; fi; printf '%%s\\n' %s >> \"$tmp_file\"; mv \"$tmp_file\" \"$env_file\"",
		internal.ShellEscape(remoteEnvFile),
		tagKey,
		internal.ShellEscape(tagLine),
	)
	internal.ProgressSub(fmt.Sprintf("ssh %s: 更新 .env", host))
	if err := internal.RunCmd(
		[]string{"ssh", host, updateEnvCmd},
		fmt.Sprintf("ssh %s: 更新 .env", host),
	); err != nil {
		return fmt.Errorf("compose deploy 更新 env 失败: host=%s env_file=%s tag_key=%s: %w", host, remoteEnvFile, tagKey, err)
	}

	restartCmd := fmt.Sprintf("set -e; cd %s && %s", internal.ShellEscape(deployPath), up)
	internal.ProgressSub(fmt.Sprintf("ssh %s: docker compose up", host))
	if err := internal.RunCmd(
		[]string{"ssh", host, restartCmd},
		fmt.Sprintf("ssh %s: docker compose up", host),
	); err != nil {
		return fmt.Errorf("compose deploy 重启失败: host=%s path=%s up=%s: %w", host, deployPath, up, err)
	}

	return nil
}

// validateRenderedComposeConfig 校验 compose driver 渲染后的关键字段，避免把空值带到远端命令阶段。
func validateRenderedComposeConfig(host, deployPath, envFile, tagKey, up, localFile, remoteFile, localEnvFile string) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("deploy.compose.host 渲染结果不能为空")
	}
	if strings.TrimSpace(deployPath) == "" {
		return fmt.Errorf("deploy.compose.path 渲染结果不能为空")
	}
	if strings.TrimSpace(envFile) == "" {
		return fmt.Errorf("deploy.compose.env_file 渲染结果不能为空")
	}
	if strings.TrimSpace(tagKey) == "" {
		return fmt.Errorf("deploy.compose.tag_key 渲染结果不能为空")
	}
	if strings.TrimSpace(up) == "" {
		return fmt.Errorf("deploy.compose.up 渲染结果不能为空")
	}
	if strings.TrimSpace(remoteFile) != "" && strings.TrimSpace(localFile) == "" {
		return fmt.Errorf("deploy.compose.remote_file 只有在 deploy.compose.local_file 已设置时才有意义")
	}
	if err := validateComposeLocalSource(localFile, "deploy.compose.local_file"); err != nil {
		return err
	}
	if err := validateComposeLocalSource(localEnvFile, "deploy.compose.local_env_file"); err != nil {
		return err
	}
	return nil
}

// renderOptionalComposeValue 渲染 compose 可选字段；空值时直接返回空串。
func renderOptionalComposeValue(ctx internal.RenderContext, value, field string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	rendered, err := ctx.RenderString(value)
	if err != nil {
		return "", fmt.Errorf("渲染 %s 失败: %w", field, err)
	}
	return rendered, nil
}

// validateComposeLocalSource 校验本地上传源文件是否存在且不是目录。
func validateComposeLocalSource(filePath, field string) error {
	if strings.TrimSpace(filePath) == "" {
		return nil
	}
	cleaned := filepath.Clean(filePath)
	info, err := os.Stat(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s 指向的本地文件不存在: %s", field, cleaned)
		}
		return fmt.Errorf("读取 %s 失败: %w", field, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s 必须是文件，当前是目录: %s", field, cleaned)
	}
	return nil
}

// composeRemotePath 将 compose 相关远端路径归一化到 deploy.path 下。
func composeRemotePath(deployPath, filePath string) string {
	trimmed := strings.TrimSpace(filePath)
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return strings.TrimRight(deployPath, "/") + "/" + strings.TrimLeft(trimmed, "/")
}

// composeRemoteFilePath 解析远端 compose 文件路径；未显式配置时继承本地文件名。
func composeRemoteFilePath(deployPath, remoteFile, localFile string) string {
	if strings.TrimSpace(remoteFile) != "" {
		return composeRemotePath(deployPath, remoteFile)
	}
	if strings.TrimSpace(localFile) == "" {
		return ""
	}
	return composeRemotePath(deployPath, filepath.Base(localFile))
}

// ensureRemoteComposePaths 创建 compose 部署目录及其上传目标父目录。
func ensureRemoteComposePaths(host, deployPath, remoteEnvFile, remoteComposeFile string) error {
	dirs := uniqueRemoteDirs(deployPath, remoteEnvFile, remoteComposeFile)
	args := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		args = append(args, internal.ShellEscape(dir))
	}
	ensureCmd := fmt.Sprintf("set -e; mkdir -p %s", strings.Join(args, " "))
	internal.ProgressSub(fmt.Sprintf("ssh %s: 准备 compose 目录", host))
	if err := internal.RunCmd(
		[]string{"ssh", host, ensureCmd},
		fmt.Sprintf("ssh %s: prepare compose directories", host),
	); err != nil {
		return fmt.Errorf("compose deploy 准备远端目录失败: host=%s path=%s: %w", host, deployPath, err)
	}
	return nil
}

// uniqueRemoteDirs 计算需要提前创建的远端目录集合。
func uniqueRemoteDirs(deployPath, remoteEnvFile, remoteComposeFile string) []string {
	seen := map[string]struct{}{}
	var dirs []string
	for _, candidate := range []string{deployPath, path.Dir(remoteEnvFile), path.Dir(remoteComposeFile)} {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		dirs = append(dirs, trimmed)
	}
	return dirs
}

// uploadComposeArtifact 在配置存在时把本地文件上传到远端指定位置。
func uploadComposeArtifact(host, localFile, remoteFile, label string) error {
	if strings.TrimSpace(localFile) == "" {
		return nil
	}
	cleanedLocal := filepath.Clean(localFile)
	remoteTarget := fmt.Sprintf("%s:%s", host, remoteFile)
	internal.ProgressSub(fmt.Sprintf("scp %s -> %s", cleanedLocal, remoteTarget))
	if err := internal.RunCmd(
		[]string{"scp", cleanedLocal, remoteTarget},
		fmt.Sprintf("upload %s to %s", label, host),
	); err != nil {
		return fmt.Errorf("compose deploy 上传 %s 失败: local=%s remote=%s: %w", label, cleanedLocal, remoteTarget, err)
	}
	return nil
}

// doBinaryInstallDeploy 执行 binary-install driver 的远程安装逻辑。
func doBinaryInstallDeploy(profile internal.Profile, version string) error {
	ctx := internal.NewRenderContext(cfg, profile, version)
	renderedLocal, err := ctx.RenderString(cfg.Publish.SCP.Local)
	if err != nil {
		return fmt.Errorf("渲染 publish.scp.local 失败: %w", err)
	}
	host, err := ctx.RenderString(cfg.Deploy.BinaryInstall.Host)
	if err != nil {
		return fmt.Errorf("渲染 deploy.binary_install.host 失败: %w", err)
	}
	remoteTempPath, err := ctx.RenderString(cfg.Deploy.BinaryInstall.RemoteTempPath)
	if err != nil {
		return fmt.Errorf("渲染 deploy.binary_install.remote_temp_path 失败: %w", err)
	}
	remoteInstallPath, err := ctx.RenderString(cfg.Deploy.BinaryInstall.RemoteInstallPath)
	if err != nil {
		return fmt.Errorf("渲染 deploy.binary_install.remote_install_path 失败: %w", err)
	}
	chmod, err := ctx.RenderString(cfg.Deploy.BinaryInstall.Chmod)
	if err != nil {
		return fmt.Errorf("渲染 deploy.binary_install.chmod 失败: %w", err)
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
	internal.PrintInfo(fmt.Sprintf("binary-install deploy: host=%s temp=%s install=%s artifact=%s", host, remoteTempPath, remoteInstallPath, renderedLocal))
	if err := internal.RunCmd(args, fmt.Sprintf("ssh %s: install binary", host)); err != nil {
		return fmt.Errorf("binary-install deploy 失败: host=%s remote_temp_path=%s remote_install_path=%s: %w", host, remoteTempPath, remoteInstallPath, err)
	}
	return nil
}

// doSSHDeploy 执行 ssh driver 的远程命令列表。
func doSSHDeploy(profile internal.Profile, version string) error {
	ctx := internal.NewRenderContext(cfg, profile, version)
	host, err := ctx.RenderString(cfg.Deploy.SSH.Host)
	if err != nil {
		return fmt.Errorf("渲染 deploy.ssh.host 失败: %w", err)
	}
	commands, err := ctx.RenderSlice(cfg.Deploy.SSH.Commands)
	if err != nil {
		return fmt.Errorf("渲染 deploy.ssh.commands 失败: %w", err)
	}
	for _, command := range commands {
		internal.PrintInfo(fmt.Sprintf("ssh deploy: host=%s command=%s", host, command))
		internal.ProgressSub(fmt.Sprintf("ssh %s", host))
		if err := internal.RunCmd(
			[]string{"ssh", host, command},
			fmt.Sprintf("ssh %s", host),
		); err != nil {
			return fmt.Errorf("ssh deploy 失败: host=%s command=%s: %w", host, command, err)
		}
	}
	return nil
}
