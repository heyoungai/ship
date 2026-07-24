package cmd

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/heyoungai/ship/internal"
	"github.com/heyoungai/ship/internal/ai"
	"github.com/spf13/cobra"
)

var (
	aiPrompt    string
	aiModel     string
	aiBaseURL   string
	aiMaxTurns  int
	aiDryRun    bool
	aiToolTrace bool
)

var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "极简发布顾问 agent（通用工具 + 短 system）",
	Long: `ship ai 是薄 harness：read/write/edit/bash/grep/find，领域靠短 prompt 与现有 ship CLI。

需要 OPENAI_API_KEY（可选 OPENAI_BASE_URL、SHIP_AI_MODEL）。
无密钥时请用确定性 ship init。

交互为线模式 REPL：流式输出助手回复，并默认展示工具调用行。`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(aiPrompt) != "" {
			return runAIPrint(cmd.Context(), aiPrompt)
		}
		return runAIREPL(cmd.Context())
	},
}

var aiInitCmd = &cobra.Command{
	Use:   "init",
	Short: "让顾问为当前目录生成或补全 ship.toml",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		msg := `Explore this project and create or update ship.toml for ship (schema = 2).
Read config.example.toml if present. Prefer existing conventions in the repo.
Leave unknown registry/SSH/remote values as # TODO: comments. Do not invent secrets.
After writing, ensure the file is coherent.`
		if aiDryRun {
			msg += "\nNote: dry-run is enabled; write/edit will not persist files."
		}
		return runAIPrint(cmd.Context(), msg)
	},
}

func init() {
	aiCmd.PersistentFlags().StringVarP(&aiPrompt, "prompt", "p", "", "非交互单次提问（print 模式）")
	aiCmd.PersistentFlags().StringVar(&aiModel, "model", "", "模型名（默认 SHIP_AI_MODEL 或 gpt-4.1-mini）")
	aiCmd.PersistentFlags().StringVar(&aiBaseURL, "base-url", "", "OpenAI-compatible API base（默认 OPENAI_BASE_URL 或官方）")
	aiCmd.PersistentFlags().IntVar(&aiMaxTurns, "max-turns", 20, "单次用户轮次内最大 tool loop 轮数")
	aiCmd.PersistentFlags().BoolVar(&aiDryRun, "dry-run", false, "工具不落盘（write/edit 只预览）")
	aiCmd.PersistentFlags().BoolVar(&aiToolTrace, "trace", false, "额外详细工具输出（工具行默认已开启）")
	aiCmd.AddCommand(aiInitCmd)
}

func newAIAgent() (*ai.Agent, *ai.Provider, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	sb, err := ai.NewSandbox(cwd)
	if err != nil {
		return nil, nil, err
	}
	provider, err := ai.ResolveProviderFromEnv(aiModel, aiBaseURL)
	if err != nil {
		return nil, nil, err
	}
	agentsMD, err := ai.LoadAgentsMD(cwd)
	if err != nil {
		return nil, nil, err
	}
	rep := newAISessionReporter(os.Stdout)
	agent, err := ai.NewAgent(ai.Config{
		Provider:  provider,
		Sandbox:   sb,
		System:    ai.SystemWithAgents(ai.DefaultSystemPrompt, agentsMD),
		MaxTurns:  aiMaxTurns,
		DryRun:    aiDryRun,
		Yes:       assumeYes,
		Writer:    os.Stdout,
		Reporter:  rep,
		ToolTrace: aiToolTrace,
		ConfirmWrite: func(path string) (bool, error) {
			return confirmAction(fmt.Sprintf("顾问将写入 %s，是否继续？", path))
		},
	})
	if err != nil {
		return nil, nil, err
	}
	return agent, provider, nil
}

func printAIBanner(provider *ai.Provider, interactive bool) {
	host := provider.DisplayHost()
	if host == "" {
		host = "api.openai.com"
	}
	// Prefer parsing if user passed a full URL somehow via DisplayHost already stripped.
	if u, err := url.Parse("https://" + host); err == nil && u.Host != "" {
		host = u.Host
	}
	mode := "REPL"
	if !interactive {
		mode = "print"
	}
	line := fmt.Sprintf("ship ai · model=%s · host=%s · mode=%s", provider.Model, host, mode)
	if aiDryRun {
		line += " · dry-run"
	}
	internal.PrintInfo(line)
	if interactive {
		internal.PrintInfo("输入问题；/quit 退出 · /help 帮助")
	}
}

func runAIPrint(ctx context.Context, userText string) error {
	agent, provider, err := newAIAgent()
	if err != nil {
		return err
	}
	if isInteractiveTerminal() {
		printAIBanner(provider, false)
	}
	_, err = agent.RunUser(ctx, userText)
	return err
}

func runAIREPL(ctx context.Context) error {
	agent, provider, err := newAIAgent()
	if err != nil {
		return err
	}
	printAIBanner(provider, true)
	sc := bufio.NewScanner(os.Stdin)
	// allow long pastes
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for {
		fmt.Fprint(os.Stdout, internal.InfoStyle.Render("you")+" › ")
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		switch {
		case line == "/quit" || line == "/exit" || line == "/q":
			return nil
		case line == "/help":
			fmt.Fprintln(os.Stdout, "命令: /quit 退出 · 工具: read write edit bash grep find · ship deploy/run/push/rollback 已拦截")
			fmt.Fprintln(os.Stdout, "输出: 流式助手回复；默认展示工具调用行")
			continue
		}
		if _, err := agent.RunUser(ctx, line); err != nil {
			internal.PrintWarning(err.Error())
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}
