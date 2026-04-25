package migrations

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"twitch-smm-bot/internal/logger"
)

const migrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

type Migration struct {
	Version string
	Name    string
	SQL     string
}

type Migrator struct {
	pool *pgxpool.Pool
}

func NewMigrator(pool *pgxpool.Pool) *Migrator {
	return &Migrator{pool: pool}
}

// EnsureMigrationsTable создает таблицу для отслеживания миграций
func (m *Migrator) EnsureMigrationsTable(ctx context.Context) error {
	_, err := m.pool.Exec(ctx, migrationsTable)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}
	return nil
}

// GetAppliedMigrations возвращает список примененных миграций
func (m *Migrator) GetAppliedMigrations(ctx context.Context) (map[string]bool, error) {
	rows, err := m.pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("query migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// ApplyMigration применяет одну миграцию
func (m *Migrator) ApplyMigration(ctx context.Context, migration Migration) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Выполняем SQL миграции
	if _, err := tx.Exec(ctx, migration.SQL); err != nil {
		return fmt.Errorf("execute migration %s: %w", migration.Version, err)
	}

	// Записываем версию в таблицу миграций
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", migration.Version); err != nil {
		return fmt.Errorf("record migration %s: %w", migration.Version, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", migration.Version, err)
	}

	logger.Info("Applied migration",
		zap.String("version", migration.Version),
		zap.String("name", migration.Name),
	)

	return nil
}

// LoadMigrationsFromFS загружает миграции из файловой системы
func LoadMigrationsFromFS(fsys fs.FS, dir string) ([]Migration, error) {
	var migrations []Migration

	err := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		content, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", path, err)
		}

		filename := filepath.Base(path)
		parts := strings.SplitN(filename, "_", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid migration filename format: %s (expected: VERSION_name.sql)", filename)
		}

		version := parts[0]
		name := strings.TrimSuffix(parts[1], ".sql")

		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			SQL:     string(content),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Сортируем миграции по версии
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// Migrate применяет все непримененные миграции
func (m *Migrator) Migrate(ctx context.Context, migrations []Migration) error {
	if err := m.EnsureMigrationsTable(ctx); err != nil {
		return err
	}

	applied, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	var pending []Migration
	for _, migration := range migrations {
		if !applied[migration.Version] {
			pending = append(pending, migration)
		}
	}

	if len(pending) == 0 {
		logger.Info("No pending migrations")
		return nil
	}

	logger.Info("Applying migrations",
		zap.Int("count", len(pending)),
	)

	for _, migration := range pending {
		if err := m.ApplyMigration(ctx, migration); err != nil {
			return err
		}
	}

	logger.Info("All migrations applied successfully")
	return nil
}
