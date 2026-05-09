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

	// 新增样式
	HeaderStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true) // 蓝色粗体，section header
	StepNumStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))           // 灰色，步骤编号
	SuccessTagStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true) // 绿色粗体，成功标签
	ErrorTagStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)  // 红色粗体，错误标签
	TableHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true) // 青色粗体，表头
)
