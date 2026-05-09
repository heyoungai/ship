package internal

import "github.com/charmbracelet/lipgloss"

// 终端输出样式
var (
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))  // 绿色
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))   // 红色
	InfoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))  // 青色
	WarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))  // 黄色
	StepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))  // 蓝色
	DimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // 灰色
	SpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))  // 蓝色
	BoldStyle    = lipgloss.NewStyle().Bold(true)
)
