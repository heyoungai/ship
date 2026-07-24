package cmd

import (
	"testing"

	"github.com/heyoungai/ship/internal"
	"github.com/spf13/cobra"
)

func TestApplyDockerPullFlag_OnlyWhenChanged(t *testing.T) {
	cfg := &internal.Config{}
	cfg.Build.Docker.Pull = false

	cmd := &cobra.Command{Use: "build"}
	registerDockerPullFlag(cmd)

	if err := applyDockerPullFlag(cmd, cfg); err != nil {
		t.Fatalf("applyDockerPullFlag(unchanged): %v", err)
	}
	if cfg.Build.Docker.Pull {
		t.Fatal("unchanged --pull must not overwrite ship.toml pull=false")
	}

	if err := cmd.Flags().Set("pull", "true"); err != nil {
		t.Fatalf("set pull=true: %v", err)
	}
	if err := applyDockerPullFlag(cmd, cfg); err != nil {
		t.Fatalf("applyDockerPullFlag(changed): %v", err)
	}
	if !cfg.Build.Docker.Pull {
		t.Fatal("explicit --pull=true should override config")
	}
}
