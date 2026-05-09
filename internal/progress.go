package internal

import "fmt"

var progressCurrentStep int
var progressTotalSteps int
var progressStepName string

// ProgressInit 初始化进度条，打印总步骤数
func ProgressInit(total int) {
	progressTotalSteps = total
	progressCurrentStep = 0
	progressStepName = ""
	fmt.Printf("  %s\n", DimStyle.Render(fmt.Sprintf("共 %d 个阶段", total)))
}

// ProgressStep 更新大步骤（如 构建镜像、打 Tag）
// 打印一行进度信息：  [1/4] 构建镜像
func ProgressStep(step int, name string) {
	progressCurrentStep = step
	progressStepName = name
	header := HeaderStyle.Render(fmt.Sprintf("[%d/%d]", progressCurrentStep, progressTotalSteps))
	fmt.Printf("\n  %s %s\n", header, BoldStyle.Render(name))
}

// ProgressSub 打印子步骤信息（如 bun run build、docker buildx build）
func ProgressSub(label string) {
	fmt.Printf("  %s %s\n", DimStyle.Render("·"), label)
}
