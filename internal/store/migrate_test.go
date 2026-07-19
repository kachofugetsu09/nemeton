package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenBacksUpExistingDatabaseBeforeMigration(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "nemeton.db")
	backupRoot := filepath.Join(root, "backups")

	// 1. Create a real pre-migration SQLite database with durable content.
	legacy, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	if _, err := legacy.Exec(`CREATE TABLE legacy_marker(value TEXT NOT NULL); INSERT INTO legacy_marker(value) VALUES ('preserve-me')`); err != nil {
		legacy.Close()
		t.Fatalf("create legacy database: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	// 2. Opening the store must migrate only after creating a verified backup.
	database, err := Open(context.Background(), databasePath, backupRoot)
	if err != nil {
		t.Fatalf("open and migrate store: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close migrated store: %v", err)
	}
	backups, err := filepath.Glob(filepath.Join(backupRoot, "nemeton-pre-migration-*.db"))
	if err != nil {
		t.Fatalf("find migration backup: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("migration backup count = %d, want 1", len(backups))
	}
	info, err := os.Stat(backups[0])
	if err != nil {
		t.Fatalf("inspect migration backup: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("migration backup permissions = %04o, want 0600", info.Mode().Perm())
	}

	// 3. The backup must contain the exact pre-migration content.
	backup, err := sql.Open("sqlite", "file:"+backups[0]+"?mode=ro")
	if err != nil {
		t.Fatalf("open migration backup: %v", err)
	}
	defer backup.Close()
	var marker string
	if err := backup.QueryRow(`SELECT value FROM legacy_marker`).Scan(&marker); err != nil {
		t.Fatalf("read legacy marker from backup: %v", err)
	}
	if marker != "preserve-me" {
		t.Fatalf("backup marker = %q, want preserve-me", marker)
	}
}
