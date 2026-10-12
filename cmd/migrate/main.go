package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"AccountingForExpenses/internal/infrastructure/logging"
	"AccountingForExpenses/internal/infrastructure/postgres"
)

// Запускает миграции отдельным процессом и возвращает ненулевой код при ошибке.
func main() {
	slog.SetDefault(logging.New(os.Stdout, "expenses-migrate", slog.LevelInfo))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		slog.Error("Не удалось применить миграции", "error", err)
		os.Exit(1)
	}
	slog.Info("Миграции применены")
}

// run читает DATABASE_URL и применяет миграции с ограничением времени выполнения.
func run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	pool, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	return postgres.Migrate(ctx, pool)
}
