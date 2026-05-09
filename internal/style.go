package internal

import "github.com/pterm/pterm"

type renderStyle struct {
	style *pterm.Style
}

func newRenderStyle(colors ...pterm.Color) renderStyle {
	return renderStyle{style: pterm.NewStyle(colors...)}
}

func (s renderStyle) Render(text string) string {
	return s.style.Sprint(text)
}

// 终端输出样式。保留这一层只是为了让命令层少改代码，
// 底层实现已经切到 PTerm，避免继续依赖旧的手写/旧版 Charm 样式链。
var (
	SuccessStyle = newRenderStyle(pterm.FgLightGreen)
	ErrorStyle   = newRenderStyle(pterm.FgLightRed)
	InfoStyle    = newRenderStyle(pterm.FgLightCyan)
	WarnStyle    = newRenderStyle(pterm.FgYellow)
	StepStyle    = newRenderStyle(pterm.FgLightBlue)
	DimStyle     = newRenderStyle(pterm.FgGray)
	SpinnerStyle = newRenderStyle(pterm.FgLightBlue)
	BoldStyle    = newRenderStyle(pterm.Bold)

	HeaderStyle      = newRenderStyle(pterm.FgLightBlue, pterm.Bold)
	StepNumStyle     = newRenderStyle(pterm.FgGray)
	SuccessTagStyle  = newRenderStyle(pterm.FgLightGreen, pterm.Bold)
	ErrorTagStyle    = newRenderStyle(pterm.FgLightRed, pterm.Bold)
	TableHeaderStyle = newRenderStyle(pterm.FgLightCyan, pterm.Bold)
)
