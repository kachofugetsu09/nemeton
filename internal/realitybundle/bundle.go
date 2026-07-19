package realitybundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Bundle struct {
	Schema        string          `json:"schema"`
	SourceRoot    string          `json:"source_root"`
	HeadOID       string          `json:"head_oid"`
	Branch        string          `json:"branch"`
	StagedPatch   string          `json:"staged_patch_base64"`
	UnstagedPatch string          `json:"unstaged_patch_base64"`
	Untracked     []UntrackedFile `json:"untracked"`
	Observation   Observation     `json:"observation"`
}

type UntrackedFile struct {
	Path       string `json:"path"`
	Mode       uint32 `json:"mode"`
	Kind       string `json:"kind"`
	DataBase64 string `json:"data_base64"`
}

type Observation struct {
	HeadOID         string `json:"head_oid"`
	StatusBase64    string `json:"status_base64"`
	StagedBase64    string `json:"staged_base64"`
	WorktreeBase64  string `json:"worktree_base64"`
	UntrackedDigest string `json:"untracked_digest"`
}

type Environment struct {
	Root         string
	Workdir      string
	Repository   string
	Home         string
	ProviderHome string
	Logs         string
	Output       string
}

func Capture(ctx context.Context, sourceRoot string) (Bundle, []byte, error) {
	// 1. Capture every mutable Git view twice around untracked file reads.
	first, err := captureOnce(ctx, sourceRoot)
	if err != nil {
		return Bundle{}, nil, err
	}
	second, err := captureOnce(ctx, sourceRoot)
	if err != nil {
		return Bundle{}, nil, err
	}
	firstData, err := json.Marshal(first)
	if err != nil {
		return Bundle{}, nil, fmt.Errorf("encode first Reality Bundle observation: %w", err)
	}
	secondData, err := json.Marshal(second)
	if err != nil {
		return Bundle{}, nil, fmt.Errorf("encode second Reality Bundle observation: %w", err)
	}
	if !bytes.Equal(firstData, secondData) {
		return Bundle{}, nil, fmt.Errorf("repository changed while capturing Reality Bundle")
	}
	return first, firstData, nil
}

func captureOnce(ctx context.Context, sourceRoot string) (Bundle, error) {
	root, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return Bundle{}, fmt.Errorf("resolve Reality Bundle source: %w", err)
	}
	head, err := gitText(ctx, root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return Bundle{}, err
	}
	branch, err := gitText(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		var commandError *commandError
		if !errors.As(err, &commandError) || commandError.ExitCode != 1 {
			return Bundle{}, err
		}
		branch = ""
	}
	staged, err := git(ctx, root, "diff", "--cached", "--binary", "--full-index", "--no-ext-diff")
	if err != nil {
		return Bundle{}, err
	}
	unstaged, err := git(ctx, root, "diff", "--binary", "--full-index", "--no-ext-diff")
	if err != nil {
		return Bundle{}, err
	}
	status, err := git(ctx, root, "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "status", "--porcelain=v2", "-z", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return Bundle{}, err
	}
	files, err := captureUntracked(ctx, root)
	if err != nil {
		return Bundle{}, err
	}
	untrackedData, err := json.Marshal(files)
	if err != nil {
		return Bundle{}, fmt.Errorf("encode untracked Reality files: %w", err)
	}
	untrackedSum := sha256.Sum256(untrackedData)
	return Bundle{
		Schema: "nemeton.reality-bundle.v1", SourceRoot: root, HeadOID: head,
		Branch: branch, StagedPatch: base64.StdEncoding.EncodeToString(staged),
		UnstagedPatch: base64.StdEncoding.EncodeToString(unstaged), Untracked: files,
		Observation: Observation{HeadOID: head,
			StatusBase64:    base64.StdEncoding.EncodeToString(status),
			StagedBase64:    base64.StdEncoding.EncodeToString(staged),
			WorktreeBase64:  base64.StdEncoding.EncodeToString(unstaged),
			UntrackedDigest: hex.EncodeToString(untrackedSum[:])},
	}, nil
}

func captureUntracked(ctx context.Context, root string) ([]UntrackedFile, error) {
	output, err := git(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []UntrackedFile
	for _, raw := range bytes.Split(output, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		path := string(raw)
		if err := safeRelativePath(path); err != nil {
			return nil, err
		}
		absolute := filepath.Join(root, filepath.FromSlash(path))
		info, err := os.Lstat(absolute)
		if err != nil {
			return nil, fmt.Errorf("inspect untracked file %s: %w", path, err)
		}
		var data []byte
		kind := "file"
		if info.Mode().IsRegular() {
			data, err = os.ReadFile(absolute)
		} else if info.Mode()&os.ModeSymlink != 0 {
			kind = "symlink"
			var target string
			target, err = os.Readlink(absolute)
			data = []byte(target)
		} else {
			return nil, fmt.Errorf("unsupported untracked file type at %s", path)
		}
		if err != nil {
			return nil, fmt.Errorf("read untracked file %s: %w", path, err)
		}
		files = append(files, UntrackedFile{Path: filepath.ToSlash(path),
			Mode: uint32(info.Mode().Perm()), Kind: kind,
			DataBase64: base64.StdEncoding.EncodeToString(data)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func Decode(data []byte) (Bundle, error) {
	var bundle Bundle
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return Bundle{}, fmt.Errorf("decode Reality Bundle: %w", err)
	}
	if bundle.Schema != "nemeton.reality-bundle.v1" || bundle.HeadOID == "" || !filepath.IsAbs(bundle.SourceRoot) {
		return Bundle{}, fmt.Errorf("invalid Reality Bundle identity")
	}
	return bundle, nil
}

func Prepare(ctx context.Context, bundle Bundle, root string) (Environment, error) {
	// 1. Create a fresh managed environment with provider-local state.
	if !filepath.IsAbs(root) {
		return Environment{}, fmt.Errorf("Meeting environment root must be absolute: %s", root)
	}
	if _, err := os.Stat(root); err == nil {
		return Environment{}, fmt.Errorf("Meeting environment already exists: %s", root)
	} else if !os.IsNotExist(err) {
		return Environment{}, fmt.Errorf("inspect Meeting environment: %w", err)
	}
	environment := Environment{Root: root, Workdir: filepath.Join(root, "workdir"),
		Home: filepath.Join(root, "home"), ProviderHome: filepath.Join(root, "provider-home"),
		Logs: filepath.Join(root, "logs"), Output: filepath.Join(root, "output")}
	for _, path := range []string{environment.Workdir, environment.Home, environment.ProviderHome, environment.Logs, environment.Output} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return Environment{}, fmt.Errorf("create Meeting environment directory %s: %w", path, err)
		}
	}

	// 2. Clone the exact commit with independent Git metadata and restore dirty bytes.
	repository := filepath.Join(environment.Workdir, filepath.Base(bundle.SourceRoot))
	if _, err := run(ctx, "", nil, "git", "clone", "--no-checkout", "--no-hardlinks", bundle.SourceRoot, repository); err != nil {
		return Environment{}, err
	}
	if _, err := run(ctx, repository, nil, "git", "checkout", "--detach", bundle.HeadOID); err != nil {
		return Environment{}, err
	}
	staged, err := base64.StdEncoding.DecodeString(bundle.StagedPatch)
	if err != nil {
		return Environment{}, fmt.Errorf("decode staged Reality patch: %w", err)
	}
	unstaged, err := base64.StdEncoding.DecodeString(bundle.UnstagedPatch)
	if err != nil {
		return Environment{}, fmt.Errorf("decode unstaged Reality patch: %w", err)
	}
	if err := applyPatch(ctx, repository, staged, true); err != nil {
		return Environment{}, err
	}
	if err := applyPatch(ctx, repository, unstaged, false); err != nil {
		return Environment{}, err
	}
	if err := restoreUntracked(repository, bundle.Untracked); err != nil {
		return Environment{}, err
	}
	environment.Repository = repository
	return environment, nil
}

func Observe(ctx context.Context, sourceRoot string) (Observation, error) {
	bundle, err := captureOnce(ctx, sourceRoot)
	if err != nil {
		return Observation{}, err
	}
	return bundle.Observation, nil
}

func applyPatch(ctx context.Context, repository string, patch []byte, staged bool) error {
	if len(patch) == 0 {
		return nil
	}
	args := []string{"git", "apply", "--binary"}
	if staged {
		args = append(args, "--index")
	}
	_, err := run(ctx, repository, patch, args[0], args[1:]...)
	return err
}

func restoreUntracked(repository string, files []UntrackedFile) error {
	for _, file := range files {
		if err := safeRelativePath(file.Path); err != nil {
			return err
		}
		data, err := base64.StdEncoding.DecodeString(file.DataBase64)
		if err != nil {
			return fmt.Errorf("decode untracked file %s: %w", file.Path, err)
		}
		target := filepath.Join(repository, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return fmt.Errorf("create untracked parent %s: %w", file.Path, err)
		}
		switch file.Kind {
		case "file":
			if err := os.WriteFile(target, data, os.FileMode(file.Mode)); err != nil {
				return fmt.Errorf("restore untracked file %s: %w", file.Path, err)
			}
		case "symlink":
			if err := os.Symlink(string(data), target); err != nil {
				return fmt.Errorf("restore untracked symlink %s: %w", file.Path, err)
			}
		default:
			return fmt.Errorf("unsupported untracked kind %q", file.Kind)
		}
	}
	return nil
}

func safeRelativePath(path string) error {
	clean := filepath.Clean(filepath.FromSlash(path))
	if path == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe Reality Bundle path: %q", path)
	}
	return nil
}

type commandError struct {
	Command  string
	ExitCode int
	Stderr   string
}

func (e *commandError) Error() string {
	return fmt.Sprintf("%s failed with exit code %d: %s", e.Command, e.ExitCode, strings.TrimSpace(e.Stderr))
}

func gitText(ctx context.Context, root string, args ...string) (string, error) {
	output, err := git(ctx, root, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	command := append([]string{"git"}, args...)
	return run(ctx, root, nil, command[0], command[1:]...)
}

func run(ctx context.Context, root string, stdin []byte, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, &commandError{Command: strings.Join(append([]string{name}, args...), " "), ExitCode: exitError.ExitCode(), Stderr: stderr.String()}
		}
		return nil, fmt.Errorf("execute %s: %w", strings.Join(append([]string{name}, args...), " "), err)
	}
	return stdout.Bytes(), nil
}
