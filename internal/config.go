package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Registry 定义镜像仓库配置。
type Registry struct {
	Type      string `toml:"type"`
	URL       string `toml:"url"`
	Namespace string `toml:"namespace"`
	Image     string `toml:"image"`
}

// Profile 定义矩阵构建变体（如多品牌）。
type Profile struct {
	Name    string            `toml:"name"`
	Default bool              `toml:"default"`
	Env     map[string]string `toml:"env"`
	Vars    map[string]string `toml:"vars"`
}

// ProjectConfig 定义项目元信息。
type ProjectConfig struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

// VersionConfig 定义版本解析策略。
type VersionConfig struct {
	Source      string `toml:"source"`
	Fallback    string `toml:"fallback"`
	Static      string `toml:"static"`
	OverrideEnv string `toml:"override_env"`
}

// FeatureConfig 定义功能开关。
type FeatureConfig struct {
	Publish  bool `toml:"publish"`
	Deploy   bool `toml:"deploy"`
	Rollback bool `toml:"rollback"`
	Verify   bool `toml:"verify"`
}

// Step 定义 prepare / pre_deploy 等可选步骤。
type Step struct {
	Name     string            `toml:"name"`
	Run      string            `toml:"run"`
	Cwd      string            `toml:"cwd"`
	Shell    string            `toml:"shell"`
	Env      map[string]string `toml:"env"`
	Profiles []string          `toml:"profiles"`
	Enabled  *bool             `toml:"enabled"`
}

// StepsConfig 定义阶段前后的可选步骤集合。
type StepsConfig struct {
	Prepare     []Step `toml:"prepare"`
	PostBuild   []Step `toml:"post_build"`
	PrePublish  []Step `toml:"pre_publish"`
	PostPublish []Step `toml:"post_publish"`
	PreDeploy   []Step `toml:"pre_deploy"`
	PostDeploy  []Step `toml:"post_deploy"`
}

// TemplateSpec 定义渲染生成文件。
type TemplateSpec struct {
	Path     string   `toml:"path"`
	Content  string   `toml:"content"`
	From     string   `toml:"from"`
	Mode     string   `toml:"mode"`
	Profiles []string `toml:"profiles"`
}

// BuildDockerConfig 定义 Docker 构建细节。
type BuildDockerConfig struct {
	Image                  string            `toml:"image"`
	Context                string            `toml:"context"`
	Dockerfile             string            `toml:"dockerfile"`
	Platforms              []string          `toml:"platforms"`
	EnvFile                string            `toml:"env_file"`
	LocalBuild             string            `toml:"local_build"`
	BuildArgs              map[string]string `toml:"build_args"`
	BuildArgsFromEnv       bool              `toml:"build_args_from_env"`
	Load                   bool              `toml:"load"`
	LatestOnDefaultProfile bool              `toml:"latest_on_default_profile"`
	DisableBuildkit        bool              `toml:"disable_buildkit"`
	CacheBust              bool              `toml:"cache_bust"`
}

// BuildGoBinaryConfig 定义 Go 二进制构建细节。
type BuildGoBinaryConfig struct {
	Main           string   `toml:"main"`
	Output         string   `toml:"output"`
	GoOS           string   `toml:"goos"`
	GoArch         string   `toml:"goarch"`
	CGOEnabled     bool     `toml:"cgo_enabled"`
	ExecutableName string   `toml:"executable_name"`
	Ldflags        []string `toml:"ldflags"`
}

// BuildCommandConfig 定义纯命令构建细节。
type BuildCommandConfig struct {
	Run     string            `toml:"run"`
	Cwd     string            `toml:"cwd"`
	Outputs []string          `toml:"outputs"`
	Env     map[string]string `toml:"env"`
}

// BuildConfig 兼容 v1/v2 的构建配置。
type BuildConfig struct {
	Driver string `toml:"driver"`

	// 运行时派生字段，供当前命令层继续复用
	Platforms  string `toml:"-"`
	Dockerfile string `toml:"-"`
	EnvFile    string `toml:"-"`
	LocalBuild string `toml:"-"`

	// v2 driver 配置
	Docker  BuildDockerConfig   `toml:"docker"`
	Go      BuildGoBinaryConfig `toml:"go"`
	Command BuildCommandConfig  `toml:"command"`
}

// PublishRegistryConfig 定义 registry 发布配置。
type PublishRegistryConfig struct {
	Push                      bool       `toml:"push"`
	TagLatestOnDefaultProfile bool       `toml:"tag_latest_on_default_profile"`
	Targets                   []Registry `toml:"targets"`
}

// PublishSCPConfig 定义 scp 发布配置。
type PublishSCPConfig struct {
	Local  string `toml:"local"`
	Host   string `toml:"host"`
	Remote string `toml:"remote"`
}

// PublishConfig 定义发布阶段。
type PublishConfig struct {
	Driver   string                `toml:"driver"`
	Registry PublishRegistryConfig `toml:"registry"`
	SCP      PublishSCPConfig      `toml:"scp"`
}

// DeployComposeConfig 定义 compose 部署配置。
type DeployComposeConfig struct {
	Host         string `toml:"host"`
	Path         string `toml:"path"`
	LocalFile    string `toml:"local_file"`
	RemoteFile   string `toml:"remote_file"`
	LocalEnvFile string `toml:"local_env_file"`
	EnvFile      string `toml:"env_file"`
	AutoEnvFile  bool   `toml:"auto_env_file"` // 当 env_file 非默认值时自动注入 --env-file 到 up 命令
	TagKey       string `toml:"tag_key"`
	Up           string `toml:"up"`
}

// DeployBinaryInstallConfig 定义二进制安装部署配置。
type DeployBinaryInstallConfig struct {
	Host              string `toml:"host"`
	RemoteTempPath    string `toml:"remote_temp_path"`
	RemoteInstallPath string `toml:"remote_install_path"`
	UseSSHTTY         bool   `toml:"use_ssh_tty"`
	SudoNoPasswd      bool   `toml:"sudo_nopasswd"`
	Chmod             string `toml:"chmod"`
}

// DeploySSHConfig 定义自定义 SSH 部署配置。
type DeploySSHConfig struct {
	Host     string   `toml:"host"`
	Commands []string `toml:"commands"`
}

// DeployConfig 兼容 v1/v2 的部署配置。
type DeployConfig struct {
	Driver string `toml:"driver"`

	// 运行时派生字段，供当前命令层继续复用
	Enabled bool   `toml:"-"`
	Host    string `toml:"-"`
	Path    string `toml:"-"`

	Healthcheck   DeployHealthcheck         `toml:"healthcheck"`
	Compose       DeployComposeConfig       `toml:"compose"`
	BinaryInstall DeployBinaryInstallConfig `toml:"binary_install"`
	SSH           DeploySSHConfig           `toml:"ssh"`
}

// VerifyHTTPConfig 定义 HTTP 校验。
type VerifyHTTPConfig struct {
	URL             string `toml:"url"`
	ExpectedStatus  int    `toml:"expected_status"`
	Attempts        int    `toml:"attempts"`
	IntervalSeconds int    `toml:"interval_seconds"`
	TimeoutSeconds  int    `toml:"timeout_seconds"`
}

// VerifySSHConfig 定义 SSH 校验。
type VerifySSHConfig struct {
	Host    string `toml:"host"`
	Command string `toml:"command"`
}

// VerifyCommandConfig 定义本地命令校验。
type VerifyCommandConfig struct {
	Run   string `toml:"run"`
	Shell string `toml:"shell"`
}

// VerifyConfig 定义部署后校验。
type VerifyConfig struct {
	Driver  string              `toml:"driver"`
	HTTP    VerifyHTTPConfig    `toml:"http"`
	SSH     VerifySSHConfig     `toml:"ssh"`
	Command VerifyCommandConfig `toml:"command"`
}

// Config 定义完整配置。
type Config struct {
	Schema int `toml:"schema"`

	Runtime RuntimeOptions `toml:"config"`

	Project   ProjectConfig     `toml:"project"`
	Version   VersionConfig     `toml:"version"`
	Features  FeatureConfig     `toml:"features"`
	Vars      map[string]string `toml:"vars"`
	Steps     StepsConfig       `toml:"steps"`
	Templates []TemplateSpec    `toml:"templates"`

	Build   BuildConfig   `toml:"build"`
	Publish PublishConfig `toml:"publish"`
	Deploy  DeployConfig  `toml:"deploy"`
	Verify  VerifyConfig  `toml:"verify"`

	Matrix []Profile `toml:"matrix"`

	// 运行时派生字段，供当前命令层继续复用
	Registries []Registry `toml:"-"`

	// 运行时填充，不来自 TOML
	ImageName string `toml:"-"`
}

func (c *Config) applyDefaults() {
	// v2 默认值
	c.Version.Source = "git-tag"
	c.Version.Fallback = "error"
	c.Version.OverrideEnv = "SHIP_VERSION"

	c.Features.Publish = true
	c.Features.Deploy = true
	c.Features.Rollback = true
	c.Features.Verify = true

	c.Build.Driver = "docker"
	c.Build.Docker.Context = "."
	c.Build.Docker.Dockerfile = "./Dockerfile"
	c.Build.Docker.Platforms = []string{"linux/amd64"}
	c.Build.Docker.EnvFile = "./.env.local"
	c.Build.Docker.Load = true
	c.Build.Docker.BuildArgsFromEnv = true
	c.Build.Docker.LatestOnDefaultProfile = true

	c.Publish.Driver = "registry"
	c.Publish.Registry.Push = true
	c.Publish.Registry.TagLatestOnDefaultProfile = true

	c.Deploy.Driver = "none"
	c.Deploy.Compose.EnvFile = ".env"
	c.Deploy.Compose.AutoEnvFile = true
	c.Deploy.Compose.TagKey = "APP_IMAGE_TAG"
	c.Deploy.Compose.Up = "docker compose up -d"
	c.Deploy.Healthcheck.ApplyDefaults()

	c.Verify.Driver = "none"
	c.Verify.HTTP.ExpectedStatus = 200
	c.Verify.HTTP.Attempts = 20
	c.Verify.HTTP.IntervalSeconds = 3
	c.Verify.HTTP.TimeoutSeconds = 5
}

// DefaultProfile 返回默认 profile（无 matrix 时的单次构建）。
func (c *Config) DefaultProfile() Profile {
	for _, p := range c.Matrix {
		if p.Default {
			return p
		}
	}
	return Profile{Name: "", Default: true}
}

// GetProfiles 获取要构建的 profile 列表。
// name 为空时返回所有 profile（无 matrix 则返回单个默认 profile）。
func (c *Config) GetProfiles(name string) ([]Profile, error) {
	if len(c.Matrix) == 0 {
		if name != "" {
			return nil, fmt.Errorf("当前项目未配置 matrix，不能选择 profile: %s", name)
		}
		return []Profile{{Name: "", Default: true}}, nil
	}
	if name != "" {
		for _, p := range c.Matrix {
			if p.Name == name {
				return []Profile{p}, nil
			}
		}
		return nil, fmt.Errorf("未找到 profile: %s", name)
	}
	return c.Matrix, nil
}

// ImageRef 生成镜像引用（image:tag）。
func (c *Config) ImageRef(tag string) string {
	return fmt.Sprintf("%s:%s", c.ImageName, tag)
}

// ImageTag 生成带 profile 后缀的 tag。
// 无名或 default profile 不加后缀，如 v2.0.0。
// 其他 profile 加后缀，如 v2.0.0-brand-a / v2.0.0-probe。
func ImageTag(version string, profile Profile) string {
	if profile.Name == "" || profile.Default {
		return version
	}
	return fmt.Sprintf("%s-%s", version, profile.Name)
}

// BuildSourceTag 返回 docker build 阶段产出的本地镜像 tag（无 run ID 时的兼容回退）。
// 正式流水线应使用 BuildSourceTagForRun，避免并发 run 抢占共享 latest。
func (c *Config) BuildSourceTag(profile Profile) string {
	return c.BuildSourceTagForRun("", profile)
}

// BuildSourceTagForRun 返回带 run ID 的本地临时镜像 tag：ship-build-<run-id>-<profile>。
// runID 为空时回退到历史行为（latest / build-default / latest-<name>），供兼容测试使用。
func (c *Config) BuildSourceTagForRun(runID string, profile Profile) string {
	if strings.TrimSpace(runID) != "" {
		name := profile.Name
		if name == "" {
			name = "default"
		}
		return fmt.Sprintf("ship-build-%s-%s", runID, sanitizeDockerTagPart(name))
	}
	if profile.Name == "" {
		if c.Build.Docker.LatestOnDefaultProfile {
			return "latest"
		}
		return "build-default"
	}
	return fmt.Sprintf("latest-%s", profile.Name)
}

func sanitizeDockerTagPart(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// RegistryTargets 生成注册表镜像引用列表。
func (c *Config) RegistryTargets(tag string) []string {
	var targets []string
	for _, reg := range c.Registries {
		var ref string
		if reg.Type == "dockerhub" {
			ref = fmt.Sprintf("%s/%s:%s", reg.Namespace, reg.Image, tag)
		} else {
			ref = fmt.Sprintf("%s/%s/%s:%s", reg.URL, reg.Namespace, reg.Image, tag)
		}
		targets = append(targets, ref)
	}
	return targets
}

// LoadBuildArgs 读取 .env 文件，返回 --build-arg 列表。
func LoadBuildArgs(envFile string) []string {
	if envFile == "" {
		return nil
	}

	path := filepath.Clean(envFile)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("  %s 未找到 %s，将不注入 build args\n", InfoStyle.Render("▸"), envFile)
		return nil
	}

	envMap, err := godotenv.Read(path)
	if err != nil {
		fmt.Printf("  %s 读取 %s 失败: %v\n", ErrorStyle.Render("✖"), envFile, err)
		return nil
	}

	var args []string
	for k, v := range envMap {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	return args
}

// DetectLocalBuild 根据锁文件推断本地构建命令。
func DetectLocalBuild() string {
	for _, pair := range [][2]string{
		{"bun.lock", "bun run build"},
		{"yarn.lock", "yarn build"},
		{"pnpm-lock.yaml", "pnpm run build"},
		{"package-lock.json", "npm run build"},
	} {
		if _, err := os.Stat(pair[0]); err == nil {
			return pair[1]
		}
	}
	return ""
}

// MergeEnv 合并环境变量：基础 env + profile env。
func MergeEnv(base map[string]string, profileEnv map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(profileEnv))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range profileEnv {
		merged[k] = v
	}
	return merged
}

// EnvToSlice 将 map 转为 "KEY=VALUE" 字符串切片。
func EnvToSlice(envMap map[string]string) []string {
	s := make([]string, 0, len(envMap))
	for k, v := range envMap {
		s = append(s, fmt.Sprintf("%s=%s", k, v))
	}
	return s
}

// FormatProfileName 格式化 profile 名称用于显示。
// 无 matrix 时返回空字符串，有 matrix 时返回 profile 名。
func FormatProfileName(p Profile) string {
	return p.Name
}

// UsesTagStage 返回当前配置是否需要独立 tag 阶段。
func (c *Config) UsesTagStage() bool {
	return c.Build.Driver == "docker" && c.Features.Publish && c.Publish.Driver == "registry"
}

// UsesPublishStage 返回当前配置是否需要发布阶段。
func (c *Config) UsesPublishStage() bool {
	return c.Features.Publish && c.Publish.Driver != "none"
}

// UsesDeployStage 返回当前配置是否需要部署阶段。
func (c *Config) UsesDeployStage() bool {
	return c.Features.Deploy && c.Deploy.Driver != "none"
}

// UsesVerifyStage 返回当前配置是否需要 verify 阶段。
func (c *Config) UsesVerifyStage() bool {
	return (c.Features.Verify && c.Verify.Driver != "none") || c.Deploy.Healthcheck.Enabled()
}

// ShouldTagLatest 返回当前 profile 是否需要额外维护远端 latest 标签。
func (c *Config) ShouldTagLatest(profile Profile) bool {
	return profile.Default && c.Publish.Registry.TagLatestOnDefaultProfile
}

// StringSliceContains 检查字符串切片是否包含指定元素。
func StringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
