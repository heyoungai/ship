package cmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/heyoungai/ship/internal"

	"github.com/spf13/cobra"
)

var deployVersion string

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "远程部署：更新版本号并重启容器",
	RunE: func(cmd *cobra.Command, args []string) error {
		// deploy 消费已发布版本，不创建 source worktree，也不修改 registry latest。
		session, err := prepareReleaseSession(cfg, deployVersion, false)
		if err != nil {
			return err
		}
		defer session.Close()
		ver := session.Version()

		manifest, err := internal.RequireReleaseManifest(session.StateRoot(), ver)
		if err != nil {
			return err
		}
		session.Manifest = manifest
		if !manifest.HasPublishedImage() && cfg.Build.Driver == "docker" && cfg.Publish.Driver == "registry" {
			return fmt.Errorf("版本 %s 的 manifest 中没有已发布的 container-image；请先 ship run 或 ship push", ver)
		}

		pin, degraded := internal.ResolveComposePin(cfg.Deploy.Compose.Pin, manifest.PrimaryImageDigest())
		if degraded {
			internal.PrintWarning("manifest 无可用 registry pin digest，deploy.compose.pin 降级为 tag")
		}
		if pin == "digest" {
			if p, reason := effectiveComposeDigestPin(cfg, session, pin); p != pin {
				pin = p
				internal.PrintWarning(reason)
			}
		}
		if err := verifyManifestDigests(manifest, pin == "digest"); err != nil {
			return err
		}
		if digest := manifest.PrimaryImageDigest(); digest != "" {
			internal.PrintInfo(fmt.Sprintf("deploy from manifest: run_id=%s digest=%s pin=%s", manifest.RunID, digest, pin))
		} else {
			internal.PrintInfo(fmt.Sprintf("deploy from manifest: run_id=%s pin=%s (no digest)", manifest.RunID, pin))
		}

		profile := cfg.DefaultProfile()
		meta := historyMetaFromSession(session)
		if err := executeDeployStage(cfg, ver, profile, session); err != nil {
			return recordDeploymentResult(err, ver, "deploy", "fail", err.Error(), meta)
		}
		if err := internal.ExecuteVerify(cfg, profile, ver); err != nil {
			return recordDeploymentResult(err, ver, "deploy", "fail", err.Error(), meta)
		}
		return recordDeploymentResult(nil, ver, "deploy", "success", "", meta)
	},
}

func init() {
	deployCmd.Flags().StringVarP(&deployVersion, "version", "v", "", "正式 release tag（git-tag 模式下必须存在）")
}

// doDeploy 按当前 deploy.driver 执行部署。
// session 可为 nil（测试）；非 nil 时用于将未跟踪本地文件锚定到 InvocationRoot。
func doDeploy(cfg *internal.Config, version string, profile internal.Profile, session *releaseSession) error {
	renderedCfg, renderedProfile, err := internal.RenderConfigForProfile(cfg, profile, version)
	if err != nil {
		return err
	}
	cfg = renderedCfg
	profile = renderedProfile

	switch cfg.Deploy.Driver {
	case "compose":
		if err := doComposeDeploy(cfg, version, profile, session); err != nil {
			return err
		}
	case "binary-install":
		if err := doBinaryInstallDeploy(cfg, profile, version); err != nil {
			return err
		}
	case "ssh":
		if err := doSSHDeploy(cfg, profile, version); err != nil {
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
func doComposeDeploy(cfg *internal.Config, version string, profile internal.Profile, session *releaseSession) error {
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

	// 当 auto_env_file 启用（默认）且 env_file 不是默认的 ".env" 且 up 命令中未显式包含 --env-file 时，
	// 自动注入 --env-file 参数，确保 docker compose 读取正确的环境变量文件。
	// 可通过 deploy.compose.auto_env_file = false 关闭此行为。
	if cfg.Deploy.Compose.AutoEnvFile && envFile != ".env" && !strings.Contains(up, "--env-file") {
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
	if session != nil {
		localFile, localEnvFile, err = resolveComposeLocalPaths(session, localFile, localEnvFile)
		if err != nil {
			return err
		}
	}
	if err := validateRenderedComposeConfig(host, deployPath, envFile, tagKey, up, localFile, remoteFile, localEnvFile); err != nil {
		return err
	}
	deployPath = strings.TrimRight(deployPath, "/")
	remoteEnvFile := composeRemotePath(deployPath, envFile)
	remoteComposeFile := composeRemoteFilePath(deployPath, remoteFile, localFile)
	if err := ensureRemoteComposePaths(host, deployPath, remoteEnvFile, remoteComposeFile); err != nil {
		return err
	}
	if err := uploadComposeArtifact(host, localFile, remoteComposeFile, "compose file"); err != nil {
		return err
	}
	if err := uploadComposeArtifact(host, localEnvFile, remoteEnvFile, ".env file"); err != nil {
		return err
	}

	envUpdates, err := composeEnvUpdates(cfg, version, profile, session)
	if err != nil {
		return err
	}
	internal.PrintInfo(fmt.Sprintf("compose deploy: host=%s path=%s env_file=%s compose_file=%s updates=%v", host, deployPath, remoteEnvFile, remoteComposeFile, envUpdates))
	if err := updateRemoteEnvKeys(host, remoteEnvFile, envUpdates); err != nil {
		return err
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

// composeEnvUpdates 按 pin 模式生成远端 env 键值。
func composeEnvUpdates(cfg *internal.Config, version string, profile internal.Profile, session *releaseSession) (map[string]string, error) {
	tagKey := strings.TrimSpace(cfg.Deploy.Compose.TagKey)
	if tagKey == "" {
		return nil, fmt.Errorf("deploy.compose.tag_key 不能为空")
	}
	updates := map[string]string{tagKey: version}

	digest := ""
	imageRef := ""
	if session != nil && session.Manifest != nil {
		art := selectImageArtifact(session.Manifest, profile)
		digest = art.Digest
		imageRef = art.Ref
	}

	pin, degraded := internal.ResolveComposePin(cfg.Deploy.Compose.Pin, digest)
	if degraded {
		internal.PrintWarning("manifest 无可用 registry pin digest，本次部署 pin 降级为 tag")
	}
	if pin == "digest" {
		if p, reason := effectiveComposeDigestPin(cfg, session, pin); p != pin {
			pin = p
			internal.PrintWarning(reason)
		}
	}
	if pin == "digest" {
		digestKey := strings.TrimSpace(cfg.Deploy.Compose.DigestKey)
		if digestKey == "" {
			digestKey = "APP_IMAGE_DIGEST"
		}
		updates[digestKey] = digest
		if imageKey := strings.TrimSpace(cfg.Deploy.Compose.ImageKey); imageKey != "" {
			full := internal.ImageDigestRef(imageRef, digest)
			if full == "" {
				return nil, fmt.Errorf("pin=digest 且配置了 image_key，但无法从 manifest 构造 repo@digest")
			}
			updates[imageKey] = full
		}
	}
	return updates, nil
}

func selectImageArtifact(m *internal.ReleaseManifest, profile internal.Profile) internal.ArtifactRecord {
	if m == nil {
		return internal.ArtifactRecord{}
	}
	want := internal.FormatProfileName(profile)
	if want == "" {
		want = "default"
	}
	var fallback internal.ArtifactRecord
	for _, a := range m.Artifacts {
		if a.Type != internal.ArtifactTypeImage {
			continue
		}
		ap := a.Profile
		if ap == "" {
			ap = "default"
		}
		if fallback.Ref == "" {
			fallback = a
		}
		if ap == want {
			return a
		}
	}
	return fallback
}

// updateRemoteEnvKeys 在远端 env 文件中写入/替换多个 KEY=VALUE。
func updateRemoteEnvKeys(host, remoteEnvFile string, updates map[string]string) error {
	if len(updates) == 0 {
		return nil
	}
	keys := make([]string, 0, len(updates))
	for k := range updates {
		keys = append(keys, k)
	}
	// 稳定顺序便于测试与日志
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	var b strings.Builder
	b.WriteString("set -e; env_file=")
	b.WriteString(internal.ShellEscape(remoteEnvFile))
	b.WriteString("; tmp_file=\"${env_file}.ship.tmp\"; ")
	b.WriteString(": > \"$tmp_file\"; ")
	b.WriteString("if [ -f \"$env_file\" ]; then ")
	// 过滤掉将要更新的 key
	grepArgs := make([]string, 0, len(keys))
	for _, k := range keys {
		grepArgs = append(grepArgs, fmt.Sprintf("-e '^%s='", k))
	}
	b.WriteString("grep -v ")
	b.WriteString(strings.Join(grepArgs, " "))
	b.WriteString(" \"$env_file\" > \"$tmp_file\" || true; fi; ")
	for _, k := range keys {
		line := fmt.Sprintf("%s=%s", k, updates[k])
		b.WriteString(fmt.Sprintf("printf '%%s\\n' %s >> \"$tmp_file\"; ", internal.ShellEscape(line)))
	}
	b.WriteString("mv \"$tmp_file\" \"$env_file\"")

	internal.ProgressSub(fmt.Sprintf("ssh %s: 更新 .env (%s)", host, strings.Join(keys, ",")))
	if err := internal.RunCmd(
		[]string{"ssh", host, b.String()},
		fmt.Sprintf("ssh %s: 更新 .env", host),
	); err != nil {
		return fmt.Errorf("compose deploy 更新 env 失败: host=%s env_file=%s: %w", host, remoteEnvFile, err)
	}
	return nil
}

func verifyManifestDigests(manifest *internal.ReleaseManifest, hardFail bool) error {
	if manifest == nil {
		return nil
	}
	for _, a := range manifest.Artifacts {
		if a.Type != internal.ArtifactTypeImage || a.Ref == "" || a.Digest == "" {
			continue
		}
		remotePin, exists, err := internal.ResolveRegistryPinDigest(a.Ref)
		if err != nil {
			if hardFail {
				return fmt.Errorf("无法校验远端 digest (%s): %w", a.Ref, err)
			}
			internal.PrintWarning(fmt.Sprintf("无法校验远端 digest (%s): %v（当前按 tag 部署，仅警告）", a.Ref, err))
			continue
		}
		if !exists {
			if hardFail {
				return fmt.Errorf("远端不存在镜像 %s，无法按 digest 部署", a.Ref)
			}
			continue
		}
		if remotePin == "" {
			// index 存在但无 pin digest：尝试用 manifest 指纹做成员比对（兼容旧逻辑）。
			fp, fpExists, fpErr := internal.InspectRemoteDigest(a.Ref)
			if fpErr != nil {
				if hardFail {
					return fmt.Errorf("无法校验远端 digest (%s): %w", a.Ref, fpErr)
				}
				internal.PrintWarning(fmt.Sprintf("无法校验远端 digest (%s): %v（当前按 tag 部署，仅警告）", a.Ref, fpErr))
				continue
			}
			if !fpExists {
				if hardFail {
					return fmt.Errorf("远端不存在镜像 %s，无法按 digest 部署", a.Ref)
				}
				continue
			}
			if internal.DigestsMatch(a.Digest, fp) {
				continue
			}
			msg := fmt.Sprintf("远端 %s digest 与 manifest 不一致：remote=<index> manifest=%s", a.Ref, a.Digest)
			if hardFail {
				return fmt.Errorf("%s；若为 v2.7.0 误记的本地 config digest，请设 pin=\"tag\" 或重新 push", msg)
			}
			internal.PrintWarning(msg + "（当前按 tag 部署，仅警告）")
			continue
		}
		if internal.DigestsMatch(a.Digest, remotePin) {
			continue
		}
		// 再比对完整 manifest 指纹，识别「config digest ∈ index 成员」等兼容情况。
		if fp, ok, err := internal.InspectRemoteDigest(a.Ref); err == nil && ok && internal.DigestsMatch(a.Digest, fp) {
			continue
		}
		msg := fmt.Sprintf("远端 %s digest 与 manifest 不一致：remote=%s manifest=%s", a.Ref, remotePin, a.Digest)
		if hardFail {
			return fmt.Errorf("%s；若为 v2.7.0 误记的本地 config digest，请设 pin=\"tag\" 或重新 push", msg)
		}
		internal.PrintWarning(msg + "（当前按 tag 部署，仅警告）")
	}
	return nil
}

// effectiveComposeDigestPin 在 pin=digest 时检查 compose 是否真的按 @digest 拉取。
// 若本地 compose 可检查且未使用 digest 引用，则降级为 tag，避免只写 env 却仍按 tag 部署时硬失败。
func effectiveComposeDigestPin(cfg *internal.Config, session *releaseSession, pin string) (string, string) {
	if pin != "digest" || cfg == nil {
		return pin, ""
	}
	if strings.TrimSpace(cfg.Deploy.Compose.ImageKey) != "" {
		return "digest", ""
	}
	digestKey := strings.TrimSpace(cfg.Deploy.Compose.DigestKey)
	if digestKey == "" {
		digestKey = "APP_IMAGE_DIGEST"
	}
	localFile := strings.TrimSpace(cfg.Deploy.Compose.LocalFile)
	if localFile == "" || strings.Contains(localFile, "{{") {
		return "digest", ""
	}
	path := resolveComposeCheckPath(session, localFile)
	uses, checked := composeFileUsesDigestPin(path, digestKey)
	if !checked {
		return "digest", ""
	}
	if !uses {
		return "tag", fmt.Sprintf(
			"compose 未使用 @${%s}（或等价 digest 引用），deploy.compose.pin 降级为 tag",
			digestKey,
		)
	}
	return "digest", ""
}

func resolveComposeCheckPath(session *releaseSession, localFile string) string {
	if filepath.IsAbs(localFile) {
		return localFile
	}
	if session != nil {
		if root := session.SourceRoot(); root != "" {
			candidate := filepath.Join(root, localFile)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		if root := session.InvocationRoot(); root != "" {
			candidate := filepath.Join(root, localFile)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return localFile
}

func composeFileUsesDigestPin(path, digestKey string) (uses bool, checked bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	content := string(data)
	candidates := []string{
		"@${" + digestKey + "}",
		"@$" + digestKey,
		"@{" + digestKey + "}",
	}
	for _, c := range candidates {
		if strings.Contains(content, c) {
			return true, true
		}
	}
	// image_key 场景外：任意 @${...DIGEST...} 也视为 digest pin。
	if strings.Contains(content, "@${") && strings.Contains(strings.ToUpper(content), "DIGEST") {
		return true, true
	}
	return false, true
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
func doBinaryInstallDeploy(cfg *internal.Config, profile internal.Profile, version string) error {
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
func doSSHDeploy(cfg *internal.Config, profile internal.Profile, version string) error {
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
