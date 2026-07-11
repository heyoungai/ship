package internal

import "testing"

func TestRenderConfigForProfile_RendersTemplatedConfigNodes(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Project.Name = "demo"
	cfg.Project.Description = "{{ vars.description }}"
	cfg.Vars = map[string]string{
		"description":  "api service",
		"registry":     "registry.example.com",
		"namespace":    "team",
		"name_suffix":  "{{ project.name }}",
		"image_name":   "{{ vars.name_suffix }}-api",
		"tag_key":      "APP_IMAGE_TAG",
		"selector":     "brand-a",
		"compose_file": "{{ vars.image_name }}.yaml",
	}
	cfg.Build.Driver = "docker"
	cfg.Build.Docker.Image = "{{ vars.image_name }}"
	cfg.Publish.Driver = "registry"
	cfg.Publish.Registry.Targets = []Registry{{
		Type:      "private",
		URL:       "{{ vars.registry }}",
		Namespace: "{{ vars.namespace }}",
		Image:     "{{ vars.image_name }}",
	}}
	cfg.Deploy.Driver = "compose"
	cfg.Deploy.Compose.Host = "{{ vars.namespace }}-host"
	cfg.Deploy.Compose.Path = "/srv/{{ vars.image_name }}"
	cfg.Deploy.Compose.TagKey = "{{ vars.tag_key }}"
	cfg.Deploy.Compose.Up = "docker compose -f {{ vars.compose_file }} up -d"
	cfg.Verify.Driver = "ssh"
	cfg.Verify.SSH.Host = "{{ vars.namespace }}-host"
	cfg.Verify.SSH.Command = "echo {{ project.description }}"
	cfg.Steps.Prepare = []Step{{
		Run:      "echo {{ vars.image_name }}",
		Profiles: []string{"{{ vars.selector }}"},
	}}
	cfg.Templates = []TemplateSpec{{
		Path:     "./dist/{{ vars.image_name }}.env",
		Content:  "NAME={{ vars.image_name }}",
		Profiles: []string{"{{ vars.selector }}"},
	}}

	profile := Profile{
		Name: "brand-a",
		Vars: map[string]string{
			"artifact": "{{ project.name }}-artifact",
		},
		Env: map[string]string{
			"TARGET_IMAGE": "{{ vars.image_name }}",
		},
	}

	renderedCfg, renderedProfile, err := RenderConfigForProfile(cfg, profile, "v1.2.3")
	if err != nil {
		t.Fatalf("RenderConfigForProfile error: %v", err)
	}

	if cfg.Publish.Registry.Targets[0].Image != "{{ vars.image_name }}" {
		t.Fatal("RenderConfigForProfile should not mutate the source config")
	}
	if renderedCfg.Project.Description != "api service" {
		t.Fatalf("Project.Description = %q, want api service", renderedCfg.Project.Description)
	}
	if renderedCfg.Build.Docker.Image != "demo-api" {
		t.Fatalf("Build.Docker.Image = %q, want demo-api", renderedCfg.Build.Docker.Image)
	}
	if renderedCfg.ImageName != "demo-api" {
		t.Fatalf("ImageName = %q, want demo-api", renderedCfg.ImageName)
	}
	if renderedCfg.Publish.Registry.Targets[0].URL != "registry.example.com" {
		t.Fatalf("Publish.Registry.Targets[0].URL = %q", renderedCfg.Publish.Registry.Targets[0].URL)
	}
	if renderedCfg.Publish.Registry.Targets[0].Namespace != "team" {
		t.Fatalf("Publish.Registry.Targets[0].Namespace = %q", renderedCfg.Publish.Registry.Targets[0].Namespace)
	}
	if renderedCfg.Publish.Registry.Targets[0].Image != "demo-api" {
		t.Fatalf("Publish.Registry.Targets[0].Image = %q", renderedCfg.Publish.Registry.Targets[0].Image)
	}
	if len(renderedCfg.Registries) != 1 || renderedCfg.Registries[0].Image != "demo-api" {
		t.Fatalf("Registries = %#v, want rendered registry target", renderedCfg.Registries)
	}
	if renderedCfg.Deploy.Compose.Up != "docker compose -f demo-api.yaml up -d" {
		t.Fatalf("Deploy.Compose.Up = %q", renderedCfg.Deploy.Compose.Up)
	}
	if renderedCfg.Steps.Prepare[0].Profiles[0] != "brand-a" {
		t.Fatalf("Steps.Prepare[0].Profiles[0] = %q", renderedCfg.Steps.Prepare[0].Profiles[0])
	}
	if renderedCfg.Templates[0].Path != "./dist/demo-api.env" {
		t.Fatalf("Templates[0].Path = %q", renderedCfg.Templates[0].Path)
	}
	if renderedCfg.Verify.SSH.Command != "echo api service" {
		t.Fatalf("Verify.SSH.Command = %q", renderedCfg.Verify.SSH.Command)
	}
	if renderedProfile.Vars["artifact"] != "demo-artifact" {
		t.Fatalf("profile vars artifact = %q", renderedProfile.Vars["artifact"])
	}
	if renderedProfile.Env["TARGET_IMAGE"] != "demo-api" {
		t.Fatalf("profile env TARGET_IMAGE = %q", renderedProfile.Env["TARGET_IMAGE"])
	}
}
