package executor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// ClaudeExecutor runs tasks via the Claude Code CLI.
type ClaudeExecutor struct {
	ClaudePath string
	WorkDir    string
	Env        map[string]string
}

func (e *ClaudeExecutor) Execute(ctx context.Context, req Request, onProgress ProgressFunc) (*Result, error) {
	// Write prompt to file (avoids CLI arg length limits)
	promptPath, err := WritePromptFile(e.WorkDir, req.TaskID, req.Prompt)
	if err != nil {
		return nil, fmt.Errorf("writing prompt: %w", err)
	}
	defer CleanupPromptFile(e.WorkDir, req.TaskID)

	// Build command arguments
	args := []string{
		"-p",
		"--verbose",
		"--output-format", "stream-json",
	}

	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	if req.SessionID != "" {
		args = append(args, "--resume", req.SessionID)
	}

	for _, tool := range req.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	cmd := exec.CommandContext(ctx, e.ClaudePath, args...)

	if req.ProjectPath != "" {
		cmd.Dir = req.ProjectPath
	}

	// Environment
	cmd.Env = os.Environ()
	for k, v := range e.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Pipe prompt via stdin
	promptFile, err := os.Open(promptPath)
	if err != nil {
		return nil, fmt.Errorf("opening prompt file: %w", err)
	}
	defer func() { _ = promptFile.Close() }()
	cmd.Stdin = promptFile

	// Capture stdout (stream-json) and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	// Start the process
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting claude: %w", err)
	}

	slog.Info("claude code started",
		"task_id", req.TaskID,
		"pid", cmd.Process.Pid)

	if onProgress != nil {
		onProgress("started", fmt.Sprintf("PID %d", cmd.Process.Pid))
	}

	// Parse stream-json output in background goroutines.
	// WaitGroup ensures parseStream finishes writing to result before we read it.
	var wg sync.WaitGroup
	result := &Result{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.parseStream(req.TaskID, stdout, result, onProgress)
	}()
	go e.captureStderr(req.TaskID, stderr)

	// Wait for the process to complete, then wait for stream parsing to finish
	waitErr := cmd.Wait()
	wg.Wait()
	result.Duration = time.Since(start)

	if waitErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](waitErr); ok {
			result.ExitCode = exitErr.ExitCode()
			slog.Warn("claude code exited with error",
				"task_id", req.TaskID,
				"exit_code", result.ExitCode,
				"duration", result.Duration)
			return result, fmt.Errorf("claude exited with code %d: %w", result.ExitCode, waitErr)
		}
		return result, fmt.Errorf("waiting for claude: %w", waitErr)
	}

	slog.Info("claude code completed",
		"task_id", req.TaskID,
		"duration", result.Duration,
		"cost_usd", result.CostUSD,
		"turns", result.Turns)

	return result, nil
}

func (e *ClaudeExecutor) parseStream(taskID string, r io.Reader, result *Result, onProgress ProgressFunc) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		event, err := ParseStreamLine(scanner.Bytes())
		if err != nil {
			slog.Debug("stream parse error", "task_id", taskID, "error", err)
			continue
		}
		if event == nil {
			continue
		}

		switch event.Type {
		case "system":
			if event.Subtype == "init" && event.SessionID != "" {
				result.SessionID = event.SessionID
				slog.Debug("session initialized", "task_id", taskID, "session_id", event.SessionID)
			}

		case "assistant":
			output := ExtractOutput(event)
			if output != "" {
				result.Output += output
			}
			if onProgress != nil {
				if progress := ExtractProgress(event); progress != "" {
					onProgress("progress", progress)
				}
			}

		case "result":
			result.CostUSD = event.CostUSD
			result.Turns = event.NumTurns
			if event.Duration > 0 {
				result.Duration = time.Duration(event.Duration) * time.Millisecond
			}
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("stream scanner error", "task_id", taskID, "error", err)
	}
}

func (e *ClaudeExecutor) captureStderr(taskID string, r io.Reader) {
	data, err := io.ReadAll(r)
	if err != nil {
		slog.Debug("stderr read error", "task_id", taskID, "error", err)
		return
	}
	if len(data) > 0 {
		slog.Debug("claude stderr", "task_id", taskID, "stderr", truncateStr(string(data), 500))
	}
}

// GracefulKill sends SIGTERM, waits, then SIGKILL if the process is still alive.
func GracefulKill(pid int) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}

	_ = proc.Signal(syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_, _ = proc.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = proc.Kill()
	}
}
