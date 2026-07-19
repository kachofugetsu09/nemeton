package artifact

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Record struct {
	Digest     string `json:"digest"`
	MediaType  string `json:"media_type"`
	ByteSize   int64  `json:"byte_size"`
	StorageURI string `json:"storage_uri"`
}

type Store struct {
	root string
}

func New(root string) Store {
	return Store{root: root}
}

func (s Store) Put(data []byte, mediaType string) (Record, error) {
	// 1. Resolve the immutable content address.
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	relative := filepath.Join("sha256", digest[:2], digest[2:])
	target := filepath.Join(s.root, relative)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return Record{}, fmt.Errorf("create artifact directory: %w", err)
	}

	// 2. Reuse an existing object only after verifying its exact content.
	if existing, err := os.ReadFile(target); err == nil {
		if !bytes.Equal(existing, data) {
			return Record{}, fmt.Errorf("artifact content mismatch at %s", target)
		}
		return Record{Digest: digest, MediaType: mediaType, ByteSize: int64(len(data)), StorageURI: relative}, nil
	} else if !os.IsNotExist(err) {
		return Record{}, fmt.Errorf("read existing artifact %s: %w", target, err)
	}

	// 3. Write, sync, and atomically publish a new object.
	temporary, err := os.CreateTemp(filepath.Dir(target), ".artifact-*")
	if err != nil {
		return Record{}, fmt.Errorf("create artifact temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return Record{}, fmt.Errorf("set artifact permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return Record{}, fmt.Errorf("write artifact: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return Record{}, fmt.Errorf("sync artifact: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return Record{}, fmt.Errorf("close artifact: %w", err)
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return Record{}, fmt.Errorf("publish artifact: %w", err)
	}
	if err := syncDirectory(filepath.Dir(target)); err != nil {
		return Record{}, err
	}
	return Record{Digest: digest, MediaType: mediaType, ByteSize: int64(len(data)), StorageURI: relative}, nil
}

func (s Store) Verify(record Record) error {
	if err := validateRecord(record); err != nil {
		return err
	}
	path := filepath.Join(s.root, filepath.Clean(record.StorageURI))
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open artifact %s: %w", record.Digest, err)
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return fmt.Errorf("read artifact %s: %w", record.Digest, err)
	}
	if size != record.ByteSize {
		return fmt.Errorf("artifact %s size mismatch: got %d, want %d", record.Digest, size, record.ByteSize)
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); actual != record.Digest {
		return fmt.Errorf("artifact %s digest mismatch: got %s", record.Digest, actual)
	}
	return nil
}

func (s Store) Orphans(referenced map[string]bool) ([]string, error) {
	var orphans []string
	err := filepath.WalkDir(s.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		if !referenced[filepath.Clean(relative)] {
			orphans = append(orphans, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan artifact store for orphans: %w", err)
	}
	sort.Strings(orphans)
	return orphans, nil
}

func validateRecord(record Record) error {
	if len(record.Digest) != sha256.Size*2 {
		return fmt.Errorf("invalid artifact digest length: %q", record.Digest)
	}
	if _, err := hex.DecodeString(record.Digest); err != nil || strings.ToLower(record.Digest) != record.Digest {
		return fmt.Errorf("invalid artifact digest: %q", record.Digest)
	}
	expected := filepath.Join("sha256", record.Digest[:2], record.Digest[2:])
	if filepath.IsAbs(record.StorageURI) || filepath.Clean(record.StorageURI) != expected {
		return fmt.Errorf("artifact %s has invalid storage URI %q", record.Digest, record.StorageURI)
	}
	if record.ByteSize < 0 {
		return fmt.Errorf("artifact %s has negative size %d", record.Digest, record.ByteSize)
	}
	if record.MediaType == "" {
		return fmt.Errorf("artifact %s has an empty media type", record.Digest)
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open artifact directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync artifact directory: %w", err)
	}
	return nil
}
