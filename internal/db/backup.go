package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupManager coordinates scheduled and on-demand SQLite backup snapshots
// and enforces retention policies per OQ #21.
type BackupManager struct {
	db             *DB
	backupDir      string
	interval       time.Duration
	retentionCount int
}

// NewBackupManager creates a BackupManager configured with interval hours and retention count.
// Defaults: interval = 6h, retention = 7.
func NewBackupManager(db *DB, dataDir string, intervalHours, retentionCount int) *BackupManager {
	if intervalHours <= 0 {
		intervalHours = 6
	}
	if retentionCount <= 0 {
		retentionCount = 7
	}
	return &BackupManager{
		db:             db,
		backupDir:      filepath.Join(dataDir, "backups"),
		interval:       time.Duration(intervalHours) * time.Hour,
		retentionCount: retentionCount,
	}
}

// Backup creates a point-in-time SQLite snapshot at destPath using VACUUM INTO.
func (d *DB) Backup(ctx context.Context, destPath string) error {
	if d.path == ":memory:" || strings.HasPrefix(d.path, "file::memory:") {
		return errors.New("cannot backup in-memory database")
	}
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating backup directory %q: %w", dir, err)
	}

	// Remove target file if it already exists (VACUUM INTO requires non-existent destination)
	if _, err := os.Stat(destPath); err == nil {
		_ = os.Remove(destPath)
	}

	escaped := strings.ReplaceAll(destPath, "'", "''")
	query := fmt.Sprintf("VACUUM INTO '%s';", escaped)

	_, err := d.sqlDB.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("creating database snapshot: %w", err)
	}
	return nil
}

// Snapshot generates a new timestamped backup file in DATA_DIR/backups
// and prunes older backups according to retention policy.
func (bm *BackupManager) Snapshot(ctx context.Context) (string, error) {
	if err := os.MkdirAll(bm.backupDir, 0o750); err != nil {
		return "", fmt.Errorf("creating backup directory %q: %w", bm.backupDir, err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("supervisor-%s.db", timestamp)
	destPath := filepath.Join(bm.backupDir, filename)

	if err := bm.db.Backup(ctx, destPath); err != nil {
		return "", err
	}

	// Prune older backups
	if _, err := bm.Prune(); err != nil {
		logger.Error("failed to prune old backup snapshots", "err", err)
	}

	return destPath, nil
}

// Prune scans DATA_DIR/backups and retains the newest retentionCount snapshots,
// deleting any older snapshots. Returns the count of deleted files.
func (bm *BackupManager) Prune() (int, error) {
	backups, err := bm.ListBackups()
	if err != nil {
		return 0, err
	}

	if len(backups) <= bm.retentionCount {
		return 0, nil
	}

	toDelete := backups[bm.retentionCount:]
	deletedCount := 0
	for _, path := range toDelete {
		if err := os.Remove(path); err == nil {
			deletedCount++
			logger.Debug("pruned old backup snapshot", "path", path)
		} else {
			logger.Error("failed to delete expired backup snapshot", "path", path, "err", err)
		}
	}

	if deletedCount > 0 {
		logger.Info("pruned expired backup snapshots", "deleted", deletedCount, "retained", bm.retentionCount)
	}
	return deletedCount, nil
}

// ListBackups returns all snapshot files in DATA_DIR/backups, sorted newest first.
func (bm *BackupManager) ListBackups() ([]string, error) {
	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading backup directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "supervisor-") && strings.HasSuffix(name, ".db") {
			files = append(files, filepath.Join(bm.backupDir, name))
		}
	}

	// Sort newest first (reverse lexicographical order on supervisor-YYYYMMDD-HHMMSS.db)
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})

	return files, nil
}

// Start runs the periodic backup snapshot loop until ctx is canceled.
func (bm *BackupManager) Start(ctx context.Context) {
	logger.Info("backup scheduler started",
		"interval", bm.interval.String(),
		"retention_count", bm.retentionCount,
		"backup_dir", bm.backupDir,
	)

	ticker := time.NewTicker(bm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("backup scheduler stopped")
			return
		case <-ticker.C:
			snapshotCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			path, err := bm.Snapshot(snapshotCtx)
			cancel()
			if err != nil {
				logger.Error("scheduled backup snapshot failed", "err", err)
			} else {
				logger.Info("scheduled backup snapshot completed", "path", path)
			}
		}
	}
}
