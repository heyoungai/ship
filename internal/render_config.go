package internal

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

var (
	configRootType  = reflect.TypeOf(Config{})
	profileRootType = reflect.TypeOf(Profile{})
)

const maxTemplateRenderDepth = 10

// RenderConfigForProfile 基于指定 profile/version 渲染一份可执行配置。
//
// 这里的目标不是只修补某几个字段，而是把“模板渲染”提升为统一阶段：
// 1. 先克隆原始配置，避免污染加载后的配置对象
// 2. 第一轮渲染全局配置，先展开 project / vars / build / publish 等公共节点
// 3. 第二轮渲染当前 profile，让 profile.env / profile.vars 也支持模板
// 4. 第三轮再用“已渲染 profile”回填配置，确保 publish/deploy/verify 等节点也能读取 profile 变量
// 5. 最后重新 normalize，刷新 ImageName / Registries / legacy 映射等派生字段
func RenderConfigForProfile(cfg *Config, profile Profile, version string) (*Config, Profile, error) {
	clonedCfg, err := cloneConfig(cfg)
	if err != nil {
		return nil, Profile{}, fmt.Errorf("克隆配置失败: %w", err)
	}
	clonedProfile, err := cloneProfile(profile)
	if err != nil {
		return nil, Profile{}, fmt.Errorf("克隆 profile 失败: %w", err)
	}

	if err := renderValueRecursive(reflect.ValueOf(clonedCfg).Elem(), NewRenderContext(cfg, profile, version), "config"); err != nil {
		return nil, Profile{}, err
	}
	if err := renderValueRecursive(reflect.ValueOf(&clonedProfile).Elem(), NewRenderContext(clonedCfg, profile, version), "profile"); err != nil {
		return nil, Profile{}, err
	}
	if err := renderValueRecursive(reflect.ValueOf(clonedCfg).Elem(), NewRenderContext(clonedCfg, clonedProfile, version), "config"); err != nil {
		return nil, Profile{}, err
	}

	clonedCfg.normalize()
	return clonedCfg, clonedProfile, nil
}

func cloneConfig(cfg *Config) (*Config, error) {
	var cloned Config
	if err := deepClone(cfg, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func cloneProfile(profile Profile) (Profile, error) {
	var cloned Profile
	if err := deepClone(profile, &cloned); err != nil {
		return Profile{}, err
	}
	return cloned, nil
}

func deepClone(from any, to any) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, to)
}

// renderValueRecursive 递归渲染配置中的字符串、[]string 和 map[string]string。
//
// 这里有两个刻意保留的边界：
// 1. Matrix 不在单个 profile 渲染时展开，避免当前 profile 的变量污染其他 profile
// 2. Registries 会在 normalize() 中从 publish.registry.targets 重新派生，因此直接跳过并重建
func renderValueRecursive(value reflect.Value, ctx RenderContext, path string) error {
	if !value.IsValid() {
		return nil
	}

	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		return renderValueRecursive(value.Elem(), ctx, path)
	}

	switch value.Kind() {
	case reflect.Struct:
		valueType := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := valueType.Field(i)
			if !field.IsExported() {
				continue
			}
			if shouldSkipRenderedField(path, valueType, field.Name) {
				continue
			}
			if err := renderValueRecursive(value.Field(i), ctx, childPath(path, field.Name)); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if value.Type().Elem().Kind() == reflect.String {
			for i := 0; i < value.Len(); i++ {
				rendered, err := renderStringFully(ctx, value.Index(i).String(), fmt.Sprintf("%s[%d]", path, i))
				if err != nil {
					return err
				}
				value.Index(i).SetString(rendered)
			}
			return nil
		}
		for i := 0; i < value.Len(); i++ {
			if err := renderValueRecursive(value.Index(i), ctx, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := renderValueRecursive(value.Index(i), ctx, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String || value.Type().Elem().Kind() != reflect.String {
			return nil
		}
		for _, key := range value.MapKeys() {
			rendered, err := renderStringFully(ctx, value.MapIndex(key).String(), fmt.Sprintf("%s[%s]", path, key.String()))
			if err != nil {
				return err
			}
			value.SetMapIndex(key, reflect.ValueOf(rendered).Convert(value.Type().Elem()))
		}
	case reflect.String:
		if !value.CanSet() {
			return nil
		}
		rendered, err := renderStringFully(ctx, value.String(), path)
		if err != nil {
			return err
		}
		value.SetString(rendered)
	}

	return nil
}

// renderStringFully 会把同一个字符串持续渲染到“没有模板占位符”为止。
//
// 这样可以覆盖链式引用，例如：
// vars.image_name -> {{ vars.name_suffix }}-api
// vars.name_suffix -> {{ project.name }}
// 最终需要把 {{ vars.image_name }} 展开成 demo-api，而不是只展开一层后停在半成品状态。
//
// 同时这里也加入最大深度保护，避免 self-reference / 循环引用把渲染流程卡死。
func renderStringFully(ctx RenderContext, input, path string) (string, error) {
	current := input
	for i := 0; i < maxTemplateRenderDepth; i++ {
		if !strings.Contains(current, "{{") {
			return current, nil
		}

		rendered, err := ctx.RenderString(current)
		if err != nil {
			return "", fmt.Errorf("渲染 %s 失败: %w", path, err)
		}
		if rendered == current {
			return "", fmt.Errorf("渲染 %s 失败: 模板无法继续展开，可能存在循环引用: %s", path, current)
		}
		current = rendered
	}

	return "", fmt.Errorf("渲染 %s 失败: 模板展开超过最大层级(%d)，可能存在循环引用: %s", path, maxTemplateRenderDepth, current)
}

func shouldSkipRenderedField(path string, valueType reflect.Type, fieldName string) bool {
	if path == "config" && valueType == configRootType {
		return fieldName == "Matrix" || fieldName == "Registries"
	}
	if path == "profile" && valueType == profileRootType {
		return false
	}
	return false
}

func childPath(path, fieldName string) string {
	if path == "" {
		return fieldName
	}
	return path + "." + fieldName
}
