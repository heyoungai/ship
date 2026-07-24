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

	if DigestsMatch(localDigest, remoteDigest) {
		PrintInfo(fmt.Sprintf("远端已存在相同内容: %s (%s)，跳过 push", remoteRef, shortDigest(remoteDigest)))
		return true, nil
	}

	return false, fmt.Errorf(
		"拒绝覆盖远端版本 %s：已有 digest %s，本地 digest %s；请发布新版本而不是覆盖正式 tag",
		remoteRef, shortDigest(remoteDigest), shortDigest(localDigest),
	)
}

// ResolveRegistryPinDigest 解析可用于 image@sha256:... 钉扎的 registry content digest。
// 优先 docker buildx imagetools（对 buildx multi-arch index 返回 index digest）；
// 不再把本地 config digest（docker image inspect .Id）当作 pin 身份。
func ResolveRegistryPinDigest(ref string) (digest string, exists bool, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false, fmt.Errorf("镜像引用为空")
	}

	if d, ok, err := registryDigestViaImagetools(ref); err != nil {
		// imagetools 在部分环境下不可用；继续 fallback，不把该错误直接抛出。
		_ = err
	} else if ok {
		return d, true, nil
	}

	fp, exists, err := remoteManifestDigest(ref)
	if err != nil {
		return "", false, err
	}
	if !exists {
		return "", false, nil
	}
	if d := PinDigestToken(fp); IsPinableDigest(d) {
		return d, true, nil
	}
	// manifest list/index：没有 imagetools 时无法得到可 @digest 的 index digest。
	// 返回 exists=true 且 digest=""，调用方应跳过写入 pin，而不是回退本地 config digest。
	return "", true, nil
}

func registryDigestViaImagetools(ref string) (digest string, ok bool, err error) {
	cmd := exec.Command("docker", "buildx", "imagetools", "inspect", ref, "--format", "{{.Digest}}")
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
		return "", false, fmt.Errorf("imagetools inspect 失败 (%s): %s", ref, strings.TrimSpace(string(out)))
	}
	d := strings.TrimSpace(string(out))
	if !IsPinableDigest(d) {
		return "", false, nil
	}
	return d, true, nil
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

// IsPinableDigest 判断是否为可用于 repo@digest 的 registry content digest。
// 拒绝本地 config 指纹（含空格/layer JSON）、以及 invent 的 index:/list: 聚合串。
func IsPinableDigest(d string) bool {
	d = strings.TrimSpace(d)
	if d == "" {
		return false
	}
	if strings.HasPrefix(d, "index:") || strings.HasPrefix(d, "list:") {
		return false
	}
	if strings.ContainsAny(d, " ,[") {
		return false
	}
	if !strings.HasPrefix(d, "sha256:") {
		return false
	}
	hex := d[len("sha256:"):]
	if hex == "" {
		return false
	}
	for _, r := range hex {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// PinDigestToken 从指纹中提取可用于 pin 的 sha256 token；无法钉扎时返回空。
func PinDigestToken(fingerprint string) string {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return ""
	}
	if strings.HasPrefix(fingerprint, "index:") || strings.HasPrefix(fingerprint, "list:") {
		return ""
	}
	fields := strings.Fields(fingerprint)
	if len(fields) == 0 {
		return ""
	}
	tok := fields[0]
	if !IsPinableDigest(tok) {
		return ""
	}
	return tok
}

// DigestsMatch 比较 recorded 与 remote 指纹/ pin digest 是否指向同一发布身份。
// 支持：完全相等、config/layer 兼容、以及 pin digest ∈ index/list 成员。
func DigestsMatch(recorded, remote string) bool {
	recorded = strings.TrimSpace(recorded)
	remote = strings.TrimSpace(remote)
	if recorded == "" || remote == "" {
		return false
	}
	if recorded == remote {
		return true
	}
	if digestsCompatible(recorded, remote) {
		return true
	}

	recPin := PinDigestToken(recorded)
	if recPin == "" && IsPinableDigest(recorded) {
		recPin = recorded
	}
	remPin := PinDigestToken(remote)
	if remPin == "" && IsPinableDigest(remote) {
		remPin = remote
	}
	if recPin != "" && remPin != "" && recPin == remPin {
		return true
	}
	if recPin != "" {
		for _, m := range indexMembers(remote) {
			if m == recPin {
				return true
			}
		}
	}
	if remPin != "" {
		for _, m := range indexMembers(recorded) {
			if m == remPin {
				return true
			}
		}
	}
	return false
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
	// recorded/config digest ∈ remote index 成员（manifest digest）
	if IsPinableDigest(localID) {
		for _, m := range indexMembers(remote) {
			if m == localID {
				return true
			}
		}
	}
	if IsPinableDigest(remoteID) {
		for _, m := range indexMembers(local) {
			if m == remoteID {
				return true
			}
		}
	}
	// Compare layer sets when both encode layers（非 index 聚合）。
	localLayers := extractLayerDigests(local)
	remoteLayers := extractLayerDigests(remote)
	if isIndexFingerprint(local) || isIndexFingerprint(remote) {
		// index 成员是 manifest digest，与 RootFS layers 不可比。
		return false
	}
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

func isIndexFingerprint(fingerprint string) bool {
	fingerprint = strings.TrimSpace(fingerprint)
	return strings.HasPrefix(fingerprint, "index:") || strings.HasPrefix(fingerprint, "list:")
}

func indexMembers(fingerprint string) []string {
	fingerprint = strings.TrimSpace(fingerprint)
	if !isIndexFingerprint(fingerprint) {
		return nil
	}
	_, rest, _ := strings.Cut(fingerprint, ":")
	return splitNonEmpty(rest, ",")
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
