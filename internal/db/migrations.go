package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

// defaultMigrationsFS embeds all migration files.
//
//go:embed all:migrations
var defaultMigrationsFS embed.FS

// Migrate executes pending goose migrations against the database.
// If fsys is nil, default embedded migrations from internal/db/migrations are applied.
// Migration progress and errors are strictly logged per docs/06.
func (d *DB) Migrate(ctx context.Context, fsys fs.FS) error {
	if fsys == nil {
		sub, err := fs.Sub(defaultMigrationsFS, "migrations")
		if err != nil {
			logger.Error("failed to resolve embedded migrations directory", "err", err)
			return fmt.Errorf("%w: failed to read embedded migrations: %v", ErrMigration, err)
		}
		fsys = sub
	}

	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		d.sqlDB,
		fsys,
		goose.WithSlog(logger),
	)
	if err != nil {
		if errors.Is(err, goose.ErrNoMigrations) {
			logger.Debug("no database migrations found in filesystem")
			return nil
		}
		logger.Error("failed to initialize goose provider", "err", err)
		return fmt.Errorf("%w: failed to create provider: %v", ErrMigration, err)
	}

	res, err := provider.Up(ctx)
	if err != nil {
		logger.Error("database migration failed", "err", err)
		return fmt.Errorf("%w: %v", ErrMigration, err)
	}

	if len(res) > 0 {
		logger.Info("database migrations applied", "count", len(res))
		for _, r := range res {
			if r.Source != nil {
				logger.Debug("applied migration", "path", r.Source.Path, "version", r.Source.Version)
			}
		}
	} else {
		logger.Debug("database migrations up to date")
	}

	return nil
}

// Version returns the current applied migration version from the database.
func (d *DB) Version(ctx context.Context, fsys fs.FS) (int64, error) {
	if fsys == nil {
		sub, err := fs.Sub(defaultMigrationsFS, "migrations")
		if err != nil {
			return 0, fmt.Errorf("%w: reading embedded migrations: %v", ErrMigration, err)
		}
		fsys = sub
	}

	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		d.sqlDB,
		fsys,
		goose.WithSlog(logger),
	)
	if err != nil {
		if errors.Is(err, goose.ErrNoMigrations) {
			return 0, nil
		}
		return 0, fmt.Errorf("%w: %v", ErrMigration, err)
	}

	return provider.GetDBVersion(ctx)
}
