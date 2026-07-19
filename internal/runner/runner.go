package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const maximumStreamLine = 16 << 20

type Request struct {
	Provider   string
	Model      string
	Prompt     string
	Workdir    string
	OutputDir  string
	SessionID  string
	Timeout    time.Duration
	PinSession func(string) error
}

type Result struct {
	Provider   string
	Version    string
	Command    []string
	SessionID  string
	Output     string
	RawStream  []byte
	Stderr     string
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time
}

type Delta struct {
	Provider string `json:"provider"`
	Type     string `json:"type"`
	Content  string `json:"content"`
}

type Backend interface {
	Execute(context.Context, Request, func(Delta)) (Result, error)
}

type Registry struct {
	backends map[string]Backend
}

func NewRegistry(backends map[string]Backend) *Registry {
	copy := make(map[string]Backend, len(backends))
	for name, backend := range backends {
		copy[name] = backend
	}
	return &Registry{backends: copy}
}

func Production() *Registry {
	return NewRegistry(map[string]Backend{"codex": Codex{}, "opencode": OpenCode{}})
}

func (r *Registry) Execute(ctx context.Context, request Request, emit func(Delta)) (Result, error) {
	backend, ok := r.backends[request.Provider]
	if !ok {
		return Result{}, fmt.Errorf("unsupported Provider %q", request.Provider)
	}
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}
	return backend.Execute(ctx, request, emit)
}

type processResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
	started  time.Time
	finished time.Time
}

func runStreamed(ctx context.Context, name string, args, env []string, cwd string, onLine func([]byte)) (processResult, error) {
	// 1. Start the Provider in its own Unix process group.
	command := exec.Command(name, args...)
	command.Dir = cwd
	command.Env = append(os.Environ(), env...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return processResult{}, fmt.Errorf("open %s stdout: %w", name, err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return processResult{}, fmt.Errorf("open %s stderr: %w", name, err)
	}
	started := time.Now().UTC()
	if err := command.Start(); err != nil {
		return processResult{}, fmt.Errorf("start %s: %w", name, err)
	}

	// 2. Drain both streams while retaining exact evidence.
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	var stdoutErr error
	var stderrErr error
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		scanner := bufio.NewScanner(io.TeeReader(stdout, &stdoutBuffer))
		scanner.Buffer(make([]byte, 64*1024), maximumStreamLine)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			onLine(line)
		}
		stdoutErr = scanner.Err()
	}()
	go func() {
		defer waitGroup.Done()
		_, stderrErr = io.Copy(&stderrBuffer, stderr)
	}()

	// 3. Terminate the whole tool process tree on cancellation.
	waitResult := make(chan error, 1)
	go func() {
		waitGroup.Wait()
		waitResult <- command.Wait()
	}()
	var waitErr error
	select {
	case waitErr = <-waitResult:
	case <-ctx.Done():
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		select {
		case waitErr = <-waitResult:
		case <-time.After(5 * time.Second):
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			waitErr = <-waitResult
		}
	}
	result := processResult{stdout: stdoutBuffer.Bytes(), stderr: stderrBuffer.Bytes(),
		exitCode: 0, started: started, finished: time.Now().UTC()}
	if stdoutErr != nil {
		return result, fmt.Errorf("read %s stdout: %w", name, stdoutErr)
	}
	if stderrErr != nil {
		return result, fmt.Errorf("read %s stderr: %w", name, stderrErr)
	}
	if waitErr != nil {
		var exitError *exec.ExitError
		if errors.As(waitErr, &exitError) {
			result.exitCode = exitError.ExitCode()
		} else {
			return result, fmt.Errorf("wait for %s: %w", name, waitErr)
		}
		if ctx.Err() != nil {
			return result, fmt.Errorf("%s execution stopped: %w", name, ctx.Err())
		}
		return result, fmt.Errorf("%s exited with code %d: %s", name, result.exitCode, strings.TrimSpace(string(result.stderr)))
	}
	return result, nil
}

func executableVersion(ctx context.Context, name string) (string, error) {
	command := exec.CommandContext(ctx, name, "--version")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read %s version: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func outputPath(directory, prefix string) (string, error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create Provider output directory: %w", err)
	}
	file, err := os.CreateTemp(directory, prefix+"-*.json")
	if err != nil {
		return "", fmt.Errorf("create Provider output file: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close Provider output file: %w", err)
	}
	return path, nil
}

func decodeLine(line []byte) map[string]any {
	var value map[string]any
	if json.Unmarshal(line, &value) != nil {
		return nil
	}
	return value
}

func stringField(value map[string]any, name string) string {
	text, _ := value[name].(string)
	return text
}

func nestedMap(value map[string]any, name string) map[string]any {
	nested, _ := value[name].(map[string]any)
	return nested
}

func readFinalOutput(path string) (string, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("read Provider final output: %w", err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", fmt.Errorf("Provider returned empty final output")
	}
	return text, nil
}
