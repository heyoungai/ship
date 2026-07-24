package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PersistFileArtifact 将构建产物复制到 StateRoot/artifacts/<run_id>/<profile>/，并返回目标路径与 sha256。
func PersistFileArtifact(stateRoot, runID, profile, srcPath string) (destPath, digest string, err error) {
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		return "", "", fmt.Errorf("源文件路径为空")
	}
	if strings.TrimSpace(stateRoot) == "" {
		return "", "", fmt.Errorf("StateRoot 为空")
	}
	if strings.TrimSpace(runID) == "" {
		return "", "", fmt.Errorf("run_id 为空")
	}
	if profile == "" {
		profile = "default"
	}

	absSrc, err := filepath.Abs(srcPath)
	if err != nil {
		return "", "", fmt.Errorf("解析源路径失败: %w", err)
	}
	info, err := os.Stat(absSrc)
	if err != nil {
		return "", "", fmt.Errorf("读取源文件失败: %w", err)
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("源路径是目录，当前仅支持单文件产物: %s", absSrc)
	}

	destDir := filepath.Join(stateRoot, "artifacts", runID, sanitizeReleaseFileName(profile))
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", "", fmt.Errorf("创建 artifacts 目录失败: %w", err)
	}
	destPath = filepath.Join(destDir, filepath.Base(absSrc))

	if err := copyFile(absSrc, destPath); err != nil {
		return "", "", err
	}
	digest, err = fileSHA256(destPath)
	if err != nil {
		return "", "", err
	}
	return destPath, "sha256:" + digest, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}
	return out.Close()
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ImageRepoFromRef 从 registry 引用去掉 :tag，保留仓库路径（用于拼 @digest）。
// 例：registry.example.com/ns/app:v1.2.3 → registry.example.com/ns/app
func ImageRepoFromRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if i := strings.LastIndex(ref, "@"); i >= 0 {
		return ref[:i]
	}
	// 只剥离最后一段 :tag；保留 registry:port/path 中的端口冒号。
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		after := ref[i+1:]
		if !strings.Contains(after, "/") {
			return ref[:i]
		}
	}
	return ref
}

// ImageDigestRef 生成 repo@sha256:... 形式。
// digest 必须是可钉扎的 registry content digest；拒绝 config 指纹与 index: 聚合串。
func ImageDigestRef(imageRef, digest string) string {
	repo := ImageRepoFromRef(imageRef)
	digest = strings.TrimSpace(digest)
	if repo == "" || digest == "" {
		return ""
	}
	if !strings.HasPrefix(digest, "sha256:") {
		digest = "sha256:" + digest
	}
	if !IsPinableDigest(digest) {
		return ""
	}
	return repo + "@" + digest
}

// ResolveComposePin 返回实际 pin 模式：配置为 digest 但无可用 pin digest 时降级为 tag。
func ResolveComposePin(configuredPin, digest string) (pin string, degraded bool) {
	pin = strings.ToLower(strings.TrimSpace(configuredPin))
	if pin == "" {
		pin = "digest"
	}
	if pin != "digest" && pin != "tag" {
		pin = "digest"
	}
	if pin == "digest" && !IsPinableDigest(digest) {
		return "tag", true
	}
	return pin, false
}
