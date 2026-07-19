//go:build darwin

package config

import (
	"fmt"

	"golang.org/x/sys/unix"
)

const maximumUnixSocketPathBytes = 103 // sockaddr_un.sun_path has 104 bytes including NUL.

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
