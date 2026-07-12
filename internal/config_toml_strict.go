package internal

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	unknownKeysError  = "error"
	unknownKeysWarn   = "warn"
	unknownKeysIgnore = "ignore"
)

// RuntimeOptions 控制 ship.toml 的加载行为。
type RuntimeOptions struct {
	// UnknownKeys 控制遇到未识别配置项时的行为：error | warn | ignore。
	UnknownKeys string `toml:"unknown_keys"`
}

func decodeConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	meta, err := toml.Decode(string(data), cfg)
	if err != nil {
		return err
	}

	keys := collectUnknownConfigKeys(meta)
	if len(keys) == 0 {
		return nil
	}

	switch resolveUnknownKeysMode(cfg) {
	case unknownKeysIgnore:
		return nil
	case unknownKeysWarn:
		PrintWarning(formatUnknownConfigKeysMessage(keys))
		return nil
	default:
		return fmt.Errorf("%s", formatUnknownConfigKeysMessage(keys))
	}
}

func collectUnknownConfigKeys(meta toml.MetaData) []string {
	undecoded := meta.Undecoded()
	if len(undecoded) == 0 {
		return nil
	}

	keys := make([]string, 0, len(undecoded))
	for _, key := range undecoded {
		keys = append(keys, key.String())
	}
	sort.Strings(keys)
	return keys
}

func resolveUnknownKeysMode(cfg *Config) string {
	if mode := strings.TrimSpace(cfg.Runtime.UnknownKeys); mode != "" {
		return mode
	}
	if mode := strings.TrimSpace(os.Getenv("SHIP_UNKNOWN_KEYS")); mode != "" {
		return mode
	}
	return unknownKeysError
}

func formatUnknownConfigKeysMessage(keys []string) string {
	var b strings.Builder
	b.WriteString("ship.toml 包含未识别的配置项（不会被 ship 读取）：")
	for _, key := range keys {
		b.WriteString("\n- ")
		b.WriteString(key)
	}
	b.WriteString("\n请检查拼写，或改用 steps.* / 已支持的 deploy.compose 字段")
	b.WriteString("\n可通过 [config] unknown_keys = \"warn\" 或环境变量 SHIP_UNKNOWN_KEYS=warn 降级为警告")
	return b.String()
}
