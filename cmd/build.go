package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"ship/internal"
	"strings"

	"github.com/spf13/cobra"
)

var (
	buildEnvFile string
	buildProfile string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "构建产物",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := cfg.GetProfiles(buildProfile)
		if err != nil {
			return err
		}
		internal.ProgressInit(len(profiles))
		for i, p := range profiles {
			internal.ProgressStep(i+1, "构建镜像")
			if err := doBuild(p, buildEnvFile); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	buildCmd.Flags().StringVar(&buildEnvFile, "env-file", "", ".env 文件路径 (默认使用配置)")
	buildCmd.Flags().StringVarP(&buildProfile, "profile", "p", "", "指定 profile 名称 (默认全部)")
}

// doBuild 按当前 build.driver 执行单个 profile 的构建。
func doBuild(profile internal.Profile, envFile string) error {
	switch cfg.Build.Driver {
	case "docker":
		return doDockerBuild(profile, envFile)
	case "go-binary":
		return doGoBinaryBuild(profile)
	case "command":
		return doCommandBuild(profile)
	default:
		return fmt.Errorf("当前不支持的 build.driver: %s", cfg.Build.Driver)
	}
}

func doDockerBuild(profile internal.Profile, envFile string) error {
	if envFile == "" {
		envFile = cfg.Build.EnvFile
	}

	name := internal.FormatProfileName(profile)
	nameLabel := ""
	if name != "" {
		nameLabel = " " + internal.BoldStyle.Render("["+name+"]")
	}

	// 1. 可选：本地构建（如 Next.js）
	localBuild := cfg.Build.LocalBuild
	if localBuild == "" {
		localBuild = internal.DetectLocalBuild()
	}
	if localBuild != "" {
		fmt.Printf("  %s 本地构建%s\n", internal.StepStyle.Render("▸"), nameLabel)
		internal.ProgressSub(localBuild)

		buildArgs := internal.LoadBuildArgs(envFile)
		envMap := make(map[string]string)
		for i := 0; i < len(buildArgs)-1; i += 2 {
			if buildArgs[i] == "--build-arg" {
				key, value, ok := strings.Cut(buildArgs[i+1], "=")
				if ok {
					envMap[key] = value
				}
			}
		}
		envMap = internal.MergeEnv(envMap, profile.Env)

		if err := internal.RunCmdWithEnv(
			internal.ShellCommandArgs(localBuild),
			localBuild,
			envMap,
		); err != nil {
			return err
		}
	}

	// 2. Docker buildx build
	buildArgs := internal.LoadBuildArgs(envFile)
	argCount := len(buildArgs) / 2
	fmt.Printf("  %s Docker 构建%s  %s\n",
		internal.StepStyle.Render("▸"),
		nameLabel,
		internal.DimStyle.Render(fmt.Sprintf("(%d build-args)", argCount)))

	tag := internal.ImageTag("latest", profile)
	internal.ProgressSub(cfg.ImageRef(tag))
	outputArgs, err := internal.BuildxOutputArgs(cfg.Build.Platforms)
	if err != nil {
		return err
	}

	args := []string{
		"docker", "buildx", "build",
		"--platform", cfg.Build.Platforms,
		"--file", cfg.Build.Dockerfile,
	}
	args = append(args, outputArgs...)
	args = append(args, buildArgs...)

	for k, v := range profile.Env {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, "--tag", cfg.ImageRef(tag), ".")

	if err := internal.RunCmd(args, cfg.ImageRef(tag)); err != nil {
		return err
	}

	return nil
}

func doGoBinaryBuild(profile internal.Profile) error {
	name := internal.FormatProfileName(profile)
	nameLabel := ""
	if name != "" {
		nameLabel = " " + internal.BoldStyle.Render("["+name+"]")
	}

	output := cfg.Build.Go.Output
	if output == "" {
		return fmt.Errorf("build.go.output 不能为空")
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
	if len(cfg.Build.Go.Ldflags) > 0 {
		args = append(args, "-ldflags", strings.Join(cfg.Build.Go.Ldflags, " "))
	}
	args = append(args, "-o", output, cfg.Build.Go.Main)

	envMap := internal.MergeEnv(nil, profile.Env)
	if cfg.Build.Go.GoOS != "" {
		envMap["GOOS"] = cfg.Build.Go.GoOS
	}
	if cfg.Build.Go.GoArch != "" {
		envMap["GOARCH"] = cfg.Build.Go.GoArch
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

func doCommandBuild(profile internal.Profile) error {
	name := internal.FormatProfileName(profile)
	nameLabel := ""
	if name != "" {
		nameLabel = " " + internal.BoldStyle.Render("["+name+"]")
	}

	fmt.Printf("  %s 自定义构建%s\n", internal.StepStyle.Render("▸"), nameLabel)
	internal.ProgressSub(cfg.Build.Command.Run)

	return internal.RunCmdWithOptions(
		internal.ShellCommandArgs(cfg.Build.Command.Run),
		fmt.Sprintf("custom build%s", nameLabel),
		cfg.Build.Command.Cwd,
		internal.MergeEnv(cfg.Build.Command.Env, profile.Env),
	)
}
