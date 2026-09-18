package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ReleaseManifestSchema = 1
	ArtifactTypeImage     = "container-image"
	ArtifactTypeBinary    = "binary"
	ArtifactTypeFile      = "file"
)

// ManifestSource 记录产物对应的源码身份。
type ManifestSource struct {
	Ref    string `json:"ref,omitempty"`
	Commit string `json:"commit,omitempty"`
	Dirty  bool   `json:"dirty"`
	Mode   string `json:"mode,omitempty"`
}

// ArtifactRecord 描述一次 release 中的单个产物。
type ArtifactRecord struct {
	Type     string `json:"type"`
	Profile  string `json:"profile,omitempty"`
	Platform string `json:"platform,omitempty"`
	Ref      string `json:"ref,omitempty"`       // registry 引用，如 registry/ns/img:v1.2.3
	LocalRef string `json:"local_ref,omitempty"` // 本地 daemon 引用
	Digest   string `json:"digest,omitempty"`
}

// ReleaseManifest 是 publish/deploy/history 的共同事实来源。
type ReleaseManifest struct {
	Schema      int              `json:"schema"`
	RunID       string           `json:"run_id"`
	Version     string           `json:"version"`
	Source      ManifestSource   `json:"source"`
	Artifacts   []ArtifactRecord `json:"artifacts"`
	CreatedAt   string           `json:"created_at"`
	ShipVersion string           `json:"ship_version,omitempty"`
}

// NewReleaseManifest 基于 identity 与 run 构造空 manifest。
func NewReleaseManifest(identity ReleaseIdentity, runID, shipVersion string) *ReleaseManifest {
	return &ReleaseManifest{
		Schema: ReleaseManifestSchema,
		RunID:  runID,
		Version: identity.Version,
		Source: ManifestSource{
			Ref:    identity.SourceRef,
			Commit: identity.SourceCommit,
			Dirty:  identity.Dirty,
			Mode:   identity.SourceMode,
		},
		Artifacts:   nil,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		ShipVersion: shipVersion,
	}
}

// UpsertArtifact 按 type+profile+ref 合并产物记录。
func (m *ReleaseManifest) UpsertArtifact(art ArtifactRecord) {
	if m == nil {
		return
	}
	key := artifactKey(art)
	for i := range m.Artifacts {
		if artifactKey(m.Artifacts[i]) == key {
			m.Artifacts[i] = mergeArtifact(m.Artifacts[i], art)
			return
		}
	}
	m.Artifacts = append(m.Artifacts, art)
}

func artifactKey(a ArtifactRecord) string {
	profile := a.Profile
	if profile == "" {
		profile = "default"
	}
	ref := a.Ref
	if ref == "" {
		ref = a.LocalRef
	}
	return a.Type + "|" + profile + "|" + ref
}

func mergeArtifact(old, neu ArtifactRecord) ArtifactRecord {
	if neu.Type != "" {
		old.Type = neu.Type
	}
	if neu.Profile != "" {
		old.Profile = neu.Profile
	}
	if neu.Platform != "" {
		old.Platform = neu.Platform
	}
	if neu.Ref != "" {
		old.Ref = neu.Ref
	}
	if neu.LocalRef != "" {
		old.LocalRef = neu.LocalRef
	}
	if neu.Digest != "" {
		old.Digest = neu.Digest
	}
	return old
}

// artifactProfileName 把空 profile 规范化为 default，与 Upsert/select 的键一致。
func artifactProfileName(a ArtifactRecord) string {
	if strings.TrimSpace(a.Profile) == "" {
		return "default"
	}
	return a.Profile
}

// SelectImageArtifact 为指定 profile 选出部署用的 container-image。
// build 会先写入仅有 local_ref 的记录，push 再追加带 registry ref/digest 的记录；
// 必须优先选可钉扎的已发布产物，而不能返回同 profile 的第一条本地构建记录。
func (m *ReleaseManifest) SelectImageArtifact(profile Profile) ArtifactRecord {
	if m == nil {
		return ArtifactRecord{}
	}
	want := FormatProfileName(profile)
	if want == "" {
		want = "default"
	}
	best := ArtifactRecord{}
	bestScore := -1
	for _, a := range m.Artifacts {
		if a.Type != ArtifactTypeImage {
			continue
		}
		score := 0
		if artifactProfileName(a) == want {
			score += 8
		}
		if IsPinableDigest(a.Digest) {
			score += 2
		}
		if strings.TrimSpace(a.Ref) != "" {
			score += 1
		}
		if score > bestScore {
			bestScore = score
			best = a
		}
	}
	return best
}

// PrimaryImageDigest 返回默认/首个可钉扎 container-image 的 digest。
func (m *ReleaseManifest) PrimaryImageDigest() string {
	if m == nil {
		return ""
	}
	art := m.SelectImageArtifact(Profile{})
	if IsPinableDigest(art.Digest) {
		return art.Digest
	}
	for _, a := range m.Artifacts {
		if a.Type == ArtifactTypeImage && IsPinableDigest(a.Digest) {
			return a.Digest
		}
	}
	return ""
}

// HasPublishedImage 返回是否至少有一个带 registry ref 的镜像产物。
func (m *ReleaseManifest) HasPublishedImage() bool {
	if m == nil {
		return false
	}
	for _, a := range m.Artifacts {
		if a.Type == ArtifactTypeImage && strings.TrimSpace(a.Ref) != "" {
			return true
		}
	}
	return false
}

// RunManifestPath 返回 .ship/runs/<run_id>/manifest.json。
func RunManifestPath(stateRoot, runID string) string {
	return filepath.Join(stateRoot, "runs", runID, "manifest.json")
}

// ReleaseIndexPath 返回 .ship/releases/<version>.json。
func ReleaseIndexPath(stateRoot, version string) string {
	safe := sanitizeReleaseFileName(version)
	return filepath.Join(stateRoot, "releases", safe+".json")
}

func sanitizeReleaseFileName(version string) string {
	version = strings.TrimSpace(version)
	var b strings.Builder
	for _, r := range version {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

// SaveReleaseManifest 写入 runs/<run_id>/manifest.json，并在 indexRelease 时更新 releases 索引。
func SaveReleaseManifest(stateRoot string, m *ReleaseManifest, indexRelease bool) error {
	if m == nil {
		return fmt.Errorf("manifest 为空")
	}
	if strings.TrimSpace(stateRoot) == "" {
		return fmt.Errorf("StateRoot 为空")
	}
	if strings.TrimSpace(m.RunID) == "" {
		return fmt.Errorf("manifest.run_id 为空")
	}

	runPath := RunManifestPath(stateRoot, m.RunID)
	if err := os.MkdirAll(filepath.Dir(runPath), 0o755); err != nil {
		return fmt.Errorf("创建 run 目录失败: %w", err)
	}
	if err := writeJSONFile(runPath, m); err != nil {
		return err
	}

	if indexRelease && strings.TrimSpace(m.Version) != "" {
		idxPath := ReleaseIndexPath(stateRoot, m.Version)
		if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err != nil {
			return fmt.Errorf("创建 releases 目录失败: %w", err)
		}
		if err := writeJSONFile(idxPath, m); err != nil {
			return err
		}
	}
	return nil
}

// LoadReleaseManifestFile 从指定路径加载 manifest。
func LoadReleaseManifestFile(path string) (*ReleaseManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m ReleaseManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 release manifest 失败: %w", err)
	}
	if m.Schema == 0 {
		m.Schema = ReleaseManifestSchema
	}
	return &m, nil
}

// FindReleaseManifest 按版本查找已发布 manifest：先 releases 索引，再扫描 runs。
func FindReleaseManifest(stateRoot, version string) (*ReleaseManifest, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, fmt.Errorf("version 不能为空")
	}
	idx := ReleaseIndexPath(stateRoot, version)
	if m, err := LoadReleaseManifestFile(idx); err == nil {
		if m.Version == version {
			return m, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取 release 索引失败: %w", err)
	}

	runsDir := filepath.Join(stateRoot, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("未找到版本 %s 的 release manifest；请先执行 ship run / ship build+push", version)
		}
		return nil, fmt.Errorf("扫描 runs 目录失败: %w", err)
	}

	var best *ReleaseManifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(runsDir, e.Name(), "manifest.json")
		m, err := LoadReleaseManifestFile(path)
		if err != nil {
			continue
		}
		if m.Version != version {
			continue
		}
		if best == nil || m.CreatedAt > best.CreatedAt {
			best = m
		}
	}
	if best == nil {
		return nil, fmt.Errorf("未找到版本 %s 的 release manifest；请先执行 ship run / ship build+push", version)
	}
	return best, nil
}

// RequireReleaseManifest 查找 manifest，用于独立 push/deploy。
func RequireReleaseManifest(stateRoot, version string) (*ReleaseManifest, error) {
	m, err := FindReleaseManifest(stateRoot, version)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", path, err)
	}
	return nil
}

// InspectRemoteDigest 导出远端 manifest 指纹查询（可能为 config+layers 或 index: 聚合串）。
// 若需要可用于 @digest 的 pin 身份，请使用 ResolveRegistryPinDigest。
func InspectRemoteDigest(ref string) (digest string, exists bool, err error) {
	return remoteManifestDigest(ref)
}

// InspectLocalDigest 导出本地镜像指纹查询（config Id + layers，不可直接当 pin）。
func InspectLocalDigest(ref string) (string, error) {
	return localImageDigest(ref)
}
