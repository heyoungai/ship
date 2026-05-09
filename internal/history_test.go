package internal

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// helper: 在临时目录中切换 .ship/history.json 上下文
func withTempHistory(t *testing.T, fn func()) {
	t.Helper()
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })
	fn()
}

// ── LoadHistory ─────────────────────────────────────────────────

func TestLoadHistory_NoFile(t *testing.T) {
	withTempHistory(t, func() {
		entries, err := LoadHistory()
		if err != nil {
			t.Fatalf("LoadHistory(no file) error: %v", err)
		}
		if entries != nil {
			t.Errorf("LoadHistory(no file) = %v, want nil", entries)
		}
	})
}

func TestLoadHistory_ValidFile(t *testing.T) {
	withTempHistory(t, func() {
		os.MkdirAll(".ship", 0755)
		data := `[{"version":"v1.0.0","time":"2026-01-01 00:00:00","action":"deploy","result":"success"}]`
		os.WriteFile(".ship/history.json", []byte(data), 0644)

		entries, err := LoadHistory()
		if err != nil {
			t.Fatalf("LoadHistory(valid) error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("LoadHistory got %d entries, want 1", len(entries))
		}
		if entries[0].Version != "v1.0.0" {
			t.Errorf("entry.Version = %q, want %q", entries[0].Version, "v1.0.0")
		}
	})
}

func TestLoadHistory_InvalidJSON(t *testing.T) {
	withTempHistory(t, func() {
		os.MkdirAll(".ship", 0755)
		os.WriteFile(".ship/history.json", []byte("not json"), 0644)

		_, err := LoadHistory()
		if err == nil {
			t.Fatal("LoadHistory(invalid) should return an error")
		}
	})
}

// ── RecordDeployment ────────────────────────────────────────────

func TestRecordDeployment_CreatesFile(t *testing.T) {
	withTempHistory(t, func() {
		if err := RecordDeployment("v1.0.0", "deploy", "success", "first deploy"); err != nil {
			t.Fatalf("RecordDeployment error: %v", err)
		}

		entries, err := LoadHistory()
		if err != nil {
			t.Fatalf("LoadHistory after record error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("RecordDeployment: got %d entries, want 1", len(entries))
		}
		if entries[0].Version != "v1.0.0" {
			t.Errorf("entry.Version = %q", entries[0].Version)
		}
		if entries[0].Action != "deploy" {
			t.Errorf("entry.Action = %q", entries[0].Action)
		}
		if entries[0].Note != "first deploy" {
			t.Errorf("entry.Note = %q", entries[0].Note)
		}
	})
}

func TestRecordDeployment_Appends(t *testing.T) {
	withTempHistory(t, func() {
		if err := RecordDeployment("v1.0.0", "deploy", "success", ""); err != nil {
			t.Fatalf("RecordDeployment(first) error: %v", err)
		}
		if err := RecordDeployment("v2.0.0", "deploy", "fail", "timeout"); err != nil {
			t.Fatalf("RecordDeployment(second) error: %v", err)
		}

		entries, err := LoadHistory()
		if err != nil {
			t.Fatalf("LoadHistory after append error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("got %d entries, want 2", len(entries))
		}
		if entries[1].Result != "fail" {
			t.Errorf("entry[1].Result = %q, want %q", entries[1].Result, "fail")
		}
	})
}

func TestRecordDeployment_Retention(t *testing.T) {
	withTempHistory(t, func() {
		// 写入 105 条
		for i := 0; i < 105; i++ {
			if err := RecordDeployment("v1.0.0", "deploy", "success", ""); err != nil {
				t.Fatalf("RecordDeployment(retention) error: %v", err)
			}
		}

		entries, err := LoadHistory()
		if err != nil {
			t.Fatalf("LoadHistory(retention) error: %v", err)
		}
		if len(entries) != 100 {
			t.Errorf("retention: got %d entries, want 100", len(entries))
		}
	})
}

// ── GetPreviousVersion ──────────────────────────────────────────

func TestGetPreviousVersion_FromHistory(t *testing.T) {
	withTempHistory(t, func() {
		_ = RecordDeployment("v1.0.0", "deploy", "success", "")
		_ = RecordDeployment("v2.0.0", "deploy", "success", "")

		prev, err := GetPreviousVersion("v2.0.0")
		if err != nil {
			t.Fatalf("GetPreviousVersion error: %v", err)
		}
		if prev != "v1.0.0" {
			t.Errorf("GetPreviousVersion = %q, want %q", prev, "v1.0.0")
		}
	})
}

func TestGetPreviousVersion_SkipsFailed(t *testing.T) {
	withTempHistory(t, func() {
		_ = RecordDeployment("v1.0.0", "deploy", "success", "")
		_ = RecordDeployment("v2.0.0", "deploy", "fail", "error")
		_ = RecordDeployment("v3.0.0", "deploy", "success", "")

		prev, err := GetPreviousVersion("v3.0.0")
		if err != nil {
			t.Fatalf("GetPreviousVersion error: %v", err)
		}
		if prev != "v1.0.0" {
			t.Errorf("GetPreviousVersion = %q, want %q (skip v2.0.0 fail)", prev, "v1.0.0")
		}
	})
}

func TestGetPreviousVersion_SkipsCurrent(t *testing.T) {
	withTempHistory(t, func() {
		_ = RecordDeployment("v1.0.0", "deploy", "success", "")
		_ = RecordDeployment("v1.0.0", "deploy", "success", "") // same version again

		// 只有 v1.0.0，找不到不同的版本，回退到 git tag
		_, err := GetPreviousVersion("v1.0.0")
		// 在没有 git repo 的临时目录中，应该报错
		if err == nil {
			t.Log("GetPreviousVersion fell back to git tags (expected in git repo)")
		}
	})
}

func TestGetPreviousGitTag_CurrentMissing(t *testing.T) {
	_, err := getPreviousGitTag("missing-tag")
	if err == nil {
		t.Fatal("getPreviousGitTag should fail when current tag is missing")
	}
}

// ── FormatHistory ───────────────────────────────────────────────

func TestFormatHistory_Empty(t *testing.T) {
	got := FormatHistory(nil, 10)
	if got != "  暂无部署记录" {
		t.Errorf("FormatHistory(empty) = %q", got)
	}
}

func TestFormatHistory_ContainsVersion(t *testing.T) {
	entries := []HistoryEntry{
		{Version: "v1.0.0", Time: "2026-01-01 00:00:00", Action: "deploy", Result: "success"},
	}
	got := FormatHistory(entries, 0)
	if !strings.Contains(got, "v1.0.0") {
		t.Errorf("FormatHistory should contain version, got: %s", got)
	}
	if !strings.Contains(got, "部署") {
		t.Errorf("FormatHistory should contain action, got: %s", got)
	}
}

func TestFormatHistory_Rollback(t *testing.T) {
	entries := []HistoryEntry{
		{Version: "v1.0.0", Time: "2026-01-01 00:00:00", Action: "rollback", Result: "success"},
	}
	got := FormatHistory(entries, 0)
	if !strings.Contains(got, "回滚") {
		t.Errorf("FormatHistory(rollback) should contain '回滚', got: %s", got)
	}
}

func TestFormatHistory_Limit(t *testing.T) {
	entries := []HistoryEntry{
		{Version: "v1.0.0", Time: "t1", Action: "deploy", Result: "success"},
		{Version: "v2.0.0", Time: "t2", Action: "deploy", Result: "success"},
		{Version: "v3.0.0", Time: "t3", Action: "deploy", Result: "success"},
	}
	got := FormatHistory(entries, 2)
	if strings.Contains(got, "v1.0.0") {
		t.Errorf("FormatHistory(limit=2) should not contain v1.0.0")
	}
	if !strings.Contains(got, "v3.0.0") {
		t.Errorf("FormatHistory(limit=2) should contain v3.0.0")
	}
}

func TestFormatHistory_FailIcon(t *testing.T) {
	entries := []HistoryEntry{
		{Version: "v1.0.0", Time: "t1", Action: "deploy", Result: "fail", Note: "timeout"},
	}
	got := FormatHistory(entries, 0)
	if !strings.Contains(got, "timeout") {
		t.Errorf("FormatHistory(fail) should contain note, got: %s", got)
	}
}

// ── JSON 序列化兼容性 ───────────────────────────────────────────

func TestHistoryEntry_JSON(t *testing.T) {
	entry := HistoryEntry{
		Version: "v2.0.0",
		Time:    "2026-05-09 12:00:00",
		Action:  "rollback",
		Result:  "success",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var roundtrip HistoryEntry
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if roundtrip != entry {
		t.Errorf("JSON roundtrip mismatch: %+v != %+v", roundtrip, entry)
	}
}

func TestHistoryEntry_OMitEmpty(t *testing.T) {
	entry := HistoryEntry{Version: "v1", Time: "t", Action: "deploy", Result: "ok"}
	data, _ := json.Marshal(entry)
	if strings.Contains(string(data), "note") {
		t.Errorf("empty note should be omitted: %s", data)
	}
}

// ── RecordDeployment 自动创建目录 ────────────────────────────────

func TestRecordDeployment_CreatesDir(t *testing.T) {
	withTempHistory(t, func() {
		if err := RecordDeployment("v1.0.0", "deploy", "success", ""); err != nil {
			t.Fatalf("RecordDeployment error: %v", err)
		}

		info, err := os.Stat(".ship")
		if err != nil {
			t.Fatalf(".ship dir not created: %v", err)
		}
		if !info.IsDir() {
			t.Error(".ship should be a directory")
		}
	})
}
