package internal

import "fmt"

// ANSI 转义序列，用于终端进度条控制
const (
	ansiSave    = "\033[s"    // 保存光标位置
	ansiRestore = "\033[u"    // 恢复光标位置
	ansiClearLine = "\033[2K" // 清除当前行
)

var progressCurrentStep int
var progressTotalSteps int
var progressStepName string
var progressSubStep string

// ProgressInit 初始化进度条，打印总步骤数
func ProgressInit(total int) {
	progressTotalSteps = total
	progressCurrentStep = 0
	progressStepName = ""
	progressSubStep = ""
	fmt.Printf("  %s\n", DimStyle.Render(fmt.Sprintf("共 %d 个阶段", total)))
}

// ProgressStep 更新大步骤（如 构建镜像、打 Tag）
func ProgressStep(step int, name string) {
	progressCurrentStep = step
	progressStepName = name
	progressSubStep = ""
	printProgress()
}

// ProgressSub 更新子步骤（如 bun run build、docker buildx build）
func ProgressSub(label string) {
	progressSubStep = label
	printProgress()
}

// ProgressDone 完成子步骤，回到进度条位置刷新
func ProgressDone() {
	fmt.Print(ansiRestore)
	fmt.Print(ansiClearLine)
	printProgress()
}

// ProgressFinish 所有步骤完成，清除进度条
func ProgressFinish() {
	fmt.Print(ansiRestore)
	fmt.Print(ansiClearLine)
}

// printProgress 在当前位置打印进度条并保存光标
func printProgress() {
	header := HeaderStyle.Render(fmt.Sprintf("[%d/%d]", progressCurrentStep, progressTotalSteps))
	step := BoldStyle.Render(progressStepName)

	line := fmt.Sprintf("  %s %s", header, step)
	if progressSubStep != "" {
		line += fmt.Sprintf("  %s %s", DimStyle.Render("·"), progressSubStep)
	}

	fmt.Print(ansiSave)
	fmt.Print(ansiClearLine)
	fmt.Print(line)
	fmt.Print(ansiRestore)
}
