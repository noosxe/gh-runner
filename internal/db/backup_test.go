package db

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBackupSnapshotAndPruning verifies that snapshots are created via VACUUM INTO,
// older files are pruned according to retentionCount, and the backup file is fully readable.
func TestBackupSnapshotAndPruning(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	dbPath := filepath.Join(dataDir, "supervisor.db")
	aesKey := bytes.Repeat([]byte{0x44}, 32)

	database, err := Open(Options{
		Path:          dbPath,
		EncryptionKey: aesKey,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	// Insert test setting
	if _, err := database.SetAppSetting(ctx, SetAppSettingParams{
		Key:   "backup_test_key",
		Value: "val_12345",
	}); err != nil {
		t.Fatalf("SetAppSetting failed: %v", err)
	}

	// Retention count: 3
	bm := NewBackupManager(database, dataDir, 6, 3)

	// Create 5 snapshots manually to verify rotation
	var created []string
	for i := 0; i < 5; i++ {
		// Ensure distinct timestamps
		time.Sleep(1100 * time.Millisecond)
		path, err := bm.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot %d failed: %v", i, err)
		}
		created = append(created, path)
	}

	// Verify only 3 remain in backup directory
	remaining, err := bm.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(remaining) != 3 {
		t.Fatalf("expected exactly 3 retained backups, got %d", len(remaining))
	}

	// The remaining should be the 3 newest
	for _, p := range remaining {
		if !strings.HasPrefix(filepath.Base(p), "supervisor-") || !strings.HasSuffix(p, ".db") {
			t.Errorf("unexpected backup file name format: %s", p)
		}
	}

	// The first 2 created should be deleted
	for i := 0; i < 2; i++ {
		if _, err := os.Stat(created[i]); !os.IsNotExist(err) {
			t.Errorf("expected pruned file %s to be deleted, but it still exists", created[i])
		}
	}

	// Open the newest snapshot and verify data
	backupDB, err := Open(Options{
		Path:          remaining[0],
		EncryptionKey: aesKey,
	})
	if err != nil {
		t.Fatalf("opening snapshot failed: %v", err)
	}
	defer func() { _ = backupDB.Close() }()

	setting, err := backupDB.GetAppSetting(ctx, "backup_test_key")
	if err != nil || setting.Value != "val_12345" {
		t.Fatalf("expected data preserved in snapshot: got %v (err: %v)", setting, err)
	}
}

// TestBackupRestoreWorkflow tests the full disaster recovery scenario from docs/backup-and-recovery.md:
// 1. Live DB has data
// 2. Snapshot taken
// 3. Live DB gets corrupted
// 4. Supervisor refuses to boot
// 5. Restore snapshot over live DB
// 6. Supervisor boots cleanly and data is restored!
func TestBackupRestoreWorkflow(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	dbPath := filepath.Join(dataDir, "supervisor.db")
	aesKey := bytes.Repeat([]byte{0x66}, 32)

	// Step 1: Initialize live DB and write data
	database, err := Open(Options{
		Path:          dbPath,
		EncryptionKey: aesKey,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ctx := context.Background()
	_, err = database.CreateEncryptedAuthProfile(ctx, "restore-profile", "pat", sql.NullInt64{}, "", "secret-pat-token")
	if err != nil {
		t.Fatalf("CreateEncryptedAuthProfile: %v", err)
	}

	// Step 2: Take snapshot
	bm := NewBackupManager(database, dataDir, 6, 5)
	snapshotPath, err := bm.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	_ = database.Close()

	// Step 3: Corrupt live DB file
	if err := os.WriteFile(dbPath, []byte("TOTALLY_CORRUPTED_FILE_DATA"), 0o644); err != nil {
		t.Fatalf("corrupting file: %v", err)
	}

	// Step 4: Supervisor refuses to boot on corrupted DB
	_, err = Open(Options{
		Path:          dbPath,
		EncryptionKey: aesKey,
	})
	if err == nil || !strings.Contains(err.Error(), "corrupted") {
		t.Fatalf("expected corrupted database error, got: %v", err)
	}

	// Step 5: Disaster recovery: restore snapshot over DB file
	snapshotBytes, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("reading snapshot: %v", err)
	}
	if err := os.WriteFile(dbPath, snapshotBytes, 0o644); err != nil {
		t.Fatalf("restoring snapshot: %v", err)
	}
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")

	// Step 6: Reopen DB: boots cleanly and data is verified
	restoredDB, err := Open(Options{
		Path:          dbPath,
		EncryptionKey: aesKey,
	})
	if err != nil {
		t.Fatalf("restored DB failed to open: %v", err)
	}
	defer func() { _ = restoredDB.Close() }()

	profile, err := restoredDB.GetDecryptedAuthProfileByName(ctx, "restore-profile")
	if err != nil || profile.Token != "secret-pat-token" {
		t.Fatalf("restored profile mismatch: got %+v (err: %v)", profile, err)
	}
}

// TestBackupManagerSchedulerStopsOnCancel verifies that Start loop terminates on context cancellation.
func TestBackupManagerSchedulerStopsOnCancel(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	dbPath := filepath.Join(dataDir, "supervisor.db")
	aesKey := bytes.Repeat([]byte{0x77}, 32)

	database, err := Open(Options{
		Path:          dbPath,
		EncryptionKey: aesKey,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = database.Close() }()

	bm := &BackupManager{
		db:             database,
		backupDir:      filepath.Join(dataDir, "backups"),
		interval:       50 * time.Millisecond,
		retentionCount: 5,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		bm.Start(ctx)
		close(done)
	}()

	// Wait briefly for at least one snapshot
	time.Sleep(120 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Scheduler stopped cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("BackupManager.Start did not stop after context cancel")
	}

	backups, err := bm.ListBackups()
	if err != nil || len(backups) == 0 {
		t.Fatalf("expected at least 1 backup generated by ticker, got %d (err: %v)", len(backups), err)
	}
}
