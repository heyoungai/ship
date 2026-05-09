package cmd

import (
	"fmt"
	"ship/internal"

	"github.com/spf13/cobra"
)

var buildEnvFile string

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "构建 Docker 镜像",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles := cfg.GetProfiles("")
		for _, p := range profiles {
			if err := doBuild(p, buildEnvFile); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	buildCmd.Flags().StringVar(&buildEnvFile, "env-file", "", ".env 文件路径 (默认使用配置)")
}

// doBuild 执行单个 profile 的 Docker 镜像构建
func doBuild(profile internal.Profile, envFile string) error {
	if envFile == "" {
		envFile = cfg.Build.EnvFile
	}

	name := internal.FormatProfileName(profile)

	// 1. 可选：本地构建（如 Next.js）
	localBuild := cfg.Build.LocalBuild
	if localBuild == "" {
		localBuild = internal.DetectLocalBuild()
	}
	if localBuild != "" {
		fmt.Printf("  %s 本地构建 %s\n", internal.StepStyle.Render("▸"), internal.BoldStyle.Render("["+name+"]"))

		// 加载 .env + profile env
		buildArgs := internal.LoadBuildArgs(envFile)
		envMap := make(map[string]string)
		for i := 0; i < len(buildArgs)-1; i += 2 {
			if buildArgs[i] == "--build-arg" {
				parts := splitFirst(buildArgs[i+1], "=")
				if len(parts) == 2 {
					envMap[parts[0]] = parts[1]
				}
			}
		}
		envMap = internal.MergeEnv(envMap, profile.Env)

		if err := internal.RunCmdWithEnv(
			[]string{"sh", "-c", localBuild},
			fmt.Sprintf("本地构建 [%s]", name),
			envMap,
		); err != nil {
			return err
		}
	}

	// 2. Docker buildx build
	buildArgs := internal.LoadBuildArgs(envFile)
	argCount := len(buildArgs) / 2
	fmt.Printf("  %s 构建镜像 %s  %s\n",
		internal.StepStyle.Render("▸"),
		internal.BoldStyle.Render("["+name+"]"),
		internal.DimStyle.Render(fmt.Sprintf("(%d build-args)", argCount)))

	tag := internal.ImageTag("latest", profile)
	args := []string{
		"docker", "buildx", "build",
		"--platform", cfg.Build.Platforms,
		"--file", cfg.Build.Dockerfile,
	}
	args = append(args, buildArgs...)

	// 注入 profile env 作为 build-arg
	for k, v := range profile.Env {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, "--tag", cfg.ImageRef(tag), ".")

	if err := internal.RunCmd(args, fmt.Sprintf("构建 [%s]", name)); err != nil {
		return err
	}

	fmt.Printf("  %s %s\n", internal.SuccessStyle.Render("✔"), cfg.ImageRef(tag))
	return nil
}

// splitFirst 按第一个分隔符分割字符串
func splitFirst(s, sep string) []string {
	idx := -1
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			idx = i
			break
		}
	}
	if idx < 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}
