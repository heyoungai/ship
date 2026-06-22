package cmd

import "github.com/heyoungai/ship/internal"

// executeBuildProfile 执行单个 profile 的 prepare → templates → build → post_build。
func executeBuildProfile(version string, profile internal.Profile, envFile string) error {
	if err := internal.ExecuteSteps("prepare", cfg.Steps.Prepare, cfg, profile, version); err != nil {
		return err
	}
	if err := internal.ExecuteTemplates(cfg, profile, version); err != nil {
		return err
	}
	if err := doBuild(profile, envFile, version); err != nil {
		return err
	}
	return internal.ExecuteSteps("post_build", cfg.Steps.PostBuild, cfg, profile, version)
}

// executePublishProfile 执行单个 profile 的 pre_publish → publish → post_publish。
func executePublishProfile(version string, profile internal.Profile) error {
	if err := internal.ExecuteSteps("pre_publish", cfg.Steps.PrePublish, cfg, profile, version); err != nil {
		return err
	}
	if err := doPush(version, profile); err != nil {
		return err
	}
	return internal.ExecuteSteps("post_publish", cfg.Steps.PostPublish, cfg, profile, version)
}

// executeDeployStage 执行 pre_deploy → deploy → post_deploy。
func executeDeployStage(version string, profile internal.Profile) error {
	if err := internal.ExecuteSteps("pre_deploy", cfg.Steps.PreDeploy, cfg, profile, version); err != nil {
		return err
	}
	if err := doDeploy(version, profile); err != nil {
		return err
	}
	return internal.ExecuteSteps("post_deploy", cfg.Steps.PostDeploy, cfg, profile, version)
}

// selectDeployProfile 选择 deploy/verify 阶段使用的 profile 上下文。
func selectDeployProfile(profiles []internal.Profile) internal.Profile {
	if len(profiles) == 1 {
		return profiles[0]
	}
	for _, profile := range profiles {
		if profile.Default {
			return profile
		}
	}
	if len(profiles) > 0 {
		return profiles[0]
	}
	return cfg.DefaultProfile()
}
