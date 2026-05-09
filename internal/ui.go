package internal

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pterm/pterm"
)

var progressTotalSteps int

// SetProgressTotal 仅设置阶段总数，不额外输出内容。
func SetProgressTotal(total int) {
	progressTotalSteps = total
}

// PrintBanner 输出一级标题，用于 run/history 等命令的整体抬头。
func PrintBanner(title string) {
	pterm.Println()
	pterm.DefaultHeader.Println(title)
}

// ProgressInit 初始化阶段输出。
func ProgressInit(total int) {
	SetProgressTotal(total)
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

// PrintRunPlan 用面板展示 run 命令的执行上下文，避免开场信息散落在多行里。
func PrintRunPlan(version string, profiles []string, envFile string, deployEnabled bool, steps []string) {
	deployText := "关闭"
	if deployEnabled {
		deployText = "开启"
	}
	envText := envFile
	if strings.TrimSpace(envText) == "" {
		envText = "(使用配置默认值)"
	}
	profileText := strings.Join(profiles, ", ")
	if strings.TrimSpace(profileText) == "" {
		profileText = "default"
	}

	summary := pterm.DefaultBox.WithTitle("执行信息").Sprint(strings.Join([]string{
		fmt.Sprintf("版本: %s", version),
		fmt.Sprintf("Profiles: %s", profileText),
		fmt.Sprintf("Env File: %s", envText),
		fmt.Sprintf("远程部署: %s", deployText),
	}, "\n"))
	stepLines := make([]string, len(steps))
	for i, step := range steps {
		stepLines[i] = fmt.Sprintf("%d. %s", i+1, step)
	}
	stepPanel := pterm.DefaultBox.WithTitle("执行计划").Sprint(strings.Join(stepLines, "\n"))

	_ = pterm.DefaultPanel.WithPanels(pterm.Panels{
		{
			{Data: summary},
			{Data: stepPanel},
		},
	}).WithPadding(2).Render()
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
