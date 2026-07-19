//go:build linux

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
	remote := map[int64]string{
		0x6969:     "NFS",
		0x517B:     "SMB",
		0xFF534D42: "CIFS",
	}
	if name, ok := remote[int64(stat.Type)]; ok {
		return fmt.Errorf("data directory must use a local filesystem: %s is on %s", path, name)
	}
	return nil
}
