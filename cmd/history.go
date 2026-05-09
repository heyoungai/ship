package cmd

import (
	"fmt"
	"ship/internal"

	"github.com/spf13/cobra"
)

var historyLimit int

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "查看部署历史记录",
	Run: func(cmd *cobra.Command, args []string) {
		doHistory()
	},
}

func init() {
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "n", 20, "显示最近 N 条记录")
}

// doHistory 显示部署历史
func doHistory() {
	entries := internal.LoadHistory()
	fmt.Printf("\n  %s\n", internal.HeaderStyle.Render("▸ 部署历史"))
	fmt.Println(internal.FormatHistory(entries, historyLimit))
}
