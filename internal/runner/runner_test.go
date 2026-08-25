package runner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCodexCatalogUsesAuthenticatedAppServerPicker(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "codex"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--version" ]]; then echo "codex-test 1"; exit 0; fi
if [[ "${1:-}" != "app-server" ]]; then exit 20; fi
read -r initialize
[[ "$initialize" == *'"method":"initialize"'* && "$initialize" == *'"name":"nemeton"'* ]] || exit 21
echo '{"id":1,"result":{"userAgent":"nemeton-test"}}'
read -r initialized
[[ "$initialized" == '{"method":"initialized"}' ]] || exit 22
read -r first_request
[[ "$first_request" == *'"method":"model/list"'* && "$first_request" == *'"includeHidden":false'* && "$first_request" == *'"cursor":null'* ]] || exit 23
echo '{"method":"remoteControl/status/changed","params":{"status":"disabled"}}'
echo '{"id":2,"result":{"data":[{"model":"gpt-frontier","displayName":"GPT Frontier","description":"Frontier model","isDefault":true,"defaultReasoningEffort":"low","supportedReasoningEfforts":[{"reasoningEffort":"low"},{"reasoningEffort":"high"},{"reasoningEffort":"ultra"}]}],"nextCursor":"1"}}'
read -r second_request
[[ "$second_request" == *'"cursor":"1"'* ]] || exit 24
echo '{"id":3,"result":{"data":[{"model":"gpt-fast","displayName":"GPT Fast","description":"Fast model","isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[{"reasoningEffort":"low"},{"reasoningEffort":"medium"},{"reasoningEffort":"high"}]}],"nextCursor":null}}'
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	catalog, err := NewRegistry(map[string]Backend{"codex": Codex{}}).Catalog(ctx, "codex")
	if err != nil {
		t.Fatalf("read Codex catalog: %v", err)
	}
	expected := Catalog{Provider: "codex", Version: "codex-test 1", Models: []Model{
		{ID: "gpt-frontier", Label: "GPT Frontier", Description: "Frontier model", Default: true,
			OptionValues: []string{"low", "high", "ultra"}, DefaultOption: "low"},
		{ID: "gpt-fast", Label: "GPT Fast", Description: "Fast model",
			OptionValues: []string{"low", "medium", "high"}, DefaultOption: "medium"},
	}}
	if !reflect.DeepEqual(catalog, expected) {
		t.Fatalf("Catalog = %#v, want %#v", catalog, expected)
	}
}

func TestCodexAndOpenCodeProcessContracts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Fatal("Runner contract is Unix-only")
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "codex"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--version" ]]; then echo "codex-test 1"; exit 0; fi
out=""
previous=""
for arg in "$@"; do
  if [[ "$previous" == "-o" ]]; then out="$arg"; fi
  previous="$arg"
done
echo '{"type":"thread.started","thread_id":"codex-session"}'
echo '{"type":"item.completed","item":{"type":"agent_message","text":"done"}}'
printf '{"ok":true}' > "$out"
`)
	writeExecutable(t, filepath.Join(bin, "opencode"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--version" ]]; then echo "opencode-test 1"; exit 0; fi
echo '{"type":"step_start","sessionID":"opencode-session","part":{}}'
echo '{"type":"text","sessionID":"opencode-session","part":{"text":"{\"ok\":true}"}}'
echo '{"type":"step_finish","sessionID":"opencode-session","part":{"reason":"stop"}}'
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	workdir := t.TempDir()
	output := t.TempDir()
	for _, item := range []struct {
		name    string
		backend Backend
		session string
		options map[string]string
		option  string
	}{
		{name: "codex", backend: Codex{}, session: "codex-session", options: map[string]string{"reasoning_effort": "high"}, option: `model_reasoning_effort="high"`},
		{name: "opencode", backend: OpenCode{}, session: "opencode-session", options: map[string]string{"variant": "high"}, option: "high"},
	} {
		t.Run(item.name, func(t *testing.T) {
			var pinned string
			result, err := item.backend.Execute(context.Background(), Request{
				Provider: item.name, Model: "test-model", Options: item.options,
				Prompt: "test", Workdir: workdir, OutputDir: output,
				PinSession: func(sessionID string) error { pinned = sessionID; return nil },
			}, func(Delta) {})
			if err != nil {
				t.Fatalf("execute %s: %v", item.name, err)
			}
			if result.SessionID != item.session || pinned != item.session || result.Output != `{"ok":true}` {
				t.Fatalf("%s result = %#v", item.name, result)
			}
			if !slices.Contains(result.Command, "test-model") || !slices.Contains(result.Command, item.option) {
				t.Fatalf("%s command lacks model/options: %v", item.name, result.Command)
			}
		})
	}
}

func TestOpenCodeFailsClosedOnBrokenEventStream(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Fatal("Runner contract is Unix-only")
	}
	for _, testCase := range []struct {
		name      string
		stream    string
		wantError string
	}{
		{
			name: "invalid NDJSON",
			stream: `echo 'not-json'
echo '{"type":"step_start","sessionID":"session","part":{}}'
echo '{"type":"text","sessionID":"session","part":{"text":"done"}}'
echo '{"type":"step_finish","sessionID":"session","part":{"reason":"stop"}}'`,
			wantError: "invalid OpenCode NDJSON event",
		},
		{
			name: "missing continuation",
			stream: `echo '{"type":"step_start","sessionID":"session","part":{}}'
echo '{"type":"tool_use","sessionID":"session","part":{"tool":"read","state":{"status":"completed"}}}'
echo '{"type":"step_finish","sessionID":"session","part":{"reason":"tool-calls"}}'`,
			wantError: "required continuation step",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			bin := t.TempDir()
			writeExecutable(t, filepath.Join(bin, "opencode"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--version" ]]; then echo "opencode-test 1"; exit 0; fi
`+testCase.stream+"\n")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			_, err := (OpenCode{}).Execute(context.Background(), Request{
				Provider: "opencode", Prompt: "test", Workdir: t.TempDir(),
			}, func(Delta) {})
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("Execute error = %v, want %q", err, testCase.wantError)
			}
		})
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}
