package internal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// RunCmd 执行外部命令，实时输出 stdout/stderr，失败时返回错误
//
// 输出格式：
//   $ docker buildx build ...
//   [执行中] 构建镜像
//     (命令输出逐行显示)
//   [完成]   构建镜像
func RunCmd(args []string, label string) error {
	fmt.Printf("  %s %s\n", DimStyle.Render("$"), DimStyle.Render(strings.Join(args, " ")))
	fmt.Printf("  %s %s\n", InfoStyle.Render("·"), label)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建 stderr 管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动命令失败: %w", err)
	}

	done := make(chan struct{}, 2)
	go streamOutput(stdout, done)
	go streamOutput(stderr, done)
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		fmt.Printf("  %s %s  %v\n", ErrorStyle.Render("✖"), label, err)
		return err
	}

	fmt.Printf("  %s %s\n", SuccessStyle.Render("✔"), label)
	return nil
}

// RunCmdWithEnv 执行外部命令，支持注入额外环境变量
func RunCmdWithEnv(args []string, label string, env map[string]string) error {
	fmt.Printf("  %s %s\n", DimStyle.Render("$"), DimStyle.Render(strings.Join(args, " ")))
	fmt.Printf("  %s %s\n", InfoStyle.Render("·"), label)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建 stderr 管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动命令失败: %w", err)
	}

	done := make(chan struct{}, 2)
	go streamOutput(stdout, done)
	go streamOutput(stderr, done)
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		fmt.Printf("  %s %s  %v\n", ErrorStyle.Render("✖"), label, err)
		return err
	}

	fmt.Printf("  %s %s\n", SuccessStyle.Render("✔"), label)
	return nil
}

// streamOutput 逐行读取 io.Reader 并输出到终端
func streamOutput(r io.Reader, done chan struct{}) {
	defer func() { done <- struct{}{} }()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fmt.Printf("    %s\n", scanner.Text())
	}
}

// GetLatestTag 获取最新的 git tag
func GetLatestTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取 git tag 失败: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveVersion 解析版本号，空字符串时自动获取最新 git tag
func ResolveVersion(version string) (string, error) {
	if version != "" {
		return version, nil
	}
	return GetLatestTag()
}
