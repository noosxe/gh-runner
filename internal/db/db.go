package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var (
	// ErrCorrupted indicates the SQLite database file is corrupted, truncated,
	// or not a valid SQLite database (OQ #21).
	ErrCorrupted = errors.New("database file is corrupted")

	// ErrMigration indicates a database migration failed to apply.
	ErrMigration = errors.New("database migration failed")
)

// Options configures the database connection and migration runner.
type Options struct {
	// Path is the SQLite database file path (e.g. /data/supervisor.db or :memory:).
	Path string

	// Migrations is the filesystem containing migration files. If nil,
	// default embedded migrations are used.
	Migrations fs.FS

	// SkipMigrations skips running migrations on open when true.
	SkipMigrations bool

	// MaxOpenConns sets the maximum number of open connections. Defaults to 1
	// if <= 0 to serialize SQLite transactions and prevent database lock contention.
	MaxOpenConns int
}

// DB wraps an opened SQLite database connection pool and its configuration.
type DB struct {
	sqlDB *sql.DB
	path  string
}

// Open initializes and verifies a SQLite database connection with modernc.org/sqlite
// (pure Go, CGO-free). It creates the parent directory if needed, enables WAL mode,
// foreign keys, and busy timeout, verifies database integrity, and executes pending
// goose migrations (unless SkipMigrations is true). If the database file is corrupted,
// it refuses to start with an actionable error directing the admin to restore from
// a backup snapshot (OQ #21).
func Open(opts Options) (*DB, error) {
	if strings.TrimSpace(opts.Path) == "" {
		return nil, errors.New("database path must not be empty")
	}

	dsn, err := buildDSN(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("building database DSN: %w", err)
	}

	if opts.Path != ":memory:" && !strings.HasPrefix(opts.Path, "file::memory:") {
		dir := filepath.Dir(opts.Path)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("creating database directory %q: %w", dir, err)
			}
		}
	}

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	maxConns := opts.MaxOpenConns
	if maxConns <= 0 {
		maxConns = 1
	}
	sqlDB.SetMaxOpenConns(maxConns)

	ctx := context.Background()

	// Verify connectivity and database file integrity.
	if err := checkIntegrity(ctx, sqlDB); err != nil {
		_ = sqlDB.Close()
		logger.Error("database corrupted", "path", opts.Path, "err", err)
		return nil, fmt.Errorf("database file %q is corrupted: %w; refuse to start; restore from a backup in DATA_DIR/backups (OQ #21)", opts.Path, err)
	}

	database := &DB{
		sqlDB: sqlDB,
		path:  opts.Path,
	}

	if !opts.SkipMigrations {
		if err := database.Migrate(ctx, opts.Migrations); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	}

	logger.Info("database opened successfully", "path", opts.Path)
	return database, nil
}

// OpenPath is a convenience wrapper that opens the database at path with default options.
func OpenPath(path string) (*DB, error) {
	return Open(Options{Path: path})
}

// buildDSN crafts the DSN with appropriate pragmas.
func buildDSN(path string) (string, error) {
	isMem := path == ":memory:" || strings.HasPrefix(path, "file::memory:")

	// Default pragmas: busy_timeout=5000ms, foreign_keys=ON.
	// WAL mode and NORMAL synchronous are standard for disk-backed databases.
	pragmas := []string{
		"_pragma=busy_timeout(5000)",
		"_pragma=foreign_keys(ON)",
	}
	if !isMem {
		pragmas = append(pragmas, "_pragma=journal_mode(WAL)", "_pragma=synchronous(NORMAL)")
	}

	query := strings.Join(pragmas, "&")
	if strings.Contains(path, "?") {
		return path + "&" + query, nil
	}
	return path + "?" + query, nil
}

// checkIntegrity executes PRAGMA quick_check to ensure the SQLite file is valid.
func checkIntegrity(ctx context.Context, sqlDB *sql.DB) error {
	var check string
	err := sqlDB.QueryRowContext(ctx, "PRAGMA quick_check;").Scan(&check)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCorrupted, err)
	}
	if strings.ToLower(strings.TrimSpace(check)) != "ok" {
		return fmt.Errorf("%w: integrity check returned %q", ErrCorrupted, check)
	}
	return nil
}

// Ping verifies the database connection is alive.
func (d *DB) Ping(ctx context.Context) error {
	if d.sqlDB == nil {
		return errors.New("database is not open")
	}
	return d.sqlDB.PingContext(ctx)
}

// Close closes the underlying SQLite database connection.
func (d *DB) Close() error {
	if d.sqlDB == nil {
		return nil
	}
	return d.sqlDB.Close()
}

// SQL returns the underlying *sql.DB handle.
func (d *DB) SQL() *sql.DB {
	return d.sqlDB
}

// Path returns the configured database file path.
func (d *DB) Path() string {
	return d.path
}
