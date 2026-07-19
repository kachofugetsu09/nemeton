package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	DataDir       string
	DatabasePath  string
	SocketPath    string
	LockPath      string
	ArtifactRoot  string
	BackupRoot    string
	WorktreesRoot string
}

func Resolve(dataDir string) (Config, error) {
	if runtime.GOOS == "windows" {
		return Config{}, fmt.Errorf("Windows is not supported")
	}
	resolved, err := resolveDataDir(dataDir)
	if err != nil {
		return Config{}, err
	}
	worktrees, err := resolveWorktreesRoot()
	if err != nil {
		return Config{}, err
	}
	return Config{
		DataDir:       resolved,
		DatabasePath:  filepath.Join(resolved, "nemeton.db"),
		SocketPath:    filepath.Join(resolved, "nemetond.sock"),
		LockPath:      filepath.Join(resolved, "nemetond.lock"),
		ArtifactRoot:  filepath.Join(resolved, "artifacts"),
		BackupRoot:    filepath.Join(resolved, "backups"),
		WorktreesRoot: worktrees,
	}, nil
}

func (c Config) Prepare() error {
	info, err := os.Stat(c.DataDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(c.DataDir, 0o700); err != nil {
			return fmt.Errorf("create data directory %s: %w", c.DataDir, err)
		}
		info, err = os.Stat(c.DataDir)
	}
	if err != nil {
		return fmt.Errorf("inspect data directory %s: %w", c.DataDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("data path is not a directory: %s", c.DataDir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("data directory permissions must be 0700: %s has %04o", c.DataDir, info.Mode().Perm())
	}
	if err := ensureLocalFilesystem(c.DataDir); err != nil {
		return err
	}
	for _, path := range []string{c.ArtifactRoot, c.BackupRoot} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create data subdirectory %s: %w", path, err)
		}
	}
	return nil
}

func resolveDataDir(override string) (string, error) {
	if override != "" {
		return absolutePath("--data-dir", override)
	}
	if value := os.Getenv("NEMETON_DATA_DIR"); value != "" {
		return absolutePath("NEMETON_DATA_DIR", value)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Nemeton"), nil
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return absolutePath("XDG_DATA_HOME", filepath.Join(xdg, "nemeton"))
	}
	return filepath.Join(home, ".local", "share", "nemeton"), nil
}

func resolveWorktreesRoot() (string, error) {
	if value := os.Getenv("NEMETON_WORKTREES_ROOT"); value != "" {
		return absolutePath("NEMETON_WORKTREES_ROOT", value)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home for worktree root: %w", err)
	}
	return filepath.Join(home, "nemeton-workspaces"), nil
}

func absolutePath(source, value string) (string, error) {
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("%s must be an absolute path: %s", source, value)
	}
	return filepath.Clean(value), nil
}
