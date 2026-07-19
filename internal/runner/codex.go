package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type Codex struct{}

func (Codex) Execute(ctx context.Context, request Request, emit func(Delta)) (Result, error) {
	version, err := executableVersion(ctx, "codex")
	if err != nil {
		return Result{}, err
	}
	finalPath, err := outputPath(request.OutputDir, "codex-final")
	if err != nil {
		return Result{}, err
	}
	defer os.Remove(finalPath)
	args := []string{"exec"}
	if request.SessionID != "" {
		args = append(args, "resume", request.SessionID)
	}
	args = append(args, "--json", "--dangerously-bypass-approvals-and-sandbox",
		"--dangerously-bypass-hook-trust", "-o", finalPath)
	if request.Model != "" {
		args = append(args, "--model", request.Model)
	}
	command := append([]string{"codex"}, args...)
	command = append(command, "<prompt:runner_input>")
	args = append(args, request.Prompt)
	var sessionID string
	process, runErr := runStreamed(ctx, "codex", args, []string{"PWD=" + request.Workdir}, request.Workdir, func(line []byte) {
		value := decodeLine(line)
		if value == nil {
			emit(Delta{Provider: "codex", Type: "raw", Content: string(line)})
			return
		}
		if stringField(value, "type") == "thread.started" {
			sessionID = stringField(value, "thread_id")
			if request.PinSession != nil && sessionID != "" && sessionID != request.SessionID {
				if err := request.PinSession(sessionID); err != nil {
					emit(Delta{Provider: "codex", Type: "session_persist_failed", Content: err.Error()})
				}
			}
		}
		emit(Delta{Provider: "codex", Type: stringField(value, "type"), Content: string(line)})
	})
	result := Result{Provider: "codex", Version: version, Command: command, SessionID: sessionID,
		RawStream: process.stdout, Stderr: string(process.stderr), ExitCode: process.exitCode,
		StartedAt: process.started, FinishedAt: process.finished}
	if sessionID == "" {
		result.SessionID = request.SessionID
	}
	if runErr != nil {
		return result, fmt.Errorf("Codex %s: %w", version, runErr)
	}
	output, err := readFinalOutput(finalPath)
	if err != nil {
		return result, err
	}
	result.Output = strings.TrimSpace(output)
	return result, nil
}
