package internal

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// EnsureRegistryTagImmutable 在 push 前检查远端同名 tag：
// - 不存在：允许发布
// - 存在且与本地内容等价：视为幂等成功（返回 skip=true）
// - 存在且内容不同：拒绝覆盖
func EnsureRegistryTagImmutable(localRef, remoteRef string) (skip bool, err error) {
	remoteDigest, exists, err := remoteManifestDigest(remoteRef)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	localDigest, err := localImageDigest(localRef)
	if err != nil {
		return false, fmt.Errorf("读取本地镜像 digest 失败 (%s): %w", localRef, err)
	}

	if digestsCompatible(localDigest, remoteDigest) {
		PrintInfo(fmt.Sprintf("远端已存在相同内容: %s (%s)，跳过 push", remoteRef, shortDigest(remoteDigest)))
		return true, nil
	}

	return false, fmt.Errorf(
		"拒绝覆盖远端版本 %s：已有 digest %s，本地 digest %s；请发布新版本而不是覆盖正式 tag",
		remoteRef, shortDigest(remoteDigest), shortDigest(localDigest),
	)
}

func remoteManifestDigest(ref string) (digest string, exists bool, err error) {
	cmd := exec.Command("docker", "manifest", "inspect", ref)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.ToLower(string(out) + err.Error())
		if strings.Contains(msg, "no such") ||
			strings.Contains(msg, "not found") ||
			strings.Contains(msg, "manifest unknown") ||
			strings.Contains(msg, "name unknown") ||
			strings.Contains(msg, "does not exist") {
			return "", false, nil
		}
		// 部分环境未登录或网络错误：保守起见返回错误，避免静默覆盖。
		return "", false, fmt.Errorf("检查远端 manifest 失败 (%s): %s", ref, strings.TrimSpace(string(out)))
	}

	digest, err = parseManifestDigest(out)
	if err != nil {
		return "", true, err
	}
	return digest, true, nil
}

func localImageDigest(ref string) (string, error) {
	// Prefer config digest (Id)；再叠加 RootFS layers 作为内容指纹。
	cmd := exec.Command(
		"docker", "image", "inspect", ref,
		"--format", "{{.Id}} {{json .RootFS.Layers}}",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func parseManifestDigest(raw []byte) (string, error) {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("解析 manifest JSON 失败: %w", err)
	}

	switch v := payload.(type) {
	case map[string]any:
		if d := digestFromManifestObject(v); d != "" {
			return d, nil
		}
		// manifest list / index
		if manifests, ok := v["manifests"].([]any); ok && len(manifests) > 0 {
			var parts []string
			for _, item := range manifests {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if d, _ := m["digest"].(string); d != "" {
					parts = append(parts, d)
				}
			}
			if len(parts) > 0 {
				return "index:" + strings.Join(parts, ","), nil
			}
		}
	case []any:
		// docker manifest inspect 对 multi-arch 有时返回数组
		var parts []string
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if d := digestFromManifestObject(m); d != "" {
				parts = append(parts, d)
			}
		}
		if len(parts) > 0 {
			return "list:" + strings.Join(parts, ","), nil
		}
	}

	return "", fmt.Errorf("无法从 manifest inspect 结果提取 digest")
}

func digestFromManifestObject(m map[string]any) string {
	if cfg, ok := m["config"].(map[string]any); ok {
		if d, _ := cfg["digest"].(string); d != "" {
			var layers []string
			if arr, ok := m["layers"].([]any); ok {
				for _, item := range arr {
					if lm, ok := item.(map[string]any); ok {
						if ld, _ := lm["digest"].(string); ld != "" {
							layers = append(layers, ld)
						}
					}
				}
			}
			if len(layers) > 0 {
				return d + " " + strings.Join(layers, ",")
			}
			return d
		}
	}
	if d, _ := m["digest"].(string); d != "" {
		return d
	}
	return ""
}

func digestsCompatible(local, remote string) bool {
	local = strings.TrimSpace(local)
	remote = strings.TrimSpace(remote)
	if local == "" || remote == "" {
		return false
	}
	if local == remote {
		return true
	}
	// local: "sha256:abc [\"sha256:layer1\",...]"
	// remote: "sha256:abc sha256:layer1,sha256:layer2" or config-only
	localID := strings.Fields(local)[0]
	remoteID := strings.Fields(remote)[0]
	if localID != "" && localID == remoteID {
		return true
	}
	// Compare layer sets when both encode layers.
	localLayers := extractLayerDigests(local)
	remoteLayers := extractLayerDigests(remote)
	if len(localLayers) == 0 || len(remoteLayers) == 0 {
		return false
	}
	if len(localLayers) != len(remoteLayers) {
		return false
	}
	for i := range localLayers {
		if localLayers[i] != remoteLayers[i] {
			return false
		}
	}
	return true
}

func extractLayerDigests(fingerprint string) []string {
	fingerprint = strings.TrimSpace(fingerprint)
	if strings.HasPrefix(fingerprint, "index:") || strings.HasPrefix(fingerprint, "list:") {
		_, rest, _ := strings.Cut(fingerprint, ":")
		return splitNonEmpty(rest, ",")
	}
	// JSON array form from docker inspect
	if idx := strings.Index(fingerprint, "["); idx >= 0 {
		var layers []string
		_ = json.Unmarshal([]byte(fingerprint[idx:]), &layers)
		return layers
	}
	// space-separated after config digest
	fields := strings.Fields(fingerprint)
	if len(fields) > 1 {
		return splitNonEmpty(fields[1], ",")
	}
	return nil
}

func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func shortDigest(d string) string {
	d = strings.TrimSpace(d)
	fields := strings.Fields(d)
	if len(fields) == 0 {
		return d
	}
	id := fields[0]
	if len(id) > 19 {
		return id[:19] + "…"
	}
	return id
}
