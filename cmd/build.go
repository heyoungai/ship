package cmd

import (
	"fmt"
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
	Short: "构建 Docker 镜像",
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

// doBuild 执行单个 profile 的 Docker 镜像构建
func doBuild(profile internal.Profile, envFile string) error {
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
