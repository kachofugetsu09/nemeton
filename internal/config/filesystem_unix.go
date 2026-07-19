//go:build unix && !linux && !darwin

package config

const maximumUnixSocketPathBytes = 103 // Conservative best-effort limit outside supported hosts.

func ensureLocalFilesystem(string) error {
	return nil
}
