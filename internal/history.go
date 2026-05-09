package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
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

// LoadHistory 读取部署历史记录
func LoadHistory() []HistoryEntry {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return nil
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

// RecordDeployment 记录一次部署操作到历史
func RecordDeployment(version, action, result, note string) {
	entries := LoadHistory()
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

	_ = os.MkdirAll(historyDir, 0755)
	data, _ := json.MarshalIndent(entries, "", "  ")
	_ = os.WriteFile(historyFile, data, 0644)
}

// GetPreviousVersion 获取可回滚的上一个版本
// 优先从历史记录找，找不到则从 git tag 推断
func GetPreviousVersion(currentVersion string) (string, error) {
	entries := LoadHistory()

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

	for i, tag := range tags {
		if tag == current && i+1 < len(tags) {
			return tags[i+1], nil
		}
	}

	if len(tags) > 0 {
		return tags[0], nil
	}

	return "", fmt.Errorf("没有可回滚的版本")
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
		return DimStyle.Render("  暂无部署记录")
	}

	start := 0
	if limit > 0 && limit < len(entries) {
		start = len(entries) - limit
	}

	var rows [][]string
	for _, e := range entries[start:] {
		icon := SuccessStyle.Render("✔")
		action := "部署"
		if e.Result == "fail" {
			icon = ErrorStyle.Render("✖")
		}
		if e.Action == "rollback" {
			action = "回滚"
		}
		note := e.Note
		if note == "" {
			note = DimStyle.Render("-")
		}
		rows = append(rows, []string{e.Time, icon + " " + action, e.Version, note})
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("245"))).
		Headers("时间", "操作", "版本", "备注").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return TableHeaderStyle.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	return t.Render()
}
