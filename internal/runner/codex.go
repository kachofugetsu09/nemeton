package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Codex struct{}

type appServerResponse struct {
	ID     *int            `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type codexModelList struct {
	Data []struct {
		Model                     string `json:"model"`
		DisplayName               string `json:"displayName"`
		Description               string `json:"description"`
		IsDefault                 bool   `json:"isDefault"`
		DefaultReasoningEffort    string `json:"defaultReasoningEffort"`
		SupportedReasoningEfforts []struct {
			ReasoningEffort string `json:"reasoningEffort"`
		} `json:"supportedReasoningEfforts"`
	} `json:"data"`
	NextCursor *string `json:"nextCursor"`
}

// Catalog reads the authenticated Codex picker catalog through app-server.
func (Codex) Catalog(ctx context.Context) (Catalog, error) {
	// 1. Start the same versioned protocol boundary used by Codex interfaces.
	version, err := executableVersion(ctx, "codex")
	if err != nil {
		return Catalog{}, err
	}
	command := exec.CommandContext(ctx, "codex", "app-server", "--stdio")
	stdin, err := command.StdinPipe()
	if err != nil {
		return Catalog{}, fmt.Errorf("open Codex app-server stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return Catalog{}, fmt.Errorf("open Codex app-server stdout: %w", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return Catalog{}, fmt.Errorf("open Codex app-server stderr: %w", err)
	}
	if err := command.Start(); err != nil {
		return Catalog{}, fmt.Errorf("start Codex app-server: %w", err)
	}
	var stderrBuffer bytes.Buffer
	stderrDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&stderrBuffer, stderr)
		stderrDone <- copyErr
	}()
	finished := false
	defer func() {
		if !finished {
			_ = command.Process.Kill()
			_ = command.Wait()
			<-stderrDone
		}
	}()
	encoder := json.NewEncoder(stdin)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), maximumStreamLine)

	// 2. Complete the required handshake before requesting visible picker models.
	initialize := map[string]any{"method": "initialize", "id": 1, "params": map[string]any{
		"clientInfo": map[string]string{"name": "nemeton", "title": "Nemeton", "version": "0.1.0"},
	}}
	if err := encoder.Encode(initialize); err != nil {
		return Catalog{}, fmt.Errorf("initialize Codex app-server: %w", err)
	}
	if _, err := readAppServerResponse(scanner, 1); err != nil {
		return Catalog{}, err
	}
	if err := encoder.Encode(map[string]string{"method": "initialized"}); err != nil {
		return Catalog{}, fmt.Errorf("acknowledge Codex app-server initialization: %w", err)
	}

	// 3. Preserve the server's model and reasoning-effort order across every page.
	models := make([]Model, 0)
	var cursor *string
	for requestID := 2; ; requestID++ {
		request := map[string]any{"method": "model/list", "id": requestID, "params": map[string]any{
			"cursor": cursor, "limit": 100, "includeHidden": false,
		}}
		if err := encoder.Encode(request); err != nil {
			return Catalog{}, fmt.Errorf("request Codex model catalog: %w", err)
		}
		result, err := readAppServerResponse(scanner, requestID)
		if err != nil {
			return Catalog{}, err
		}
		var page codexModelList
		if err := json.Unmarshal(result, &page); err != nil {
			return Catalog{}, fmt.Errorf("decode Codex model catalog: %w", err)
		}
		for _, item := range page.Data {
			options := make([]string, 0, len(item.SupportedReasoningEfforts))
			for _, option := range item.SupportedReasoningEfforts {
				options = append(options, option.ReasoningEffort)
			}
			models = append(models, Model{ID: item.Model, Label: item.DisplayName,
				Description: item.Description, Default: item.IsDefault,
				OptionValues: options, DefaultOption: item.DefaultReasoningEffort})
		}
		cursor = page.NextCursor
		if cursor == nil {
			break
		}
	}
	if err := validateCodexCatalog(models); err != nil {
		return Catalog{}, err
	}

	// 4. Close stdin and require a clean app-server shutdown.
	if err := stdin.Close(); err != nil {
		return Catalog{}, fmt.Errorf("close Codex app-server stdin: %w", err)
	}
	for scanner.Scan() {
	}
	stdoutErr := scanner.Err()
	stderrErr := <-stderrDone
	waitErr := command.Wait()
	finished = true
	if stdoutErr != nil {
		return Catalog{}, fmt.Errorf("drain Codex app-server stdout: %w", stdoutErr)
	}
	if stderrErr != nil {
		return Catalog{}, fmt.Errorf("read Codex app-server stderr: %w", stderrErr)
	}
	if waitErr != nil {
		return Catalog{}, fmt.Errorf("Codex app-server stopped: %w: %s", waitErr, strings.TrimSpace(stderrBuffer.String()))
	}
	return Catalog{Provider: "codex", Version: version, Models: models}, nil
}

func readAppServerResponse(scanner *bufio.Scanner, requestID int) (json.RawMessage, error) {
	for scanner.Scan() {
		var response appServerResponse
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			return nil, fmt.Errorf("decode Codex app-server message: %w", err)
		}
		if response.ID == nil || *response.ID != requestID {
			continue
		}
		if response.Error != nil {
			return nil, fmt.Errorf("Codex app-server request %d failed (%d): %s", requestID, response.Error.Code, response.Error.Message)
		}
		if len(response.Result) == 0 {
			return nil, fmt.Errorf("Codex app-server request %d returned no result", requestID)
		}
		return response.Result, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Codex app-server response: %w", err)
	}
	return nil, fmt.Errorf("Codex app-server closed before response %d", requestID)
}

func validateCodexCatalog(models []Model) error {
	if len(models) == 0 {
		return fmt.Errorf("Codex app-server returned no visible models")
	}
	seen := make(map[string]bool, len(models))
	defaults := 0
	for _, model := range models {
		if model.ID == "" || model.Label == "" || len(model.OptionValues) == 0 || model.DefaultOption == "" {
			return fmt.Errorf("Codex app-server returned an incomplete model entry")
		}
		if seen[model.ID] {
			return fmt.Errorf("Codex app-server returned duplicate model %q", model.ID)
		}
		seen[model.ID] = true
		if model.Default {
			defaults++
		}
		foundDefaultOption := false
		for _, option := range model.OptionValues {
			if option == model.DefaultOption {
				foundDefaultOption = true
			}
		}
		if !foundDefaultOption {
			return fmt.Errorf("Codex model %q default effort %q is not supported", model.ID, model.DefaultOption)
		}
	}
	if defaults != 1 {
		return fmt.Errorf("Codex app-server returned %d default models; expected exactly one", defaults)
	}
	return nil
}

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
	if effort := request.Options["reasoning_effort"]; effort != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", effort))
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
