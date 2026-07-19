package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kachofugetsu09/nemeton/migrations"
	"modernc.org/sqlite"
)

type migration struct {
	version int
	name    string
	sql     string
}

type onlineBackuper interface {
	NewBackup(string) (*sqlite.Backup, error)
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	var result []migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, found := strings.Cut(entry.Name(), "_")
		if !found {
			return nil, fmt.Errorf("migration name must start with a numeric version: %s", entry.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %s", entry.Name())
		}
		data, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		result = append(result, migration{version: version, name: entry.Name(), sql: string(data)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	for index, item := range result {
		if item.version != index+1 {
			return nil, fmt.Errorf("migration sequence has a gap at version %d", index+1)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no embedded migrations found")
	}
	return result, nil
}

func currentSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&count); err != nil {
		return 0, fmt.Errorf("inspect schema_migrations table: %w", err)
	}
	if count == 0 {
		return 0, nil
	}
	var version int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

func applyMigrations(ctx context.Context, db *sql.DB, items []migration, current int) error {
	for _, item := range items {
		if item.version <= current {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", item.name, err)
		}
		if _, err := tx.ExecContext(ctx, item.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)`, item.version, item.name, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", item.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", item.name, err)
		}
	}
	return nil
}

func backupDatabase(ctx context.Context, db *sql.DB, backupRoot string) (string, error) {
	if err := os.MkdirAll(backupRoot, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	name := "nemeton-pre-migration-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".db"
	path := filepath.Join(backupRoot, name)
	connection, err := db.Conn(ctx)
	if err != nil {
		return "", fmt.Errorf("open SQLite connection for backup: %w", err)
	}
	defer connection.Close()
	err = connection.Raw(func(driverConnection any) error {
		backuper, ok := driverConnection.(onlineBackuper)
		if !ok {
			return fmt.Errorf("SQLite driver does not expose online backup")
		}
		backup, err := backuper.NewBackup(path)
		if err != nil {
			return err
		}
		for more := true; more; {
			more, err = backup.Step(-1)
			if err != nil {
				backup.Finish()
				return err
			}
		}
		return backup.Finish()
	})
	if err != nil {
		return "", fmt.Errorf("create SQLite online backup: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("set backup permissions: %w", err)
	}
	if err := verifyBackup(ctx, path); err != nil {
		return "", err
	}
	return path, nil
}

func verifyBackup(ctx context.Context, path string) error {
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open migration backup: %w", err)
	}
	defer database.Close()
	var result string
	if err := database.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return fmt.Errorf("verify migration backup integrity: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("migration backup integrity check failed: %s", result)
	}
	return nil
}
