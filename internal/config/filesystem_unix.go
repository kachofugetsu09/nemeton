//go:build unix && !linux && !darwin

package config

func ensureLocalFilesystem(string) error {
	return nil
}
