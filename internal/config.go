package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
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

// LoadConfig 加载配置：默认值 → ship.toml → 环境变量覆盖 → 归一化 → 校验。
func LoadConfig(imageName string) (*Config, error) {
	cfg := &Config{ImageName: imageName}
	cfg.applyDefaults()

	if _, err := os.Stat("ship.toml"); err == nil {
		if _, err := toml.DecodeFile("ship.toml", cfg); err != nil {
			return nil, fmt.Errorf("读取 ship.toml 失败: %w", err)
		}
	}

	if v := os.Getenv("PLATFORMS"); v != "" {
		cfg.Build.Docker.Platforms = SplitCSV(v)
	}
	if v := os.Getenv("DOCKERFILE"); v != "" {
		cfg.Build.Docker.Dockerfile = v
	}
	if v := os.Getenv("IMAGE_NAME"); v != "" {
		cfg.Build.Docker.Image = v
		cfg.ImageName = v
	}
	if v := os.Getenv("REMOTE_HOST"); v != "" {
		cfg.Deploy.Compose.Host = v
	}
	if v := os.Getenv("REMOTE_PROJECT_PATH"); v != "" {
		cfg.Deploy.Compose.Path = v
	}
	if v := os.Getenv("ENV_FILE"); v != "" {
		cfg.Build.Docker.EnvFile = v
	}

	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
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

func (c *Config) normalize() {
	c.Deploy.Healthcheck.ApplyDefaults()
	c.normalizeV2()
}

func (c *Config) normalizeV2() {
	switch {
	case c.Build.Go.Main != "" || c.Build.Go.Output != "":
		c.Build.Driver = "go-binary"
	case c.Build.Command.Run != "":
		c.Build.Driver = "command"
	case c.Build.Driver == "":
		c.Build.Driver = "docker"
	}

	if len(c.Build.Docker.Platforms) == 0 && c.Build.Platforms != "" {
		c.Build.Docker.Platforms = SplitCSV(c.Build.Platforms)
	}
	if len(c.Build.Docker.Platforms) == 0 {
		c.Build.Docker.Platforms = []string{"linux/amd64"}
	}
	if c.Build.Docker.Context == "" {
		c.Build.Docker.Context = "."
	}
	if c.Build.Docker.Dockerfile == "" {
		c.Build.Docker.Dockerfile = c.Build.Dockerfile
	}
	if c.Build.Docker.Dockerfile == "" {
		c.Build.Docker.Dockerfile = "./Dockerfile"
	}
	if c.Build.Docker.EnvFile == "" {
		c.Build.Docker.EnvFile = c.Build.EnvFile
	}
	if c.Build.Docker.EnvFile == "" {
		c.Build.Docker.EnvFile = "./.env.local"
	}
	if c.Build.Docker.LocalBuild == "" {
		c.Build.Docker.LocalBuild = c.Build.LocalBuild
	}

	switch {
	case c.Publish.SCP.Local != "" || c.Publish.SCP.Host != "" || c.Publish.SCP.Remote != "":
		c.Publish.Driver = "scp"
	case len(c.Publish.Registry.Targets) > 0:
		c.Publish.Driver = "registry"
	case c.Publish.Driver == "":
		c.Publish.Driver = "none"
	}

	if c.Deploy.Compose.Host == "" {
		c.Deploy.Compose.Host = c.Deploy.Host
	}
	if c.Deploy.Compose.Path == "" {
		c.Deploy.Compose.Path = c.Deploy.Path
	}
	if c.Deploy.Host == "" {
		c.Deploy.Host = c.Deploy.Compose.Host
	}
	if c.Deploy.Path == "" {
		c.Deploy.Path = c.Deploy.Compose.Path
	}
	if c.Deploy.Compose.EnvFile == "" {
		c.Deploy.Compose.EnvFile = ".env"
	}
	if c.Deploy.Compose.TagKey == "" {
		c.Deploy.Compose.TagKey = "APP_IMAGE_TAG"
	}
	if c.Deploy.Compose.Up == "" {
		c.Deploy.Compose.Up = "docker compose up -d"
	}

	switch {
	case c.Deploy.BinaryInstall.Host != "" || c.Deploy.BinaryInstall.RemoteInstallPath != "":
		c.Deploy.Driver = "binary-install"
	case c.Deploy.SSH.Host != "" || len(c.Deploy.SSH.Commands) > 0:
		c.Deploy.Driver = "ssh"
	case c.Deploy.Compose.Host != "" || c.Deploy.Compose.Path != "" || c.Deploy.Enabled:
		c.Deploy.Driver = "compose"
	case c.Deploy.Driver == "":
		c.Deploy.Driver = "none"
	}

	switch {
	case c.Verify.Command.Run != "":
		c.Verify.Driver = "command"
	case c.Verify.SSH.Host != "" || c.Verify.SSH.Command != "":
		c.Verify.Driver = "ssh"
	case c.Verify.HTTP.URL != "":
		c.Verify.Driver = "http"
	case c.Verify.Driver == "":
		c.Verify.Driver = "none"
	}

	if c.Build.Driver == "docker" {
		c.Build.Platforms = strings.Join(c.Build.Docker.Platforms, ",")
		c.Build.Dockerfile = c.Build.Docker.Dockerfile
		c.Build.EnvFile = c.Build.Docker.EnvFile
		c.Build.LocalBuild = c.Build.Docker.LocalBuild
		if c.Build.Docker.Image != "" {
			c.ImageName = c.Build.Docker.Image
		}
	}

	if c.ImageName == "" && len(c.Publish.Registry.Targets) > 0 {
		c.ImageName = c.Publish.Registry.Targets[0].Image
	}
	if c.Build.Driver == "docker" && c.Build.Docker.Image == "" {
		c.Build.Docker.Image = c.ImageName
	}
	if c.Build.Driver == "go-binary" && c.Build.Go.ExecutableName == "" && c.Build.Go.Output != "" {
		c.Build.Go.ExecutableName = filepath.Base(c.Build.Go.Output)
	}

	if c.Publish.Driver == "scp" && c.Publish.SCP.Local == "" && c.Build.Driver == "go-binary" {
		c.Publish.SCP.Local = c.Build.Go.Output
	}
	if c.Publish.Driver == "scp" && c.Publish.SCP.Host == "" && c.Deploy.BinaryInstall.Host != "" {
		c.Publish.SCP.Host = c.Deploy.BinaryInstall.Host
	}
	if c.Deploy.Driver == "binary-install" && c.Deploy.BinaryInstall.Host == "" && c.Publish.SCP.Host != "" {
		c.Deploy.BinaryInstall.Host = c.Publish.SCP.Host
	}
	if c.Deploy.Driver == "binary-install" && c.Deploy.BinaryInstall.RemoteTempPath == "" && c.Publish.SCP.Remote != "" {
		c.Deploy.BinaryInstall.RemoteTempPath = c.Publish.SCP.Remote
	}
	if c.Deploy.Driver == "binary-install" && c.Deploy.BinaryInstall.Chmod == "" {
		c.Deploy.BinaryInstall.Chmod = "+x"
	}

	c.Registries = append([]Registry(nil), c.Publish.Registry.Targets...)
	c.Deploy.Enabled = c.Features.Deploy && c.Deploy.Driver != "none"
}

// Validate 校验必填字段，一次性返回所有缺失项。
func (c *Config) Validate() error {
	var missing []string

	defaultProfiles := 0
	for _, profile := range c.Matrix {
		if profile.Default {
			defaultProfiles++
		}
	}
	if defaultProfiles > 1 {
		missing = append(missing, "matrix.default 只能有一个 profile 为 true")
	}

	if c.Schema != 2 {
		missing = append(missing, "schema = 2")
	} else {
		c.validateV2(&missing)
	}

	if len(missing) > 0 {
		var b strings.Builder
		b.WriteString("配置缺失或无效：")
		for _, item := range missing {
			b.WriteString("\n- ")
			b.WriteString(item)
		}
		b.WriteString("\n参考 config.example.toml 创建或修正 ship.toml")
		return errors.New(b.String())
	}
	return nil
}

func (c *Config) validateV2(missing *[]string) {
	if c.Version.Source != "" && !StringSliceContains([]string{"git-tag", "env", "static"}, c.Version.Source) {
		*missing = append(*missing, "version.source 仅支持 git-tag | env | static")
	}
	if c.Version.Fallback != "" && !StringSliceContains([]string{"error", "dev", "static"}, c.Version.Fallback) {
		*missing = append(*missing, "version.fallback 仅支持 error | dev | static")
	}
	if c.Version.Fallback == "static" && c.Version.Static == "" {
		*missing = append(*missing, "version.static (当 version.fallback = static 时必填)")
	}

	switch c.Build.Driver {
	case "docker":
		if c.Build.Docker.Image == "" && c.ImageName == "" {
			*missing = append(*missing, "build.docker.image (或 publish.registry.targets[].image)")
		}
		if c.Build.Docker.Dockerfile == "" {
			*missing = append(*missing, "build.docker.dockerfile")
		}
		if len(c.Build.Docker.Platforms) == 0 {
			*missing = append(*missing, "build.docker.platforms")
		}
		if !c.Build.Docker.Load {
			*missing = append(*missing, "build.docker.load 当前分阶段流程中必须为 true")
		}
		if c.Build.Docker.DisableBuildkit {
			*missing = append(*missing, "build.docker.disable_buildkit 当前 build.driver = docker 时暂不支持")
		}
	case "go-binary":
		if c.Build.Go.Main == "" {
			*missing = append(*missing, "build.go.main")
		}
		if c.Build.Go.Output == "" {
			*missing = append(*missing, "build.go.output")
		}
	case "command":
		if c.Build.Command.Run == "" {
			*missing = append(*missing, "build.command.run")
		}
	default:
		*missing = append(*missing, "build.driver 仅支持 docker | go-binary | command")
	}

	if c.Features.Publish {
		switch c.Publish.Driver {
		case "registry":
			if len(c.Publish.Registry.Targets) == 0 {
				*missing = append(*missing, "publish.registry.targets")
			} else {
				c.validateRegistryTargets(missing)
			}
		case "scp":
			if c.Publish.SCP.Local == "" {
				*missing = append(*missing, "publish.scp.local")
			}
			if c.Publish.SCP.Host == "" {
				*missing = append(*missing, "publish.scp.host")
			}
			if c.Publish.SCP.Remote == "" {
				*missing = append(*missing, "publish.scp.remote")
			}
		case "none":
		default:
			*missing = append(*missing, "publish.driver 仅支持 registry | scp | none")
		}
	}

	if c.Features.Deploy {
		switch c.Deploy.Driver {
		case "compose":
			if strings.TrimSpace(c.Deploy.Compose.Host) == "" {
				*missing = append(*missing, "deploy.compose.host")
			}
			if strings.TrimSpace(c.Deploy.Compose.Path) == "" {
				*missing = append(*missing, "deploy.compose.path")
			}
			if strings.TrimSpace(c.Deploy.Compose.EnvFile) == "" {
				*missing = append(*missing, "deploy.compose.env_file")
			}
			if strings.TrimSpace(c.Deploy.Compose.TagKey) == "" {
				*missing = append(*missing, "deploy.compose.tag_key")
			}
			if strings.TrimSpace(c.Deploy.Compose.Up) == "" {
				*missing = append(*missing, "deploy.compose.up")
			}
			if strings.TrimSpace(c.Deploy.Compose.RemoteFile) != "" && strings.TrimSpace(c.Deploy.Compose.LocalFile) == "" {
				*missing = append(*missing, "deploy.compose.local_file")
			}
		case "binary-install":
			if c.Deploy.BinaryInstall.Host == "" {
				*missing = append(*missing, "deploy.binary_install.host")
			}
			if c.Deploy.BinaryInstall.RemoteTempPath == "" {
				*missing = append(*missing, "deploy.binary_install.remote_temp_path")
			}
			if c.Deploy.BinaryInstall.RemoteInstallPath == "" {
				*missing = append(*missing, "deploy.binary_install.remote_install_path")
			}
		case "ssh":
			if c.Deploy.SSH.Host == "" {
				*missing = append(*missing, "deploy.ssh.host")
			}
			if len(c.Deploy.SSH.Commands) == 0 {
				*missing = append(*missing, "deploy.ssh.commands")
			}
		case "none":
		default:
			*missing = append(*missing, "deploy.driver 仅支持 compose | binary-install | ssh | none")
		}
	}

	if c.Features.Verify {
		switch c.Verify.Driver {
		case "http":
			if c.Verify.HTTP.URL == "" {
				*missing = append(*missing, "verify.http.url")
			}
		case "ssh":
			if c.Verify.SSH.Host == "" {
				*missing = append(*missing, "verify.ssh.host")
			}
			if c.Verify.SSH.Command == "" {
				*missing = append(*missing, "verify.ssh.command")
			}
		case "command":
			if c.Verify.Command.Run == "" {
				*missing = append(*missing, "verify.command.run")
			}
		case "none":
		default:
			*missing = append(*missing, "verify.driver 仅支持 http | ssh | command | none")
		}
	}
}

func (c *Config) validateRegistryTargets(missing *[]string) {
	for index, target := range c.Publish.Registry.Targets {
		prefix := fmt.Sprintf("publish.registry.targets[%d]", index)
		switch target.Type {
		case "dockerhub", "private":
		case "":
			*missing = append(*missing, prefix+".type")
		default:
			*missing = append(*missing, prefix+".type 仅支持 dockerhub | private")
		}
		if target.Type == "private" && target.URL == "" {
			*missing = append(*missing, prefix+".url")
		}
		if target.Namespace == "" {
			*missing = append(*missing, prefix+".namespace")
		}
		if target.Image == "" {
			*missing = append(*missing, prefix+".image")
		}
	}
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
// 默认 profile 不加后缀，如 v2.0.0。
// 其他 profile 加后缀，如 v2.0.0-brand-a。
func ImageTag(version string, profile Profile) string {
	if profile.Name == "" {
		return version
	}
	return fmt.Sprintf("%s-%s", version, profile.Name)
}

// BuildSourceTag 返回 docker build 阶段产出的本地镜像 tag。
// 默认 profile 在启用 latest_on_default_profile 时使用 latest，否则退化到稳定的 build-default。
func (c *Config) BuildSourceTag(profile Profile) string {
	if profile.Name == "" {
		if c.Build.Docker.LatestOnDefaultProfile {
			return "latest"
		}
		return "build-default"
	}
	return fmt.Sprintf("latest-%s", profile.Name)
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
