package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var placeholderPattern = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// RenderContext 提供模板插值和阶段执行所需的上下文数据。
type RenderContext struct {
	Config  *Config
	Profile Profile
	Version string
	Module  string
	Vars    map[string]string
	Env     map[string]string
}

// NewRenderContext 基于配置、profile 和版本构造统一的渲染上下文。
func NewRenderContext(cfg *Config, profile Profile, version string) RenderContext {
	vars := make(map[string]string, len(cfg.Vars)+len(profile.Vars))
	for key, value := range cfg.Vars {
		vars[key] = value
	}
	for key, value := range profile.Vars {
		vars[key] = value
	}

	env := make(map[string]string, len(profile.Env))
	for key, value := range profile.Env {
		env[key] = value
	}

	return RenderContext{
		Config:  cfg,
		Profile: profile,
		Version: version,
		Module:  detectGoModule(),
		Vars:    vars,
		Env:     env,
	}
}

// RenderString 使用当前上下文渲染单个字符串中的 {{ ... }} 占位符。
func (c RenderContext) RenderString(input string) (string, error) {
	if !strings.Contains(input, "{{") {
		return input, nil
	}

	var renderErr error
	rendered := placeholderPattern.ReplaceAllStringFunc(input, func(match string) string {
		if renderErr != nil {
			return match
		}
		parts := placeholderPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			renderErr = fmt.Errorf("无法解析模板占位符: %s", match)
			return match
		}
		value, ok := c.lookup(strings.TrimSpace(parts[1]))
		if !ok {
			renderErr = fmt.Errorf("未知模板变量: %s", strings.TrimSpace(parts[1]))
			return match
		}
		return value
	})
	if renderErr != nil {
		return "", renderErr
	}
	return rendered, nil
}

// RenderMap 渲染 map[string]string 中的所有值。
func (c RenderContext) RenderMap(input map[string]string) (map[string]string, error) {
	if len(input) == 0 {
		return map[string]string{}, nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		rendered, err := c.RenderString(value)
		if err != nil {
			return nil, err
		}
		output[key] = rendered
	}
	return output, nil
}

// RenderSlice 渲染字符串切片中的所有元素。
func (c RenderContext) RenderSlice(input []string) ([]string, error) {
	if len(input) == 0 {
		return nil, nil
	}
	output := make([]string, 0, len(input))
	for _, value := range input {
		rendered, err := c.RenderString(value)
		if err != nil {
			return nil, err
		}
		output = append(output, rendered)
	}
	return output, nil
}

// StepAppliesToProfile 返回 step 是否应在当前 profile 上执行。
func StepAppliesToProfile(step Step, profile Profile) bool {
	if step.Enabled != nil && !*step.Enabled {
		return false
	}
	return profileMatches(step.Profiles, profile)
}

// TemplateAppliesToProfile 返回模板是否应在当前 profile 上渲染。
func TemplateAppliesToProfile(spec TemplateSpec, profile Profile) bool {
	return profileMatches(spec.Profiles, profile)
}

// ExecuteSteps 执行指定阶段的 steps。
func ExecuteSteps(group string, steps []Step, cfg *Config, profile Profile, version string) error {
	_ = steps

	renderedCfg, renderedProfile, err := RenderConfigForProfile(cfg, profile, version)
	if err != nil {
		return fmt.Errorf("渲染 %s 配置失败: %w", group, err)
	}
	cfg = renderedCfg
	profile = renderedProfile

	steps, err = stepGroup(cfg, group)
	if err != nil {
		return err
	}

	ctx := NewRenderContext(cfg, profile, version)
	for _, step := range steps {
		if !StepAppliesToProfile(step, profile) {
			continue
		}

		run, err := ctx.RenderString(step.Run)
		if err != nil {
			return fmt.Errorf("渲染 %s step %q 失败: %w", group, step.Name, err)
		}
		cwd, err := ctx.RenderString(step.Cwd)
		if err != nil {
			return fmt.Errorf("渲染 %s step %q cwd 失败: %w", group, step.Name, err)
		}
		env, err := ctx.RenderMap(step.Env)
		if err != nil {
			return fmt.Errorf("渲染 %s step %q env 失败: %w", group, step.Name, err)
		}
		args, err := ShellCommandArgsWithMode(step.Shell, run)
		if err != nil {
			return err
		}

		label := step.Name
		if strings.TrimSpace(label) == "" {
			label = group
		}
		ProgressSub(fmt.Sprintf("step %s", label))
		if err := RunCmdWithOptions(args, fmt.Sprintf("step %s", label), cwd, MergeEnv(profile.Env, env)); err != nil {
			return err
		}
	}
	return nil
}

func stepGroup(cfg *Config, group string) ([]Step, error) {
	switch group {
	case "prepare":
		return cfg.Steps.Prepare, nil
	case "post_build":
		return cfg.Steps.PostBuild, nil
	case "pre_publish":
		return cfg.Steps.PrePublish, nil
	case "post_publish":
		return cfg.Steps.PostPublish, nil
	case "pre_deploy":
		return cfg.Steps.PreDeploy, nil
	case "post_deploy":
		return cfg.Steps.PostDeploy, nil
	default:
		return nil, fmt.Errorf("未知 steps group: %s", group)
	}
}

// ExecuteTemplates 渲染并写出匹配当前 profile 的模板文件。
func ExecuteTemplates(cfg *Config, profile Profile, version string) error {
	renderedCfg, renderedProfile, err := RenderConfigForProfile(cfg, profile, version)
	if err != nil {
		return fmt.Errorf("渲染 templates 配置失败: %w", err)
	}
	cfg = renderedCfg
	profile = renderedProfile

	ctx := NewRenderContext(cfg, profile, version)
	for _, spec := range cfg.Templates {
		if !TemplateAppliesToProfile(spec, profile) {
			continue
		}

		path, err := ctx.RenderString(spec.Path)
		if err != nil {
			return fmt.Errorf("渲染模板路径失败: %w", err)
		}
		content, err := loadTemplateContent(ctx, spec)
		if err != nil {
			return err
		}
		mode, err := parseFileMode(spec.Mode)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("创建模板目录失败: %w", err)
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			return fmt.Errorf("写入模板文件 %s 失败: %w", path, err)
		}
		ProgressSub(fmt.Sprintf("template %s", path))
		PrintInfo(fmt.Sprintf("已渲染模板: %s", path))
	}
	return nil
}

// ExecuteVerify 按 verify.driver 或兼容的 deploy.healthcheck 执行部署后校验。
func ExecuteVerify(cfg *Config, profile Profile, version string) error {
	renderedCfg, renderedProfile, err := RenderConfigForProfile(cfg, profile, version)
	if err != nil {
		return fmt.Errorf("渲染 verify 配置失败: %w", err)
	}
	cfg = renderedCfg
	profile = renderedProfile

	ctx := NewRenderContext(cfg, profile, version)

	switch cfg.Verify.Driver {
	case "none":
		if cfg.Deploy.Healthcheck.Enabled() {
			url, err := ctx.RenderString(cfg.Deploy.Healthcheck.URL)
			if err != nil {
				return fmt.Errorf("渲染 deploy.healthcheck.url 失败: %w", err)
			}
			if strings.TrimSpace(url) == "" {
				return fmt.Errorf("deploy.healthcheck.url 渲染结果不能为空")
			}
			healthcheck := DeployHealthcheck{
				URL:             url,
				ExpectedStatus:  cfg.Deploy.Healthcheck.ExpectedStatus,
				Attempts:        cfg.Deploy.Healthcheck.Attempts,
				IntervalSeconds: cfg.Deploy.Healthcheck.IntervalSeconds,
				TimeoutSeconds:  cfg.Deploy.Healthcheck.TimeoutSeconds,
			}
			PrintInfo(fmt.Sprintf("verify legacy healthcheck: url=%s expected=%d attempts=%d interval=%ds timeout=%ds", healthcheck.URL, healthcheck.ExpectedStatus, healthcheck.Attempts, healthcheck.IntervalSeconds, healthcheck.TimeoutSeconds))
			if err := WaitForHealthcheck(healthcheck); err != nil {
				return fmt.Errorf("legacy deploy.healthcheck 失败: %w", err)
			}
			return nil
		}
		return nil
	case "http":
		url, err := ctx.RenderString(cfg.Verify.HTTP.URL)
		if err != nil {
			return fmt.Errorf("渲染 verify.http.url 失败: %w", err)
		}
		if strings.TrimSpace(url) == "" {
			return fmt.Errorf("verify.http.url 渲染结果不能为空")
		}
		healthcheck := VerifyHTTPConfigToHealthcheck(cfg.Verify.HTTP, url)
		PrintInfo(fmt.Sprintf("verify http: url=%s expected=%d attempts=%d interval=%ds timeout=%ds", healthcheck.URL, healthcheck.ExpectedStatus, healthcheck.Attempts, healthcheck.IntervalSeconds, healthcheck.TimeoutSeconds))
		ProgressSub(fmt.Sprintf("verify http %s", url))
		if err := WaitForHealthcheck(healthcheck); err != nil {
			return fmt.Errorf("verify.http 失败: %w", err)
		}
		return nil
	case "ssh":
		host, err := ctx.RenderString(cfg.Verify.SSH.Host)
		if err != nil {
			return fmt.Errorf("渲染 verify.ssh.host 失败: %w", err)
		}
		if strings.TrimSpace(host) == "" {
			return fmt.Errorf("verify.ssh.host 渲染结果不能为空")
		}
		command, err := ctx.RenderString(cfg.Verify.SSH.Command)
		if err != nil {
			return fmt.Errorf("渲染 verify.ssh.command 失败: %w", err)
		}
		if strings.TrimSpace(command) == "" {
			return fmt.Errorf("verify.ssh.command 渲染结果不能为空")
		}
		PrintInfo(fmt.Sprintf("verify ssh: host=%s command=%s", host, command))
		ProgressSub(fmt.Sprintf("verify ssh %s", host))
		if err := RunCmd([]string{"ssh", host, command}, fmt.Sprintf("verify ssh %s", host)); err != nil {
			return fmt.Errorf("verify.ssh 失败: host=%s command=%s: %w", host, command, err)
		}
		return nil
	case "command":
		run, err := ctx.RenderString(cfg.Verify.Command.Run)
		if err != nil {
			return fmt.Errorf("渲染 verify.command.run 失败: %w", err)
		}
		if strings.TrimSpace(run) == "" {
			return fmt.Errorf("verify.command.run 渲染结果不能为空")
		}
		args, err := ShellCommandArgsWithMode(cfg.Verify.Command.Shell, run)
		if err != nil {
			return fmt.Errorf("解析 verify.command.shell 失败: %w", err)
		}
		PrintInfo(fmt.Sprintf("verify command: run=%s shell=%s", run, cfg.Verify.Command.Shell))
		ProgressSub("verify command")
		if err := RunCmd(args, "verify command"); err != nil {
			return fmt.Errorf("verify.command 失败: run=%s: %w", run, err)
		}
		return nil
	default:
		return fmt.Errorf("当前不支持的 verify.driver: %s", cfg.Verify.Driver)
	}
}

// VerifyHTTPConfigToHealthcheck 将 verify.http 配置转换为复用的健康检查配置。
func VerifyHTTPConfigToHealthcheck(cfg VerifyHTTPConfig, url string) DeployHealthcheck {
	return DeployHealthcheck{
		URL:             url,
		ExpectedStatus:  firstNonZero(cfg.ExpectedStatus, 200),
		Attempts:        firstNonZero(cfg.Attempts, 20),
		IntervalSeconds: firstNonZero(cfg.IntervalSeconds, 3),
		TimeoutSeconds:  firstNonZero(cfg.TimeoutSeconds, 5),
	}
}

// detectGoModule 从 go.mod 中读取当前模块路径。
func detectGoModule() string {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// loadTemplateContent 读取并渲染模板的最终内容。
func loadTemplateContent(ctx RenderContext, spec TemplateSpec) (string, error) {
	hasContent := strings.TrimSpace(spec.Content) != ""
	hasFrom := strings.TrimSpace(spec.From) != ""
	if hasContent == hasFrom {
		return "", fmt.Errorf("模板 %s 必须且只能设置 content 或 from 之一", spec.Path)
	}

	content := spec.Content
	if hasFrom {
		fromPath, err := ctx.RenderString(spec.From)
		if err != nil {
			return "", fmt.Errorf("渲染模板来源失败: %w", err)
		}
		data, err := os.ReadFile(fromPath)
		if err != nil {
			return "", fmt.Errorf("读取模板文件 %s 失败: %w", fromPath, err)
		}
		content = string(data)
	}

	rendered, err := ctx.RenderString(content)
	if err != nil {
		return "", fmt.Errorf("渲染模板内容失败: %w", err)
	}
	return rendered, nil
}

// parseFileMode 解析模板文件权限，空值时返回 0644。
func parseFileMode(mode string) (os.FileMode, error) {
	trimmed := strings.TrimSpace(mode)
	if trimmed == "" {
		return 0o644, nil
	}
	value, err := strconv.ParseUint(trimmed, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("解析模板文件权限 %q 失败: %w", mode, err)
	}
	return os.FileMode(value), nil
}

// lookup 根据键名返回模板变量值。
func (c RenderContext) lookup(key string) (string, bool) {
	switch key {
	case "version":
		return c.Version, true
	case "module":
		return c.Module, c.Module != ""
	case "image_name":
		return c.Config.ImageName, true
	case "project.name":
		return c.Config.Project.Name, true
	case "project.description":
		return c.Config.Project.Description, true
	case "profile.name":
		return c.Profile.Name, true
	case "profile.default":
		return strconv.FormatBool(c.Profile.Default), true
	case "build.platforms":
		return strings.Join(c.Config.Build.Docker.Platforms, ","), true
	case "build.driver":
		return c.Config.Build.Driver, true
	case "build.docker.image":
		return c.Config.Build.Docker.Image, true
	case "publish.driver":
		return c.Config.Publish.Driver, true
	case "deploy.driver":
		return c.Config.Deploy.Driver, true
	case "verify.driver":
		return c.Config.Verify.Driver, true
	case "deploy.enabled":
		return strconv.FormatBool(c.Config.Deploy.Enabled), true
	}

	if strings.HasPrefix(key, "vars.") {
		value, ok := c.Vars[strings.TrimPrefix(key, "vars.")]
		return value, ok
	}
	if strings.HasPrefix(key, "env.") {
		value, ok := c.Env[strings.TrimPrefix(key, "env.")]
		return value, ok
	}
	return "", false
}

// profileMatches 判断 profiles 选择器是否命中当前 profile。
func profileMatches(selectors []string, profile Profile) bool {
	if len(selectors) == 0 {
		return true
	}
	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		switch selector {
		case "", "*":
			return true
		case profile.Name:
			return true
		case "default":
			if profile.Default {
				return true
			}
		}
	}
	return false
}

// firstNonZero 返回 value 非零时的值，否则返回 fallback。
func firstNonZero(value, fallback int) int {
	if value != 0 {
		return value
	}
	return fallback
}
