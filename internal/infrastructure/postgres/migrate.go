package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"AccountingForExpenses/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate применяет встроенные SQL-миграции одной транзакцией, пропуская уже выполненные.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	return applyMigrations(ctx, pool, migrations.Files)
}

// applyMigrations блокирует параллельный запуск и проверяет неизменность выполненных SQL-файлов.
func applyMigrations(ctx context.Context, pool *pgxpool.Pool, files fs.FS) error {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return err
	}
	transaction, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(transaction)

	// Блокировка действует до завершения транзакции, в том числе при аварийном выходе.
	if _, err := transaction.Exec(ctx, `SELECT pg_advisory_xact_lock(714209381)`); err != nil {
		return err
	}
	if _, err := transaction.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
  name TEXT PRIMARY KEY,
  checksum TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
 )`); err != nil {
		return err
	}

	for _, name := range names {
		script, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(script))
		var savedChecksum string
		err = transaction.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE name = $1`, name).Scan(&savedChecksum)
		if err == nil {
			if savedChecksum != checksum {
				return fmt.Errorf("применённая миграция %s изменена", name)
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := transaction.Exec(ctx, string(script)); err != nil {
			return fmt.Errorf("миграция %s: %w", name, err)
		}
		if _, err := transaction.Exec(ctx, `INSERT INTO schema_migrations (name, checksum) VALUES ($1, $2)`, name, checksum); err != nil {
			return err
		}
	}
	return transaction.Commit(ctx)
}

// rollback освобождает транзакцию даже после отмены исходного контекста.
func rollback(transaction pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = transaction.Rollback(ctx)
}
