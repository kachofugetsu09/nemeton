//go:build darwin

package config

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func ensureLocalFilesystem(path string) error {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return fmt.Errorf("inspect data filesystem for %s: %w", path, err)
	}
	if stat.Flags&unix.MNT_LOCAL == 0 {
		return fmt.Errorf("data directory must use a local filesystem: %s", path)
	}
	return nil
}
