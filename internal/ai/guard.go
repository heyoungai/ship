package ai

import (
	"fmt"
	"regexp"
	"strings"
)

// Dangerous ship subcommands blocked in bash (release side effects).
var blockedShipSub = regexp.MustCompile(`(?i)(?:^|[;&|]|&&|\|\|)\s*(?:(?:\./)?ship|ship\.exe)\s+(deploy|run|push|rollback)\b`)

// GuardBash returns an error message if the command is blocked.
func GuardBash(command string) error {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return fmt.Errorf("bash command is empty")
	}
	if blockedShipSub.MatchString(cmd) {
		return fmt.Errorf("blocked: ship deploy/run/push/rollback are not allowed in the advisor; ask the user to run them")
	}
	// Also catch bare forms without leading ship path quirks: "ship deploy"
	lower := strings.ToLower(cmd)
	for _, bad := range []string{"ship deploy", "ship run", "ship push", "ship rollback", "ship.exe deploy", "ship.exe run", "ship.exe push", "ship.exe rollback"} {
		if strings.Contains(lower, bad) {
			return fmt.Errorf("blocked: ship deploy/run/push/rollback are not allowed in the advisor; ask the user to run them")
		}
	}
	return nil
}
