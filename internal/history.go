package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/pterm/pterm"
)

const historyDir = ".ship"
const historyFile = ".ship/history.json"

// HistoryEntry 记录一次部署操作
type HistoryEntry struct {
	Version string `json:"version"`
	Time    string `json:"time"`
	Action  string `json:"action"` // deploy | rollback
	Result  string `json:"result"` // success | fail
	Note    string `json:"note,omitempty"`
}

// LoadHistory 读取部署历史记录。
func LoadHistory() ([]HistoryEntry, error) {
	return loadHistoryUnlocked()
}

// RecordDeployment 记录一次部署操作到历史。
// 使用文件锁避免多个 ship 进程并发写入时丢失记录。
func RecordDeployment(version, action, result, note string) error {
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("创建历史目录失败: %w", err)
	}

	lock := flock.New(historyFile + ".lock")
	locked, err := lock.TryLock()
	if err != nil {
		return fmt.Errorf("获取历史文件锁失败: %w", err)
	}
	if !locked {
		return fmt.Errorf("历史文件被其他进程锁定，请稍后重试")
	}
	defer lock.Unlock()

	entries, err := loadHistoryUnlocked()
	if err != nil {
		return err
	}
	entries = append(entries, HistoryEntry{
		Version: version,
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		Action:  action,
		Result:  result,
		Note:    note,
	})

	// 保留最近 100 条
	if len(entries) > 100 {
		entries = entries[len(entries)-100:]
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化部署历史失败: %w", err)
	}
	if err := os.WriteFile(historyFile, data, 0644); err != nil {
		return fmt.Errorf("写入部署历史失败: %w", err)
	}
	return nil
}

// loadHistoryUnlocked 读取历史记录（无锁版本，供持锁时调用）。
func loadHistoryUnlocked() ([]HistoryEntry, error) {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取部署历史失败: %w", err)
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("解析部署历史失败: %w", err)
	}
	return entries, nil
}

// GetPreviousVersion 获取可回滚的上一个版本
// 优先从历史记录找，找不到则从 git tag 推断
func GetPreviousVersion(currentVersion string) (string, error) {
	entries, err := LoadHistory()
	if err != nil {
		return "", err
	}

	// 从后往前找第一个成功部署且不是当前版本的记录
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Result == "success" && entries[i].Version != currentVersion {
			return entries[i].Version, nil
		}
	}

	// 历史中找不到，尝试获取上一个 git tag
	return getPreviousGitTag(currentVersion)
}

// getPreviousGitTag 获取指定 tag 之前的上一个 git tag
func getPreviousGitTag(current string) (string, error) {
	tags, err := getSortedTags()
	if err != nil {
		return "", fmt.Errorf("获取 git tag 列表失败: %w", err)
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("没有可回滚的版本")
	}

	for i, tag := range tags {
		if tag == current && i+1 < len(tags) {
			return tags[i+1], nil
		}
		if tag == current {
			return "", fmt.Errorf("当前版本 %s 已是最早的 git tag，无法继续回滚", current)
		}
	}

	return "", fmt.Errorf("当前版本 %s 不在 git tag 列表中，无法推断回滚版本", current)
}

// getSortedTags 获取按版本号倒序排列的所有 git tag
func getSortedTags() ([]string, error) {
	out, err := exec.Command("git", "tag", "--sort=-v:refname").Output()
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			tags = append(tags, line)
		}
	}
	return tags, nil
}

// FormatHistory 格式化历史记录用于终端显示
func FormatHistory(entries []HistoryEntry, limit int) string {
	if len(entries) == 0 {
		return "暂无部署记录"
	}

	start := 0
	if limit > 0 && limit < len(entries) {
		start = len(entries) - limit
	}

	data := pterm.TableData{{"时间", "操作", "版本", "备注"}}
	for _, e := range entries[start:] {
		action := "部署"
		if e.Result == "fail" {
			action = "失败部署"
		}
		if e.Action == "rollback" {
			action = "回滚"
			if e.Result == "fail" {
				action = "失败回滚"
			}
		}
		note := e.Note
		if note == "" {
			note = "-"
		}
		data = append(data, []string{e.Time, action, e.Version, note})
	}

	rendered, err := pterm.DefaultTable.WithHasHeader().WithData(data).Srender()
	if err != nil {
		return fmt.Sprintf("渲染部署历史失败: %v", err)
	}
	return rendered
}
