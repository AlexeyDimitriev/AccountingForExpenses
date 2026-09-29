package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"AccountingForExpenses/internal/infrastructure/config"
)

// TestRunInvalidDatabase проверяет отказ запуска до открытия HTTP-порта при неверной конфигурации БД.
func TestRunInvalidDatabase(t *testing.T) {
	err := run(context.Background(), config.Config{DatabaseURL: "postgres://%zz", ConnectTimeout: time.Second}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("expected database configuration error")
	}
}
