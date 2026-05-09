package internal

import (
	"fmt"
	"sort"

	"github.com/pterm/pterm"
)

var progressTotalSteps int

// PrintBanner 输出一级标题，用于 run/history 等命令的整体抬头。
func PrintBanner(title string) {
	pterm.Println()
	pterm.DefaultHeader.Println(title)
}

// ProgressInit 初始化阶段输出。
func ProgressInit(total int) {
	progressTotalSteps = total
	PrintBanner(fmt.Sprintf("共 %d 个阶段", total))
}

// ProgressStep 输出当前大步骤。
func ProgressStep(step int, name string) {
	pterm.DefaultSection.Println(fmt.Sprintf("[%d/%d] %s", step, progressTotalSteps, name))
}

// ProgressSub 输出子步骤说明。
func ProgressSub(label string) {
	pterm.Info.Println(label)
}

// PrintSuccess 输出成功提示。
func PrintSuccess(message string) {
	pterm.Success.Println(message)
}

// PrintWarning 输出警告提示。
func PrintWarning(message string) {
	pterm.Warning.Println(message)
}

// PrintInfo 输出说明提示。
func PrintInfo(message string) {
	pterm.Info.Println(message)
}

// PrintKeyValueTable 用表格展示键值对结果，适合 init 探测信息等静态输出。
func PrintKeyValueTable(rows map[string]string) {
	keys := make([]string, 0, len(rows))
	for key, value := range rows {
		if value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return
	}

	data := pterm.TableData{{"项目", "结果"}}
	for _, key := range keys {
		data = append(data, []string{key, rows[key]})
	}
	pterm.DefaultTable.WithHasHeader().WithData(data).Render()
}
