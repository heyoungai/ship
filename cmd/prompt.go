package cmd

import (
	"errors"
	"fmt"
	"os"

	"charm.land/huh/v2"
	"golang.org/x/term"
)

var assumeYes bool

var errNonInteractivePrompt = errors.New("当前终端不可交互，请使用 --yes 明确确认")

func init() {
	rootCmd.PersistentFlags().BoolVarP(&assumeYes, "yes", "y", false, "自动确认交互提示")
}

func confirmAction(title string) (bool, error) {
	if assumeYes {
		return true, nil
	}
	if !isInteractiveTerminal() {
		return false, errNonInteractivePrompt
	}

	confirmed := false
	if err := huh.NewConfirm().
		Title(title).
		Affirmative("继续").
		Negative("取消").
		Value(&confirmed).
		Run(); err != nil {
		return false, fmt.Errorf("交互确认失败: %w", err)
	}

	return confirmed, nil
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
