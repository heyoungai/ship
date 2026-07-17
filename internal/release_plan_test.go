package internal

import (
	"strings"
	"testing"
)

func TestCompileReleasePlan_DockerStages(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Build.Driver = "docker"
	cfg.Build.Docker.Image = "home"
	cfg.ImageName = "home"
	cfg.Features.Publish = true
	cfg.Features.Deploy = true
	cfg.Publish.Driver = "registry"
	cfg.Publish.Registry.Push = true
	cfg.Deploy.Driver = "compose"
	cfg.Registries = []Registry{{
		Type: "private", URL: "reg.example.com", Namespace: "ns", Image: "home",
	}}

	identity := ReleaseIdentity{
		Version:      "v1.0.0",
		SourceMode:   SourceModeGitTag,
		SourceRef:    "refs/tags/v1.0.0",
		SourceCommit: "abc",
	}
	roots := ExecutionRoots{RunID: "rid", InvocationRoot: "/inv", SourceRoot: "/src", StateRoot: "/inv/.ship"}

	plan, err := CompileReleasePlan(cfg, identity, roots, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Stages, ",")
	if !strings.Contains(joined, "build") || !strings.Contains(joined, "tag") ||
		!strings.Contains(joined, "publish") || !strings.Contains(joined, "deploy") {
		t.Fatalf("stages=%v", plan.Stages)
	}
	if len(plan.Expected) != 1 || plan.Expected[0].Type != ArtifactTypeImage {
		t.Fatalf("expected=%+v", plan.Expected)
	}
}

func TestCompileReleasePlan_SkipDeploy(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Build.Driver = "docker"
	cfg.Build.Docker.Image = "home"
	cfg.ImageName = "home"
	cfg.Features.Deploy = true
	cfg.Deploy.Driver = "compose"

	plan, err := CompileReleasePlan(cfg, ReleaseIdentity{Version: "v1"}, ExecutionRoots{RunID: "r"}, PlanOptions{SkipDeploy: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range plan.Stages {
		if s == "deploy" || s == "verify" {
			t.Fatalf("should skip deploy/verify, got %v", plan.Stages)
		}
	}
}
