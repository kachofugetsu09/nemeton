package runner

import (
	"context"
	"fmt"
	"strings"
)

type OpenCode struct{}

func (OpenCode) Execute(ctx context.Context, request Request, emit func(Delta)) (Result, error) {
	version, err := executableVersion(ctx, "opencode")
	if err != nil {
		return Result{}, err
	}
	args := []string{"run", "--format", "json", "--dangerously-skip-permissions", "--dir", request.Workdir}
	if request.Model != "" {
		args = append(args, "--model", request.Model)
	}
	if variant := request.Options["variant"]; variant != "" {
		args = append(args, "--variant", variant)
	}
	if request.SessionID != "" {
		args = append(args, "--session", request.SessionID)
	}
	command := append([]string{"opencode"}, args...)
	command = append(command, "<prompt:runner_input>")
	args = append(args, request.Prompt)
	var sessionID string
	var output strings.Builder
	var protocolError string
	openStep := false
	stepHasContinuationTool := false
	awaitingContinuation := false
	process, runErr := runStreamed(ctx, "opencode", args, []string{
		"PWD=" + request.Workdir,
		`OPENCODE_CONFIG_CONTENT={"permission":"allow"}`,
	}, request.Workdir, func(line []byte) {
		value := decodeLine(line)
		if value == nil {
			emit(Delta{Provider: "opencode", Type: "raw", Content: string(line)})
			if protocolError == "" {
				protocolError = "invalid OpenCode NDJSON event"
			}
			return
		}
		if id := stringField(value, "sessionID"); id != "" && sessionID == "" {
			sessionID = id
			if request.PinSession != nil && sessionID != request.SessionID {
				if err := request.PinSession(sessionID); err != nil {
					emit(Delta{Provider: "opencode", Type: "session_persist_failed", Content: err.Error()})
				}
			}
		}
		typeName := stringField(value, "type")
		part := nestedMap(value, "part")
		switch typeName {
		case "step_start":
			openStep = true
			stepHasContinuationTool = false
			awaitingContinuation = false
		case "step_finish":
			openStep = false
			reason := stringField(part, "reason")
			awaitingContinuation = reason == "tool-calls" || (reason != "" && stepHasContinuationTool)
			stepHasContinuationTool = false
		case "tool_use":
			metadata := nestedMap(part, "metadata")
			providerExecuted, _ := metadata["providerExecuted"].(bool)
			if !providerExecuted {
				stepHasContinuationTool = true
			}
		case "text":
			output.WriteString(stringField(part, "text"))
		case "error":
			if protocolError == "" {
				protocolError = string(line)
			}
		}
		emit(Delta{Provider: "opencode", Type: typeName, Content: string(line)})
	})
	result := Result{Provider: "opencode", Version: version, Command: command, SessionID: sessionID,
		Output: strings.TrimSpace(output.String()), RawStream: process.stdout,
		Stderr: string(process.stderr), ExitCode: process.exitCode,
		StartedAt: process.started, FinishedAt: process.finished}
	if sessionID == "" {
		result.SessionID = request.SessionID
	}
	if runErr != nil {
		return result, fmt.Errorf("OpenCode %s: %w", version, runErr)
	}
	if protocolError != "" {
		return result, fmt.Errorf("OpenCode %s emitted an error event: %s", version, protocolError)
	}
	if openStep {
		return result, fmt.Errorf("OpenCode %s stream ended with an open step", version)
	}
	if awaitingContinuation {
		return result, fmt.Errorf("OpenCode %s stream ended before the required continuation step", version)
	}
	if result.Output == "" {
		return result, fmt.Errorf("OpenCode %s returned empty final output", version)
	}
	return result, nil
}
