package internal

import (
	"encoding/json"
	"fmt"
	"strings"
)

const ReleasePlanSchema = 1

// ExpectedArtifact 描述 Plan 预期产出的产物。
type ExpectedArtifact struct {
	Type     string `json:"type"`
	Profile  string `json:"profile,omitempty"`
	Platform string `json:"platform,omitempty"`
	RefHint  string `json:"ref_hint,omitempty"`
}

// PlanOptions 控制 CompileReleasePlan 的阶段开关。
type PlanOptions struct {
	ProfileFilter string
	SkipDeploy    bool
	SkipPublish   bool
	EnvFile       string
}

// ReleasePlan 是 ship plan / doctor / run 共享的轻量执行计划。
type ReleasePlan struct {
	Schema    int                `json:"schema"`
	RunID     string             `json:"run_id"`
	Identity  ReleaseIdentity    `json:"identity"`
	Profiles  []string           `json:"profiles"`
	Stages    []string           `json:"stages"`
	Roots     ExecutionRoots     `json:"roots"`
	Expected  []ExpectedArtifact `json:"expected_artifacts"`
	EnvFile   string             `json:"env_file,omitempty"`
	SkipDeploy bool              `json:"skip_deploy,omitempty"`
}

// CompileReleasePlan 根据配置与 identity 编译执行计划（不产生副作用）。
func CompileReleasePlan(cfg *Config, identity ReleaseIdentity, roots ExecutionRoots, opts PlanOptions) (*ReleasePlan, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config 不能为空")
	}
	profiles, err := cfg.GetProfiles(opts.ProfileFilter)
	if err != nil {
		return nil, err
	}
	profileNames := make([]string, 0, len(profiles))
	for _, p := range profiles {
		name := FormatProfileName(p)
		if name == "" {
			name = "default"
		}
		profileNames = append(profileNames, name)
	}

	stages := []string{"build"}
	if cfg.UsesTagStage() {
		stages = append(stages, "tag")
	}
	shouldPublish := cfg.UsesPublishStage() && !opts.SkipPublish
	if shouldPublish {
		stages = append(stages, "publish")
	}
	shouldDeploy := !opts.SkipDeploy && cfg.UsesDeployStage()
	if shouldDeploy {
		stages = append(stages, "deploy")
	}
	if !opts.SkipDeploy && cfg.UsesVerifyStage() {
		stages = append(stages, "verify")
	}

	expected := make([]ExpectedArtifact, 0, len(profiles))
	for _, p := range profiles {
		switch cfg.Build.Driver {
		case "docker":
			remoteTag := ImageTag(identity.Version, p)
			refs := cfg.RegistryTargets(remoteTag)
			refHint := ""
			if len(refs) > 0 {
				refHint = refs[0]
			} else {
				refHint = cfg.ImageRef(cfg.BuildSourceTagForRun(roots.RunID, p))
			}
			platform := cfg.Build.Platforms
			if platform == "" && len(cfg.Build.Docker.Platforms) > 0 {
				platform = strings.Join(cfg.Build.Docker.Platforms, ",")
			}
			expected = append(expected, ExpectedArtifact{
				Type:     ArtifactTypeImage,
				Profile:  profileNameOrDefault(p),
				Platform: platform,
				RefHint:  refHint,
			})
		case "go-binary":
			expected = append(expected, ExpectedArtifact{
				Type:    ArtifactTypeBinary,
				Profile: profileNameOrDefault(p),
			})
		default:
			expected = append(expected, ExpectedArtifact{
				Type:    ArtifactTypeFile,
				Profile: profileNameOrDefault(p),
			})
		}
	}

	return &ReleasePlan{
		Schema:     ReleasePlanSchema,
		RunID:      roots.RunID,
		Identity:   identity,
		Profiles:   profileNames,
		Stages:     stages,
		Roots:      roots,
		Expected:   expected,
		EnvFile:    opts.EnvFile,
		SkipDeploy: opts.SkipDeploy,
	}, nil
}

func profileNameOrDefault(p Profile) string {
	if p.Name == "" {
		return "default"
	}
	return p.Name
}

// PrintReleasePlan 以人类可读形式打印计划。
func PrintReleasePlan(plan *ReleasePlan) {
	if plan == nil {
		return
	}
	PrintBanner(fmt.Sprintf("ship plan  version=%s  run_id=%s", plan.Identity.Version, plan.RunID))
	PrintReleaseIdentity(plan.Identity)
	PrintInfo(fmt.Sprintf("profiles=%s", strings.Join(plan.Profiles, ", ")))
	PrintInfo(fmt.Sprintf("stages=%s", strings.Join(plan.Stages, " → ")))
	PrintInfo(fmt.Sprintf("invocation_root=%s", plan.Roots.InvocationRoot))
	PrintInfo(fmt.Sprintf("source_root=%s", plan.Roots.SourceRoot))
	PrintInfo(fmt.Sprintf("state_root=%s", plan.Roots.StateRoot))
	if plan.EnvFile != "" {
		PrintInfo(fmt.Sprintf("env_file=%s", plan.EnvFile))
	}
	for _, a := range plan.Expected {
		PrintInfo(fmt.Sprintf("expected: type=%s profile=%s platform=%s ref=%s",
			a.Type, a.Profile, a.Platform, a.RefHint))
	}
}

// ReleasePlanJSON 返回 plan 的 JSON 字节。
func ReleasePlanJSON(plan *ReleasePlan) ([]byte, error) {
	return json.MarshalIndent(plan, "", "  ")
}
