package realitybundle

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCaptureAndPreparePreservesDirtyReality(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "repository")
	runTestCommand(t, "", "git", "init", "-b", "main", repository)
	runTestCommand(t, repository, "git", "config", "user.name", "Nemeton Test")
	runTestCommand(t, repository, "git", "config", "user.email", "nemeton@example.invalid")
	writeTestFile(t, filepath.Join(repository, "tracked.txt"), "base\n")
	runTestCommand(t, repository, "git", "add", "tracked.txt")
	runTestCommand(t, repository, "git", "commit", "-m", "base")

	writeTestFile(t, filepath.Join(repository, "tracked.txt"), "staged\n")
	runTestCommand(t, repository, "git", "add", "tracked.txt")
	writeTestFile(t, filepath.Join(repository, "tracked.txt"), "staged\nunstaged\n")
	writeTestFile(t, filepath.Join(repository, "untracked.txt"), "untracked bytes\n")

	bundle, data, err := Capture(context.Background(), repository)
	if err != nil {
		t.Fatalf("capture Reality Bundle: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("decode Reality Bundle: %v", err)
	}
	if decoded.HeadOID != bundle.HeadOID || len(decoded.Untracked) != 1 {
		t.Fatalf("decoded Reality Bundle = %#v", decoded)
	}
	observed, err := Observe(context.Background(), repository)
	if err != nil {
		t.Fatalf("observe unchanged source: %v", err)
	}
	if observed != bundle.Observation {
		t.Fatalf("unchanged source observation = %#v, want %#v", observed, bundle.Observation)
	}

	first, err := Prepare(context.Background(), decoded, filepath.Join(root, "seat-a"))
	if err != nil {
		t.Fatalf("prepare first seat: %v", err)
	}
	second, err := Prepare(context.Background(), decoded, filepath.Join(root, "seat-b"))
	if err != nil {
		t.Fatalf("prepare second seat: %v", err)
	}
	for _, environment := range []Environment{first, second} {
		tracked, err := os.ReadFile(filepath.Join(environment.Repository, "tracked.txt"))
		if err != nil || string(tracked) != "staged\nunstaged\n" {
			t.Fatalf("restored tracked content = %q, err=%v", tracked, err)
		}
		untracked, err := os.ReadFile(filepath.Join(environment.Repository, "untracked.txt"))
		if err != nil || string(untracked) != "untracked bytes\n" {
			t.Fatalf("restored untracked content = %q, err=%v", untracked, err)
		}
		if output := runTestCommand(t, environment.Repository, "git", "diff", "--cached", "--name-only"); output != "tracked.txt\n" {
			t.Fatalf("restored staged paths = %q", output)
		}
		if output := runTestCommand(t, environment.Repository, "git", "diff", "--name-only"); output != "tracked.txt\n" {
			t.Fatalf("restored unstaged paths = %q", output)
		}
	}

	if err := os.WriteFile(filepath.Join(first.Repository, "seat-only.txt"), []byte("a"), 0o600); err != nil {
		t.Fatalf("write first seat experiment: %v", err)
	}
	if _, err := os.Stat(filepath.Join(second.Repository, "seat-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("second seat observed first seat experiment: %v", err)
	}

	writeTestFile(t, filepath.Join(repository, "untracked.txt"), "changed after capture\n")
	changed, err := Observe(context.Background(), repository)
	if err != nil {
		t.Fatalf("observe changed source: %v", err)
	}
	if changed == bundle.Observation {
		t.Fatal("changed source retained the frozen observation")
	}
}

func runTestCommand(t *testing.T, directory, name string, args ...string) string {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %v: %v\n%s", name, args, err, output)
	}
	return string(output)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
