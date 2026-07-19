package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

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

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}
