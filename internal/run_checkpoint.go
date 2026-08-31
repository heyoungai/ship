package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const RunCheckpointSchema = 1

const (
	StagePending     = "pending"
	StageRunning     = "running"
	StageSucceeded   = "succeeded"
	StageFailed      = "failed"
	StageInterrupted = "interrupted"
)

// StageCheckpoint 记录单个 pipeline 阶段（或 profile 子阶段）的进度。
type StageCheckpoint struct {
	Status     string `json:"status"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	Error      string `json:"error,omitempty"`
}

// RunCheckpoint 记录 run 的控制流；发布产物本身仍由 manifest.json 表示。
type RunCheckpoint struct {
	Schema       int                        `json:"schema"`
	RunID        string                     `json:"run_id"`
	Identity     ReleaseIdentity            `json:"identity"`
	RecipeDigest string                     `json:"recipe_digest"`
	Profiles     []string                   `json:"profiles"`
	Stages       map[string]StageCheckpoint `json:"stages"`
	Artifacts    []ArtifactRecord           `json:"artifacts,omitempty"`
	CreatedAt    string                     `json:"created_at"`
	UpdatedAt    string                     `json:"updated_at"`
	ShipVersion  string                     `json:"ship_version,omitempty"`
}

func NewRunCheckpoint(runID string, identity ReleaseIdentity, recipeDigest string, profiles []string, shipVersion string) *RunCheckpoint {
	return &RunCheckpoint{
		Schema:       RunCheckpointSchema,
		RunID:        runID,
		Identity:     identity,
		RecipeDigest: recipeDigest,
		Profiles:     normalizedProfileNames(profiles),
		Stages:       map[string]StageCheckpoint{},
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		ShipVersion:  shipVersion,
	}
}

func RunCheckpointPath(stateRoot, runID string) string {
	return filepath.Join(stateRoot, "runs", runID, "run.json")
}

// SaveRunCheckpoint 先写同目录临时文件再 rename，避免中断时留下半份 checkpoint。
func SaveRunCheckpoint(stateRoot string, checkpoint *RunCheckpoint) error {
	if checkpoint == nil {
		return fmt.Errorf("run checkpoint 为空")
	}
	if strings.TrimSpace(checkpoint.RunID) == "" {
		return fmt.Errorf("run checkpoint.run_id 为空")
	}
	if strings.TrimSpace(stateRoot) == "" {
		return fmt.Errorf("StateRoot 为空")
	}
	if checkpoint.Schema == 0 {
		checkpoint.Schema = RunCheckpointSchema
	}
	if checkpoint.Stages == nil {
		checkpoint.Stages = map[string]StageCheckpoint{}
	}
	checkpoint.Profiles = normalizedProfileNames(checkpoint.Profiles)
	checkpoint.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	path := RunCheckpointPath(stateRoot, checkpoint.RunID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建 run checkpoint 目录失败: %w", err)
	}
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 run checkpoint 失败: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".run-*.tmp")
	if err != nil {
		return fmt.Errorf("创建 run checkpoint 临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入 run checkpoint 临时文件失败: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("设置 run checkpoint 权限失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭 run checkpoint 临时文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("原子保存 run checkpoint 失败: %w", err)
	}
	return nil
}

func LoadRunCheckpoint(stateRoot, runID string) (*RunCheckpoint, error) {
	path := RunCheckpointPath(stateRoot, runID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var checkpoint RunCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("解析 run checkpoint 失败: %w", err)
	}
	if checkpoint.RunID == "" {
		checkpoint.RunID = runID
	}
	if checkpoint.Schema == 0 {
		checkpoint.Schema = RunCheckpointSchema
	}
	if checkpoint.Stages == nil {
		checkpoint.Stages = map[string]StageCheckpoint{}
	}
	checkpoint.Profiles = normalizedProfileNames(checkpoint.Profiles)
	return &checkpoint, nil
}

func FindRunCheckpoints(stateRoot string) ([]*RunCheckpoint, error) {
	entries, err := os.ReadDir(filepath.Join(stateRoot, "runs"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("扫描 run checkpoints 失败: %w", err)
	}
	checkpoints := make([]*RunCheckpoint, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		checkpoint, err := LoadRunCheckpoint(stateRoot, entry.Name())
		if err == nil {
			checkpoints = append(checkpoints, checkpoint)
		}
	}
	sort.Slice(checkpoints, func(i, j int) bool { return checkpoints[i].UpdatedAt > checkpoints[j].UpdatedAt })
	return checkpoints, nil
}

func StageKey(stage, profile string) string {
	profile = strings.TrimSpace(profile)
	if profile == "" || profile == "default" {
		if stage == "deploy" || stage == "verify" {
			return stage
		}
		return stage + ":default"
	}
	return stage + ":" + profile
}

func (c *RunCheckpoint) StageStatus(stage, profile string) string {
	if c == nil {
		return StagePending
	}
	status := c.Stages[StageKey(stage, profile)].Status
	if status == "" {
		return StagePending
	}
	return status
}

func (c *RunCheckpoint) MarkStage(stage, profile, status, message string) {
	if c == nil {
		return
	}
	if c.Stages == nil {
		c.Stages = map[string]StageCheckpoint{}
	}
	key := StageKey(stage, profile)
	entry := c.Stages[key]
	now := time.Now().UTC().Format(time.RFC3339)
	entry.Status = status
	switch status {
	case StageRunning:
		entry.StartedAt = now
		entry.FinishedAt = ""
		entry.Error = ""
	case StageSucceeded:
		entry.FinishedAt = now
		entry.Error = ""
	default:
		entry.FinishedAt = now
		entry.Error = strings.TrimSpace(message)
	}
	c.Stages[key] = entry
}

// MarkRunningInterrupted 将进程崩溃留下的 running 阶段转换为可恢复状态。
func (c *RunCheckpoint) MarkRunningInterrupted() bool {
	if c == nil {
		return false
	}
	changed := false
	for key, entry := range c.Stages {
		if entry.Status != StageRunning {
			continue
		}
		entry.Status = StageInterrupted
		entry.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		entry.Error = "进程在该阶段完成前中断"
		c.Stages[key] = entry
		changed = true
	}
	return changed
}

func (c *RunCheckpoint) SyncArtifacts(manifest *ReleaseManifest) {
	if c == nil || manifest == nil {
		return
	}
	c.Artifacts = append([]ArtifactRecord(nil), manifest.Artifacts...)
}

// ConfigFingerprint 返回有效配置的稳定摘要，用于拒绝跨 recipe 的危险复用。
func ConfigFingerprint(cfg *Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config 为空")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("序列化 config fingerprint 失败: %w", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func normalizedProfileNames(profiles []string) []string {
	seen := make(map[string]struct{}, len(profiles))
	result := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		profile = strings.TrimSpace(profile)
		if profile == "" {
			profile = "default"
		}
		if _, exists := seen[profile]; exists {
			continue
		}
		seen[profile] = struct{}{}
		result = append(result, profile)
	}
	sort.Strings(result)
	return result
}

// SameProfileNames 比较 profile 集合，不依赖原始顺序。
func SameProfileNames(left, right []string) bool {
	left = normalizedProfileNames(left)
	right = normalizedProfileNames(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
