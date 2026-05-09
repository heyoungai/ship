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

// Registry 定义镜像仓库配置
type Registry struct {
	Type      string `toml:"type"`
	URL       string `toml:"url"`
	Namespace string `toml:"namespace"`
	Image     string `toml:"image"`
}

// Profile 定义矩阵构建变体（如多品牌）
type Profile struct {
	Name    string            `toml:"name"`
	Default bool              `toml:"default"`
	Env     map[string]string `toml:"env"`
}

// Config 定义完整配置
type Config struct {
	Build struct {
		Platforms  string `toml:"platforms"`
		Dockerfile string `toml:"dockerfile"`
		EnvFile    string `toml:"env_file"`
		LocalBuild string `toml:"local_build"`
	} `toml:"build"`

	Registries []Registry `toml:"registries"`

	Deploy struct {
		Enabled     bool              `toml:"enabled"`
		Host        string            `toml:"host"`
		Path        string            `toml:"path"`
		Healthcheck DeployHealthcheck `toml:"healthcheck"`
	} `toml:"deploy"`

	Matrix []Profile `toml:"matrix"`

	// 运行时填充，不来自 TOML
	ImageName string `toml:"-"`
}

// LoadConfig 加载配置：内置默认值 → ship.toml → 环境变量覆盖 → 校验必填字段
func LoadConfig(imageName string) (*Config, error) {
	cfg := &Config{ImageName: imageName}

	// 1. 内置默认值（仅通用字段，项目特定字段不设默认值）
	cfg.Build.Platforms = "linux/amd64"
	cfg.Build.Dockerfile = "./Dockerfile"
	cfg.Build.EnvFile = "./.env.local"
	cfg.Deploy.Enabled = false

	// 2. 读取 ship.toml（如果存在）
	if _, err := os.Stat("ship.toml"); err == nil {
		if _, err := toml.DecodeFile("ship.toml", cfg); err != nil {
			return nil, fmt.Errorf("读取 ship.toml 失败: %w", err)
		}
	}

	// 3. 环境变量覆盖
	if v := os.Getenv("PLATFORMS"); v != "" {
		cfg.Build.Platforms = v
	}
	if v := os.Getenv("DOCKERFILE"); v != "" {
		cfg.Build.Dockerfile = v
	}
	if v := os.Getenv("IMAGE_NAME"); v != "" {
		cfg.ImageName = v
	}
	if v := os.Getenv("REMOTE_HOST"); v != "" {
		cfg.Deploy.Host = v
	}
	if v := os.Getenv("REMOTE_PROJECT_PATH"); v != "" {
		cfg.Deploy.Path = v
	}
	if v := os.Getenv("ENV_FILE"); v != "" {
		cfg.Build.EnvFile = v
	}
	cfg.Deploy.Healthcheck.ApplyDefaults()

	// 4. 自动推导：image_name 未设置时从第一个 registry 的 image 推导
	if cfg.ImageName == "" && len(cfg.Registries) > 0 {
		cfg.ImageName = cfg.Registries[0].Image
	}

	// 5. 校验必填字段
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate 校验必填字段，一次性返回所有缺失项
func (c *Config) Validate() error {
	var missing []string

	if c.ImageName == "" {
		missing = append(missing, "image_name (环境变量 IMAGE_NAME 或 ship.toml registries[].image)")
	}
	if c.Deploy.Enabled {
		if c.Deploy.Host == "" {
			missing = append(missing, "deploy.host (环境变量 REMOTE_HOST 或 ship.toml [deploy].host)")
		}
		if c.Deploy.Path == "" {
			missing = append(missing, "deploy.path (环境变量 REMOTE_PROJECT_PATH 或 ship.toml [deploy].path)")
		}
	}
	if len(c.Registries) == 0 {
		missing = append(missing, "registries (ship.toml [[registries]])")
	}
	defaultProfiles := 0
	for _, profile := range c.Matrix {
		if profile.Default {
			defaultProfiles++
		}
	}
	if defaultProfiles > 1 {
		missing = append(missing, "matrix.default 只能有一个 profile 为 true")
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

// DefaultProfile 返回默认 profile（无 matrix 时的单次构建）
func (c *Config) DefaultProfile() Profile {
	for _, p := range c.Matrix {
		if p.Default {
			return p
		}
	}
	return Profile{Name: "", Default: true}
}

// GetProfiles 获取要构建的 profile 列表
// name 为空时返回所有 profile（无 matrix 则返回单个默认 profile）
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

// ImageRef 生成镜像引用（image:tag）
func (c *Config) ImageRef(tag string) string {
	return fmt.Sprintf("%s:%s", c.ImageName, tag)
}

// ImageTag 生成带 profile 后缀的 tag
// 默认 profile 不加后缀，如 v2.0.0
// 其他 profile 加后缀，如 v2.0.0-linglu
func ImageTag(version string, profile Profile) string {
	if profile.Name == "" {
		return version
	}
	return fmt.Sprintf("%s-%s", version, profile.Name)
}

// RegistryTargets 生成注册表镜像引用列表
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

// LoadBuildArgs 读取 .env 文件，返回 --build-arg 列表
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

// DetectLocalBuild 根据锁文件推断本地构建命令
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

// MergeEnv 合并环境变量：基础 env + profile env
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

// EnvToSlice 将 map 转为 "KEY=VALUE" 字符串切片
func EnvToSlice(envMap map[string]string) []string {
	s := make([]string, 0, len(envMap))
	for k, v := range envMap {
		s = append(s, fmt.Sprintf("%s=%s", k, v))
	}
	return s
}

// FormatProfileName 格式化 profile 名称用于显示
// 无 matrix 时返回空字符串，有 matrix 时返回 profile 名
func FormatProfileName(p Profile) string {
	return p.Name
}

// StringSliceContains 检查字符串切片是否包含指定元素
func StringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
