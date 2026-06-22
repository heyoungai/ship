package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"github.com/heyoungai/ship/internal"
	"strings"

	"github.com/spf13/cobra"
)

var (
	buildEnvFile string
	buildProfile string
	buildVersion string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "构建产物",
	RunE: func(cmd *cobra.Command, args []string) error {
		version, err := internal.ResolveVersion(cfg, buildVersion)
		if err != nil {
			return err
		}
		profiles, err := cfg.GetProfiles(buildProfile)
		if err != nil {
			return err
		}
		internal.ProgressInit(len(profiles))
		for i, p := range profiles {
			internal.ProgressStep(i+1, buildStepTitle())
			if err := executeBuildProfile(version, p, buildEnvFile); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	buildCmd.Flags().StringVarP(&buildVersion, "version", "v", "", "版本号 (默认按配置自动解析)")
	buildCmd.Flags().StringVar(&buildEnvFile, "env-file", "", ".env 文件路径 (默认使用配置)")
	buildCmd.Flags().StringVarP(&buildProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
}

// doBuild 按当前 build.driver 执行单个 profile 的构建。
func doBuild(profile internal.Profile, envFile, version string) error {
	switch cfg.Build.Driver {
	case "docker":
		return doDockerBuild(profile, envFile, version)
	case "go-binary":
		return doGoBinaryBuild(profile, version)
	case "command":
		return doCommandBuild(profile, version)
	default:
		return fmt.Errorf("当前不支持的 build.driver: %s", cfg.Build.Driver)
	}
}

// doDockerBuild 执行 Docker driver 的构建，并对 v2 模板变量做渲染。
func doDockerBuild(profile internal.Profile, envFile, version string) error {
	ctx := internal.NewRenderContext(cfg, profile, version)
	if envFile == "" {
		envFile = cfg.Build.EnvFile
	}
	renderedEnvFile, err := ctx.RenderString(envFile)
	if err != nil {
		return err
	}
	renderedLocalBuild, err := ctx.RenderString(cfg.Build.LocalBuild)
	if err != nil {
		return err
	}
	renderedDockerfile, err := ctx.RenderString(cfg.Build.Dockerfile)
	if err != nil {
		return err
	}
	renderedContext, err := ctx.RenderString(cfg.Build.Docker.Context)
	if err != nil {
		return err
	}
	buildArgs, err := dockerBuildArgs(ctx, renderedEnvFile)
	if err != nil {
		return err
	}

	name := internal.FormatProfileName(profile)
	nameLabel := ""
	if name != "" {
		nameLabel = " " + internal.BoldStyle.Render("["+name+"]")
	}

	// 1. 可选：本地构建（如 Next.js）
	localBuild := renderedLocalBuild
	if localBuild == "" {
		localBuild = internal.DetectLocalBuild()
	}
	if localBuild != "" {
		fmt.Printf("  %s 本地构建%s\n", internal.StepStyle.Render("▸"), nameLabel)
		internal.ProgressSub(localBuild)

		envMap := buildArgsToEnv(buildArgs)
		envMap = internal.MergeEnv(envMap, profile.Env)
		localBuildArgs, err := internal.ShellCommandArgsWithMode("auto", localBuild)
		if err != nil {
			return err
		}

		if err := internal.RunCmdWithEnv(
			localBuildArgs,
			localBuild,
			envMap,
		); err != nil {
			return err
		}
	}

	// 2. Docker buildx build
	argCount := len(buildArgs) / 2
	fmt.Printf("  %s Docker 构建%s  %s\n",
		internal.StepStyle.Render("▸"),
		nameLabel,
		internal.DimStyle.Render(fmt.Sprintf("(%d build-args)", argCount)))

	tag := cfg.BuildSourceTag(profile)
	internal.ProgressSub(cfg.ImageRef(tag))
	outputArgs, err := internal.BuildxOutputArgs(cfg.Build.Platforms, cfg.Build.Docker.Load)
	if err != nil {
		return err
	}

	args := []string{
		"docker", "buildx", "build",
		"--platform", cfg.Build.Platforms,
		"--file", renderedDockerfile,
	}
	if cfg.Build.Docker.CacheBust {
		args = append(args, "--no-cache")
	}
	args = append(args, outputArgs...)
	args = append(args, buildArgs...)

	for k, v := range profile.Env {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, "--tag", cfg.ImageRef(tag), renderedContext)

	if err := internal.RunCmd(args, cfg.ImageRef(tag)); err != nil {
		return err
	}

	return nil
}

// doGoBinaryBuild 执行 Go 二进制构建，并渲染 output / ldflags 等模板变量。
func doGoBinaryBuild(profile internal.Profile, version string) error {
	ctx := internal.NewRenderContext(cfg, profile, version)
	name := internal.FormatProfileName(profile)
	nameLabel := ""
	if name != "" {
		nameLabel = " " + internal.BoldStyle.Render("["+name+"]")
	}

	output, err := ctx.RenderString(cfg.Build.Go.Output)
	if err != nil {
		return err
	}
	if output == "" {
		return fmt.Errorf("build.go.output 不能为空")
	}
	mainPath, err := ctx.RenderString(cfg.Build.Go.Main)
	if err != nil {
		return err
	}
	ldflags, err := ctx.RenderSlice(cfg.Build.Go.Ldflags)
	if err != nil {
		return err
	}
	goos, err := ctx.RenderString(cfg.Build.Go.GoOS)
	if err != nil {
		return err
	}
	goarch, err := ctx.RenderString(cfg.Build.Go.GoArch)
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(output)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("创建构建目录失败: %w", err)
		}
	}

	fmt.Printf("  %s Go 构建%s  %s\n",
		internal.StepStyle.Render("▸"),
		nameLabel,
		internal.DimStyle.Render("go-binary"))
	internal.ProgressSub(output)

	args := []string{"go", "build"}
	if len(ldflags) > 0 {
		args = append(args, "-ldflags", strings.Join(ldflags, " "))
	}
	args = append(args, "-o", output, mainPath)

	envMap := internal.MergeEnv(nil, profile.Env)
	if goos != "" {
		envMap["GOOS"] = goos
	}
	if goarch != "" {
		envMap["GOARCH"] = goarch
	}
	if cfg.Build.Go.CGOEnabled {
		envMap["CGO_ENABLED"] = "1"
	} else {
		envMap["CGO_ENABLED"] = "0"
	}

	return internal.RunCmdWithOptions(
		args,
		fmt.Sprintf("go build%s -> %s", nameLabel, output),
		"",
		envMap,
	)
}

// doCommandBuild 执行 command driver 的构建命令，并渲染 run/cwd/env。
func doCommandBuild(profile internal.Profile, version string) error {
	ctx := internal.NewRenderContext(cfg, profile, version)
	name := internal.FormatProfileName(profile)
	nameLabel := ""
	if name != "" {
		nameLabel = " " + internal.BoldStyle.Render("["+name+"]")
	}
	run, err := ctx.RenderString(cfg.Build.Command.Run)
	if err != nil {
		return err
	}
	cwd, err := ctx.RenderString(cfg.Build.Command.Cwd)
	if err != nil {
		return err
	}
	env, err := ctx.RenderMap(cfg.Build.Command.Env)
	if err != nil {
		return err
	}
	args, err := internal.ShellCommandArgsWithMode("auto", run)
	if err != nil {
		return err
	}

	fmt.Printf("  %s 自定义构建%s\n", internal.StepStyle.Render("▸"), nameLabel)
	internal.ProgressSub(run)

	return internal.RunCmdWithOptions(
		args,
		fmt.Sprintf("custom build%s", nameLabel),
		cwd,
		internal.MergeEnv(env, profile.Env),
	)
}

// dockerBuildArgs 汇总 .env 与 build.docker.build_args 的 build-arg 参数。
func dockerBuildArgs(ctx internal.RenderContext, envFile string) ([]string, error) {
	var args []string
	if cfg.Build.Docker.BuildArgsFromEnv {
		args = append(args, internal.LoadBuildArgs(envFile)...)
	}
	renderedBuildArgs, err := ctx.RenderMap(cfg.Build.Docker.BuildArgs)
	if err != nil {
		return nil, err
	}
	for key, value := range renderedBuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}
	return args, nil
}

// buildArgsToEnv 将 build-arg 形式的参数还原为环境变量映射，供 local_build 复用。
func buildArgsToEnv(args []string) map[string]string {
	envMap := make(map[string]string)
	for i := 0; i < len(args)-1; i += 2 {
		if args[i] != "--build-arg" {
			continue
		}
		key, value, ok := strings.Cut(args[i+1], "=")
		if ok {
			envMap[key] = value
		}
	}
	return envMap
}
